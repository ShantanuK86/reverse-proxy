# Load Testing Guide

## Prerequisites

Install [k6](https://k6.io/docs/getting-started/installation/):

```bash
# macOS
brew install k6

# Docker
docker pull grafana/k6
```

## Running the Load Test

### Against Docker Compose stack

```bash
# Start the demo stack
make demo-up

# Run the load test
make loadtest

# Or manually:
k6 run --env BASE_URL=http://localhost:8080 k6_script.js
```

### Chaos test: kill a backend mid-load

In terminal 1:
```bash
k6 run --env BASE_URL=http://localhost:8080 k6_script.js
```

In terminal 2 (while load test is running):
```bash
docker compose -f demo/docker-compose.yml stop backend-2
```

Watch the proxy logs — backend-2 should be ejected after 3 failed health checks (15s).
After ejection, all traffic routes to backend-1 and backend-3 with zero errors.

### Custom parameters

```bash
# Shorter test
k6 run --duration 30s --vus 50 k6_script.js

# Higher load
k6 run --duration 5m --vus 200 k6_script.js
```

## Understanding Results

| Metric | Target | Description |
|--------|--------|-------------|
| `http_req_duration p(95)` | < 500ms | 95th percentile latency |
| `errors` | < 1% | Request error rate |
| `http_reqs` | — | Total requests completed |
