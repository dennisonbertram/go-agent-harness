# KIRA Implementation Surface Analysis

**Date**: 2026-03-12
**Purpose**: Map exact code locations for bash output assembly, JobManager, token estimation, and compaction machinery — prerequisite for implementing KIRA-inspired harness improvements.

---

## 1. Bash Output Assembly

### Where output is assembled and returned

**File**: `internal/harness/tools/head_tail_buffer.go`

The core buffer type is `headTailBuffer` (line 13), with the constant:

```go
// head_tail_buffer.go:9
const defaultMaxCommandOutputBytes = 16 * 1024  // currently 16KB — target is 30KB per KIRA design
```

The `headTailBuffer.String()` method (line 68) is where the head+tail merge and truncation marker are assembled:

```go
// head_tail_buffer.go:68-84
func (b *headTailBuffer) String() string {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.total <= b.max {
        // No truncation
        combined = append(combined, b.head...)
        combined = append(combined, b.tail...)
        return string(combined)
    }

    // Truncated: head + marker + tail
    combined = append(combined, b.head...)
    combined = append(combined, []byte(truncatedOutputMarker)...)
    combined = append(combined, b.tail...)
    return string(combined)
}
```

**Truncation marker** (line 10):
```go
truncatedOutputMarker = "\n...[truncated output]...\n"
```

**Note**: The current implementation does NOT produce structured metadata (`truncated: bool`, `max_bytes`, `truncation_strategy`, `hint`). It only embeds the plain text marker inline. The KIRA issue calls for metadata in the tool result JSON envelope.

**Final output assembly point for foreground jobs**:

```go
// bash_manager.go:136
output := mergeCommandStreams(strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
return map[string]any{
    "command":     command,
    "exit_code":   exitCode,
    "timed_out":   timedOut,
    "output":      output,
    "working_dir": NormalizeRelPath(m.root, workDir),
}, nil
```

For background jobs:

```go
// bash_manager.go:241
output := mergeCommandStreams(strings.TrimSpace(job.stdout.String()), strings.TrimSpace(job.stderr.String()))
return map[string]any{
    "shell_id":   shellID,
    "running":    !job.done,
    "exit_code":  job.exitCode,
    "timed_out":  job.timedOut,
    "output":     output,
    "started_at": job.startedAt,
}, nil
```

**Also**: `internal/harness/tools/common_exec.go:17-34` — a separate exec helper that also uses `newHeadTailBuffer(defaultMaxCommandOutputBytes)` directly (not via JobManager).

---

## 2. JobManager Definition and Return Values

**File**: `internal/harness/tools/bash_manager.go`

**Struct definition** (line 43):
```go
type JobManager struct {
    root           string
    nextID         uint64
    mu             sync.RWMutex
    jobs           map[string]*backgroundJob
    maxJobs        int
    ttl            time.Duration
    maxOutputBytes int     // <-- this is what controls truncation
    now            func() time.Time
    sandboxScope   SandboxScope
}
```

**Constructor** (line 55):
```go
func NewJobManager(workspaceRoot string, now func() time.Time) *JobManager {
    return &JobManager{
        root:           workspaceRoot,
        jobs:           make(map[string]*backgroundJob),
        maxJobs:        64,
        ttl:            30 * time.Minute,
        maxOutputBytes: defaultMaxCommandOutputBytes,  // 16KB today
        now:            now,
    }
}
```

**`RunForeground` return** (line 75 `runForeground`): returns `map[string]any` with keys: `command`, `exit_code`, `timed_out`, `output`, `working_dir`.

**`RunBackground` return** (line 147 `runBackground`): returns `map[string]any` with keys: `shell_id`, `started`, `command`, `working_dir`. (No output at start — output is fetched later via `Output()`.)

**`Output()` return** (line 221 `output`): returns `map[string]any` with keys: `shell_id`, `running`, `exit_code`, `timed_out`, `output`, `started_at`.

**Public wrappers** (exported via `internal/harness/tools/job_manager_exports.go`): `RunForeground`, `RunBackground`, `Output`, `Kill`.

**The bash tool handler** (`internal/harness/tools/core/bash.go:35-76`) calls `manager.RunForeground` or `manager.RunBackground`, then passes the result map to `tools.MarshalToolResult(result)` which serializes it to JSON. **No structured truncation metadata is added at the tool-handler layer today.**

---

## 3. Token Estimation in runner.go

**File**: `internal/harness/runner.go`

Token estimation happens inside the `execute()` function at line 645, specifically at **lines 754-760** inside the step loop:

