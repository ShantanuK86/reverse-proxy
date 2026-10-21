package proxy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/shantanuk/reverse-proxy/internal/backend"
	"github.com/shantanuk/reverse-proxy/internal/balancer"
)

// RouteRule maps a URL path prefix to a designated backend.
type RouteRule struct {
	Prefix  string
	Backend *backend.Backend
}

// Proxy is the core reverse proxy handler.
type Proxy struct {
	pool         *backend.Pool
	balancer     balancer.Balancer
	logger       *slog.Logger
	routes       []RouteRule
	mu           sync.RWMutex
	ErrorCounter func(backendURL, errorType string)
}

// New creates a new reverse proxy handler.
func New(pool *backend.Pool, bal balancer.Balancer, logger *slog.Logger) *Proxy {
	return &Proxy{
		pool:     pool,
		balancer: bal,
		logger:   logger,
		routes:   make([]RouteRule, 0),
	}
}

// SetBalancer dynamically switches the load balancing strategy.
func (p *Proxy) SetBalancer(bal balancer.Balancer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.balancer = bal
}

// GetBalancer returns the active load balancing strategy.
func (p *Proxy) GetBalancer() balancer.Balancer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.balancer
}

// AddRoute registers a path prefix route rule (for path-based routing).
func (p *Proxy) AddRoute(prefix string, b *backend.Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.routes = append(p.routes, RouteRule{Prefix: prefix, Backend: b})
}

// ClearRoutes removes all configured custom route rules.
func (p *Proxy) ClearRoutes() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.routes = make([]RouteRule, 0)
}

// MatchRoute checks if a request path matches any registered prefix route.
func (p *Proxy) MatchRoute(path string) *backend.Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, r := range p.routes {
		if strings.HasPrefix(path, r.Prefix) && r.Backend.IsAlive() {
			return r.Backend
		}
	}
	return nil
}

// ServeHTTP picks a backend and proxies the request.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var b *backend.Backend
	var err error

	// Check if this path matches a route rule first.
	if matched := p.MatchRoute(r.URL.Path); matched != nil {
		b = matched
	} else {
		bal := p.GetBalancer()
		b, err = bal.Next(p.pool)
	}

	if err != nil || b == nil {
		p.logger.Error("no backends available", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "no healthy backends available",
		})
		return
	}

	// Track active connections for least-connections balancing.
	b.IncrConns()
	defer b.DecrConns()

	// Store the selected backend in the request context for metrics/logging.
	ctx := WithBackend(r.Context(), b)
	r = r.WithContext(ctx)

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = b.URL.Scheme
			req.URL.Host = b.URL.Host
			req.Host = b.URL.Host

			// Set forwarding headers.
			if clientIP := r.RemoteAddr; clientIP != "" {
				req.Header.Set("X-Forwarded-For", clientIP)
				req.Header.Set("X-Real-IP", clientIP)
			}
			if r.TLS != nil {
				req.Header.Set("X-Forwarded-Proto", "https")
			} else {
				req.Header.Set("X-Forwarded-Proto", "http")
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Set("X-Proxy-Backend", b.URL.Host)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			p.logger.Error("backend error",
				"backend", b.URL.String(),
				"error", err,
			)
			if p.ErrorCounter != nil {
				p.ErrorCounter(b.URL.String(), "connection_error")
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":   "backend unavailable",
				"backend": b.URL.Host,
			})
		},
	}

	rp.ServeHTTP(w, r)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
