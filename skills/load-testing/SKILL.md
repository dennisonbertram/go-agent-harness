---
name: load-testing
description: "Run load and performance tests with k6 or vegeta: test scripts, ramp-up patterns, threshold configuration, results analysis. Trigger: when using k6, vegeta, load testing, performance testing, stress testing, k6 run, vegeta attack, ramp-up, throughput testing, latency testing"
version: 1
argument-hint: "[k6 run|vegeta attack] [script.js|--rate=100] [--vus 50 --duration 30s]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Load Testing

You are now operating in load and performance testing mode.

## Tool Selection

- **k6**: JavaScript-based, excellent for complex scenarios, great for APIs and web apps
- **vegeta**: Go-based, simple CLI, ideal for constant-rate HTTP load testing

## k6 — JavaScript Load Testing

### Installation

```bash
# macOS (via Homebrew)
brew install k6

# Linux (Debian/Ubuntu)
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6

# Via Docker
docker run --rm grafana/k6 run - <script.js

# Verify installation
k6 version
```

### Basic k6 Script

```javascript
// script.js
import http from 'k6/http'
import { sleep, check } from 'k6'

export const options = {
  vus: 10,           // virtual users
  duration: '30s',   // test duration
}

export default function () {
  const res = http.get('http://localhost:8080/api/health')

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  })

  sleep(1)  // think time between requests
}
```

```bash
# Run the script
k6 run script.js

# Run with more virtual users
k6 run --vus 50 --duration 60s script.js

# Run with output to JSON
k6 run --out json=results.json script.js
```

### Ramp-Up Patterns

```javascript
// Gradual ramp-up (recommended for production load tests)
export const options = {
  stages: [
    { duration: '1m', target: 20 },   // ramp up to 20 VUs over 1 minute
    { duration: '3m', target: 20 },   // hold at 20 VUs for 3 minutes
    { duration: '1m', target: 50 },   // ramp up to 50 VUs
    { duration: '3m', target: 50 },   // hold at 50 VUs
    { duration: '1m', target: 0 },    // ramp down to 0
  ],
}

// Spike test (sudden traffic burst)
export const options = {
  stages: [
    { duration: '1m', target: 10 },   // warm up
    { duration: '30s', target: 200 }, // spike to 200 VUs
    { duration: '1m', target: 10 },   // recover
    { duration: '1m', target: 0 },    // ramp down
  ],
}

// Stress test (find breaking point)
export const options = {
  stages: [
    { duration: '2m', target: 100 },
    { duration: '2m', target: 200 },
    { duration: '2m', target: 300 },
    { duration: '2m', target: 400 },
    { duration: '2m', target: 0 },
  ],
}

// Soak test (extended duration at expected load)
export const options = {
  stages: [
    { duration: '5m', target: 50 },   // ramp up
    { duration: '4h', target: 50 },   // hold for 4 hours
    { duration: '5m', target: 0 },    // ramp down
  ],
}
```

### Threshold Configuration

```javascript
// Define pass/fail criteria
export const options = {
  vus: 50,
  duration: '2m',
  thresholds: {
    // 95th percentile response time must be below 500ms
    http_req_duration: ['p(95)<500'],

    // 99th percentile must be below 1 second
    'http_req_duration{expected_response:true}': ['p(99)<1000'],

    // Error rate must be below 1%
    http_req_failed: ['rate<0.01'],

    // Custom metric threshold
    'checks{check:status is 200}': ['rate>0.99'],

    // Multiple thresholds for same metric
    http_req_duration: ['p(90)<300', 'p(95)<500', 'p(99)<1000'],
  },
}
```

### POST Requests and Authentication

```javascript
import http from 'k6/http'
import { check, sleep } from 'k6'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080'
const API_TOKEN = __ENV.API_TOKEN

export const options = {
  stages: [
    { duration: '30s', target: 20 },
    { duration: '1m', target: 20 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.05'],
  },
}

export default function () {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${API_TOKEN}`,
  }

  // POST request
  const payload = JSON.stringify({
    title: 'Test item',
    description: 'Load test payload',
  })

  const createRes = http.post(`${BASE_URL}/api/items`, payload, { headers })
  check(createRes, {
    'item created': (r) => r.status === 201,
    'has item id': (r) => r.json('id') !== undefined,
  })

  if (createRes.status === 201) {
    const id = createRes.json('id')

    // GET request with the created ID
    const getRes = http.get(`${BASE_URL}/api/items/${id}`, { headers })
    check(getRes, {
      'item found': (r) => r.status === 200,
    })
  }

  sleep(1)
}
```

### Custom Metrics

```javascript
import { Trend, Counter, Rate, Gauge } from 'k6/metrics'

const loginDuration = new Trend('login_duration')
const loginErrors = new Counter('login_errors')
const loginSuccessRate = new Rate('login_success')
const activeUsers = new Gauge('active_users')

export default function () {
  const start = Date.now()

  const res = http.post('/api/auth/login', { ... })

  loginDuration.add(Date.now() - start)

  if (res.status !== 200) {
    loginErrors.add(1)
    loginSuccessRate.add(false)
  } else {
    loginSuccessRate.add(true)
  }

  activeUsers.add(1)
}
```

### Running k6

```bash
# Basic run
k6 run script.js

# With environment variables
k6 run --env BASE_URL=https://staging.example.com --env API_TOKEN=$TOKEN script.js

# With output to various sinks
k6 run --out json=results.json script.js
k6 run --out influxdb=http://localhost:8086/k6 script.js
k6 run --out csv=results.csv script.js

