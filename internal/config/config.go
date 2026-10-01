package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for the reverse proxy.
type Config struct {
	Listen      ListenConfig      `yaml:"listen"`
	TLS         TLSConfig         `yaml:"tls"`
	Backends    []BackendConfig   `yaml:"backends"`
	Balancer    BalancerConfig    `yaml:"balancer"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	Timeouts    TimeoutConfig     `yaml:"timeouts"`
}

// ListenConfig defines the addresses the proxy listens on.
type ListenConfig struct {
	HTTPAddr  string `yaml:"http_addr"`
	HTTPSAddr string `yaml:"https_addr"`
}

// TLSConfig holds TLS termination settings.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// BackendConfig represents a single upstream backend server.
type BackendConfig struct {
	URL string `yaml:"url"`
}

// BalancerConfig defines the load balancing strategy.
type BalancerConfig struct {
	Strategy string `yaml:"strategy"`
}

// HealthCheckConfig controls active health checking behavior.
type HealthCheckConfig struct {
	Enabled            bool          `yaml:"enabled"`
	Path               string        `yaml:"path"`
	Interval           time.Duration `yaml:"interval"`
	Timeout            time.Duration `yaml:"timeout"`
	UnhealthyThreshold int           `yaml:"unhealthy_threshold"`
	HealthyThreshold   int           `yaml:"healthy_threshold"`
}

// TimeoutConfig sets various connection and shutdown timeouts.
type TimeoutConfig struct {
	Read          time.Duration `yaml:"read"`
	Write         time.Duration `yaml:"write"`
	Idle          time.Duration `yaml:"idle"`
	ShutdownGrace time.Duration `yaml:"shutdown_grace"`
}

// Load reads and parses the YAML configuration file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.setDefaults()

	return cfg, nil
}

// validate checks for required fields and valid values.
func (c *Config) validate() error {
	if len(c.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}

	for i, b := range c.Backends {
		if b.URL == "" {
			return fmt.Errorf("backend %d has empty URL", i)
		}
	}

	strategy := c.Balancer.Strategy
	if strategy != "" && strategy != "round-robin" && strategy != "least-connections" {
		return fmt.Errorf("unsupported balancer strategy: %q (must be \"round-robin\" or \"least-connections\")", strategy)
	}

	if c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			return fmt.Errorf("TLS is enabled but cert_file is not set")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("TLS is enabled but key_file is not set")
		}
	}

	return nil
}

// setDefaults applies sensible defaults for unset fields.
func (c *Config) setDefaults() {
	if c.Listen.HTTPAddr == "" {
		c.Listen.HTTPAddr = ":8080"
	}
	if c.Listen.HTTPSAddr == "" {
		c.Listen.HTTPSAddr = ":8443"
	}
	if c.Balancer.Strategy == "" {
		c.Balancer.Strategy = "round-robin"
	}
	if c.HealthCheck.Path == "" {
		c.HealthCheck.Path = "/healthz"
	}
	if c.HealthCheck.Interval == 0 {
		c.HealthCheck.Interval = 5 * time.Second
	}
	if c.HealthCheck.Timeout == 0 {
		c.HealthCheck.Timeout = 2 * time.Second
	}
	if c.HealthCheck.UnhealthyThreshold == 0 {
		c.HealthCheck.UnhealthyThreshold = 3
	}
	if c.HealthCheck.HealthyThreshold == 0 {
		c.HealthCheck.HealthyThreshold = 2
	}
	if c.Timeouts.Read == 0 {
		c.Timeouts.Read = 5 * time.Second
	}
	if c.Timeouts.Write == 0 {
		c.Timeouts.Write = 10 * time.Second
	}
	if c.Timeouts.Idle == 0 {
		c.Timeouts.Idle = 120 * time.Second
	}
	if c.Timeouts.ShutdownGrace == 0 {
		c.Timeouts.ShutdownGrace = 30 * time.Second
	}
}
