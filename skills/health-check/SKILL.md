---
name: health-check
description: "Check service health via HTTP endpoints with response time, status code, and body validation. Trigger: when checking service health, verifying HTTP endpoints, readiness/liveness probes, uptime monitoring"
version: 1
argument-hint: "[url]"
allowed-tools:
  - bash
  - read
  - grep
---
# Health Check

You are now operating in service health check mode.

## Basic HTTP Health Check

```bash
# Status code + response time
curl -s -o /dev/null -w "%{http_code} %{time_total}s" http://localhost:8080/health

# Full response with timing
curl -s -w "\nStatus: %{http_code}\nTime: %{time_total}s\n" http://localhost:8080/health

# Fail on non-2xx (exit code 1 if error)
curl -s -f http://localhost:8080/health || echo "UNHEALTHY"

# Follow redirects
curl -s -L -o /dev/null -w "%{http_code}" http://localhost:8080/
```

## Parse JSON Health Response

```bash
# Pretty-print health endpoint
curl -s http://localhost:8080/health | jq .

# Check specific field
curl -s http://localhost:8080/health | jq -r '.status'

# Assert status equals "ok"
STATUS=$(curl -s http://localhost:8080/health | jq -r '.status')
[ "$STATUS" = "ok" ] && echo "PASS" || echo "FAIL: status=$STATUS"
```

## Response Time Thresholds

```bash
# Measure and check against threshold
TIME=$(curl -s -o /dev/null -w "%{time_total}" http://localhost:8080/health)
echo "Response time: ${TIME}s"

# Warn if over 1 second
if (( $(echo "$TIME > 1.0" | bc -l) )); then
  echo "WARNING: slow response (${TIME}s > 1.0s threshold)"
fi

# Critical if over 5 seconds
if (( $(echo "$TIME > 5.0" | bc -l) )); then
  echo "CRITICAL: very slow response (${TIME}s)"
fi
```

## Check Multiple Endpoints

```bash
# Check a list of endpoints
for url in \
  "http://localhost:8080/health" \
  "http://localhost:8080/ready" \
  "http://localhost:8080/metrics"; do
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$url")
  echo "$url → $STATUS"
done
```

## Kubernetes Readiness / Liveness Probe Pattern

```yaml
# In Kubernetes deployment manifest
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

```bash
# Simulate liveness check
curl -s -f http://localhost:8080/health/live && echo "LIVE" || echo "NOT LIVE"

# Simulate readiness check
curl -s -f http://localhost:8080/health/ready && echo "READY" || echo "NOT READY"
```

## Go Health Endpoint Implementation Pattern

```go
// Minimal health handler
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, `{"status":"ok"}`)
})

// Readiness check with dependency verification
http.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
    if err := db.PingContext(r.Context()); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        fmt.Fprintf(w, `{"status":"not ready","error":%q}`, err.Error())
        return
    }
    w.Header().Set("Content-Type", "application/json")
    fmt.Fprintln(w, `{"status":"ready"}`)
})
```

## SSL Certificate Check

```bash
# Check SSL certificate expiry
curl -s -o /dev/null -w "%{ssl_verify_result}" https://example.com
# 0 = valid, non-zero = error

# Show certificate details
echo | openssl s_client -connect example.com:443 2>/dev/null | openssl x509 -noout -dates

# Days until expiry
EXPIRY=$(echo | openssl s_client -connect example.com:443 2>/dev/null | openssl x509 -noout -enddate | cut -d= -f2)
EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s 2>/dev/null || date -j -f "%b %d %T %Y %Z" "$EXPIRY" +%s)
DAYS=$(( (EXPIRY_EPOCH - $(date +%s)) / 86400 ))
echo "Certificate expires in $DAYS days"
```

## Interpreting Results

| HTTP Status | Meaning | Action |
|-------------|---------|--------|
| 200 | Healthy | None |
| 503 | Service Unavailable | Check logs, dependencies |
| 404 | Endpoint missing | Verify route configuration |
| 000 | Connection refused | Service may be down |
| 504 | Gateway timeout | Check upstream services |
