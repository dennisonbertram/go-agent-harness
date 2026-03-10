# Scaling Sub-Agent Architecture: 1,000+ Concurrent Agents on Bare Metal

**Date**: 2026-03-09
**Status**: Research
**Scope**: Architecture analysis and scaling strategy for running 1,000+ concurrent LLM agent loops within a single go-agent-harness process on dedicated hardware.

---

## 1. Executive Summary

The go-agent-harness is architected as a single-process, event-driven HTTP server where each agent run executes as a goroutine. This design is inherently well-suited for scaling to 1,000+ concurrent agents within one process: Go's runtime scheduler handles millions of goroutines, shared in-process state avoids serialization overhead, and a single binary keeps deployment simple.

The key bottlenecks that must be addressed are:

1. **SQLite single-connection serialization** — both the conversation store and observational memory store use `SetMaxOpenConns(1)`, serializing all database access across all agents.
2. **OpenAI HTTP client transport limits** — a single `*http.Client` with default transport settings will exhaust idle connections under 1,000 concurrent streaming requests.
3. **API rate limits** — OpenAI TPM/RPM limits on a single key will be hit well before 1,000 concurrent agents are active.
4. **Runner mutex contention** — the single `sync.RWMutex` on `Runner` protects all run state; under 1,000 agents this becomes a hot lock.
5. **Tool registry allocation pressure** — `DefinitionsForRun` allocates and sorts tool slices on every LLM turn.
6. **Event subscriber fan-out** — `emit()` holds the Runner lock while collecting subscribers, creating lock contention proportional to event frequency times agent count.

All of these are solvable without architectural changes. The core model is correct: one process, many goroutines.

---

## 2. Current Architecture Analysis

### 2.1 Runner Struct

The `Runner` is the central orchestrator, defined at `internal/harness/runner.go:52-64`:

```go
type Runner struct {
    provider         Provider
    tools            *Registry
    config           RunnerConfig
    providerRegistry *catalog.ProviderRegistry
    activations      *ActivationTracker
    skillConstraints *SkillConstraintTracker

    mu            sync.RWMutex
    runs          map[string]*runState
    conversations map[string][]Message
    idSeq         uint64
}
```

Key observations:
- **Single `sync.RWMutex`** (`mu`) protects both `runs` and `conversations` maps. Every `setStatus`, `setMessages`, `emit`, `GetRun`, `Subscribe`, and `recordAccounting` call acquires this lock.
- **`idSeq`** uses `atomic.AddUint64` (`runner.go:1363-1366`), which is lock-free and scales well.
- **`provider`** is a single `Provider` instance shared across all runs.

### 2.2 runState Struct

Per-run state is defined at `runner.go:19-29`:

```go
type runState struct {
    run                Run
    staticSystemPrompt string
    promptResolved     *systemprompt.ResolvedPrompt
    usageTotals        usageTotalsAccumulator
    costTotals         RunCostTotals
    messages           []Message
    events             []Event
    subscribers        map[chan Event]struct{}
    nextEventSeq       uint64
}
```

Each `runState` is stored in the Runner's `runs` map. The `messages` slice grows with each LLM turn and tool call. The `events` slice grows with every emitted event (multiple per step). For a run with 8 steps and 3 tool calls per step, expect ~100+ events.

Critically, `runState` is **not independently locked** — all access goes through `Runner.mu`. This means updating run A's messages blocks reading run B's status.

### 2.3 Tool Registry

Defined at `internal/harness/registry.go:26-29`:

```go
type Registry struct {
    mu    sync.RWMutex
    tools map[string]registeredTool
}
```

`DefinitionsForRun` (`registry.go:104-121`) acquires `RLock`, iterates all tools, filters by tier and activation status, allocates a new `[]ToolDefinition` slice, and sorts it. This is called on every LLM turn for every run (`runner.go:492`).

`Execute` (`registry.go:70-78`) acquires `RLock`, looks up the tool, then releases the lock before calling the handler. The handler runs outside the lock, which is correct.

### 2.4 ActivationTracker and SkillConstraintTracker

Both use independent `sync.RWMutex` locks (`activation.go:11-14`, `skill_constraint.go:21-24`). They store per-run maps keyed by `runID`. These are well-isolated and will not be bottlenecks at 1,000 agents — their operations are O(1) map lookups.

### 2.5 Provider Layer (OpenAI Client)

Defined at `internal/provider/openai/client.go:29-36`:

```go
type Client struct {
    apiKey          string
    baseURL         string
    model           string
    client          *http.Client
    pricingResolver pricing.Resolver
    providerName    string
}
```

