package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/shantanuk/reverse-proxy/internal/proxy"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
	written bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.status = code
		sw.written = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.written {
		sw.status = http.StatusOK
		sw.written = true
	}
	return sw.ResponseWriter.Write(b)
}

// Logging returns a middleware that logs each request.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration", duration,
				"remote_addr", r.RemoteAddr,
			}

			if b := proxy.BackendFromContext(r.Context()); b != nil {
				attrs = append(attrs, "backend", b.URL.Host)
			}

			logger.Info("request", attrs...)
		})
	}
}
