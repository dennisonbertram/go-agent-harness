---
name: prometheus-ops
description: "Operate Prometheus monitoring: PromQL queries, promtool validation, metrics exposition, alerting rules, scrape configs. Trigger: when querying Prometheus, PromQL, prometheus metrics, promtool check, alerting rules, Prometheus scrape config, metrics exposition format"
version: 1
argument-hint: "[query <promql>|check|alerts|targets|metrics]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Prometheus Operations

You are now operating in Prometheus monitoring mode.

## Installation

```bash
# macOS
brew install prometheus

# Verify
prometheus --version
promtool --version

# Download binary (Linux)
PROM_VERSION="2.51.0"
curl -LO "https://github.com/prometheus/prometheus/releases/download/v${PROM_VERSION}/prometheus-${PROM_VERSION}.linux-amd64.tar.gz"
tar -xzf prometheus-*.tar.gz
```

## Querying Prometheus via API

```bash
# Set Prometheus URL (default local instance)
PROM_URL="${PROMETHEUS_URL:-http://localhost:9090}"

# Instant query (current value)
curl -s "${PROM_URL}/api/v1/query" \
  --data-urlencode 'query=up' | jq '.data.result'

# Range query (over a time window)
curl -s "${PROM_URL}/api/v1/query_range" \
  --data-urlencode 'query=rate(http_requests_total[5m])' \
  --data-urlencode 'start=2024-01-01T00:00:00Z' \
  --data-urlencode 'end=2024-01-01T01:00:00Z' \
  --data-urlencode 'step=60' | jq '.data.result'

# List all active targets
curl -s "${PROM_URL}/api/v1/targets" | jq '.data.activeTargets[] | {job: .labels.job, instance: .labels.instance, health: .health}'

# List all metric names
curl -s "${PROM_URL}/api/v1/label/__name__/values" | jq '.data[]'

# Get metadata for a specific metric
curl -s "${PROM_URL}/api/v1/metadata?metric=http_requests_total" | jq '.data'

# List current alerts
curl -s "${PROM_URL}/api/v1/alerts" | jq '.data.alerts[] | {name: .labels.alertname, state: .state}'
```

## PromQL Essentials

### Selectors

```promql
# All time series for a metric
http_requests_total

# Filter by label
http_requests_total{job="api", method="GET"}

# Regex label match
http_requests_total{handler=~"/api/.*"}

# Negative label match
http_requests_total{status!="200"}

# Range vector (for rate/increase calculations)
http_requests_total[5m]
```

### Rate and Increase Functions

```promql
# Request rate per second (over 5 minute window)
rate(http_requests_total[5m])

# Total increase over 1 hour
increase(http_requests_total[1h])

# Rate per job and method
sum by (job, method) (rate(http_requests_total[5m]))

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# Error ratio (percentage)
rate(http_requests_total{status=~"5.."}[5m])
  / rate(http_requests_total[5m])
```

### Aggregation

```promql
# Sum across all instances
sum(http_requests_total)

# Sum grouped by label
sum by (status_code) (http_requests_total)

# Without (sum all except specified labels)
sum without (instance) (http_requests_total)

# Average CPU across instances
avg by (job) (rate(process_cpu_seconds_total[5m]))

# Max memory across all pods
max by (namespace) (container_memory_usage_bytes)

# Count instances that are down
count(up == 0)
```

### Latency (Histogram)

```promql
# 95th percentile request latency
histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))

# 50th percentile by job
histogram_quantile(0.50,
  sum by (job, le) (rate(http_request_duration_seconds_bucket[5m])))

# Average latency
rate(http_request_duration_seconds_sum[5m])
  / rate(http_request_duration_seconds_count[5m])
```

### Common SLI Patterns

```promql
# Availability (ratio of successful requests)
1 - (
  rate(http_requests_total{status=~"5.."}[5m])
    / rate(http_requests_total[5m])
)

# Request throughput
sum(rate(http_requests_total[5m]))

# Apdex score (fraction of satisfying + tolerable requests)
(
  sum(rate(http_request_duration_seconds_bucket{le="0.3"}[5m])) +
  sum(rate(http_request_duration_seconds_bucket{le="1.2"}[5m]))
) / 2 / sum(rate(http_request_duration_seconds_count[5m]))
```

## Metrics Exposition Format

