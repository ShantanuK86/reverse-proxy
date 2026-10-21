# 🔀 Go Reverse Proxy, Load Balancer & "Proxy Panic" 🎮

A production-grade HTTP reverse proxy and load balancer in Go featuring **Proxy Panic** — an interactive in-browser networking game, visual request topology simulator, and observability dashboard.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen.svg)]()

---

## 🎮 Play "Proxy Panic" (In-Browser Simulator)

Instead of just reading code, **Proxy Panic** lets you run the reverse proxy directly in your browser, trigger chaos events, survive traffic spikes, and inspect real-time routing forensics.

```
                    ┌────────────────────────┐
                    │      Client Nodes      │
                    └───────────┬────────────┘
                                │ (HTTP/HTTPS)
                                ▼
                    ┌────────────────────────┐
                    │    Go Reverse Proxy    │
                    │  [Proxy Panic Engine]  │
                    └───────────┬────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐       ┌───────────────┐       ┌───────────────┐
│ Backend Alpha │       │ Backend Beta  │       │ Backend Gamma │
│  (Fast: 15ms) │       │ (Std: 25ms)   │       │ (Heavy: 45ms) │
└───────────────┘       └───────────────┘       └───────────────┘
```

### 🚀 5 Interactive Missions:
1. **Mission 1 — First Hop:** Send your first proxied request to `/api/users`. Inspect hop-by-hop stripping and `X-Forwarded-*` header injection.
2. **Mission 2 — Traffic Stampede:** Burst 20 concurrent requests. Compare **Round-Robin** vs **Least-Connections** distribution in real-time.
3. **Mission 3 — One Server Down:** Click **Take Server Beta Offline**. Watch the active health checker detect 3 consecutive failures, eject Beta, and safely reroute 100% of traffic with zero dropped requests! Revive Beta and watch it automatically re-admitted after 2 successful probes.
4. **Mission 4 — Header Detective:** Inspect real headers received by the backend. Answer RFC 7230 forensic questions and earn XP.
5. **Mission 5 — Route Master:** Apply prefix-based routing rules (`/api/auth/*` ➔ Alpha) and verify that route precedence overrides default balancing.

### ⚡ 60-Second Arcade Survival Challenge:
* 100 requests incoming in surges.
* Dynamic chaos events: nodes slow down or crash!
* Adapt your load balancing policy on the fly to keep success rate above 95% and earn the **SRE Maestro** badge for LinkedIn!

### 🔍 Detective-Style Request Trace Drawer:
Click any request in the live feed to open the microscopic forensic timeline:
* `[0.0ms] Client Ingress`: Remote IP & headers recorded.
* `[0.2ms] Middleware Pipeline`: Panic recovery & Prometheus timer started.
* `[0.6ms] Balancer Decision`: Selected backend via active strategy.
* `[1.0ms] Header Transformation`: Stripped hop-by-hop (`Connection`, `Keep-Alive`), injected `X-Forwarded-For`.
* `[16.1ms] Upstream RoundTrip`: Reused persistent keep-alive TCP socket pool.
* `[16.4ms] Client Delivery`: Attached `X-Proxy-Backend` and streamed response.

---

## ⚡ Quick Start

### 1. Launch Proxy Panic (Zero Dependencies!)

Run the proxy with embedded frontend assets and self-contained mock backends:

```bash
go run ./cmd/proxy
```

Open your browser at **[http://localhost:8080](http://localhost:8080)** to play the game immediately!

### 2. Deploy to Render in 60 Seconds

The Go service is 100% cloud-ready for **[Render](https://render.com)**, **Fly.io**, or **Docker**:
1. Fork or push this repository to GitHub.
2. On Render, click **New + ➔ Web Service** and connect your repo.
3. Set **Runtime** to `Go`, **Build Command** to `go build -o proxy ./cmd/proxy`, and **Start Command** to `./proxy`.
4. The service reads Render's `$PORT` environment variable automatically and serves **Proxy Panic** on your public `onrender.com` URL!

---

## 🛠️ Docker Compose Demo Stack (Optional)

If you prefer testing with separate multi-container Docker services:

```bash
# Start proxy + 3 backend containers + Prometheus
make demo-up

# Proxy UI:      http://localhost:8080
# Prometheus:    http://localhost:9091

# Run k6 load test
make loadtest

# Teardown
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