```go
// runner.go:753-760 (inside execute(), inside the per-step for loop)
estimatedCtxTokens := 0
for _, m := range messages {
    runes := utf8.RuneCountInString(m.Content)
    if runes > 0 {
        estimatedCtxTokens += (runes + 3) / 4
    }
}
```

**Formula**: `(runes + 3) / 4` — a simple approximation (4 chars per token). This is used only for the `prompt_context` reporting to the system prompt engine, not for triggering any compaction.

**Token pressure classification** (line 2034):
```go
func contextPressureLevel(estimatedTokens int) string {
    switch {
    case estimatedTokens > 60000: return "high"
    case estimatedTokens > 30000: return "medium"
    default:                      return "low"
    }
}
```

**Critical gap**: The token estimate is computed but **never used to trigger compaction** inside `execute()`. There is no threshold check, no auto-compact call, no guard against context overflow. The run simply continues until the LLM provider returns a context-length error or the step limit is hit.

---

## 4. Compaction Machinery

### Tool-layer compaction (`compact_history` tool)

**File**: `internal/harness/tools/compact_history.go`

Three modes exposed via the `compact_history` tool:

| Mode | Function | Behavior |
|------|----------|----------|
| `strip` | `compactStrip()` (line 267) | Removes tool result messages from compaction zone, inserts text marker |
| `summarize` | `compactSummarize()` (line 321) | Replaces compaction zone with LLM summary via `MessageSummarizer` |
| `hybrid` | `compactHybrid()` (line 361) | Strips large tool outputs (>500 token threshold at line 370), summarizes if summarizer available |

Token estimation for compaction (line 446):
```go
func estimateTextTokens(s string) int {
    runes := utf8.RuneCountInString(s)
    if runes <= 0 { return 0 }
    return (runes + 3) / 4
}
```

### HTTP-layer compaction (`CompactRun` API)

**File**: `internal/harness/runner.go`

**`CompactRun()`** (line 2060): Public API method on `*Runner`. Callable by the HTTP server to trigger compaction on an active run. Validates mode, reads messages from run state, calls `compactMessagesHTTP()`, writes new messages back.

**`compactMessagesHTTP()`** (line 2149): Internal function that applies strip/summarize/hybrid logic directly on `[]TranscriptMessage` slices (no context-based reader/replacer needed — operates outside tool execution path).

**`NewMessageSummarizer()`** (line 2510): Creates a `MessageSummarizer` backed by the runner's provider. Used by both `CompactRun` and available to be passed to `compactMessagesHTTP`.

### What is missing for auto-compaction

The machinery exists: `compactMessagesHTTP` + `NewMessageSummarizer` + token estimation formula. What is absent:

1. No `AutoCompactEnabled` / `AutoCompactThreshold` / `ModelContextWindow` config fields on `RunnerConfig`
2. No pre-step threshold check in `execute()` that compares `estimatedCtxTokens / modelContextWindow` against a threshold
3. No per-run mutex/flag to prevent concurrent auto + manual compaction
4. No `auto_compact_started` / `auto_compact_completed` events

---

## Key File:Line Summary

| Topic | File | Line |
|-------|------|------|
| `defaultMaxCommandOutputBytes` (16KB constant) | `internal/harness/tools/head_tail_buffer.go` | 9 |
| `headTailBuffer.String()` — truncation assembly | `internal/harness/tools/head_tail_buffer.go` | 68 |
| `mergeCommandStreams()` | `internal/harness/tools/head_tail_buffer.go` | 86 |
| `JobManager` struct definition | `internal/harness/tools/bash_manager.go` | 43 |
| `NewJobManager()` constructor | `internal/harness/tools/bash_manager.go` | 55 |
| Foreground output assembly (`runForeground`) | `internal/harness/tools/bash_manager.go` | 136 |
| Background output assembly (`output`) | `internal/harness/tools/bash_manager.go` | 241 |
| Bash tool handler entry | `internal/harness/tools/core/bash.go` | 35 |
| `execute()` function start | `internal/harness/runner.go` | 645 |
| Token estimation in step loop | `internal/harness/runner.go` | 754 |
| `contextPressureLevel()` | `internal/harness/runner.go` | 2034 |
| `CompactRun()` | `internal/harness/runner.go` | 2060 |
| `compactMessagesHTTP()` | `internal/harness/runner.go` | 2149 |
| `NewMessageSummarizer()` | `internal/harness/runner.go` | 2510 |
| `compactStrip()` | `internal/harness/tools/compact_history.go` | 267 |
| `compactSummarize()` | `internal/harness/tools/compact_history.go` | 321 |
| `compactHybrid()` | `internal/harness/tools/compact_history.go` | 361 |
| `estimateTextTokens()` | `internal/harness/tools/compact_history.go` | 446 |
