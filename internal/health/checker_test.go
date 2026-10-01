package health

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shantanuk/reverse-proxy/internal/backend"
)

func TestChecker_EjectsUnhealthy(t *testing.T) {
	// Backend that always returns 500.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	pool, _ := backend.NewPool([]string{ts.URL})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var ejected atomic.Bool
	checker := NewChecker(pool, Config{
		Path:               "/healthz",
		Interval:           50 * time.Millisecond,
		Timeout:            1 * time.Second,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
	}, logger)
	checker.OnStatusChange = func(b *backend.Backend, alive bool) {
		if !alive {
			ejected.Store(true)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go checker.Run(ctx)
	<-ctx.Done()

	if !ejected.Load() {
		t.Error("expected backend to be ejected")
	}
	if pool.All()[0].IsAlive() {
		t.Error("backend should be dead")
	}
}

func TestChecker_RecoversHealthy(t *testing.T) {
	var shouldFail atomic.Bool
	shouldFail.Store(true)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldFail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	pool, _ := backend.NewPool([]string{ts.URL})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var recovered atomic.Bool
	checker := NewChecker(pool, Config{
		Path:               "/healthz",
		Interval:           50 * time.Millisecond,
		Timeout:            1 * time.Second,
		UnhealthyThreshold: 2,
		HealthyThreshold:   2,
	}, logger)
	checker.OnStatusChange = func(b *backend.Backend, alive bool) {
		if alive {
			recovered.Store(true)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go checker.Run(ctx)

	// Let the backend fail for a bit, then recover.
	time.Sleep(200 * time.Millisecond)
	shouldFail.Store(false)

	<-ctx.Done()

	if !recovered.Load() {
		t.Error("expected backend to recover")
	}
	if !pool.All()[0].IsAlive() {
		t.Error("backend should be alive after recovery")
	}
}