Construction at `client.go:38-66`:
- If no `*http.Client` is provided, creates one with `Timeout: 90 * time.Second` (`client.go:52`).
- Uses Go's default `http.Transport`, which has `MaxIdleConns: 100` and `MaxIdleConnsPerHost: 2` by default.
- The `Complete` method (`client.go:68-125`) creates an HTTP request per call and uses `c.client.Do(httpReq)`. For streaming, it holds the response body open while reading SSE chunks (`client.go:135-182`).

A single `*http.Client` is goroutine-safe, but the default transport's `MaxIdleConnsPerHost: 2` means only 2 idle connections are reused to `api.openai.com`. Under 1,000 concurrent streaming requests, this causes massive connection churn.

### 2.6 Server Layer

Defined at `internal/server/http.go:13-25`:

```go
func New(runner *harness.Runner) http.Handler {
    s := &Server{runner: runner}
    mux := http.NewServeMux()
    // ... routes ...
    return mux
}

type Server struct {
    runner *harness.Runner
}
```

The server holds a single `*harness.Runner` reference. All HTTP handlers call methods on this one Runner. The SSE handler (`http.go:184-248`) keeps an HTTP connection open per subscriber, reading from a `chan Event` with buffer size 64 (`runner.go:340`).

### 2.7 SQLite Conversation Store

Defined at `internal/harness/conversation_store_sqlite.go:41-43`:

```go
type SQLiteConversationStore struct {
    db *sql.DB
}
```

Construction at `conversation_store_sqlite.go:46-73`:
- Opens SQLite with `?_txlock=immediate` DSN parameter.
- **`db.SetMaxOpenConns(1)`** — line 59. This serializes ALL database operations through a single connection.
- Enables WAL mode (`PRAGMA journal_mode=WAL`), busy timeout of 5000ms, and foreign keys.

`SaveConversation` (`conversation_store_sqlite.go:93-146`) uses a transaction that deletes all existing messages and re-inserts them. Under 1,000 agents completing runs concurrently, this creates severe serialization at the single connection.

### 2.8 SQLite Observational Memory Store

Defined at `internal/observationalmemory/store_sqlite.go:67-69`:

```go
type SQLiteStore struct {
    db *sql.DB
}
```

Construction at `store_sqlite.go:71-91`:
- Opens SQLite with WAL mode and busy timeout of 5000ms.
- **Does NOT call `SetMaxOpenConns(1)`** — unlike the conversation store, this uses Go's default connection pool. However, since SQLite only supports one writer at a time (even in WAL mode), concurrent writes will still serialize.
- Operations like `GetOrCreateRecord`, `UpdateRecord`, `CreateOperation`, and `InsertMarker` are all write operations that will contend.

### 2.9 Startup Wiring (main.go)

`cmd/harnessd/main.go:78-426` (`runWithSignals` function) creates the full dependency graph:

1. Single `openai.Config` / provider (`main.go:216-224`)
2. Single `promptEngine` (`main.go:225-228`)
3. Single `memoryManager` with its own SQLite store (`main.go:230-251`)
4. Optional single `convStore` (SQLite, `main.go:319-335`)
5. Single `activations` tracker (`main.go:338`)
6. Single `tools` registry (`main.go:339-354`)
7. Single `runner` (`main.go:355-369`)
8. Single `server` handler wrapping the runner (`main.go:378`)
9. Single `http.Server` (`main.go:379-383`)

Everything is wired into one process with shared state. This is the correct foundation for scaling via goroutines.

---

## 3. Core Scaling Model

### 3.1 One Process, Many Goroutines — Not Many Processes

The correct scaling model for 1,000+ agents is: **one harness process running 1,000+ goroutines**, each executing an independent `Runner.execute()` loop.

This is already how the system works. `StartRun` (`runner.go:110-168`) launches `go r.execute(run.ID, req)` at line 166. Each goroutine runs the full step loop (LLM call, tool execution, repeat) independently.

### 3.2 Why This Model Works

**Go's goroutine scheduler handles massive concurrency trivially.** The Go runtime multiplexes goroutines onto OS threads (GOMAXPROCS, typically equal to CPU cores). A goroutine costs ~2-8KB of stack initially, growing as needed. 1,000 goroutines consume ~2-8MB of stack space total — negligible. Go applications routinely run 100,000+ goroutines in production.

**Shared in-process state avoids serialization overhead.** Agent runs share the tool registry, provider, prompt engine, and memory manager through direct pointer references. There is no serialization/deserialization, no IPC, no network hops between components. A tool registry lookup is a map read behind an RLock — nanoseconds, not milliseconds.

**Single binary deployment simplicity.** One process means one deployment artifact, one health check endpoint, one set of metrics, one log stream. Contrast with process-per-agent (1,000 processes, 1,000 health checks, 1,000 log streams) or container-per-agent models.

**Resource efficiency.** Process-per-agent duplicates the Go runtime, tool registry, prompt engine, and provider client 1,000 times. At ~50MB base memory per Go process, that's 50GB just for runtime overhead. A single process with 1,000 goroutines uses ~200MB for the same agent capacity.