# Run with quiet mode (less output)
k6 run --quiet script.js

# Abort on first threshold breach
k6 run --abort-on-fail script.js

# Run with tags (for filtering results)
k6 run --tag environment=staging script.js

# List results summary
k6 run script.js | tee load-test-results.txt
```

## vegeta — HTTP Load Testing Tool

### Installation

```bash
# macOS (via Homebrew)
brew install vegeta

# Download binary from GitHub releases
# https://github.com/tsenart/vegeta/releases

# Verify installation
vegeta -version
```

### Basic Usage

```bash
# Simple constant-rate attack
echo "GET http://localhost:8080/api/health" | \
  vegeta attack -rate=100 -duration=30s | \
  vegeta report

# Rate: 100 requests per second for 30 seconds

# Attack with specific duration
echo "GET http://localhost:8080/" | \
  vegeta attack -rate=50 -duration=60s | \
  tee results.bin | \
  vegeta report
```

### vegeta Targets File

```bash
# targets.txt — define requests
GET http://localhost:8080/api/health
Authorization: Bearer mytoken

GET http://localhost:8080/api/users

POST http://localhost:8080/api/items
Content-Type: application/json
@body.json
```

```bash
# body.json
# {"title": "test", "description": "load test"}

# Run with targets file
vegeta attack -targets=targets.txt -rate=100 -duration=30s | \
  vegeta report
```

### vegeta Output Formats

```bash
# Text report (default)
vegeta attack -rate=100 -duration=30s -targets=targets.txt | \
  vegeta report

# JSON report
vegeta attack -rate=100 -duration=30s -targets=targets.txt | \
  vegeta report -type=json > report.json

# Histogram report
vegeta attack -rate=100 -duration=30s -targets=targets.txt | \
  vegeta report -type=hdrplot > hdr.txt

# Plot (generates HTML graph)
vegeta attack -rate=100 -duration=30s -targets=targets.txt | \
  vegeta plot > plot.html && open plot.html

# Save results for later analysis
vegeta attack -rate=100 -duration=30s -targets=targets.txt > results.bin
vegeta report < results.bin
vegeta plot < results.bin > plot.html
```

### vegeta Ramp-Up with Pacer

```bash
# Linear ramp-up from 10 to 100 rps over 30 seconds
vegeta attack \
  -rate=10 \
  -duration=30s \
  -targets=targets.txt | vegeta report

# For ramp-up, generate a target file and use a script:
# vegeta doesn't natively support ramp-up; use k6 for complex scenarios
```

### Analyzing Results

```bash
# Full text report
vegeta report < results.bin

# Example output:
# Requests      [total, rate, throughput] 3000, 100.00, 99.50/s
# Duration      [total, attack, wait]     30.005s, 30.000s, 4.567ms
# Latencies     [min, mean, 50, 90, 95, 99, max] 1.2ms, 4.5ms, 3.1ms, 8.2ms, 12.1ms, 45.6ms, 234ms
# Bytes In      [total, mean]             450000, 150.00
# Bytes Out     [total, mean]             0, 0.00
# Success       [ratio]                   99.80%
# Status Codes  [code:count]              200:2994 500:6
# Error Set:
# 500 Internal Server Error

# Parse JSON report for programmatic use
vegeta report -type=json < results.bin | jq '{
  requests: .requests,
  success_ratio: .success,
  latency_p95: .latencies.p95,
  latency_p99: .latencies.p99,
  status_codes: .status_codes
}'
```

## CI/CD Integration

```bash
#!/bin/bash
# load-test.sh — run in CI after deployment
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PASS_P95_MS="${PASS_P95_MS:-500}"

# Run k6 test
k6 run \
  --env BASE_URL="$BASE_URL" \
  --env API_TOKEN="$API_TOKEN" \
  --out json=results.json \
  load-test.js

# Check thresholds passed
if [ $? -ne 0 ]; then
  echo "Load test FAILED: thresholds not met"
  exit 1
fi

echo "Load test PASSED"
```

```yaml
# GitHub Actions
- name: Run load tests
  run: |
    k6 run \
      --env BASE_URL=${{ secrets.STAGING_URL }} \
      --env API_TOKEN=${{ secrets.API_TOKEN }} \
      load-test.js
  if: github.ref == 'refs/heads/main'
```

## Interpreting Results

### Key Metrics to Monitor

| Metric | Target | Warning | Critical |
|--------|--------|---------|----------|
| p50 latency | <100ms | >200ms | >500ms |
| p95 latency | <500ms | >1s | >2s |
| p99 latency | <1s | >2s | >5s |
| Error rate | <0.1% | >1% | >5% |
| Throughput | >target RPS | <80% target | <50% target |

```bash
# Parse k6 JSON output for key metrics
jq -r '
  select(.type == "Point" and .metric == "http_req_duration") |
  .data.tags.percentile as $p |
  select($p == "p(95)") |
  "\(.data.time) p95=\(.data.value)ms"
' results.json | tail -20

# Check if thresholds were met
jq '.root_group.checks | to_entries[] | {name: .key, passes: .value.passes, fails: .value.fails}' results.json
```

## Troubleshooting

```bash
# k6: increase file descriptor limit for high VU counts
ulimit -n 65535

# k6: debug HTTP requests
k6 run --http-debug=full script.js

# vegeta: verbose output
vegeta attack -rate=10 -duration=5s -targets=targets.txt 2>&1

# Common issues:
# Connection refused — ensure the target server is running
# Too many open files — increase ulimit before testing
# Results plateau — server is at capacity; this is expected behavior
# High latency variance — network or GC pauses; run from same region as target
```
