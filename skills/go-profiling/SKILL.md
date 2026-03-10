---
name: go-profiling
description: "Profile Go applications with pprof: CPU, memory, goroutine, block, and mutex profiles. Trigger: when profiling Go applications, memory leaks, high CPU usage, goroutine leaks, pprof, performance analysis, flame graphs"
version: 1
argument-hint: "[profile-type] (cpu|heap|goroutine|block|mutex|trace)"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Go Profiling (pprof)

You are now operating in Go profiling mode.

## Enable pprof in Your Application

Add this import to your main package (net/http/pprof registers handlers automatically):

```go
import _ "net/http/pprof"

// If your app doesn't have an HTTP server, add one:
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

## Profile Types

| Profile | URL Path | What it Measures | When to Use |
|---------|----------|-----------------|-------------|
| `heap` | `/debug/pprof/heap` | Memory allocations | Memory leaks, high RSS |
| `profile` | `/debug/pprof/profile` | CPU usage | Slow requests, high CPU |
| `goroutine` | `/debug/pprof/goroutine` | Goroutine stacks | Goroutine leaks, deadlocks |
| `block` | `/debug/pprof/block` | Blocking operations | Channel/mutex contention |
| `mutex` | `/debug/pprof/mutex` | Mutex contention | Lock contention |
| `allocs` | `/debug/pprof/allocs` | All memory allocations | Allocation churn |
| `trace` | `/debug/pprof/trace` | Execution trace | Latency analysis |

## Capture Profiles

```bash
# Heap (memory) profile
go tool pprof http://localhost:6060/debug/pprof/heap

# CPU profile (30 second sample)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Goroutine dump
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Block profile (enable with runtime.SetBlockProfileRate first)
go tool pprof http://localhost:6060/debug/pprof/block

# Mutex profile (enable with runtime.SetMutexProfileFraction first)
go tool pprof http://localhost:6060/debug/pprof/mutex

# Save profile to file for later analysis
curl -s http://localhost:6060/debug/pprof/heap > heap.pprof
curl -s "http://localhost:6060/debug/pprof/profile?seconds=30" > cpu.pprof
```

## Analyze Profiles

```bash
# Interactive CLI (after go tool pprof opens)
# top — show top functions by resource usage
# web — open browser visualization (requires graphviz)
# list <func> — show source-level annotation
# peek <func> — show callers/callees

# Text summary (non-interactive)
go tool pprof -top heap.pprof
go tool pprof -top -nodecount=20 cpu.pprof

# Generate SVG flame graph (requires graphviz)
go tool pprof -svg heap.pprof > heap.svg

# Open in browser (requires graphviz)
go tool pprof -web heap.pprof

# HTTP UI (browse all profiles interactively)
go tool pprof -http=:8081 heap.pprof
```

## Enable Block and Mutex Profiling

Block and mutex profiles require runtime configuration before they collect data:

```go
import "runtime"

// Enable block profiling (1 = sample every blocking event)
runtime.SetBlockProfileRate(1)

// Enable mutex profiling (1 = sample every mutex contention event)
runtime.SetMutexProfileFraction(1)
```

## Goroutine Stack Dump (Text)

```bash
# Full goroutine stacks as text (debug=2 for full stacks)
curl http://localhost:6060/debug/pprof/goroutine?debug=2

# Count by goroutine state
curl -s http://localhost:6060/debug/pprof/goroutine?debug=1 | grep "^goroutine"
```

## Execution Trace

```bash
# Capture 5-second trace
curl -s "http://localhost:6060/debug/pprof/trace?seconds=5" > trace.out

# Analyze trace in browser
go tool trace trace.out
```

## Benchmark Profiling

```bash
# Profile during benchmarks
go test -bench=BenchmarkFoo -cpuprofile cpu.pprof -memprofile mem.pprof ./...

# Analyze benchmark profile
go tool pprof cpu.pprof
go tool pprof mem.pprof
```

## Reading Top Output

```
(pprof) top10
Showing nodes accounting for 42.31s, 91.30% of 46.34s total
      flat  flat%   sum%        cum   cum%
    15.32s 33.06% 33.06%     15.32s 33.06%  runtime.mallocgc
     8.21s 17.72% 50.78%      8.21s 17.72%  runtime.scanobject
```

- `flat`: time spent in this function itself
- `cum`: cumulative time including callees
- High `flat` + low `cum` = the function itself is slow
- Low `flat` + high `cum` = this function calls slow functions

## Memory Leak Detection Pattern

```bash
# Take two heap snapshots separated by time
curl -s http://localhost:6060/debug/pprof/heap > heap1.pprof
sleep 60
curl -s http://localhost:6060/debug/pprof/heap > heap2.pprof

# Compare allocations
go tool pprof -base heap1.pprof heap2.pprof
# Then run: top — shows functions with growing allocation
```
