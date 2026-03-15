# Training Mode Codebase Exploration Report

**Date**: 2026-03-14
**Scope**: Interfaces and types needed for training mode implementation

---

## 1. Rollout Recorder (internal/rollout/)

The rollout package implements a JSONL-based event recorder that captures the complete timeline of a run for replay, fork, and audit purposes.

### File Structure
- `recorder.go` — Core recorder implementation
- `recorder_test.go` — Unit tests
- `integration_test.go` — Integration tests

### Key Types

#### RecordableEvent
**File**: `recorder.go:25-44`

```go
type RecordableEvent struct {
	// ID is the per-run event ID, e.g. "run_1:42".
	ID string
	// RunID is the run this event belongs to.
	RunID string
	// Type is the event type string, e.g. "run.started".
	Type string
	// Timestamp is when the event occurred (UTC).
	Timestamp time.Time
	// Payload contains event-specific data. May be nil.
	Payload map[string]any
	// Seq is the monotonic sequence number assigned by the caller at
	// event-emission time (before any lock contention on the recorder).
	// Callers MUST populate this field so that the JSONL file faithfully
	// reflects the logical emission order even when concurrent goroutines
	// race to acquire the recorder's write mutex.
	Seq uint64
}
```

#### entry (On-Disk Format)
**File**: `recorder.go:46-52`

```go
type entry struct {
	Ts   time.Time      `json:"ts"`      // Event timestamp
	Seq  uint64         `json:"seq"`     // Sequence number
	Type string         `json:"type"`    // Event type
	Data map[string]any `json:"data,omitempty"` // Event payload
}
```

#### RecorderConfig
**File**: `recorder.go:54-62`

```go
type RecorderConfig struct {
	// Dir is the root directory where rollout files are stored.
	// Files are written under <Dir>/<YYYY-MM-DD>/<RunID>.jsonl.
	Dir string
	// RunID is the identifier of the run being recorded.
	RunID string
}
```

#### Recorder
**File**: `recorder.go:75-80`

```go
type Recorder struct {
	mu     sync.Mutex
	file   *os.File
	enc    *json.Encoder
	closed bool
}
```

### Key Methods
- `NewRecorder(cfg RecorderConfig) (*Recorder, error)` — Line 85
- `NewRecorderAt(cfg RecorderConfig, now time.Time) (*Recorder, error)` — Line 91 (test helper)
- `Record(ev RecordableEvent)` — Line 128 (safe for concurrent use, errors silently)
- `Close() error` — Line 150 (idempotent)

### File Naming Convention
```
<Dir>/<YYYY-MM-DD>/<RunID>.jsonl
```

Example: `rollouts/2026-03-14/run_abc123.jsonl`

### JSONL Record Example
```json
{"ts":"2026-03-14T12:34:56.123Z","seq":1,"type":"run.started","data":{"prompt":"fix the bug"}}
{"ts":"2026-03-14T12:34:57.456Z","seq":2,"type":"llm.completion.started","data":{}}
```

### Critical Design Notes
- **Sequence ownership**: Caller assigns monotonic sequence numbers under their own ordering primitive (not the recorder)
- **Concurrent-safe**: Write mutex on Record() but errors are silently dropped (non-blocking to primary run loop)
- **Idempotent Close**: Safe to call multiple times

---

## 2. Forensics Package

The forensics package provides post-hoc analysis tools for runs, capturing decision traces, causal graphs, context usage, and error chains.

### 2.1 Tool Decision (internal/forensics/tooldecision/)

**File**: `tooldecision.go`

#### ToolDecisionSnapshot
**Lines: 10-20**

```go
type ToolDecisionSnapshot struct {
	// Step is the 1-based step number within the run.
	Step int `json:"step"`
	// CallSequence is the sequential call number within the run (call_1, call_2, …).
	// It increments across all steps, not resetting per step.
	CallSequence int `json:"call_sequence"`
	// AvailableTools is the list of tool names sent to the model for this step.
	AvailableTools []string `json:"available_tools"`
	// SelectedTools is the list of tool names the model chose to call.
	SelectedTools []string `json:"selected_tools"`
}
```

