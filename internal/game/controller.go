package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shantanuk/reverse-proxy/internal/backend"
	"github.com/shantanuk/reverse-proxy/internal/balancer"
	"github.com/shantanuk/reverse-proxy/internal/health"
	"github.com/shantanuk/reverse-proxy/internal/proxy"
)

// Controller manages the game API and playground interactions.
type Controller struct {
	pool          *backend.Pool
	proxy         *proxy.Proxy
	checker       *health.Checker
	mockMgr       *MockManager
	totalRequests atomic.Int64
	traces        []RequestTrace
	tracesMu      sync.RWMutex
}

// NewController creates a new Game Controller.
func NewController(
	pool *backend.Pool,
	proxyHandler *proxy.Proxy,
	checker *health.Checker,
	mockMgr *MockManager,
) *Controller {
	return &Controller{
		pool:    pool,
		proxy:   proxyHandler,
		checker: checker,
		mockMgr: mockMgr,
		traces:  make([]RequestTrace, 0, 50),
	}
}

// RegisterRoutes mounts all game control plane endpoints onto the given ServeMux.
func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/game/api/status", c.handleStatus)
	mux.HandleFunc("/game/api/strategy", c.handleStrategy)
	mux.HandleFunc("/game/api/chaos", c.handleChaos)
	mux.HandleFunc("/game/api/send", c.handleSend)
	mux.HandleFunc("/game/api/burst", c.handleBurst)
	mux.HandleFunc("/game/api/route-rule", c.handleRouteRule)
	mux.HandleFunc("/game/api/quiz/check", c.handleQuizCheck)
	mux.HandleFunc("/game/api/reset", c.handleReset)
}

// handleStatus returns complete live telemetry of backends, strategy, and stats.
func (c *Controller) handleStatus(w http.ResponseWriter, r *http.Request) {
	backendList := make([]map[string]any, 0)
	for _, b := range c.pool.All() {
		item := map[string]any{
			"name":         b.Name,
			"url":          b.URL.String(),
			"host":         b.URL.Host,
			"alive":        b.IsAlive(),
			"active_conns": b.GetActiveConns(),
		}

		if c.mockMgr != nil {
			if mb := c.mockMgr.GetByAddr(b.URL.Host); mb != nil {
				item["mock_id"] = mb.ID
				item["name"] = mb.Name
				item["is_healthy"] = mb.isHealthy.Load()
				item["latency_ms"] = mb.latencyMs.Load()
				item["request_count"] = mb.requestCount.Load()
				item["error_count"] = mb.errorCount.Load()
			}
		}
		backendList = append(backendList, item)
	}

	// Guarantee deterministic sort: Alpha, Beta, Gamma
	orderMap := map[string]int{"alpha": 1, "beta": 2, "gamma": 3}
	sort.Slice(backendList, func(i, j int) bool {
		idI, _ := backendList[i]["mock_id"].(string)
		idJ, _ := backendList[j]["mock_id"].(string)
		return orderMap[idI] < orderMap[idJ]
	})

	balStrategy := "round-robin"
	if b := c.proxy.GetBalancer(); b != nil {
		balStrategy = b.Name()
	}

	c.tracesMu.RLock()
	recentTraces := make([]RequestTrace, len(c.traces))
	copy(recentTraces, c.traces)
	c.tracesMu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"strategy":       balStrategy,
		"backends":       backendList,
		"total_requests": c.totalRequests.Load(),
		"recent_traces":  recentTraces,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	})
}

