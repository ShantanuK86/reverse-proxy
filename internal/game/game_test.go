package game

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shantanuk/reverse-proxy/internal/backend"
	"github.com/shantanuk/reverse-proxy/internal/balancer"
	"github.com/shantanuk/reverse-proxy/internal/health"
	"github.com/shantanuk/reverse-proxy/internal/proxy"
)

func setupTestGame(t *testing.T) (*Controller, *MockManager) {
	t.Helper()
	mockMgr, err := NewMockManager()
	if err != nil {
		t.Fatalf("NewMockManager error: %v", err)
	}

	pool, err := backend.NewPool(mockMgr.URLs())
	if err != nil {
		t.Fatalf("NewPool error: %v", err)
	}

	bal := balancer.NewRoundRobin()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxyHandler := proxy.New(pool, bal, logger)

	checker := health.NewChecker(pool, health.Config{
		Path:               "/healthz",
		Interval:           time.Second,
		Timeout:            500 * time.Millisecond,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
	}, logger)

	controller := NewController(pool, proxyHandler, checker, mockMgr)
	return controller, mockMgr
}

func TestMockManager_HealthyAndLatency(t *testing.T) {
	mgr, err := NewMockManager()
	if err != nil {
		t.Fatalf("failed to create mock manager: %v", err)
	}
	defer mgr.Close()

	if len(mgr.All()) != 3 {
		t.Fatalf("expected 3 mock backends, got %d", len(mgr.All()))
	}

	alpha := mgr.Get("alpha")
	if alpha == nil {
		t.Fatal("alpha backend not found")
	}

	if !alpha.isHealthy.Load() {
		t.Error("alpha should be healthy by default")
	}

	mgr.SetHealthy("alpha", false)
	if alpha.isHealthy.Load() {
		t.Error("alpha should be unhealthy after SetHealthy(false)")
	}

	mgr.SetLatency("alpha", 120)
	if alpha.latencyMs.Load() != 120 {
		t.Errorf("latency = %d, want 120", alpha.latencyMs.Load())
	}
}

func TestController_StatusAndStrategy(t *testing.T) {
	controller, mockMgr := setupTestGame(t)
	defer mockMgr.Close()

	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	// Test GET /game/api/status
	req := httptest.NewRequest("GET", "/game/api/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}

	var statusResp struct {
		Strategy string           `json:"strategy"`
		Backends []map[string]any `json:"backends"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decoding status response: %v", err)
	}
	if statusResp.Strategy != "round-robin" {
		t.Errorf("strategy = %q, want round-robin", statusResp.Strategy)
	}
	if len(statusResp.Backends) != 3 {
		t.Errorf("backends count = %d, want 3", len(statusResp.Backends))
	}

	// Test POST /game/api/strategy to switch to least-connections
	body, _ := json.Marshal(map[string]string{"strategy": "least-connections"})
	req = httptest.NewRequest("POST", "/game/api/strategy", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("strategy update status = %d, want 200", rec.Code)
	}
}

func TestController_SendAndTrace(t *testing.T) {
	controller, mockMgr := setupTestGame(t)
	defer mockMgr.Close()

	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]any{
		"method": "GET",
		"path":   "/api/users",
	})
	req := httptest.NewRequest("POST", "/game/api/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("send request code = %d, want 200", rec.Code)
	}

	var trace RequestTrace
	if err := json.Unmarshal(rec.Body.Bytes(), &trace); err != nil {
		t.Fatalf("unmarshaling trace: %v", err)
	}

	if trace.StatusCode != http.StatusOK {
		t.Errorf("trace status = %d, want 200", trace.StatusCode)
	}
	if len(trace.Timeline) == 0 {
		t.Error("expected non-empty trace timeline")
	}
	if len(trace.HeadersStripped) == 0 {
		t.Error("expected stripped hop-by-hop headers listed in trace")
	}
}

func TestController_BurstTraffic(t *testing.T) {
	controller, mockMgr := setupTestGame(t)
	defer mockMgr.Close()

	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]any{"count": 10, "path": "/api/burst"})
	req := httptest.NewRequest("POST", "/game/api/burst", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("burst request code = %d, want 200", rec.Code)
	}

	var burstResp struct {
		TotalSent      int            `json:"total_sent"`
		Distribution   map[string]int `json:"distribution"`
		SuccessRatePct float64        `json:"success_rate_pct"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &burstResp); err != nil {
		t.Fatalf("unmarshaling burst response: %v", err)
	}

	if burstResp.TotalSent != 10 {
		t.Errorf("total_sent = %d, want 10", burstResp.TotalSent)
	}
	if burstResp.SuccessRatePct != 100.0 {
		t.Errorf("success_rate = %f, want 100%%", burstResp.SuccessRatePct)
	}
}

func TestController_QuizCheck(t *testing.T) {
	controller, mockMgr := setupTestGame(t)
	defer mockMgr.Close()

	mux := http.NewServeMux()
	controller.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]any{
		"quiz_id": "m4_stripped_headers",
		"answers": []string{"Connection", "Keep-Alive"},
	})
	req := httptest.NewRequest("POST", "/game/api/quiz/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("quiz check code = %d, want 200", rec.Code)
	}

	var quizResp struct {
		Correct   bool `json:"correct"`
		XpAwarded int  `json:"xp_awarded"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &quizResp); err != nil {
		t.Fatalf("decoding quiz response: %v", err)
	}

	if !quizResp.Correct {
		t.Error("expected quiz answer to be marked correct")
	}
	if quizResp.XpAwarded != 250 {
		t.Errorf("xp_awarded = %d, want 250", quizResp.XpAwarded)
	}
}

func TestStaticHandler(t *testing.T) {
	handler := StaticHandler()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("root status = %d, want 200", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("PROXY PANIC")) {
		t.Error("root missing PROXY PANIC title")
	}

	// Also verify static css file
	req = httptest.NewRequest("GET", "/style.css", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("style.css status = %d, want 200", rec.Code)
	}
}
