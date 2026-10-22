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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/shantanuk/reverse-proxy/internal/backend"
	"github.com/shantanuk/reverse-proxy/internal/balancer"
	"github.com/shantanuk/reverse-proxy/internal/config"
	"github.com/shantanuk/reverse-proxy/internal/game"
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

	// Determine port (support cloud platforms like Render via PORT env var)
	httpAddr := ":8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		httpAddr = ":" + envPort
	}

	// Load configuration (if missing, use defaults)
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Warn("could not load config file, falling back to interactive game defaults", "error", err)
		cfg = &config.Config{
			Listen: config.ListenConfig{HTTPAddr: httpAddr, HTTPSAddr: ":8443"},
			Balancer: config.BalancerConfig{Strategy: "round-robin"},
			HealthCheck: config.HealthCheckConfig{
				Enabled:            true,
				Path:               "/healthz",
				Interval:           3 * time.Second,
				Timeout:            1 * time.Second,
				UnhealthyThreshold: 3,
				HealthyThreshold:   2,
			},
			Timeouts: config.TimeoutConfig{
				Read:          5 * time.Second,
				Write:         10 * time.Second,
				Idle:          120 * time.Second,
				ShutdownGrace: 10 * time.Second,
			},
		}
	} else if os.Getenv("PORT") != "" {
		cfg.Listen.HTTPAddr = httpAddr
	}

	// Initialize in-process mock backends for the interactive game
	mockMgr, err := game.NewMockManager()
	if err != nil {
		logger.Error("failed to start mock backends", "error", err)
		os.Exit(1)
	}
	defer mockMgr.Close()

	// Initialize backend pool using mock backends (Alpha, Beta, Gamma)
	pool, err := backend.NewPool(mockMgr.URLs())
	if err != nil {
		logger.Error("failed to create backend pool", "error", err)
		os.Exit(1)
	}

	// Set descriptive display names on pool backends
	allBackends := pool.All()
	for i, mb := range mockMgr.All() {
		if i < len(allBackends) {
			allBackends[i].Name = mb.Name
		}
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

	for _, b := range pool.All() {
		metrics.SetBackendHealth(b.URL.String(), true)
	}

	// Create core reverse proxy handler.
	proxyHandler := proxy.New(pool, bal, logger)
	proxyHandler.ErrorCounter = metrics.RecordBackendError

	// Build middleware chain: Recovery → Logging → Metrics → Proxy
	var proxiedHandler http.Handler = proxyHandler
	proxiedHandler = metrics.InstrumentHandler(proxiedHandler)
	proxiedHandler = middleware.Logging(logger)(proxiedHandler)
	proxiedHandler = middleware.Recovery(logger)(proxiedHandler)

	// Context for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start health checker.
	var checker *health.Checker
	if cfg.HealthCheck.Enabled {
		checker = health.NewChecker(pool, health.Config{
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

	// Create Game Controller & Multiplexer
	gameController := game.NewController(pool, proxyHandler, checker, mockMgr)

	mainMux := http.NewServeMux()

	// Mount embedded game static assets with prefix stripped
	staticFileServer := game.StaticHandler()
	mainMux.Handle("/static/", http.StripPrefix("/static/", staticFileServer))

	// Mount game API endpoints
	gameController.RegisterRoutes(mainMux)

	// Expose metrics on main port as well (essential for single-port cloud hosts like Render)
	mainMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	// Direct proxy route
	mainMux.Handle("/proxy/", http.StripPrefix("/proxy", proxiedHandler))

	// Root handler: serve game UI at / or static assets, else proxy generic traffic
	mainMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "/index.html" || path == "/style.css" || path == "/app.js" {
			staticFileServer.ServeHTTP(w, r)
			return
		}
		// Proxy all other paths through the reverse proxy
		proxiedHandler.ServeHTTP(w, r)
	})

	// Start separate metrics server on :9090 (if not colliding with main port)
	var metricsServer *http.Server
	if cfg.Listen.HTTPAddr != ":9090" {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		metricsServer = &http.Server{
			Addr:    ":9090",
			Handler: metricsMux,
		}
		go func() {
			logger.Info("metrics server starting", "addr", ":9090")
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Warn("metrics server stopped", "error", err)
			}
		}()
	}

	// Start HTTP server.
	httpServer := &http.Server{
		Addr:         cfg.Listen.HTTPAddr,
		Handler:      mainMux,
		ReadTimeout:  cfg.Timeouts.Read,
		WriteTimeout: cfg.Timeouts.Write,
		IdleTimeout:  cfg.Timeouts.Idle,
	}
	go func() {
		logger.Info("🚀 Proxy Panic running!", "addr", cfg.Listen.HTTPAddr)
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
			Handler:      mainMux,
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
	if metricsServer != nil {
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("metrics server shutdown error", "error", err)
		}
	}

	fmt.Println("server stopped gracefully")
}