Method: `CallSequenceID() string` — Returns "call_N" format (line 24)

#### AntiPatternType
**Lines: 28-35**

```go
type AntiPatternType string

const (
	// AntiPatternRetryLoop is emitted when the same tool is called with the
	// same arguments 3 or more times within a single run.
	AntiPatternRetryLoop AntiPatternType = "retry_loop"
)
```

#### AntiPatternAlert
**Lines: 37-48**

```go
type AntiPatternAlert struct {
	// Type is the category of anti-pattern detected.
	Type AntiPatternType `json:"type"`
	// ToolName is the name of the tool involved.
	ToolName string `json:"tool_name"`
	// CallCount is the number of times the tool/args pair has been seen
	// at the point the alert was raised (>= 3 for retry_loop).
	CallCount int `json:"call_count"`
	// Step is the step number at which the threshold was crossed.
	Step int `json:"step"`
}
```

#### HookMutation
**Lines: 64-78**

```go
type HookMutation struct {
	// ToolCallID is the ID of the tool call from the LLM response.
	ToolCallID string `json:"tool_call_id"`
	// HookName is the name of the hook that processed the call.
	HookName string `json:"hook_name"`
	// Action classifies what the hook did: Block, Modify, Inject, or Allow.
	Action HookMutationAction `json:"action"`
	// ArgsBefore is the JSON arguments string before the hook ran.
	ArgsBefore string `json:"args_before,omitempty"`
	// ArgsAfter is the JSON arguments string after the hook ran.
	// Empty when action is Block or Allow.
	ArgsAfter string `json:"args_after,omitempty"`
}
```

Hook actions (lines 53-62):
- `HookActionAllow` — Hook allowed call without modification
- `HookActionBlock` — Hook blocked the call
- `HookActionModify` — Hook modified arguments
- `HookActionInject` — Hook injected arguments (original was empty)

Helper: `ClassifyHookAction(blocked bool, argsBefore, argsAfter string) HookMutationAction` — Line 82

### 2.2 Causal Graph (internal/forensics/causalgraph/)

**File**: `graph.go`

#### Node
**Lines: 17-23**

```go
type Node struct {
	ID       string   `json:"id"`
	Type     NodeType `json:"type"`  // "llm_turn" or "tool_call"
	Step     int      `json:"step"`
	ToolName string   `json:"tool_name,omitempty"`
}
```

#### Edge
**Lines: 35-41**

```go
type Edge struct {
	From         string   `json:"from"`  // Source node ID
	To           string   `json:"to"`    // Target node ID
	Type         EdgeType `json:"type"`  // "context" or "data_flow"
	MatchedToken string   `json:"matched_token,omitempty"`
}
```

#### CausalGraph
**Lines: 43-47**

```go
type CausalGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
```

Method: `ToAdjacencyList() map[string][]string` — Exports graph as adjacency list (line 53)

### 2.3 Context Window (internal/forensics/contextwindow/)

**File**: `contextwindow.go`

#### WindowSnapshot
**Lines: 54-90**

```go
type WindowSnapshot struct {
	// Step is the 1-based step number within the run.
	Step int `json:"step"`
	// ProviderReportedTokens is the total token count reported by the provider.
	ProviderReportedTokens int `json:"provider_reported_tokens"`
	// ProviderReported is true when ProviderReportedTokens came from provider.
	ProviderReported bool `json:"provider_reported"`
	// EstimatedTotalTokens is a best-effort estimate of total context usage.
	EstimatedTotalTokens int `json:"estimated_total_tokens"`
	// MaxContextTokens is the model's maximum context window size in tokens.
	MaxContextTokens int `json:"max_context_tokens"`
	// UsageRatio is the fraction of context window in use (0.0 to 1.0+).
	UsageRatio float64 `json:"usage_ratio"`
	// HeadroomTokens is the estimated tokens remaining before full.
	HeadroomTokens int `json:"headroom_tokens"`
	// Breakdown provides decomposition of context usage.
	Breakdown ContextBreakdown `json:"breakdown"`
}
```

