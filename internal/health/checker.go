package health

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/shantanuk/reverse-proxy/internal/backend"
)

// Checker performs active health checks against backends.
type Checker struct {
	pool               *backend.Pool
	path               string
	interval           time.Duration
	timeout            time.Duration
	unhealthyThreshold int
	healthyThreshold   int
	logger             *slog.Logger
	client             *http.Client

	// OnStatusChange is called when a backend's health status changes.
	// alive=true means recovered, alive=false means ejected.
	OnStatusChange func(b *backend.Backend, alive bool)
}

// Config holds the configuration for the health checker.
type Config struct {
	Path               string
	Interval           time.Duration
	Timeout            time.Duration
	UnhealthyThreshold int
	HealthyThreshold   int
}

// NewChecker creates a new health checker.
func NewChecker(pool *backend.Pool, cfg Config, logger *slog.Logger) *Checker {
	return &Checker{
		pool:               pool,
		path:               cfg.Path,
		interval:           cfg.Interval,
		timeout:            cfg.Timeout,
		unhealthyThreshold: cfg.UnhealthyThreshold,
		healthyThreshold:   cfg.HealthyThreshold,
		logger:             logger,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Run starts the health check loop. It blocks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Info("health checker started",
		"interval", c.interval,
		"path", c.path,
		"unhealthy_threshold", c.unhealthyThreshold,
		"healthy_threshold", c.healthyThreshold,
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("health checker stopped")
			return
		case <-ticker.C:
			c.checkAll()
		}
	}
}

// checkAll probes all backends.
func (c *Checker) checkAll() {
	for _, b := range c.pool.All() {
		alive := c.probe(b)

		if alive {
			b.ConsecutiveFails = 0
			b.ConsecutiveSucceses++

			if !b.IsAlive() && b.ConsecutiveSucceses >= c.healthyThreshold {
				c.pool.MarkAlive(b)
				b.ConsecutiveSucceses = 0
				c.logger.Info("backend recovered",
					"backend", b.URL.String(),
				)
				if c.OnStatusChange != nil {
					c.OnStatusChange(b, true)
				}
			}
		} else {
			b.ConsecutiveSucceses = 0
			b.ConsecutiveFails++

			if b.IsAlive() && b.ConsecutiveFails >= c.unhealthyThreshold {
				c.pool.MarkDead(b)
				b.ConsecutiveFails = 0
				c.logger.Warn("backend ejected",
					"backend", b.URL.String(),
				)
				if c.OnStatusChange != nil {
					c.OnStatusChange(b, false)
				}
			}
		}
	}
}

// probe sends a health check request to a single backend.
func (c *Checker) probe(b *backend.Backend) bool {
	url := fmt.Sprintf("%s%s", b.URL.String(), c.path)

	resp, err := c.client.Get(url)
	if err != nil {
		c.logger.Debug("health check failed",
			"backend", b.URL.String(),
			"error", err,
		)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