// handleStrategy toggles the balancer between round-robin and least-connections.
func (c *Controller) handleStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Strategy string `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	bal, err := balancer.New(req.Strategy)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	c.proxy.SetBalancer(bal)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "updated",
		"strategy": bal.Name(),
	})
}

// handleChaos triggers node failure, revival, or latency injection.
func (c *Controller) handleChaos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BackendID string `json:"backend_id"`
		Action    string `json:"action"` // "kill", "revive", "toggle", "latency"
		LatencyMs int64  `json:"latency_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if c.mockMgr == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mock manager not available"})
		return
	}

	mb := c.mockMgr.Get(req.BackendID)
	if mb == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "backend not found: " + req.BackendID})
		return
	}

	switch req.Action {
	case "kill":
		mb.isHealthy.Store(false)
	case "revive":
		mb.isHealthy.Store(true)
	case "toggle":
		mb.isHealthy.Store(!mb.isHealthy.Load())
	case "latency":
		if req.LatencyMs >= 0 {
			mb.latencyMs.Store(req.LatencyMs)
		}
	}

	// Trigger health check immediately so pool state reflects change right away
	if c.checker != nil {
		c.checker.CheckAll()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"backend_id": mb.ID,
		"is_healthy": mb.isHealthy.Load(),
		"latency_ms": mb.latencyMs.Load(),
	})
}

// handleSend executes a single request through the proxy and captures a complete trace.
func (c *Controller) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Method = "GET"
		req.Path = "/api/users"
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Path == "" {
		req.Path = "/api/users"
	}

	start := time.Now()
	traceID := fmt.Sprintf("req-%d", rand.Intn(900000)+100000)

	timeline := []TraceTimelineStep{
		{
			Step:     "ingress",
			TimeMs:   0.0,
			Title:    "Client Ingress",
			Desc:     fmt.Sprintf("Client request received: %s %s from 127.0.0.1", req.Method, req.Path),
			Category: "ingress",
		},
	}

	// Prepare synthetic request for proxy
	proxyReq := httptest.NewRequest(req.Method, req.Path, nil)
	proxyReq.RemoteAddr = "127.0.0.1:54321"

	// Add test headers
	proxyReq.Header.Set("User-Agent", "ProxyPanicClient/1.0")
	proxyReq.Header.Set("Accept", "application/json")
	proxyReq.Header.Set("Connection", "keep-alive")
	for k, v := range req.Headers {
		proxyReq.Header.Set(k, v)
	}

	headersSent := make(map[string][]string)
	for k, v := range proxyReq.Header {
		headersSent[k] = v
	}

	// Record recorder for proxy
	rec := httptest.NewRecorder()

	timeline = append(timeline, TraceTimelineStep{
		Step:     "middleware",
		TimeMs:   float64(time.Since(start).Microseconds()) / 1000.0,
		Title:    "Middleware Pipeline",
		Desc:     "Panic recovery initialized; Prometheus request duration timer started.",
		Category: "ingress",
	})

	// Execute through proxy
	c.proxy.ServeHTTP(rec, proxyReq)
	duration := time.Since(start)
	c.totalRequests.Add(1)

	// Determine selected backend from response header
	selectedBackendAddr := rec.Header().Get("X-Proxy-Backend")
	selectedBackendName := selectedBackendAddr
	selectedMockID := ""
	if c.mockMgr != nil {
		if mb := c.mockMgr.GetByAddr(selectedBackendAddr); mb != nil {
			selectedBackendName = mb.Name
			selectedMockID = mb.ID
		}
	}
	if selectedBackendAddr == "" {
		selectedBackendAddr = "none (ejected / unavailable)"
		selectedBackendName = "none (ejected / unavailable)"
	}

	balName := "round-robin"
	if b := c.proxy.GetBalancer(); b != nil {
		balName = b.Name()
	}

	timeline = append(timeline, TraceTimelineStep{
		Step:     "balancer",
		TimeMs:   0.6,
		Title:    "Load Balancer Decision",
		Desc:     fmt.Sprintf("Strategy '%s' routed traffic to upstream [%s].", balName, selectedBackendName),
		Category: "balancer",
	})

	timeline = append(timeline, TraceTimelineStep{
		Step:     "mutation",
		TimeMs:   1.0,
		Title:    "Header Transformation (RFC 7230)",
		Desc:     "Removed hop-by-hop headers [Connection]. Added X-Forwarded-For: 127.0.0.1, X-Forwarded-Proto: http.",
		Category: "mutation",
	})

	timeline = append(timeline, TraceTimelineStep{
		Step:     "upstream_response",
		TimeMs:   float64(duration.Microseconds())/1000.0 - 0.3,
		Title:    "Upstream RoundTrip",
		Desc:     fmt.Sprintf("Upstream [%s] returned HTTP %d in %.2fms.", selectedBackendName, rec.Code, float64(duration.Milliseconds())),
		Category: "upstream",
	})

	timeline = append(timeline, TraceTimelineStep{
		Step:     "egress",
		TimeMs:   float64(duration.Microseconds()) / 1000.0,
		Title:    "Client Delivery",
		Desc:     "Injected X-Proxy-Backend header, updated Prometheus metrics, streamed body to client.",
		Category: "egress",
	})

	var parsedBody any
	_ = json.Unmarshal(rec.Body.Bytes(), &parsedBody)

	// Fetch headers received by backend if mock is available
	headersReceived := make(map[string][]string)
	if c.mockMgr != nil {
		if mb := c.mockMgr.GetByAddr(selectedBackendAddr); mb != nil {
			h, _ := mb.LastHeaders()
			headersReceived = h
		}
	}

	trace := RequestTrace{
		ID:               traceID,
		Method:           req.Method,
		Path:             req.Path,
		ClientIP:         "127.0.0.1",
		SelectedBackend:  selectedBackendName,
		SelectedMockID:   selectedMockID,
		SelectedURL:      selectedBackendAddr,
		BalancerStrategy: balName,
		StatusCode:       rec.Code,
		TotalDurationMs:  float64(duration.Microseconds()) / 1000.0,
		Timeline:         timeline,
		HeadersSent:      headersSent,
		HeadersReceived:  headersReceived,
		HeadersAdded: map[string]string{
			"X-Forwarded-For":   "127.0.0.1",
			"X-Forwarded-Proto": "http",
			"X-Proxy-Backend":   selectedBackendAddr,
		},
		HeadersStripped: []string{"Connection", "Keep-Alive"},
		ResponseBody:    parsedBody,
	}

	c.tracesMu.Lock()
	if len(c.traces) >= 20 {
		c.traces = c.traces[1:]
	}
	c.traces = append(c.traces, trace)
	c.tracesMu.Unlock()

	writeJSON(w, http.StatusOK, trace)
}

