package game

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// MockBackend represents an in-memory controlled upstream server.
type MockBackend struct {
	ID           string
	Name         string
	Addr         string
	URL          string
	server       *http.Server
	listener     net.Listener
	isHealthy    atomic.Bool
	latencyMs    atomic.Int64
	requestCount atomic.Int64
	errorCount   atomic.Int64
	lastHeaders  sync.Map
}

// MockManager manages the 3 internal demo backends.
type MockManager struct {
	backends map[string]*MockBackend
	mu       sync.RWMutex
}

// NewMockManager spins up 3 controlled in-process mock backends: Alpha, Beta, Gamma.
func NewMockManager() (*MockManager, error) {
	mgr := &MockManager{
		backends: make(map[string]*MockBackend),
	}

	configs := []struct {
		id      string
		name    string
		latency int64
	}{
		{"alpha", "Backend Alpha (Fast)", 15},
		{"beta", "Backend Beta (Standard)", 25},
		{"gamma", "Backend Gamma (Heavy)", 45},
	}

	for _, cfg := range configs {
		mb, err := startMockBackend(cfg.id, cfg.name, cfg.latency)
		if err != nil {
			mgr.Close()
			return nil, fmt.Errorf("starting mock backend %s: %w", cfg.id, err)
		}
		mgr.backends[cfg.id] = mb
	}

	return mgr, nil
}

func startMockBackend(id, name string, baseLatency int64) (*MockBackend, error) {
	// Listen on localhost with random free port to prevent port collisions
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	mb := &MockBackend{
		ID:       id,
		Name:     name,
		Addr:     ln.Addr().String(),
		URL:      fmt.Sprintf("http://%s", ln.Addr().String()),
		listener: ln,
	}
	mb.isHealthy.Store(true)
	mb.latencyMs.Store(baseLatency)

	mux := http.NewServeMux()

	// Health check endpoint probed by health.Checker
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if !mb.isHealthy.Load() {
			http.Error(w, `{"status":"unhealthy","error":"simulated failure"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"backend": mb.Name,
		})
	})

	// Traffic handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Store headers for Header Detective inspection
		headersCopy := make(map[string][]string)
		for k, v := range r.Header {
			headersCopy[k] = v
		}
		mb.lastHeaders.Store("headers", headersCopy)
		mb.lastHeaders.Store("host", r.Host)

		if !mb.isHealthy.Load() {
			mb.errorCount.Add(1)
			http.Error(w, `{"error":"backend simulated error"}`, http.StatusInternalServerError)
			return
		}

		// Simulate configured latency
		lat := mb.latencyMs.Load()
		if lat > 0 {
			time.Sleep(time.Duration(lat) * time.Millisecond)
		}

		mb.requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"backend":          mb.Name,
			"backend_id":       mb.ID,
			"path":             r.URL.Path,
			"timestamp":        time.Now().UTC().Format(time.RFC3339Nano),
			"received_host":    r.Host,
			"received_headers": headersCopy,
			"processed_by":     mb.Addr,
		})
	})

	srv := &http.Server{Handler: mux}
	mb.server = srv

	go func() {
		_ = srv.Serve(ln)
	}()

	return mb, nil
}

// Get returns the mock backend by ID.
func (m *MockManager) Get(id string) *MockBackend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.backends[id]
}

// All returns all mock backends.
func (m *MockManager) All() []*MockBackend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*MockBackend, 0, len(m.backends))
	for _, b := range m.backends {
		res = append(res, b)
	}
	return res
}

// URLs returns list of URLs for all mock backends.
func (m *MockManager) URLs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]string, 0, len(m.backends))
	for _, b := range m.backends {
		res = append(res, b.URL)
	}
	return res
}

// SetHealthy toggles a backend's health status.
func (m *MockManager) SetHealthy(id string, healthy bool) bool {
	b := m.Get(id)
	if b == nil {
		return false
	}
	b.isHealthy.Store(healthy)
	return true
}

// SetLatency sets simulated latency for a backend.
func (m *MockManager) SetLatency(id string, latencyMs int64) bool {
	b := m.Get(id)
	if b == nil {
		return false
	}
	b.latencyMs.Store(latencyMs)
	return true
}

// Reset restores all mock backends to healthy defaults.
func (m *MockManager) Reset() {
	defaults := map[string]int64{
		"alpha": 15,
		"beta":  25,
		"gamma": 45,
	}
	for id, b := range m.backends {
		b.isHealthy.Store(true)
		b.latencyMs.Store(defaults[id])
	}
}

// Close shuts down all in-process mock servers.
func (m *MockManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.backends {
		if b.server != nil {
			_ = b.server.Close()
		}
	}
}

// LastHeaders returns the headers captured by the backend on its most recent request.
func (mb *MockBackend) LastHeaders() (map[string][]string, string) {
	val, ok := mb.lastHeaders.Load("headers")
	if !ok {
		return make(map[string][]string), ""
	}
	hostVal, _ := mb.lastHeaders.Load("host")
	hostStr, _ := hostVal.(string)
	return val.(map[string][]string), hostStr
}

// Snapshot returns a JSON-friendly snapshot of backend state.
func (mb *MockBackend) Snapshot() map[string]any {
	return map[string]any{
		"id":            mb.ID,
		"name":          mb.Name,
		"addr":          mb.Addr,
		"url":           mb.URL,
		"is_healthy":    mb.isHealthy.Load(),
		"latency_ms":    mb.latencyMs.Load(),
		"request_count": mb.requestCount.Load(),
		"error_count":   mb.errorCount.Load(),
	}
}