### 3.3 Scaling Characteristics

Most of an agent's wall-clock time is spent waiting:
- Waiting for the LLM API response (seconds per turn)
- Waiting for tool execution (varies: bash commands, file I/O)
- Waiting for user input (minutes, if `ask_user` tool is used)

During these waits, the goroutine is parked and consumes zero CPU. This means 1,000 agents on a 16-core machine will rarely saturate CPU — the bottleneck is I/O (network to LLM API, disk for SQLite).

---

## 4. Bottleneck Analysis

### 4.1 SQLite Conversation Store — Single Connection Serialization

**Current code**: `conversation_store_sqlite.go:59`
```go
db.SetMaxOpenConns(1)
```

**Why it's a bottleneck**: `SaveConversation` is called at `runner.go:880` when a run completes. `LoadMessages` is called at `runner.go:1206` when a run starts with a conversation ID. With 1,000 agents, completions and starts overlap. `SetMaxOpenConns(1)` means only one goroutine can hold a database connection at a time. All others block in `sql.DB`'s connection pool waiting for the single connection.

Under 1,000 agents with 8 steps each, and runs completing at ~1-2 per second at steady state, the single connection becomes a chokepoint. `SaveConversation` executes a transaction with DELETE + N INSERTs — each taking 5-50ms depending on message count. At 10ms average, this serializes to 100 completions/second maximum.

**Proposed fix**: Enable WAL mode (already done) and increase connection count. SQLite WAL mode allows concurrent readers with a single writer. Since `LoadMessages` is read-only, multiple agents can read simultaneously while writes serialize.

```go
// Phase 1: Increase connections with WAL mode
db.SetMaxOpenConns(4)  // 1 writer + 3 readers
db.SetMaxIdleConns(4)

// Phase 2: Per-tenant database sharding (if needed)
type ShardedConversationStore struct {
    shards []*sql.DB
    count  int
}

func (s *ShardedConversationStore) shard(convID string) *sql.DB {
    h := fnv.New32a()
    h.Write([]byte(convID))
    return s.shards[h.Sum32()%uint32(s.count)]
}
```

**Estimated impact**: 4x throughput improvement with connection increase. 10-20x with sharding across 10 database files.

### 4.2 SQLite Observational Memory Store — Write Serialization

**Current code**: `store_sqlite.go:71-91` — no `SetMaxOpenConns` call, uses Go defaults.

**Why it's a bottleneck**: The observational memory store performs writes on every step where memory is enabled: `GetOrCreateRecord` (INSERT on first call), `UpdateRecord` (UPDATE on every observe), `CreateOperation` (INSERT with transaction, `store_sqlite.go:198-254`), `InsertMarker` (INSERT). Even with Go's default connection pool, SQLite's write lock means only one goroutine writes at a time.

`observeMemory` is called at `runner.go:542` (no tool calls) and `runner.go:659` (after tool calls). With 1,000 agents at 8 steps each, that's 8,000 observe operations during a batch. Each `CreateOperation` uses a transaction (`store_sqlite.go:199-253`) that reads `MAX(scope_sequence)` and inserts — serialized.

**Proposed fix**: Similar to conversation store. Additionally, batch observations:

```go
// Buffer observations and flush in batches
type BatchedMemoryStore struct {
    inner   Store
    pending chan observeOp
    done    chan struct{}
}

func (s *BatchedMemoryStore) Start() {
    go func() {
        defer close(s.done)
        batch := make([]observeOp, 0, 100)
        ticker := time.NewTicker(100 * time.Millisecond)
        for {
            select {
            case op := <-s.pending:
                batch = append(batch, op)
                if len(batch) >= 100 {
                    s.flush(batch)
                    batch = batch[:0]
                }
            case <-ticker.C:
                if len(batch) > 0 {
                    s.flush(batch)
                    batch = batch[:0]
                }
            }
        }
    }()
}
```

**Estimated impact**: Batching reduces SQLite transactions from N to N/100. Combined with WAL mode tuning, 20-50x throughput improvement for memory writes.

### 4.3 OpenAI Provider HTTP Client — Transport Limits

**Current code**: `client.go:51-53`
```go
httpClient = &http.Client{Timeout: 90 * time.Second}
```

This uses Go's `http.DefaultTransport`, which has:
- `MaxIdleConns: 100`
- `MaxIdleConnsPerHost: 2`
- `MaxConnsPerHost: 0` (unlimited)
- `IdleConnTimeout: 90 * time.Second`

**Why it's a bottleneck**: With 1,000 agents making concurrent streaming requests to `api.openai.com`, `MaxIdleConnsPerHost: 2` means only 2 connections are reused from the idle pool. The remaining 998 connections are created fresh for each request, causing:
1. TCP+TLS handshake overhead (~100-300ms per connection)
2. File descriptor exhaustion (each connection = 1 fd, 1,000 concurrent = 1,000 fds)
3. Port exhaustion in TIME_WAIT state after connections close