// handleBurst fires concurrent requests and reports distribution and latency metrics.
func (c *Controller) handleBurst(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Count int    `json:"count"`
		Path  string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Count <= 0 {
		req.Count = 20
	}
	if req.Count > 100 {
		req.Count = 100 // Cap burst for safety
	}
	if req.Path == "" {
		req.Path = "/api/burst"
	}

	start := time.Now()
	distribution := make(map[string]int)
	var distMu sync.Mutex
	latencies := make([]float64, 0, req.Count)
	var latMu sync.Mutex
	var errCount atomic.Int64
	var targetsMu sync.Mutex
	targets := make([]string, 0, req.Count)

	var wg sync.WaitGroup
	for i := 0; i < req.Count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqStart := time.Now()
			proxyReq := httptest.NewRequest("GET", req.Path, nil)
			proxyReq.RemoteAddr = "127.0.0.1:45123"
			rec := httptest.NewRecorder()

			c.proxy.ServeHTTP(rec, proxyReq)
			reqDur := float64(time.Since(reqStart).Microseconds()) / 1000.0

			latMu.Lock()
			latencies = append(latencies, reqDur)
			latMu.Unlock()

			c.totalRequests.Add(1)

			if rec.Code >= 400 {
				errCount.Add(1)
			}

			backendAddr := rec.Header().Get("X-Proxy-Backend")
			backendLabel := backendAddr
			targetMockID := ""
			if c.mockMgr != nil {
				if mb := c.mockMgr.GetByAddr(backendAddr); mb != nil {
					backendLabel = mb.Name
					targetMockID = mb.ID
				}
			}
			if backendAddr == "" {
				backendLabel = "502 Bad Gateway"
			}

			distMu.Lock()
			distribution[backendLabel]++
			distMu.Unlock()

			if targetMockID != "" {
				targetsMu.Lock()
				targets = append(targets, targetMockID)
				targetsMu.Unlock()
			}
		}()
	}
	wg.Wait()
	totalTime := time.Since(start)

	sort.Float64s(latencies)
	p50 := 0.0
	p95 := 0.0
	p99 := 0.0
	if len(latencies) > 0 {
		p50 = latencies[len(latencies)*50/100]
		p95 = latencies[len(latencies)*95/100]
		p99 = latencies[len(latencies)*99/100]
	}

	balStrategy := "round-robin"
	if b := c.proxy.GetBalancer(); b != nil {
		balStrategy = b.Name()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_sent":       req.Count,
		"strategy":         balStrategy,
		"distribution":     distribution,
		"targets":          targets,
		"total_duration":   fmt.Sprintf("%.2fms", float64(totalTime.Milliseconds())),
		"p50_latency_ms":   p50,
		"p95_latency_ms":   p95,
		"p99_latency_ms":   p99,
		"error_count":      errCount.Load(),
		"success_rate_pct": float64(req.Count-int(errCount.Load())) / float64(req.Count) * 100.0,
	})
}

