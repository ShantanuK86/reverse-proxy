package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/shantanuk/reverse-proxy/internal/backend"
	"github.com/shantanuk/reverse-proxy/internal/balancer"
	"github.com/shantanuk/reverse-proxy/internal/config"
	"github.com/shantanuk/reverse-proxy/internal/health"
	"github.com/shantanuk/reverse-proxy/internal/middleware"
	"github.com/shantanuk/reverse-proxy/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config/proxy.yaml", "path to config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	logger.Info("configuration loaded",
		"backends", len(cfg.Backends),
		"strategy", cfg.Balancer.Strategy,
		"tls_enabled", cfg.TLS.Enabled,
	)

	// Initialize backend pool.
	urls := make([]string, len(cfg.Backends))
	for i, b := range cfg.Backends {
		urls[i] = b.URL
	}
	pool, err := backend.NewPool(urls)
	if err != nil {
		logger.Error("failed to create backend pool", "error", err)
		os.Exit(1)
	}

	// Initialize balancer.
	bal, err := balancer.New(cfg.Balancer.Strategy)
	if err != nil {
		logger.Error("failed to create balancer", "error", err)
		os.Exit(1)
	}

	// Initialize Prometheus metrics.
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	metrics := middleware.NewMetrics(reg)

	// Set initial backend health gauges.
	for _, b := range pool.All() {
		metrics.SetBackendHealth(b.URL.String(), true)
	}

	// Create proxy handler.
	proxyHandler := proxy.New(pool, bal, logger)
	proxyHandler.ErrorCounter = metrics.RecordBackendError

	// Build middleware chain: Recovery → Logging → Metrics → Proxy
	var handler http.Handler = proxyHandler
	handler = metrics.InstrumentHandler(handler)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.Recovery(logger)(handler)

	// Context for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start health checker.
	if cfg.HealthCheck.Enabled {
		checker := health.NewChecker(pool, health.Config{
			Path:               cfg.HealthCheck.Path,
			Interval:           cfg.HealthCheck.Interval,
			Timeout:            cfg.HealthCheck.Timeout,
			UnhealthyThreshold: cfg.HealthCheck.UnhealthyThreshold,
			HealthyThreshold:   cfg.HealthCheck.HealthyThreshold,
		}, logger)
		checker.OnStatusChange = func(b *backend.Backend, alive bool) {
			metrics.SetBackendHealth(b.URL.String(), alive)
		}
		go checker.Run(ctx)
	}

	// Start metrics server on separate port.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	metricsServer := &http.Server{
		Addr:    ":9090",
		Handler: metricsMux,
	}
	go func() {
		logger.Info("metrics server starting", "addr", ":9090")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	// Start HTTP server.
	httpServer := &http.Server{
		Addr:         cfg.Listen.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  cfg.Timeouts.Read,
		WriteTimeout: cfg.Timeouts.Write,
		IdleTimeout:  cfg.Timeouts.Idle,
	}
	go func() {
		logger.Info("HTTP server starting", "addr", cfg.Listen.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Start HTTPS server (if TLS is enabled).
	var httpsServer *http.Server
	if cfg.TLS.Enabled {
		tlsConfig := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
		}
		httpsServer = &http.Server{
			Addr:         cfg.Listen.HTTPSAddr,
			Handler:      handler,
			TLSConfig:    tlsConfig,
			ReadTimeout:  cfg.Timeouts.Read,
			WriteTimeout: cfg.Timeouts.Write,
			IdleTimeout:  cfg.Timeouts.Idle,
		}
		go func() {
			logger.Info("HTTPS server starting", "addr", cfg.Listen.HTTPSAddr)
			if err := httpsServer.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTPS server error", "error", err)
			}
		}()
	}

	// Wait for interrupt signal.
	<-ctx.Done()
	logger.Info("shutdown signal received, draining connections...")

	// Graceful shutdown with deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeouts.ShutdownGrace)
	defer cancel()

	// Shutdown all servers.
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}
	if httpsServer != nil {
		if err := httpsServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTPS server shutdown error", "error", err)
		}
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown error", "error", err)
	}

	fmt.Println("server stopped gracefully")
}
