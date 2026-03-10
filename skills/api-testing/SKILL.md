---
name: api-testing
description: "Test REST APIs with curl and jq: send requests, validate responses, check status codes, headers, and JSON body. Trigger: when testing REST APIs, API testing with curl, validating HTTP endpoints, testing JSON responses, API smoke tests"
version: 1
argument-hint: "[base-url]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# API Testing (curl)

You are now operating in API testing mode.

## Basic Request Patterns

```bash
# GET request
curl -s http://localhost:8080/api/v1/users | jq .

# POST with JSON body
curl -s -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}' | jq .

# PUT (full replace)
curl -s -X PUT http://localhost:8080/api/v1/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Updated","email":"alice@example.com"}' | jq .

# PATCH (partial update)
curl -s -X PATCH http://localhost:8080/api/v1/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Patched"}' | jq .

# DELETE
curl -s -X DELETE http://localhost:8080/api/v1/users/1
```

## Status Code Validation

```bash
# Extract status code
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
echo "Status: $STATUS"

# Assert status equals 200
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
[ "$STATUS" -eq 200 ] && echo "PASS" || echo "FAIL: expected 200, got $STATUS"

# Assert status is 2xx
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/v1/users)
[ "$STATUS" -ge 200 ] && [ "$STATUS" -lt 300 ] && echo "PASS" || echo "FAIL: $STATUS"

# Capture both body and status code
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/users)
BODY=$(echo "$RESPONSE" | head -n -1)
STATUS=$(echo "$RESPONSE" | tail -n 1)
echo "Status: $STATUS"
echo "$BODY" | jq .
```

## Response Body Validation

```bash
# Check JSON field value
NAME=$(curl -s http://localhost:8080/api/v1/users/1 | jq -r '.name')
[ "$NAME" = "Alice" ] && echo "PASS" || echo "FAIL: name=$NAME"

# Check array length
COUNT=$(curl -s http://localhost:8080/api/v1/users | jq '.users | length')
[ "$COUNT" -gt 0 ] && echo "PASS: $COUNT users" || echo "FAIL: empty response"

# Validate JSON schema (field presence)
RESPONSE=$(curl -s http://localhost:8080/api/v1/users/1)
echo "$RESPONSE" | jq -e '.id and .name and .email' > /dev/null \
  && echo "PASS: schema valid" || echo "FAIL: missing required fields"
```

## Authentication

```bash
# Bearer token
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/me | jq .

# Basic auth
curl -s -u username:password http://localhost:8080/api/v1/protected | jq .

# API key header
curl -s -H "X-API-Key: $API_KEY" http://localhost:8080/api/v1/data | jq .

# Set token from login response
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret"}' | jq -r '.token')
echo "Token obtained: ${TOKEN:0:20}..."
```

## Response Time Measurement

```bash
# Measure response time
TIME=$(curl -s -o /dev/null -w "%{time_total}" http://localhost:8080/api/v1/users)
echo "Response time: ${TIME}s"

# Detailed timing breakdown
curl -s -o /dev/null -w "
  DNS lookup:   %{time_namelookup}s
  Connect:      %{time_connect}s
  TLS handshake:%{time_appconnect}s
  First byte:   %{time_starttransfer}s
  Total:        %{time_total}s
" http://localhost:8080/api/v1/users
```

## Response Headers

```bash
# Show response headers
curl -s -I http://localhost:8080/api/v1/users

# Extract specific header
CONTENT_TYPE=$(curl -s -I http://localhost:8080/api/v1/users | grep -i 'content-type' | awk '{print $2}')
echo "Content-Type: $CONTENT_TYPE"

# Assert Content-Type is JSON
curl -s -I http://localhost:8080/api/v1/users | grep -i 'content-type' | grep -q 'application/json' \
  && echo "PASS" || echo "FAIL: not JSON content type"
```

## Error Path Testing

```bash
# 404 Not Found
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/v1/users/99999)
[ "$STATUS" -eq 404 ] && echo "PASS" || echo "FAIL: expected 404, got $STATUS"

# 400 Bad Request (invalid body)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"invalid": true}')
[ "$STATUS" -eq 400 ] && echo "PASS" || echo "FAIL: expected 400, got $STATUS"

# 401 Unauthorized (missing token)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/v1/me)
[ "$STATUS" -eq 401 ] && echo "PASS" || echo "FAIL: expected 401, got $STATUS"
```

## Test Workflow

```bash
# 1. Health check
echo "=== Health Check ==="
curl -s -f http://localhost:8080/health && echo "OK" || { echo "FAIL"; exit 1; }

# 2. Create resource
echo "=== Create User ==="
CREATED=$(curl -s -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Test User","email":"test@example.com"}')
ID=$(echo "$CREATED" | jq -r '.id')
echo "Created user ID: $ID"

# 3. Read it back
echo "=== Read User ==="
FETCHED=$(curl -s "http://localhost:8080/api/v1/users/$ID")
echo "$FETCHED" | jq -e '.id == '"$ID"'' > /dev/null && echo "PASS" || echo "FAIL"

# 4. Delete it
echo "=== Delete User ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "http://localhost:8080/api/v1/users/$ID")
[ "$STATUS" -eq 204 ] && echo "PASS" || echo "FAIL: $STATUS"
```

## Debugging

```bash
# Verbose output (headers + body)
curl -v http://localhost:8080/api/v1/users 2>&1

# Show request that was sent
curl --trace-ascii - http://localhost:8080/api/v1/users 2>&1 | head -50

# Silence progress, show errors
curl -sS http://localhost:8080/api/v1/users | jq .
```

## Security Notes

- Never log bearer tokens or API keys — use `${TOKEN:0:8}...` for partial display.
- Do not hardcode credentials in scripts — use environment variables.
- Use `-k` / `--insecure` only in local dev, never in CI against staging/production.