#### ContextBreakdown
**Lines: 32-52**

```go
type ContextBreakdown struct {
	SystemPromptTokens int  `json:"system_prompt_tokens"`
	ConversationTokens int  `json:"conversation_tokens"`
	ToolResultTokens   int  `json:"tool_result_tokens"`
	Estimated          bool `json:"estimated"`  // Always true
}
```

#### CompactionTokenCounts
**Lines: 92-100**

```go
type CompactionTokenCounts struct {
	BeforeTokens int  `json:"before_tokens"`
	AfterTokens  int  `json:"after_tokens"`
	Estimated    bool `json:"estimated"`  // Always true
}
```

Helpers:
- `EstimateTokens(s string) int` — Line 21 (uses rune-count heuristic: (runes+3)/4)
- `BuildSnapshot(step, systemPromptText, messages, providerPromptTokens, providerReported, maxContextTokens)` — Line 110
- `SnapshotToPayload(s WindowSnapshot) map[string]any` — Line 170 (for SSE events)

### 2.4 Error Chain (internal/forensics/errorchain/)

**File**: `errorchain.go`

#### ErrorClass
**Lines: 18-31**

```go
type ErrorClass string

const (
	ClassToolExecution ErrorClass = "tool_execution"
	ClassHallucination ErrorClass = "hallucination"
	ClassProvider      ErrorClass = "provider"
	ClassResource      ErrorClass = "resource"
)
```

#### ChainedError
**Lines: 39-48**

```go
type ChainedError struct {
	// Class is the error taxonomy classification.
	Class ErrorClass
	// msg is the human-readable description.
	msg string
	// Cause is the underlying error that triggered this one, or nil.
	Cause error
	// Context is an optional snapshot captured at the time of the error.
	Context *Snapshot
}
```

Methods:
- `NewChainedError(class ErrorClass, msg string, cause error) *ChainedError` — Line 52
- `Error() string` — Line 61 (implements error interface)
- `Unwrap() error` — Line 70 (for errors.Is/As traversal)

#### Snapshot
**Lines: 102-113**

```go
type Snapshot struct {
	// CapturedAt is the wall-clock time the snapshot was taken.
	CapturedAt time.Time `json:"captured_at"`
	// ToolCalls holds the last Depth tool invocations.
	ToolCalls []ToolCallEntry `json:"tool_calls"`
	// Messages holds the last Depth conversation messages.
	Messages []MessageEntry `json:"messages"`
	// Depth is the configured rolling window size.
	Depth int `json:"depth"`
}
```

#### ToolCallEntry
**Lines: 82-92**

```go
type ToolCallEntry struct {
	Name     string `json:"name"`
	CallID   string `json:"call_id"`
	Args     string `json:"args"`
	ErrorMsg string `json:"error_msg,omitempty"`
}
```

#### MessageEntry
**Lines: 94-100**

```go
type MessageEntry struct {
	Role    string `json:"role"`      // "user", "assistant", "tool", "system"
	Content string `json:"content"`
}
```

#### SnapshotBuilder
**Lines: 121-138**

```go
type SnapshotBuilder struct {
	mu        sync.RWMutex
	depth     int
	toolCalls []ToolCallEntry
	messages  []MessageEntry
}
```

Methods:
- `NewSnapshotBuilder(depth int) *SnapshotBuilder` — Line 130 (uses DefaultSnapshotDepth=10 if <=0)
- `RecordToolCall(name, callID, args, errMsg string)` — Line 163
- `RecordMessage(role, content string)` — Line 182
- `Build() Snapshot` — Line 193 (returns deep copy)

