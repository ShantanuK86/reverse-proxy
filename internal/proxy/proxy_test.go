package proxy

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shantanuk/reverse-proxy/internal/backend"
	"github.com/shantanuk/reverse-proxy/internal/balancer"
)

func TestProxy_RoundRobin(t *testing.T) {
	// Start 3 test backend servers.
	backends := make([]*httptest.Server, 3)
	urls := make([]string, 3)
	for i := 0; i < 3; i++ {
		name := []string{"backend-1", "backend-2", "backend-3"}[i]
		backends[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]string{"backend": name})
		}))
		defer backends[i].Close()
		urls[i] = backends[i].URL
	}

	pool, err := backend.NewPool(urls)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}

	bal := balancer.NewRoundRobin()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := New(pool, bal, logger)

	// Send 6 requests, expect round-robin distribution.
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i, rec.Code)
		}

		// Check X-Proxy-Backend header is set.
		if got := rec.Header().Get("X-Proxy-Backend"); got == "" {
			t.Errorf("request %d: missing X-Proxy-Backend header", i)
		}
	}
}

func TestProxy_NoBackends(t *testing.T) {
	pool, _ := backend.NewPool([]string{"http://localhost:1"})
	pool.MarkDead(pool.All()[0])

	bal := balancer.NewRoundRobin()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := New(pool, bal, logger)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

func TestProxy_ForwardingHeaders(t *testing.T) {
	var gotHeaders http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	pool, _ := backend.NewPool([]string{ts.URL})
	bal := balancer.NewRoundRobin()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := New(pool, bal, logger)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if got := gotHeaders.Get("X-Forwarded-Proto"); got != "http" {
		t.Errorf("X-Forwarded-Proto = %q, want %q", got, "http")
	}
}