Streaming responses (`decodeStreamingResponse`, `client.go:135-182`) hold connections open for seconds to minutes. Under 1,000 concurrent streaming requests, the transport creates ~1,000 simultaneous TCP connections but only caches 2 when they close.

**Proposed fix**: Configure a custom transport:

```go
transport := &http.Transport{
    MaxIdleConns:        2000,
    MaxIdleConnsPerHost: 1500,  // Most connections go to one host
    MaxConnsPerHost:     1500,
    IdleConnTimeout:     120 * time.Second,
    TLSHandshakeTimeout: 15 * time.Second,
    ResponseHeaderTimeout: 90 * time.Second,
    WriteBufferSize: 64 * 1024,
    ReadBufferSize:  64 * 1024,
}

httpClient := &http.Client{
    Timeout:   0,  // No overall timeout; use context deadlines per request
    Transport: transport,
}
```

Also raise the process file descriptor limit:
```bash
ulimit -n 65536  # or set in systemd unit
```

**Estimated impact**: Eliminates connection churn. Reduces p99 latency for LLM requests by 100-300ms. Prevents fd/port exhaustion failures.

### 4.4 Tool Registry — DefinitionsForRun Allocation Pressure

**Current code**: `registry.go:104-121`
```go
func (r *Registry) DefinitionsForRun(runID string, tracker ActivationTrackerInterface) []ToolDefinition {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var defs []ToolDefinition
    for _, rt := range r.tools {
        // ... filter ...
        defs = append(defs, rt.def)
    }
    sort.Slice(defs, func(i, j int) bool {
        return defs[i].Name < defs[j].Name
    })
    return defs
}
```

Called at `runner.go:492` via `filteredToolsForRun`:
```go
Tools: r.filteredToolsForRun(runID),
```

**Why it's a bottleneck**: With ~30 tools registered, each call allocates a `[]ToolDefinition` and sorts it. At 1,000 agents with 8 steps each, that's 8,000 allocations of ~30-element slices per batch. The `RLock` is held during the iteration and sort — under high concurrency, this lock is held by many readers simultaneously (which is fine for RWMutex), but the allocation pressure creates GC load.

The bigger concern is `filteredToolsForRun` (`runner.go:798-821`) which allocates `allowed` map and `filtered` slice on top of `DefinitionsForRun`'s allocation.