Key constant: `maxSnapshotStringBytes = 64 * 1024` (line 146, prevents secret retention)

Helper: `BuildErrorContextPayload(ce *ChainedError, sb *SnapshotBuilder) map[string]any` — Line 243

---

## 3. Harness Types (internal/harness/)

**File**: `runner.go` (core loop) and `types.go` (type definitions)

### 3.1 Message Type
**File**: `types.go:36-54`

```go
type Message struct {
	MessageID        string     `json:"message_id,omitempty"`
	Role             string     `json:"role"`  // "user", "assistant", "tool", "system"
	Content          string     `json:"content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`  // For tool responses
	Name             string     `json:"name,omitempty"`          // Tool name
	IsMeta           bool       `json:"is_meta,omitempty"`       // System-generated
	IsCompactSummary bool       `json:"is_compact_summary,omitempty"`  // From compaction
	CorrelationID    string     `json:"correlation_id,omitempty"`     // Links across turns
	ConversationID   string     `json:"conversation_id,omitempty"`    // Stable ID
	Reasoning        string     `json:"reasoning,omitempty"`    // Captured when CaptureReasoning enabled
}
```

Method: `Clone() Message` — Line 57 (deep copy with independent ToolCalls)

### 3.2 ToolCall Type
**File**: `types.go:22-26`

```go
type ToolCall struct {
	ID        string `json:"id"`         // LLM-assigned tool call ID
	Name      string `json:"name"`       // Tool name
	Arguments string `json:"arguments"` // JSON string of arguments
}
```

Method: `Clone() ToolCall` — Line 32

### 3.3 RunUsageTotals
**File**: `types.go:158-163`

```go
type RunUsageTotals struct {
	PromptTokensTotal     int `json:"prompt_tokens_total"`
	CompletionTokensTotal int `json:"completion_tokens_total"`
	TotalTokens           int `json:"total_tokens"`
	LastTurnTokens        int `json:"last_turn_tokens"`
}
```

### 3.4 RunCostTotals
**File**: `types.go:165-170`

```go
type RunCostTotals struct {
	CostUSDTotal    float64    `json:"cost_usd_total"`
	LastTurnCostUSD float64    `json:"last_turn_cost_usd"`
	CostStatus      CostStatus `json:"cost_status"`
	PricingVersion  string     `json:"pricing_version,omitempty"`
}
```

CostStatus constants (lines 149-156):
- `CostStatusAvailable` — Cost data available
- `CostStatusUnpricedModel` — Model not in pricing catalog
- `CostStatusProviderUnreported` — Provider didn't report usage
- `CostStatusPending` — Cost not yet calculated

### 3.5 Event Type
**File**: `types.go:198-204`

```go
type Event struct {
	ID        string         `json:"id"`     // e.g. "run_1:42"
	RunID     string         `json:"run_id"`
	Type      EventType      `json:"type"`   // Event type constant
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}
```

### 3.6 Run Type
**File**: `types.go:216-231`

```go
type Run struct {
	ID             string          `json:"id"`
	Prompt         string          `json:"prompt"`
	Model          string          `json:"model"`
	ProviderName   string          `json:"provider_name,omitempty"`
	Status         RunStatus       `json:"status"`
	Output         string          `json:"output,omitempty"`
	Error          string          `json:"error,omitempty"`
	UsageTotals    *RunUsageTotals `json:"usage_totals,omitempty"`
	CostTotals     *RunCostTotals  `json:"cost_totals,omitempty"`
	TenantID       string          `json:"tenant_id,omitempty"`
	ConversationID string          `json:"conversation_id,omitempty"`
	AgentID        string          `json:"agent_id,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
```

RunStatus constants (lines 208-214):
- `RunStatusQueued` — Initial state
- `RunStatusRunning` — Currently executing
- `RunStatusWaitingForUser` — Awaiting user input
- `RunStatusCompleted` — Finished successfully
- `RunStatusFailed` — Terminated with error

### 3.7 RunRequest Type
**File**: `types.go:233-287`

```go
type RunRequest struct {
	Prompt           string            `json:"prompt"`
	Model            string            `json:"model,omitempty"`
	ProviderName     string            `json:"provider_name,omitempty"`
	AllowFallback    bool              `json:"allow_fallback,omitempty"`
	SystemPrompt     string            `json:"system_prompt,omitempty"`
	TenantID         string            `json:"tenant_id,omitempty"`
	ConversationID   string            `json:"conversation_id,omitempty"`
	AgentID          string            `json:"agent_id,omitempty"`
	AgentIntent      string            `json:"agent_intent,omitempty"`
	TaskContext      string            `json:"task_context,omitempty"`
	PromptProfile    string            `json:"prompt_profile,omitempty"`
	PromptExtensions *PromptExtensions `json:"prompt_extensions,omitempty"`
	MaxSteps         int               `json:"max_steps,omitempty"`
	MaxCostUSD       float64           `json:"max_cost_usd,omitempty"`
	ReasoningEffort  string            `json:"reasoning_effort,omitempty"`
	AllowedTools     []string          `json:"allowed_tools,omitempty"`
	MCPServers       []MCPServerConfig `json:"mcp_servers,omitempty"`
	Permissions      *PermissionConfig `json:"permissions,omitempty"`
	InitiatorAPIKeyPrefix string       `json:"-"`  // Server-set only
}
```

### 3.8 runState (Internal)
**File**: `runner.go:36-93`

```go
type runState struct {
	run                Run
	staticSystemPrompt string
	promptResolved     *systemprompt.ResolvedPrompt
	usageTotals        usageTotalsAccumulator
	costTotals         RunCostTotals
	messages           []Message              // Conversation history
	events             []Event                // Run events
	subscribers        map[chan Event]struct{} // Event subscribers
	nextEventSeq       uint64                 // Event sequence counter
	steeringCh         chan string            // User steering messages (buffered=10)
	maxCostUSD         float64                // Per-run spending ceiling
	allowedTools       []string               // Per-run tool filter
	permissions        PermissionConfig       // Sandbox + approval config
	recorder           *rollout.Recorder      // JSONL event recorder
	recorderMu         sync.Mutex
	recorderClosed     bool
	auditWriter        *audittrail.AuditWriter
	previousRunID      string
	currentStep        int
	continued          bool                   // Set after ContinueRun called
	snapshotBuilder    *errorchain.SnapshotBuilder
	terminated         bool                   // Terminal event emitted
	compactMu          sync.Mutex
	resetIndex         int
	scopedMCPRegistry  *ScopedMCPRegistry
}
```

### 3.9 Runner Type
**File**: `runner.go:140-156`

```go
type Runner struct {
	provider         Provider
	tools            *Registry
	config           RunnerConfig
	providerRegistry *catalog.ProviderRegistry
	activations      *ActivationTracker
	skillConstraints *SkillConstraintTracker
	envInfo          systemprompt.EnvironmentInfo

	mu                  sync.RWMutex
	runs                map[string]*runState
	conversations       map[string][]Message              // Conversation history cache
	conversationOwners  map[string]conversationOwner      // Ownership for multi-tenant scoping
}
```

---

## 4. Benchmark Format (benchmarks/terminal_bench/)

### 4.1 Task Definition (task.yaml)
**Example**: `tasks/go-race-condition-fix/task.yaml`

```yaml
instruction: |
  This Go HTTP server has a data race in its counter implementation.
  Run `go test -race ./...` to find the race condition.
  Fix the Counter struct in `counter.go` so it is safe for concurrent access.
  The fix must use sync.Mutex, sync.RWMutex, or sync/atomic.
  All existing HTTP endpoints in main.go must continue to work.

difficulty: hard
category: bugfix
parser_name: pytest
max_agent_timeout_sec: 900
max_test_timeout_sec: 180
run_tests_in_same_shell: false
test_scripts:
  - run-tests.sh
```

### 4.2 Test Format (tests/test_task.py)
**Example**: `tasks/go-race-condition-fix/tests/test_task.py`

```python
import re
import subprocess
from pathlib import Path

def test_race_detector_passes() -> None:
    """go test -race must exit 0 (no data races)."""
    result = subprocess.run(
        ["go", "test", "-race", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=120,
    )
    assert result.returncode == 0, (
        f"race detector failed:\nstdout: {result.stdout}\nstderr: {result.stderr}"
    )

def test_synchronization_primitive_present() -> None:
    """counter.go must use sync.Mutex, sync.RWMutex, or sync/atomic."""
    contents = Path("/app/counter.go").read_text()
    has_sync = (
        "sync.Mutex" in contents
        or "sync.RWMutex" in contents
        or "atomic." in contents
    )
    assert has_sync, "counter.go must use sync.Mutex, sync.RWMutex, or sync/atomic"

def test_build_succeeds() -> None:
    """go build must succeed."""
    result = subprocess.run(
        ["go", "build", "./..."],
        cwd="/app",
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result.returncode == 0

def test_endpoints_intact() -> None:
    """main.go must still define /inc, /get, and /reset handlers."""
    contents = Path("/app/main.go").read_text()
    for endpoint in ["/inc", "/get", "/reset"]:
        assert endpoint in contents, f"endpoint {endpoint} missing from main.go"
```

Test function naming convention: `test_<requirement>()` with docstring as test description.

### 4.3 Baseline Format (baseline.json)
**File**: `benchmarks/terminal_bench/baseline.json`

```json
{
  "_comment": "Baseline expectations for terminal bench tasks.",
  "tasks": {
    "go-race-condition-fix": {
      "expected_pass": true,
      "difficulty": "medium",
      "category": "debugging",
      "avg_steps": 10,
      "avg_tokens": 6000,
      "avg_cost_usd": 0.04,
      "avg_wall_time_sec": 36
    }
  }
}
```

Per-task fields:
- `expected_pass` (bool) — Should the agent pass this task?
- `difficulty` (string) — "easy", "medium", "hard"
- `category` (string) — Task domain (e.g., "debugging", "refactor", "bugfix")
- `avg_steps` (int) — Expected number of LLM turns
- `avg_tokens` (int) — Expected total tokens consumed
- `avg_cost_usd` (float) — Expected cost in dollars
- `avg_wall_time_sec` (int) — Expected wall-clock time

### 4.4 Task Directory Structure
```
tasks/<task-name>/
├── task.yaml                 # Task definition & metadata
├── docker-compose.yaml       # Container setup
├── Dockerfile                # Image definition
├── run-tests.sh              # Test execution script
├── tests/
│   └── test_task.py          # Pytest test suite
├── <source files>            # Code to fix/implement
└── data/                      # (Optional) Data files
```

### 4.5 agent.py
**File**: `benchmarks/terminal_bench/agent.py`

Likely defines:
- Agent interface for running benchmark tasks
- Harness communication protocol
- Result collection/validation

(Full contents not read due to file size; but essential for understanding test execution flow)

---

## 5. Provider Pattern (internal/provider/)

### 5.1 Provider Interface
**File**: `types.go:192-194`

```go
type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}
```

### 5.2 CompletionRequest
**File**: `types.go:68-77`

```go
type CompletionRequest struct {
	Model           string                `json:"model"`
	Messages        []Message             `json:"messages"`
	Tools           []ToolDefinition      `json:"tools,omitempty"`
	Stream          func(CompletionDelta) `json:"-"`
	ReasoningEffort string                `json:"reasoning_effort,omitempty"`
}
```

### 5.3 CompletionResult
**File**: `types.go:79-107`

```go
type CompletionResult struct {
	Content            string            `json:"content"`
	ToolCalls          []ToolCall        `json:"tool_calls,omitempty"`
	Deltas             []CompletionDelta `json:"-"`
	Usage              *CompletionUsage  `json:"usage,omitempty"`
	CostUSD            *float64          `json:"cost_usd,omitempty"`
	Cost               *CompletionCost   `json:"cost,omitempty"`
	UsageStatus        UsageStatus       `json:"usage_status,omitempty"`
	CostStatus         CostStatus        `json:"cost_status,omitempty"`
	TTFTMs             int64             `json:"ttft_ms,omitempty"`     // Time-to-first-token (ms)
	TotalDurationMs    int64             `json:"total_duration_ms,omitempty"`
	ReasoningText      string            `json:"reasoning_text,omitempty"`
	ReasoningTokens    int               `json:"reasoning_tokens,omitempty"`
	ModelVersion       string            `json:"model_version,omitempty"` // e.g., "gpt-4.1-2025-04-14"
}
```

### 5.4 OpenAI Client Implementation
**File**: `openai/client.go`

Key types:
- `Config` (lines 25-33) — Client configuration (API key, base URL, model, pricing resolver)
- `Client` (lines 35-43) — OpenAI client implementation

Key method:
- `Complete(ctx context.Context, req harness.CompletionRequest) (harness.CompletionResult, error)` (line 91)

Additional methods (not fully read):
- `usesResponsesAPI(model string) bool` (line 84) — Routes to /v1/responses vs /v1/chat/completions
- `decodeCompletionResponse(model, responseBody)` (line 184)
- `decodeStreamingResponse(model, body, streamFn)` (line 192)

Critical design:
- **API endpoint routing**: Responses API vs Chat Completions based on model catalog
- **Streaming support**: With time-to-first-token measurement
- **Error handling**: HTTP status codes propagated as fmt.Errorf

### 5.5 Pricing
**Package**: `internal/provider/pricing/`

Types and pattern (referenced but not fully explored):
- `Resolver` interface — Resolves model pricing (input/output rates)
- Used by OpenAI client to compute `CompletionCost` from `CompletionUsage`

---

## 6. Go Module Dependencies

**File**: `go.mod` (module name: `go-agent-harness`, Go version: 1.25.0)

### Key Direct Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/google/uuid` | v1.6.0 | Run/event ID generation |
| `modernc.org/sqlite` | v1.33.1 | Conversation/state persistence |
| `github.com/docker/docker` | v28.5.2 | Container workspace support |
| `github.com/hetznercloud/hcloud-go` | v2.36.0 | VM workspace support |
| `gopkg.in/yaml.v3` | v3.0.1 | Task/config YAML parsing |
| `github.com/BurntSushi/toml` | v1.6.0 | TOML config parsing |

### Indirect Dependencies (Relevant)
- OpenTelemetry (`go.opentelemetry.io/...`) — Observability/tracing
- Charmbracelet (`charmbracelet/glamour`, etc.) — Terminal UI components
- Go crypto (`golang.org/x/crypto`) — For secure operations

---

## 7. Key Architectural Patterns for Training Mode

### 7.1 Event Flow
1. **Runner.execute(runID, req)** — Main loop goroutine
2. **runState.emit(event)** — Appends to `events` slice + broadcasts to subscribers
3. **Recorder.Record(ev)** — Writes to JSONL file (non-blocking, errors silently)
4. **AuditWriter.Write(...)** — Hash-chained audit log (when enabled)

### 7.2 Conversation Persistence
- `Runner.conversations` — In-memory map of `[conversation_id][]Message`
- `RunnerConfig.ConversationStore` — Optional persistent backend (SQLite)
- Multi-tenant scoping via `conversationOwners` map

### 7.3 Tool Recording
- Each `ToolCall` has `ID` (LLM-assigned), `Name`, `Arguments` (JSON string)
- Tool results are wrapped as `Message` with `Role: "tool"`, `ToolCallID`, `Content`
- Anti-pattern detection (retry loops) implemented post-hoc from `ToolDecisionSnapshot`

### 7.4 Cost Tracking
- `RunUsageTotals` accumulates per-turn usage
- `RunCostTotals` accumulates per-turn costs
- Provider reports `Usage` + `Cost` after each completion
- Per-run `MaxCostUSD` enforces spending ceiling

### 7.5 Context Window Management
- `contextwindow.WindowSnapshot` captures per-step context state
- Estimated tokens via rune-count heuristic (not accurate for pricing)
- Provider-reported tokens (when available) override estimates
- `Breakdown` decomposes system prompt, conversation, tool results

### 7.6 Error Context
- `errorchain.SnapshotBuilder` maintains rolling window of last N tool calls + messages
- `ChainedError` wraps classification + snapshot for forensics
- String fields capped at 64 KiB to prevent unbounded secret retention

---

## 8. Integration Patterns for Training Recorder

### 8.1 Event Emission Points
All events flow through `runState.emit(event)`:
- `run.started` — When execution begins
- `llm.completion.started/finished` — Per LLM turn
- `tool.call` — Each tool invocation
- `tool.result` — Tool result received
- `run.completed` / `run.failed` — Terminal states
- `context.window.snapshot` — Context state (per-turn)
- `error.context` — Error + snapshot on failure
- `causal.graph.snapshot` — Post-run graph analysis
- Custom anti-pattern alerts

### 8.2 Rollout Recording Integration
```go
// In runState.emit()
ev := Event{ID: ..., RunID: ..., Type: type, Timestamp: ..., Payload: ...}
events = append(events, ev)

// Async record to JSONL
recordableEv := rollout.RecordableEvent{
    ID: ev.ID,
    RunID: ev.RunID,
    Type: ev.Type,
    Timestamp: ev.Timestamp,
    Payload: ev.Payload,
    Seq: nextEventSeq++,
}
if recorder != nil {
    recorder.Record(recordableEv)  // Non-blocking, errors silently
}
```

---

## 9. Training Mode Integration Points

### 9.1 Data Collection
1. **Rollout JSONL files** — Raw event stream (read via `rollout.Loader`)
2. **Conversation history** — `runState.messages` or `Runner.conversations`
3. **Forensics snapshots** — Decision, context, error, graph (emitted as event payloads)
4. **Tool call traces** — Via `tooldecision.ToolDecisionSnapshot` + `HookMutation`
5. **Cost/usage data** — Via `CompletionResult` + `RunUsageTotals`

### 9.2 Trainer CLI Entry Points
- `cmd/trainerd/` would import:
  - `internal/harness` — Message, Run, Event types
  - `internal/rollout` — Recorder, RecordableEvent (for recording training runs)
  - `internal/forensics/*` — For analyzing captured runs
  - `benchmarks/terminal_bench/` — Test task format

### 9.3 Claude Trainer Provider
When implementing Claude trainer:
1. Conform to `Provider` interface (Complete method)
2. Return `CompletionResult` with `Usage` + `Cost`
3. Support streaming via `CompletionDelta`
4. Report `ReasoningText` (if applicable)
5. Set `ModelVersion` string from API response

---

## Summary

The codebase provides a comprehensive event-driven architecture for capturing and replaying agent executions. Key interfaces needed for training mode:

1. **Event Stream** — JSONL rollout format with RecordableEvent
2. **Conversation Types** — Message, ToolCall for training data
3. **Usage/Cost Types** — RunUsageTotals, RunCostTotals, CompletionUsage
4. **Forensics Snapshots** — ToolDecisionSnapshot, WindowSnapshot, ChainedError, CausalGraph
5. **Benchmark Format** — YAML task definitions + pytest test suites + baseline.json
6. **Provider Contract** — Provider interface + CompletionRequest/Result types
7. **Module Deps** — Go 1.25, sqlite3, docker, yaml.v3

All types are designed for JSON serialization and can be persisted to disk or streamed via SSE.
