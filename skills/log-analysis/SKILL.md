---
name: log-analysis
description: "Search and analyze application logs: error frequency, grep/jq patterns, JSON structured logs, log tailing, anomaly detection. Trigger: when searching logs, analyzing log files, filtering log output, JSON log analysis, log patterns, jq logs"
version: 1
argument-hint: "[log-file or pattern]"
allowed-tools:
  - bash
  - read
  - grep
  - glob
---
# Log Analysis

You are now operating in log analysis mode.

## Search by Log Level

```bash
# Search for errors in JSON structured logs
grep '"level":"error"' app.log

# Case-insensitive level search
grep -i '"level":"error"' app.log

# Multiple levels
grep -E '"level":"(error|warn)"' app.log

# Using jq for structured JSON logs
jq 'select(.level == "error")' app.log

# Tail with live level filter
tail -f app.log | grep '"level":"error"'
tail -f app.log | jq -c 'select(.level == "error")'
```

## Search by Time Range

```bash
# Logs from a specific hour (ISO8601 format)
grep '2026-03-10T14:' app.log

# Logs between two times using awk
awk '/2026-03-10T14:[0-9][0-9]/' app.log

# Last N minutes from a JSON log (requires timestamp field)
CUTOFF=$(date -u -v-30M '+%Y-%m-%dT%H:%M' 2>/dev/null || date -u --date='30 minutes ago' '+%Y-%m-%dT%H:%M')
jq --arg t "$CUTOFF" 'select(.time >= $t)' app.log
```

## Search by Pattern

```bash
# Common error patterns
grep -E 'timeout|connection refused|deadline exceeded' app.log

# Context lines (3 lines before/after match)
grep -C 3 'panic:' app.log

# Show line numbers
grep -n 'ERROR' app.log

# Search multiple files
grep -r '"level":"error"' ./logs/

# Count matches
grep -c '"level":"error"' app.log
```

## JSON Log Processing with jq

```bash
# Pretty-print last 100 error logs
tail -100 app.log | jq 'select(.level == "error")'

# Extract specific fields
jq '{time, level, msg, error}' app.log

# Filter and format
jq -r 'select(.level == "error") | "\(.time) \(.msg)"' app.log

# Count by level
jq -r '.level' app.log | sort | uniq -c | sort -rn

# Top error messages
jq -r 'select(.level == "error") | .msg' app.log | sort | uniq -c | sort -rn | head -20
```

## Error Frequency Analysis

```bash
# Count errors per minute
jq -r 'select(.level == "error") | .time[0:16]' app.log | sort | uniq -c

# Count errors per hour
jq -r 'select(.level == "error") | .time[0:13]' app.log | sort | uniq -c

# Errors per request path
jq -r 'select(.level == "error") | .path' app.log | sort | uniq -c | sort -rn | head -10
```

## Request Latency / Performance

```bash
# Extract response times (assuming duration_ms field)
jq 'select(.duration_ms != null) | .duration_ms' app.log | sort -n

# p50, p95, p99 approximation using awk
jq '.duration_ms' app.log | sort -n | awk '
BEGIN { n=0 }
{ a[n++]=$1 }
END {
  print "p50:", a[int(n*0.50)]
  print "p95:", a[int(n*0.95)]
  print "p99:", a[int(n*0.99)]
  print "max:", a[n-1]
}'

# Slow requests (over 1 second)
jq 'select(.duration_ms > 1000)' app.log | jq -r '"SLOW: \(.duration_ms)ms \(.path // "")"'
```

## Anomaly Detection

```bash
# Spike detection: compare error rate in last 5min vs previous 5min
NOW=$(date +%s)
FIVE_MIN_AGO=$(( NOW - 300 ))
TEN_MIN_AGO=$(( NOW - 600 ))

# Count errors in each window (requires epoch timestamp)
RECENT=$(jq --argjson t "$FIVE_MIN_AGO" 'select(.ts > $t and .level == "error")' app.log | wc -l)
PREVIOUS=$(jq --argjson t1 "$TEN_MIN_AGO" --argjson t2 "$FIVE_MIN_AGO" 'select(.ts > $t1 and .ts <= $t2 and .level == "error")' app.log | wc -l)
echo "Recent errors: $RECENT, Previous period: $PREVIOUS"
```

## Tailing and Live Monitoring

```bash
# Tail with JSON formatting
tail -f app.log | jq -c '.'

# Tail only errors with key fields
tail -f app.log | jq -c 'select(.level == "error") | {time, msg, error}'

# Tail with grep for multiple patterns
tail -f app.log | grep -E 'error|panic|fatal'

# systemd journal (if using systemd)
journalctl -u harnessd -f
journalctl -u harnessd --since "10 minutes ago" | grep -i error
```

## Correlation: Trace IDs

```bash
# Follow a specific request by trace ID
grep '"trace_id":"abc123"' app.log

# With jq
jq 'select(.trace_id == "abc123")' app.log | jq -r '"\(.time) \(.level) \(.msg)"'
```

## Common Log Formats

```bash
# Apache/Nginx combined log format
awk '{print $9}' access.log | sort | uniq -c | sort -rn | head  # Status codes

# Plain text (level prefix)
grep '^ERROR' app.log
grep -E '^(ERROR|WARN)' app.log

# Go slog default output
grep 'level=ERROR' app.log
```