**Proposed fix**: Cache the core tool definitions (they don't change after startup) and only compute the delta for deferred/skill-constrained runs:

```go
type Registry struct {
    mu          sync.RWMutex
    tools       map[string]registeredTool
    // Cached sorted definitions for core tools (computed once)
    cachedCore  []ToolDefinition
    cacheValid  bool
}

func (r *Registry) DefinitionsForRun(runID string, tracker ActivationTrackerInterface) []ToolDefinition {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if !r.hasDeferredActive(runID, tracker) && r.cacheValid {
        return r.cachedCore  // Zero allocation for common case
    }
    // Fall through to current logic for runs with deferred tools
    // ...
}
```

**Estimated impact**: Eliminates ~90% of allocations in the common case (no deferred tools active). Reduces GC pressure proportionally.

### 4.5 Runner Mutex — Shared State Serialization

**Current code**: `runner.go:60`
```go
mu sync.RWMutex
```

This single mutex protects:
- `runs` map — read by `GetRun`, `Subscribe`, `PendingInput`, `SubmitInput`; written by `StartRun`
- `conversations` map — read by `loadConversationHistory`, `ConversationMessages`; written by `completeRun`
- Per-run `runState` fields — accessed by `setStatus`, `setMessages`, `emit`, `recordAccounting`, `promptContext`, `scopeKey`, `runMetadata`, `transcriptSnapshot`, `accountingTotals`

**Why it's a bottleneck**: `emit` is called frequently (every event, multiple times per step) and acquires a write lock at `runner.go:1306`. `setMessages` acquires a write lock at `runner.go:1097`. `recordAccounting` acquires a write lock at `runner.go:921`. These write locks block all concurrent readers.

With 1,000 agents, each emitting ~10 events per step across 8 steps, that's 80,000 lock acquisitions per batch, many of which are write locks. The lock contention becomes the throughput ceiling for event processing.

**Proposed fix**: Per-run locking by adding a mutex to `runState`:

```go
type runState struct {
    mu             sync.RWMutex  // Per-run lock, independent of Runner.mu
    run            Run
    // ... same fields ...
}

// Runner.mu only protects the runs map itself, not individual run states
func (r *Runner) emit(runID string, eventType EventType, payload map[string]any) {
    r.mu.RLock()
    state, ok := r.runs[runID]
    r.mu.RUnlock()  // Release Runner lock immediately
    if !ok {
        return
    }

    state.mu.Lock()  // Per-run lock
    // ... event creation and subscriber collection ...
    state.mu.Unlock()

    // Fan out to subscribers outside any lock
    for _, ch := range subscribers {
        select {
        case ch <- event:
        default:
        }
    }
}
```

**Estimated impact**: Eliminates cross-run lock contention. Run A's events don't block Run B's status reads. Scales linearly with agent count.

### 4.6 API Rate Limits — External Hard Constraint

**Current code**: `client.go:68-125` — single API key per Client, configured at `main.go:149`:
```go
apiKey := getenv("OPENAI_API_KEY")
```

**Why it's a bottleneck**: OpenAI enforces per-key rate limits:
- Tier 5: 10,000 RPM, 30,000,000 TPM for GPT-4.1-mini
- Each agent step = 1 request. 1,000 agents at 8 steps = 8,000 requests per batch.
- At steady state with staggered starts, ~100-200 RPM per agent, total 100,000-200,000 RPM — far exceeding limits.

This is the hardest bottleneck because it's external and cannot be optimized away. The solution is API key multiplexing.

**Proposed fix**: See section 5 for detailed API Key Round-Robin design.

**Estimated impact**: Linear scaling with number of API keys. 10 keys = 10x the rate limit.

---

## 5. API Key Round-Robin Design

### 5.1 ProviderPool Abstraction

```go
// ProviderPool wraps multiple Provider instances and distributes
// requests across them using round-robin with rate-limit awareness.
type ProviderPool struct {
    mu        sync.Mutex
    providers []providerEntry
    next      uint64  // atomic round-robin counter
}

type providerEntry struct {
    provider   Provider
    keyName    string
    rateLimits rateLimitState
}

type rateLimitState struct {
    mu              sync.Mutex
    requestsUsed    int
    tokensUsed      int
    windowResetAt   time.Time
    exhaustedUntil  time.Time  // Set when 429 received
    remainingReqs   int        // From x-ratelimit-remaining-requests header
    remainingTokens int        // From x-ratelimit-remaining-tokens header
}
```

### 5.2 Selection Strategy

```go
func (p *ProviderPool) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
    maxAttempts := len(p.providers)
    startIdx := atomic.AddUint64(&p.next, 1) % uint64(len(p.providers))

    for attempt := 0; attempt < maxAttempts; attempt++ {
        idx := (startIdx + uint64(attempt)) % uint64(len(p.providers))
        entry := &p.providers[idx]

        if entry.rateLimits.isExhausted() {
            continue  // Skip exhausted keys
        }

        result, err := entry.provider.Complete(ctx, req)
        if err != nil {
            if isRateLimitError(err) {
                entry.rateLimits.markExhausted(parseRetryAfter(err))
                continue  // Try next key
            }
            return result, err  // Non-rate-limit error, propagate
        }

        entry.rateLimits.recordUsage(result.Usage)
        return result, nil
    }

    // All keys exhausted — find earliest reset time and wait
    waitDuration := p.earliestReset()
    return CompletionResult{}, fmt.Errorf("all %d API keys rate-limited; earliest reset in %v", len(p.providers), waitDuration)
}
```

### 5.3 Configuration

```go
// Environment: OPENAI_API_KEYS=key1,key2,key3 (comma-separated)
// Falls back to OPENAI_API_KEY for single-key backwards compatibility
func newProviderPool(getenv func(string) string, newProvider providerFactory, pricingResolver pricing.Resolver) (*ProviderPool, error) {
    keys := strings.Split(getenv("OPENAI_API_KEYS"), ",")
    if len(keys) == 0 || (len(keys) == 1 && keys[0] == "") {
        // Fall back to single key
        key := getenv("OPENAI_API_KEY")
        if key == "" {
            return nil, fmt.Errorf("OPENAI_API_KEY or OPENAI_API_KEYS required")
        }
        keys = []string{key}
    }

    entries := make([]providerEntry, 0, len(keys))
    for i, key := range keys {
        key = strings.TrimSpace(key)
        if key == "" {
            continue
        }
        provider, err := newProvider(openai.Config{
            APIKey:          key,
            PricingResolver: pricingResolver,
        })
        if err != nil {
            return nil, fmt.Errorf("create provider for key %d: %w", i, err)
        }
        entries = append(entries, providerEntry{
            provider: provider,
            keyName:  fmt.Sprintf("key_%d", i),
        })
    }

    return &ProviderPool{providers: entries}, nil
}
```

### 5.4 Integration with Existing Provider Interface

`ProviderPool` implements `harness.Provider` (`types.go:135-137`), making it a drop-in replacement:

```go
// ProviderPool implements harness.Provider
var _ harness.Provider = (*ProviderPool)(nil)
```

In `main.go`, replace the single provider construction with:

```go
// Before (single key):
provider, err := newProvider(openai.Config{APIKey: apiKey, ...})

// After (pool):
pool, err := newProviderPool(getenv, newProvider, pricingResolver)
runner := harness.NewRunner(pool, tools, config)
```

The Runner, Server, and all other components are unaware of the pool — they call `Complete()` as before.

---

## 6. Hardware Sizing

### 6.1 Resource Profile Per Agent

| Agent State | RAM | CPU | Network | Duration |
|---|---|---|---|---|
| **Idle** (waiting for input, parked goroutine) | ~2-4 MB | ~0% | 0 | Minutes to hours |
| **LLM streaming** (waiting for API response) | ~10-30 MB (context + message history) | <1% (SSE parsing) | ~100 KB/s downstream per agent | 2-30s per turn |
| **Tool executing** (bash, file I/O) | ~5-50 MB (depends on tool output) | 5-100% of one core briefly | Varies | 0.1-30s per call |
| **Memory observe** (SQLite write) | ~5 MB (serialized observations) | <1% | 0 | 10-100ms |

### 6.2 Memory Estimation for 1,000 Agents

| Component | Per Agent | 1,000 Agents |
|---|---|---|
| Goroutine stack | 8 KB initial, 64 KB avg | 64 MB |
| runState struct | ~1 KB base | 1 MB |
| Message history (8 steps, ~4KB/msg avg) | ~50 KB | 50 MB |
| Event history (~100 events, ~500B each) | ~50 KB | 50 MB |
| LLM request payload (context + tools) | ~100-500 KB | 100-500 MB |
| HTTP response buffers (streaming) | ~64 KB scanner buffer | 64 MB |
| Tool execution overhead | ~10 MB avg | 10 GB peak (unlikely all active simultaneously) |
| **Go runtime overhead** | — | ~100 MB |
| **Total estimated** | — | **1-2 GB typical, 4 GB peak** |

Adding safety margin and OS overhead: **8 GB RAM is sufficient for 1,000 agents**. 16 GB provides comfortable headroom.

### 6.3 CPU Estimation

Most agent time is spent in I/O wait (LLM API calls). CPU-intensive phases:
- JSON marshaling/unmarshaling of LLM requests/responses
- Tool execution (bash commands, file operations)
- SQLite transactions

At 1,000 agents with staggered execution, expect ~50-100 agents actively using CPU at any moment. Each CPU-bound operation is brief (~1-10ms). **8 cores handle this comfortably.** 16 cores provide headroom for tool-heavy workloads.

### 6.4 Network Bandwidth

Each LLM request sends ~10-100 KB (context + tool definitions) and receives ~1-50 KB (response + streaming). At 1,000 agents with 8 steps each, during a burst:
- Upstream: 1,000 * 50KB = 50 MB burst
- Downstream (streaming, sustained): ~100 MB/s peak during concurrent streaming

**1 Gbps NIC is sufficient.** OpenAI's API response times (not bandwidth) will be the bottleneck.

### 6.5 Disk I/O

SQLite writes are the primary disk load. With the fixes in section 4.1-4.2:
- Conversation saves: ~10-50 IOPS sustained
- Memory observations: ~50-200 IOPS sustained
- Total: ~300 IOPS sustained

**NVMe SSD required.** Any modern NVMe provides 50,000+ IOPS, making disk I/O negligible.

### 6.6 Recommended Hetzner Configurations

| Config | Hetzner Model | CPU | RAM | Disk | Network | Monthly Cost (approx) | Agent Capacity |
|---|---|---|---|---|---|---|---|
| **Starter** | AX42 | AMD Ryzen 7 7700 (8c/16t) | 64 GB DDR5 | 2x 512 GB NVMe | 1 Gbps | ~€52/mo | 1,000-2,000 agents |
| **Recommended** | AX52 | AMD Ryzen 9 7900 (12c/24t) | 64 GB DDR5 | 2x 1 TB NVMe | 1 Gbps | ~€72/mo | 2,000-5,000 agents |
| **High Density** | AX102 | AMD Ryzen 9 7950X3D (16c/32t) | 128 GB DDR5 | 2x 1 TB NVMe | 1 Gbps | ~€130/mo | 5,000-10,000 agents |

The Starter configuration (AX42) is sufficient for 1,000 agents with significant headroom. The limiting factor at 1,000 agents is API rate limits, not hardware. Hardware becomes the bottleneck only above 5,000 agents with multiple API keys.

### 6.7 Cost Analysis

For 1,000 concurrent agents on AX42:
- **Server**: ~€52/mo (~$56/mo)
- **API costs** (the real expense): 1,000 agents × 8 steps × ~$0.001/step (GPT-4.1-mini) = $8/batch. At 10 batches/day = $80/day = ~$2,400/mo
- **Ratio**: API costs are ~43x server costs. Hardware is negligible.

---

## 7. Implementation Phases

### Phase 1: Database Layer (Connection Pooling + WAL Tuning)

**Scope**: Modify `SQLiteConversationStore` and `SQLiteStore` to support higher concurrency.

**Changes**:
1. Increase `SetMaxOpenConns` from 1 to 4 in conversation store (`conversation_store_sqlite.go:59`)
2. Add `SetMaxIdleConns(4)` to match
3. Add `PRAGMA synchronous=NORMAL` for WAL mode (safe with WAL, 2x write speed)
4. Add `PRAGMA cache_size=-16000` (16MB cache) for read performance
5. Consider separating conversation store and memory store into distinct database files to eliminate cross-system write contention

**Effort**: 1-2 days
**Dependencies**: None
**Risk**: Low — WAL mode already enabled, increasing connections is safe with WAL

### Phase 2: Provider Pool (Multi-Key Support)

**Scope**: Implement `ProviderPool` as described in section 5.

**Changes**:
1. Create `internal/provider/pool.go` with `ProviderPool` struct
2. Add `OPENAI_API_KEYS` environment variable parsing in `main.go`
3. Wire pool into Runner construction
4. Add rate limit header parsing in OpenAI client
5. Add metrics/logging for key rotation events

**Effort**: 3-5 days
**Dependencies**: None (can be done in parallel with Phase 1)
**Risk**: Medium — rate limit behavior varies by OpenAI tier; need production testing

### Phase 3: HTTP Transport Tuning

**Scope**: Configure custom `http.Transport` for high-concurrency streaming.

**Changes**:
1. Create custom transport in `openai.Config` with tuned pool sizes (`client.go:51-53`)
2. Add `MaxIdleConnsPerHost`, `MaxConnsPerHost` configuration
3. Set process-level `ulimit -n` in deployment scripts
4. Add connection pool metrics (active connections, idle connections)

**Effort**: 1-2 days
**Dependencies**: None
**Risk**: Low — straightforward configuration change

### Phase 4: Per-Agent Resource Limits and Backpressure

**Scope**: Add per-run locking to Runner and implement backpressure for run starts.

**Changes**:
1. Add `sync.RWMutex` to `runState` struct (`runner.go:19`)
2. Refactor `emit`, `setStatus`, `setMessages`, `recordAccounting` to use per-run locks
3. Add a semaphore to `StartRun` to limit concurrent active runs (similar to `cron.Scheduler.sem` at `scheduler.go:20`)
4. Add configurable `MaxConcurrentRuns` to `RunnerConfig`
5. Return `429 Too Many Requests` from HTTP layer when at capacity

```go
type Runner struct {
    // ...
    runSem chan struct{}  // Limits concurrent execute goroutines
}

func (r *Runner) StartRun(req RunRequest) (Run, error) {
    select {
    case r.runSem <- struct{}{}:
    default:
        return Run{}, ErrAtCapacity
    }
    // ... start run, release semaphore in execute() defer
}
```

**Effort**: 3-5 days
**Dependencies**: Phase 1 (database must handle concurrency first)
**Risk**: Medium — per-run locking refactor touches critical code paths; requires thorough testing with `-race`

### Phase 5: Observability and Metrics

**Scope**: Add metrics collection for scaling-relevant measurements.

**Changes**:
1. Add Prometheus metrics endpoint (`/metrics`)
2. Track: active runs, goroutine count, SQLite connection wait time, API key usage per key, event emit latency, tool execution duration, memory per run
3. Add structured logging with run context
4. Add pprof endpoints for CPU/memory profiling in production

**Effort**: 2-3 days
**Dependencies**: Phases 1-4 (metrics should measure the optimized system)
**Risk**: Low — additive, non-breaking

---

## 8. Verification Approach

### 8.1 Load Testing Framework

Build a load test harness that spawns N goroutine agents with a mock LLM provider:

```go
type MockProvider struct {
    latency time.Duration  // Simulated LLM response time
    tokens  int            // Simulated token count
}

func (m *MockProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
    time.Sleep(m.latency)  // Simulate network latency
    return CompletionResult{
        Content: "mock response",
        Usage:   &CompletionUsage{PromptTokens: m.tokens, CompletionTokens: 100},
    }, nil
}

func TestScale1000Agents(t *testing.T) {
    provider := &MockProvider{latency: 500 * time.Millisecond, tokens: 1000}
    runner := harness.NewRunner(provider, tools, config)

    var wg sync.WaitGroup
    start := time.Now()
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            _, err := runner.StartRun(harness.RunRequest{
                Prompt: fmt.Sprintf("agent %d task", n),
            })
            assert.NoError(t, err)
        }(i)
    }
    wg.Wait()
    elapsed := time.Since(start)
    t.Logf("1000 agents completed in %v", elapsed)
}
```

### 8.2 Metrics to Track

| Metric | Target at 1,000 Agents | How to Measure |
|---|---|---|
| p99 StartRun latency | <10ms | Timer around `StartRun` |
| p99 event emit latency | <1ms | Timer around `emit` |
| p99 SQLite write latency | <50ms | Timer around `SaveConversation` |
| Memory per agent (steady state) | <5 MB | `runtime.MemStats` delta / N |
| Goroutine count | N + ~50 overhead | `runtime.NumGoroutine()` |
| SQLite connection wait time | <10ms p99 | `sql.DBStats.WaitDuration` |
| GC pause time | <5ms p99 | `runtime.MemStats.PauseNs` |
| Lock contention (Runner.mu) | <1% of wall time | `sync.Mutex` contention profiling via `pprof` |

### 8.3 Stress Test Scenarios

1. **Burst start**: Start 1,000 runs simultaneously. Measure time to all running.
2. **Sustained throughput**: Start 100 runs/second for 60 seconds (6,000 total with overlap). Measure p99 latency and memory growth.
3. **Conversation persistence storm**: 500 runs completing within 1 second. Measure SQLite contention.
4. **Memory observation under load**: 1,000 agents with memory enabled, all observing simultaneously. Measure write throughput.
5. **SSE subscriber fan-out**: 1,000 runs each with 2 SSE subscribers. Measure event delivery latency.
6. **Graceful shutdown under load**: Signal SIGTERM with 500 active runs. Measure drain time and data integrity.

### 8.4 Existing Semaphore Pattern Reference

The cron scheduler (`internal/cron/scheduler.go:16-24`) already implements the semaphore pattern for concurrency limiting:

```go
type Scheduler struct {
    // ...
    sem chan struct{}  // concurrency semaphore
    wg  sync.WaitGroup
}
```

At `scheduler.go:131-132`:
```go
s.sem <- struct{}{}
s.wg.Add(1)
```

With deferred release at `scheduler.go:136`:
```go
defer func() {
    <-s.sem
    s.wg.Done()
}()
```

This exact pattern should be adopted for `Runner.StartRun` to limit concurrent active agents. The `SchedulerConfig.MaxConcurrent` field (`scheduler.go:28`) provides a configuration model to follow.

---

## 9. Open Questions

1. **Run state cleanup**: Currently, completed `runState` entries remain in the `Runner.runs` map indefinitely. At 1,000 agents producing 100+ events each, memory grows without bound. Need a TTL-based eviction or explicit cleanup policy. How long should completed run state be retained?

2. **SQLite vs Postgres at scale**: The conversation store and memory store both support Postgres (memory store has `NewPostgresStore`). At what agent count does the SQLite→Postgres migration become necessary? Postgres handles concurrent writes natively but adds deployment complexity.

3. **Provider-specific rate limit headers**: OpenAI returns `x-ratelimit-remaining-requests`, `x-ratelimit-remaining-tokens`, `x-ratelimit-reset-requests`, and `x-ratelimit-reset-tokens` headers. The current OpenAI client (`client.go:98-102`) does not read these. Should the ProviderPool parse them for smarter key selection?

4. **Agent priority and fairness**: With 1,000 agents sharing resources, how should priority be handled? Should tenant-level or agent-level quotas exist? The current system is first-come-first-served.

5. **Multi-provider distribution**: The `catalog.ProviderRegistry` already supports multiple providers. Could agents be distributed across OpenAI, Anthropic, and DeepSeek to multiply rate limits? What are the prompt/tool compatibility implications?

6. **Event history memory growth**: Each `runState` accumulates all events in `state.events` (`runner.go:1322`). With 1,000 agents averaging 100 events each (JSON payload ~500 bytes), that's ~50 MB just for event history. Should events be streamed to disk or capped?

7. **Tool execution isolation**: Tools like `bash` execute subprocesses. With 1,000 agents potentially running bash commands concurrently, process table and file descriptor exhaustion becomes a concern. Should tool execution have its own semaphore?

8. **Conversation store in-memory cache**: The `Runner.conversations` map (`runner.go:62`) caches all conversation messages in memory. At 1,000 conversations with ~50KB each, that's 50 MB — acceptable. But if conversations grow large (100+ turns), this could consume GBs. Should there be an LRU eviction policy?

9. **Streaming SSE backpressure**: The event channel buffer is 64 (`runner.go:340`). With high event rates, slow SSE clients will miss events (dropped at `runner.go:1331-1333`). Is this acceptable, or should slow clients be disconnected?

10. **Bare metal OS tuning**: Beyond `ulimit -n`, what kernel parameters need tuning? `net.core.somaxconn`, `net.ipv4.tcp_max_syn_backlog`, `net.ipv4.ip_local_port_range`, `vm.max_map_count` all potentially matter at 1,000+ concurrent network connections.
