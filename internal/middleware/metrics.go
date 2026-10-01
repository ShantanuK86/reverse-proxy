package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shantanuk/reverse-proxy/internal/proxy"
)

// Metrics holds the Prometheus metrics for the proxy.
type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	BackendErrors   *prometheus.CounterVec
	ActiveConns     *prometheus.GaugeVec
	BackendHealth   *prometheus.GaugeVec
}

// NewMetrics registers and returns all Prometheus metrics.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		RequestsTotal: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_requests_total",
				Help: "Total number of proxied requests.",
			},
			[]string{"method", "status", "backend"},
		),
		RequestDuration: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "proxy_request_duration_seconds",
				Help:    "Request latency distribution.",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "backend"},
		),
		BackendErrors: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "proxy_backend_errors_total",
				Help: "Total backend errors by type.",
			},
			[]string{"backend", "error_type"},
		),
		ActiveConns: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "proxy_active_connections",
				Help: "Current in-flight requests per backend.",
			},
			[]string{"backend"},
		),
		BackendHealth: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "proxy_backend_health",
				Help: "Backend health status (1=healthy, 0=unhealthy).",
			},
			[]string{"backend"},
		),
	}
	return m
}

// InstrumentHandler wraps the given handler with Prometheus metrics collection.
func (m *Metrics) InstrumentHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		backendLabel := "unknown"

		if b := proxy.BackendFromContext(r.Context()); b != nil {
			backendLabel = b.URL.Host
			m.ActiveConns.WithLabelValues(backendLabel).Set(float64(b.GetActiveConns()))
		}

		statusStr := strconv.Itoa(sw.status)
		m.RequestsTotal.WithLabelValues(r.Method, statusStr, backendLabel).Inc()
		m.RequestDuration.WithLabelValues(r.Method, backendLabel).Observe(duration)
	})
}

// RecordBackendError records a backend error for the given backend URL and error type.
func (m *Metrics) RecordBackendError(backendURL, errorType string) {
	m.BackendErrors.WithLabelValues(backendURL, errorType).Inc()
}

// SetBackendHealth sets the health gauge for a backend.
func (m *Metrics) SetBackendHealth(backendURL string, alive bool) {
	val := 0.0
	if alive {
		val = 1.0
	}
	m.BackendHealth.WithLabelValues(backendURL).Set(val)
}