// handleRouteRule sets or clears path routing rules (for Mission 5).
func (c *Controller) handleRouteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action      string `json:"action"` // "add", "clear"
		Prefix      string `json:"prefix"`
		BackendName string `json:"backend_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Action == "clear" {
		c.proxy.ClearRoutes()
		writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
		return
	}

	target := c.pool.GetByName(req.BackendName)
	if target == nil {
		for _, b := range c.pool.All() {
			if strings.Contains(strings.ToLower(b.Name), strings.ToLower(req.BackendName)) ||
				strings.Contains(b.URL.String(), req.BackendName) {
				target = b
				break
			}
		}
	}

	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "backend not found: " + req.BackendName})
		return
	}

	c.proxy.AddRoute(req.Prefix, target)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "added",
		"prefix":  req.Prefix,
		"backend": target.Name,
	})
}

// handleQuizCheck verifies answers for Header Detective and Route Master challenges.
func (c *Controller) handleQuizCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		QuizID  string   `json:"quiz_id"`
		Answers []string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var correct bool
	var xpAward int
	var explanation string

	switch req.QuizID {
	case "m4_stripped_headers":
		// Q: Which headers must a compliant reverse proxy strip according to RFC 7230?
		// Expected: Connection, Keep-Alive, Upgrade
		strippedSet := map[string]bool{"connection": true, "keep-alive": true, "upgrade": true}
		userMatches := 0
		for _, a := range req.Answers {
			if strippedSet[strings.ToLower(strings.TrimSpace(a))] {
				userMatches++
			}
		}
		if userMatches >= 2 {
			correct = true
			xpAward = 250
			explanation = "Correct! RFC 7230 mandates that hop-by-hop headers (Connection, Keep-Alive, Upgrade, Transfer-Encoding) apply only to a single point-to-point connection and MUST NOT be forwarded upstream."
		} else {
			explanation = "Incorrect. Hop-by-hop headers like Connection and Keep-Alive are stripped to avoid socket protocol confusion."
		}

	case "m4_forwarded_headers":
		// Q: Which header communicates the client's real IP to the backend?
		for _, a := range req.Answers {
			if strings.EqualFold(a, "x-forwarded-for") {
				correct = true
				xpAward = 150
				explanation = "Correct! X-Forwarded-For contains the IP address of the original client so upstreams can log or rate-limit real visitors."
				break
			}
		}

	case "m5_route_precedence":
		// Q: If route rule /api/auth/* points to Alpha, where does /api/auth/login go?
		for _, a := range req.Answers {
			if strings.Contains(strings.ToLower(a), "alpha") {
				correct = true
				xpAward = 250
				explanation = "Correct! Path-based routing rules evaluate prefix precedence before delegating to default load balancing."
				break
			}
		}

	default:
		correct = false
		explanation = "Unknown quiz challenge."
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"correct":     correct,
		"xp_awarded":  xpAward,
		"explanation": explanation,
	})
}

// handleReset resets mock backends and routes.
func (c *Controller) handleReset(w http.ResponseWriter, r *http.Request) {
	if c.mockMgr != nil {
		c.mockMgr.Reset()
	}
	c.proxy.ClearRoutes()
	if c.checker != nil {
		c.checker.CheckAll()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset_completed"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
