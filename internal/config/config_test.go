package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
listen:
  http_addr: ":8080"
  https_addr: ":8443"
tls:
  enabled: false
backends:
  - url: "http://localhost:9001"
  - url: "http://localhost:9002"
balancer:
  strategy: "round-robin"
health_check:
  enabled: true
  path: "/healthz"
  interval: "10s"
  timeout: "3s"
  unhealthy_threshold: 5
  healthy_threshold: 3
timeouts:
  read: "5s"
  write: "10s"
  idle: "60s"
  shutdown_grace: "15s"
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Listen.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.Listen.HTTPAddr, ":8080")
	}
	if len(cfg.Backends) != 2 {
		t.Errorf("len(Backends) = %d, want 2", len(cfg.Backends))
	}
	if cfg.Balancer.Strategy != "round-robin" {
		t.Errorf("Strategy = %q, want %q", cfg.Balancer.Strategy, "round-robin")
	}
	if cfg.HealthCheck.Interval != 10*time.Second {
		t.Errorf("Interval = %v, want %v", cfg.HealthCheck.Interval, 10*time.Second)
	}
	if cfg.HealthCheck.UnhealthyThreshold != 5 {
		t.Errorf("UnhealthyThreshold = %d, want 5", cfg.HealthCheck.UnhealthyThreshold)
	}
}

func TestLoad_NoBackends(t *testing.T) {
	content := `
backends: []
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty backends, got nil")
	}
}

func TestLoad_InvalidStrategy(t *testing.T) {
	content := `
backends:
  - url: "http://localhost:9001"
balancer:
  strategy: "random"
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid strategy, got nil")
	}
}

func TestLoad_TLSMissingCert(t *testing.T) {
	content := `
backends:
  - url: "http://localhost:9001"
tls:
  enabled: true
  key_file: "server.key"
`
	path := writeTemp(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing cert_file, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	content := `
backends:
  - url: "http://localhost:9001"
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Listen.HTTPAddr != ":8080" {
		t.Errorf("default HTTPAddr = %q, want %q", cfg.Listen.HTTPAddr, ":8080")
	}
	if cfg.Balancer.Strategy != "round-robin" {
		t.Errorf("default Strategy = %q, want %q", cfg.Balancer.Strategy, "round-robin")
	}
	if cfg.Timeouts.Read != 5*time.Second {
		t.Errorf("default Read timeout = %v, want %v", cfg.Timeouts.Read, 5*time.Second)
	}
	if cfg.HealthCheck.UnhealthyThreshold != 3 {
		t.Errorf("default UnhealthyThreshold = %d, want 3", cfg.HealthCheck.UnhealthyThreshold)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}
