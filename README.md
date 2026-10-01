# 🔀 Go Reverse Proxy & Load Balancer

A production-grade HTTP reverse proxy with load balancing, active health checks, TLS termination, and Prometheus observability — built in Go with minimal dependencies.

## Features

- **Load Balancing** — Round-robin and least-connections strategies via a pluggable interface
- **Active Health Checks** — Periodic probing with configurable thresholds; unhealthy backends are ejected and automatically re-added on recovery
- **TLS Termination** — HTTPS frontend with configurable TLS 1.2+ and cipher preferences
- **Graceful Shutdown** — Drains in-flight requests on SIGINT/SIGTERM with configurable grace period
- **Prometheus Metrics** — Request rates, p50/p99 latency histograms, backend error counters, active connection gauges, and health status
- **Docker Compose Demo** — One-command stack with 3 backends, the proxy, and Prometheus
- **Load Testing** — k6 scripts with ramp-up/down stages and automated assertions

## Architecture

```
Client → [TLS Termination] → Reverse Proxy → Load Balancer → Backend Pool
                                  ↑                              ↕
                          Health Checker ←──── periodic probes ───┘
                                  ↓
                         Prometheus /metrics
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose (for the demo)
- k6 (for load testing): `brew install k6`

### Build & Run Locally

```bash
# Build
make build

# Generate TLS certificates
make certs

# Run (starts HTTP on :8080, HTTPS on :8443, metrics on :9090)
make run
```

### Docker Compose Demo

```bash
# Start the full stack (proxy + 3 backends + Prometheus)
make demo-up

# Test it
curl http://localhost:8080/
curl http://localhost:8080/
curl http://localhost:8080/

# View Prometheus metrics
open http://localhost:9091

# Run load test
make loadtest

# Tear down
make demo-down
```

### Run Tests

```bash
make test
```

## Configuration

Configuration is via YAML (`config/proxy.yaml`):

```yaml
listen:
  http_addr: ":8080"
  https_addr: ":8443"

tls:
  enabled: true
  cert_file: "certs/server.crt"
  key_file: "certs/server.key"

backends:
  - url: "http://localhost:9001"
  - url: "http://localhost:9002"
  - url: "http://localhost:9003"

balancer:
  strategy: "round-robin"  # or "least-connections"

health_check:
  enabled: true
  path: "/healthz"
  interval: "5s"
  timeout: "2s"
  unhealthy_threshold: 3
  healthy_threshold: 2

timeouts:
  read: "5s"
  write: "10s"
  idle: "120s"
  shutdown_grace: "30s"
```

## Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `proxy_requests_total` | Counter | Total proxied requests (by method, status, backend) |
| `proxy_request_duration_seconds` | Histogram | Request latency distribution |
| `proxy_backend_errors_total` | Counter | Backend errors (by backend, error type) |
| `proxy_active_connections` | Gauge | In-flight requests per backend |
| `proxy_backend_health` | Gauge | Backend health (1=up, 0=down) |

Example PromQL queries:

```promql
# Requests per second
rate(proxy_requests_total[1m])

# p99 latency
histogram_quantile(0.99, rate(proxy_request_duration_seconds_bucket[5m]))

# Unhealthy backends
proxy_backend_health == 0
```

## Project Structure

```
├── cmd/proxy/           # Entry point
├── internal/
│   ├── backend/         # Thread-safe backend pool
│   ├── balancer/        # Load balancing strategies
│   ├── config/          # YAML configuration
│   ├── health/          # Active health checker
│   ├── middleware/       # Logging, metrics, recovery
│   └── proxy/           # Core reverse proxy handler
├── config/              # Default config file
├── certs/               # TLS cert generation
├── demo/                # Docker Compose + k6 load tests
├── Dockerfile           # Multi-stage proxy build
└── Makefile             # Build automation
```

## Dependencies

Only **2 external dependencies** — everything else is Go stdlib:

| Package | Purpose |
|---------|---------|
| `gopkg.in/yaml.v3` | Config parsing |
| `github.com/prometheus/client_golang` | Metrics |

## License

MIT