```bash
# Expose metrics endpoint (Go example)
# GET /metrics returns text in Prometheus exposition format
curl -s http://localhost:8080/metrics

# Valid exposition format:
# # HELP http_requests_total Total HTTP requests
# # TYPE http_requests_total counter
# http_requests_total{method="GET",status="200"} 1234.0 1709123456789
# http_requests_total{method="POST",status="201"} 42.0
```

### Go Metrics with prometheus/client_golang

```go
package main

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "handler", "status"},
    )

    httpDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"handler"},
    )
)

func main() {
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":8080", nil)
}
```

## promtool — Validation and Testing

```bash
# Check Prometheus configuration file
promtool check config prometheus.yml

# Check a rules file for syntax errors
promtool check rules alerts.yml

# Check a web config (TLS settings)
promtool check web-config web.yml

# Unit-test alerting rules
promtool test rules tests/alert_tests.yml

# Query metrics from command line (Prometheus must be running)
promtool query instant http://localhost:9090 'up'
promtool query range http://localhost:9090 'rate(http_requests_total[5m])' --start=2024-01-01T00:00:00Z --end=2024-01-01T01:00:00Z --step=60s

# Backfill TSDB from OpenMetrics file
promtool tsdb create-blocks-from openmetrics metrics.txt ./data/
```

## Scrape Configuration

```yaml
# prometheus.yml — scrape config examples

global:
  scrape_interval: 15s
  evaluation_interval: 15s
  scrape_timeout: 10s

scrape_configs:
  # Scrape Prometheus itself
  - job_name: prometheus
    static_configs:
      - targets: ["localhost:9090"]

  # Static targets
  - job_name: api
    metrics_path: /metrics
    scheme: http
    static_configs:
      - targets: ["api-1:8080", "api-2:8080"]
        labels:
          env: production

  # Kubernetes pod discovery
  - job_name: kubernetes-pods
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: "true"
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
```

## Alerting Rules

```yaml
# alerts.yml — alerting rule examples

groups:
  - name: service.alerts
    interval: 30s
    rules:
      # Alert if any target is down
      - alert: InstanceDown
        expr: up == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Instance {{ $labels.instance }} is down"
          description: "{{ $labels.job }}/{{ $labels.instance }} has been down for more than 5 minutes."

      # High error rate
      - alert: HighErrorRate
        expr: |
          rate(http_requests_total{status=~"5.."}[5m])
            / rate(http_requests_total[5m]) > 0.05
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High HTTP error rate on {{ $labels.job }}"
          description: "Error rate is {{ $value | humanizePercentage }} on {{ $labels.instance }}"

      # High latency (p99 > 1 second)
      - alert: HighLatency
        expr: |
          histogram_quantile(0.99,
            sum by (le, job) (rate(http_request_duration_seconds_bucket[5m]))
          ) > 1.0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High p99 latency on {{ $labels.job }}"
          description: "p99 latency is {{ $value | humanizeDuration }} on {{ $labels.job }}"
```

```bash
# Reload alerting rules without restarting Prometheus
kill -HUP $(pgrep prometheus)
# or via API:
curl -X POST http://localhost:9090/-/reload
```

## Unit Testing Alerting Rules

```yaml
# tests/alert_tests.yml
rule_files:
  - ../alerts.yml

tests:
  - interval: 1m
    input_series:
      - series: 'up{job="api", instance="api-1:8080"}'
        values: "1 1 1 0 0 0 0 0"
    alert_rule_test:
      - eval_time: 6m
        alertname: InstanceDown
        exp_alerts:
          - exp_labels:
              severity: critical
              job: api
              instance: "api-1:8080"
            exp_annotations:
              summary: "Instance api-1:8080 is down"
```

```bash
promtool test rules tests/alert_tests.yml
```

## Best Practices

- Use `rate()` for counters in dashboards and alerts — never use raw counter values in rules.
- Always use a sufficient range window in `rate()`: at least 4x the scrape interval.
- Prefer `sum by` over `sum without` for clarity.
- Add `# HELP` and `# TYPE` comments to all custom metrics before exposition.
- Use labeled histograms instead of multiple gauges for latency tracking.
- Use `promtool check rules` in CI to validate alert YAML before deployment.
- Keep alert `for` duration at least 2x the scrape interval to avoid flapping.
- Document every custom metric with a meaningful `Help` string.
