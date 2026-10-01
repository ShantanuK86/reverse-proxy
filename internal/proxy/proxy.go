package proxy

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/shantanuk/reverse-proxy/internal/backend"
	"github.com/shantanuk/reverse-proxy/internal/balancer"
)

// Proxy is the core reverse proxy handler.
type Proxy struct {
	pool     *backend.Pool
	balancer balancer.Balancer
	logger   *slog.Logger
	// ErrorCounter is called on backend errors (for metrics).
	ErrorCounter func(backendURL, errorType string)
}

// New creates a new reverse proxy handler.
func New(pool *backend.Pool, bal balancer.Balancer, logger *slog.Logger) *Proxy {
	return &Proxy{
		pool:     pool,
		balancer: bal,
		logger:   logger,
	}
}

// ServeHTTP picks a backend and proxies the request.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, err := p.balancer.Next(p.pool)
	if err != nil {
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
