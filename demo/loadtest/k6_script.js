import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

// Custom metrics.
const errorRate = new Rate("errors");
const latency = new Trend("request_latency", true);

// Test configuration.
export const options = {
  stages: [
    { duration: "30s", target: 50 },   // Ramp up to 50 VUs
    { duration: "2m", target: 100 },   // Hold at 100 VUs
    { duration: "30s", target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"],   // p95 latency < 500ms
    errors: ["rate<0.01"],              // Error rate < 1%
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export default function () {
  const res = http.get(`${BASE_URL}/`);

  // Track custom metrics.
  latency.add(res.timings.duration);
  errorRate.add(res.status >= 400);

  // Assertions.
  check(res, {
    "status is 200": (r) => r.status === 200,
    "response has backend field": (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.backend !== undefined;
      } catch {
        return false;
      }
    },
    "has X-Proxy-Backend header": (r) =>
      r.headers["X-Proxy-Backend"] !== undefined,
  });

  sleep(0.1); // 100ms think time between requests
}

// Lifecycle hooks for reporting.
export function handleSummary(data) {
  const summary = {
    total_requests: data.metrics.http_reqs.values.count,
    avg_latency_ms: data.metrics.http_req_duration.values.avg.toFixed(2),
    p50_latency_ms: data.metrics.http_req_duration.values["p(50)"]?.toFixed(2),
    p95_latency_ms: data.metrics.http_req_duration.values["p(95)"]?.toFixed(2),
    p99_latency_ms: data.metrics.http_req_duration.values["p(99)"]?.toFixed(2),
    error_rate: (
      (data.metrics.http_req_failed?.values?.rate || 0) * 100
    ).toFixed(2) + "%",
  };

  console.log("\n=== Load Test Summary ===");
  console.log(JSON.stringify(summary, null, 2));

  return {
    stdout: JSON.stringify(summary, null, 2),
  };
}
