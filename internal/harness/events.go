package harness

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EventSchemaVersion is the version stamp injected into every event payload.
// Bump this when the event schema changes in a backward-incompatible way.
const EventSchemaVersion = "1"

// EventType represents a typed SSE event name.
type EventType string

// Run lifecycle events.
const (
	EventRunStarted         EventType = "run.started"
	EventRunCompleted       EventType = "run.completed"
	EventRunFailed          EventType = "run.failed"
	EventRunWaitingForUser  EventType = "run.waiting_for_user"
	EventRunResumed         EventType = "run.resumed"
	// EventRunCostLimitReached is emitted when the cumulative cost of a run
	// reaches or exceeds the max_cost_usd ceiling specified in the RunRequest.
	// The run is then terminated with EventRunCompleted (not EventRunFailed).
	EventRunCostLimitReached EventType = "run.cost_limit_reached"
	EventRunStepStarted      EventType = "run.step.started"
	EventRunStepCompleted    EventType = "run.step.completed"
)

// LLM turn events.
const (
	EventLLMTurnRequested      EventType = "llm.turn.requested"
	EventLLMTurnCompleted      EventType = "llm.turn.completed"
	EventAssistantMessageDelta  EventType = "assistant.message.delta"
	EventAssistantThinkingDelta EventType = "assistant.thinking.delta"
	// EventReasoningComplete is emitted after each LLM turn when
	// CaptureReasoning is enabled and the provider returned reasoning text.
	// Payload fields: text (string), tokens (int), step (int).
	EventReasoningComplete EventType = "reasoning.complete"
)

// Tool execution events.
const (
	EventToolCallStarted   EventType = "tool.call.started"
	EventToolCallCompleted EventType = "tool.call.completed"
	EventToolCallDelta     EventType = "tool.call.delta"
	EventToolActivated     EventType = "tool.activated"    // Deferred tool activated via find_tool
	EventToolOutputDelta   EventType = "tool.output.delta" // Incremental output chunk from a running tool
)

// Assistant completion events.
const (
	EventAssistantMessage EventType = "assistant.message"
)

// Conversation events.
const (
	EventConversationContinued EventType = "conversation.continued"
)

// Prompt events.
const (
	EventPromptResolved EventType = "prompt.resolved"
	EventPromptWarning  EventType = "prompt.warning"
)

// Provider events.
const (
	EventProviderResolved EventType = "provider.resolved"
)

// Memory events.
const (
	EventMemoryObserveStarted      EventType = "memory.observe.started"
	EventMemoryObserveCompleted    EventType = "memory.observe.completed"
	EventMemoryObserveFailed       EventType = "memory.observe.failed"
	EventMemoryReflectionCompleted EventType = "memory.reflection.completed"
)

// Accounting events.
const (
	EventUsageDelta EventType = "usage.delta"
)

// Cost forensics events.
const (
	// EventCostAnomaly is emitted when CostAnomalyDetectionEnabled is true and
	// a single step's cost exceeds CostAnomalyStepMultiplier × the rolling
	// average cost of prior steps in the run.
	// Payload fields: step (int), anomaly_type (string), step_cost_usd (float64),
	// avg_cost_usd (float64), threshold_multiplier (float64).
	EventCostAnomaly EventType = "cost.anomaly"
)

// Hook events (message-level: pre/post LLM turn).
const (
	EventHookStarted   EventType = "hook.started"
	EventHookFailed    EventType = "hook.failed"
	EventHookCompleted EventType = "hook.completed"
)

// Tool hook events (tool-level: pre/post individual tool execution).
const (
	EventToolHookStarted   EventType = "tool_hook.started"
	EventToolHookFailed    EventType = "tool_hook.failed"
	EventToolHookCompleted EventType = "tool_hook.completed"
)

// Callback events.
const (
	EventCallbackScheduled EventType = "callback.scheduled"
	EventCallbackFired     EventType = "callback.fired"
	EventCallbackCanceled  EventType = "callback.canceled"
)

// Skill constraint events.
const (
	EventSkillConstraintActivated   EventType = "skill.constraint.activated"
	EventSkillConstraintDeactivated EventType = "skill.constraint.deactivated"
	EventToolCallBlocked            EventType = "tool.call.blocked"
)

// Meta-message events.
const (
	EventMetaMessageInjected EventType = "meta.message.injected"
)

// Steering events.
const (
	// EventSteeringReceived is emitted when a user steering message is injected
	// into the run transcript before an LLM call.
	EventSteeringReceived EventType = "steering.received"
)

// Auto-compaction events.
const (
	// EventAutoCompactStarted is emitted when proactive auto-compaction begins.
	EventAutoCompactStarted EventType = "auto_compact.started"
	// EventAutoCompactCompleted is emitted when proactive auto-compaction finishes.
	EventAutoCompactCompleted EventType = "auto_compact.completed"
)

// Skill fork events.
const (
	EventSkillForkStarted   EventType = "skill.fork.started"
	EventSkillForkCompleted EventType = "skill.fork.completed"
	EventSkillForkFailed    EventType = "skill.fork.failed"
)

// Context management events.
const (
	EventCompactHistoryCompleted EventType = "compact_history.completed"
)

// Context reset events.
const (
	// EventContextReset is emitted when an agent calls reset_context to clear
	// its conversation transcript and start a new context segment.
	// Payload fields: reset_index (int), at_step (int), persist (any).
	EventContextReset EventType = "context.reset"
)

// Context window forensics events (opt-in via RunnerConfig.ContextWindowSnapshotEnabled).
const (
	// EventContextWindowSnapshot is emitted after each LLM turn when
	// ContextWindowSnapshotEnabled is true in RunnerConfig. It captures
	// a per-step snapshot of context window usage including token counts,
	// usage ratio, available headroom, and a best-effort breakdown by
	// component (system prompt, conversation history, tool results).
	//
	// All token counts NOT sourced from the provider usage response are
	// labeled with "estimated":true in the breakdown sub-object.
	//
	// Payload fields:
	//   step (int), provider_reported_tokens (int), provider_reported (bool),
	//   estimated_total_tokens (int), max_context_tokens (int),
	//   usage_ratio (float64), headroom_tokens (int),
	//   breakdown.system_prompt_tokens (int), breakdown.conversation_tokens (int),
	//   breakdown.tool_result_tokens (int), breakdown.estimated (bool=true).
	EventContextWindowSnapshot EventType = "context.window.snapshot"

	// EventContextWindowWarning is emitted when context usage exceeds a
	// configurable threshold (ContextWindowWarningThreshold in RunnerConfig).
	// Only emitted when ContextWindowSnapshotEnabled is also true.
	//
	// Payload fields: step (int), usage_ratio (float64),
	// threshold (float64), provider_reported (bool),
	// tokens_used (int), max_context_tokens (int).
	EventContextWindowWarning EventType = "context.window.warning"
)

// LLM request envelope events (forensics, opt-in via RunnerConfig.CaptureRequestEnvelope).
const (
	// EventLLMRequestSnapshot is emitted BEFORE each provider call when
	// CaptureRequestEnvelope is enabled. It captures a compact fingerprint of
	// what the model will see: a SHA-256 hash of the prompt, tool names, memory
	// snippet, and step number. The full prompt text is intentionally omitted
	// to avoid bloat and PII leakage.
	// Payload fields: step (int), prompt_hash (string), tool_names ([]string),
	// memory_snippet (string, may be empty).
	EventLLMRequestSnapshot EventType = "llm.request.snapshot"
	// EventLLMResponseMeta is emitted AFTER each provider call when
	// CaptureRequestEnvelope is enabled. It captures provider metadata that is
	// only available once the response arrives.
	// Payload fields: step (int), latency_ms (int64), model_version (string,
	// may be empty if provider does not report it).
	EventLLMResponseMeta EventType = "llm.response.meta"
)

// Error chain events.
const (
	// EventErrorContext is emitted immediately before run.failed when
	// ErrorChainEnabled is set in RunnerConfig. It carries an error
	// classification, a context snapshot of the last N tool calls and
	// messages, and an optional cause chain.
	EventErrorContext EventType = "error.context"
)

// Audit trail events (opt-in via RunnerConfig.AuditTrailEnabled).
const (
	// EventAuditAction is emitted for each state-modifying tool call when
	// AuditTrailEnabled is set in RunnerConfig. It is written to the
	// append-only audit.jsonl file alongside the standard rollout.jsonl.
	// Payload fields: tool (string), call_id (string), arguments (string).
	EventAuditAction EventType = "audit.action"
)

// Forensics: tool decision tracing events (opt-in via RunnerConfig.TraceToolDecisions).
const (
	// EventToolDecision is emitted once per step when TraceToolDecisions is
	// enabled. Payload includes: step (int), call_sequence (int),
	// available_tools ([]string), selected_tools ([]string).
	EventToolDecision EventType = "tool.decision"
	// EventToolAntiPattern is emitted when DetectAntiPatterns is enabled and
	// the same (tool, args) pair has been seen 3 or more times in one run.
	// Payload includes: type (string), tool (string), call_count (int), step (int).
	EventToolAntiPattern EventType = "tool.antipattern"
	// EventToolHookMutation is emitted when TraceHookMutations is enabled and
	// a pre-tool-use hook modified or blocked a tool call.
	// Payload includes: tool_call_id (string), hook (string), action (string),
	// args_before (string), args_after (string).
	EventToolHookMutation EventType = "tool.hook.mutation"
)

// Causal graph events.
const (
	// EventCausalGraphSnapshot is emitted at run end when CausalGraphEnabled
	// is set in RunnerConfig. It carries the causal dependency graph for the
	// entire run, including Tier 1 (context dependencies) and Tier 2
	// (data-flow heuristic) edges.
	// Payload fields: step (int), graph (CausalGraph JSON object).
	EventCausalGraphSnapshot EventType = "causal.graph.snapshot"
)

// AllEventTypes returns all known event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventRunStarted,
		EventRunCompleted,
		EventRunFailed,
		EventRunWaitingForUser,
		EventRunResumed,
		EventRunCostLimitReached,
		EventRunStepStarted,
		EventRunStepCompleted,
		EventLLMTurnRequested,
		EventLLMTurnCompleted,
		EventAssistantMessageDelta,
		EventAssistantThinkingDelta,
		EventReasoningComplete,
		EventToolCallStarted,
		EventToolCallCompleted,
		EventToolCallDelta,
		EventToolActivated,
		EventToolOutputDelta,
		EventAssistantMessage,
		EventConversationContinued,
		EventPromptResolved,
		EventPromptWarning,
		EventProviderResolved,
		EventMemoryObserveStarted,
		EventMemoryObserveCompleted,
		EventMemoryObserveFailed,
		EventMemoryReflectionCompleted,
		EventUsageDelta,
		EventHookStarted,
		EventHookFailed,
		EventHookCompleted,
		EventCallbackScheduled,
		EventCallbackFired,
		EventCallbackCanceled,
		EventSkillConstraintActivated,
		EventSkillConstraintDeactivated,
		EventToolCallBlocked,
		EventMetaMessageInjected,
		EventSkillForkStarted,
		EventSkillForkCompleted,
		EventSkillForkFailed,
		EventToolHookStarted,
		EventToolHookFailed,
		EventToolHookCompleted,
		EventSteeringReceived,
		EventCompactHistoryCompleted,
		EventErrorContext,
		EventAutoCompactStarted,
		EventAutoCompactCompleted,
		EventToolDecision,
		EventToolAntiPattern,
		EventToolHookMutation,
		EventLLMRequestSnapshot,
		EventLLMResponseMeta,
		EventCostAnomaly,
		EventAuditAction,
		EventContextWindowSnapshot,
		EventContextWindowWarning,
		EventCausalGraphSnapshot,
		EventContextReset,
	}
}

// IsTerminalEvent reports whether the given event type signals the end of a run.
func IsTerminalEvent(et EventType) bool {
	return et == EventRunCompleted || et == EventRunFailed
}

// RunCompletedPayload is the typed payload for EventRunCompleted.
type RunCompletedPayload struct {
	Output      string         `json:"output"`
	UsageTotals map[string]any `json:"usage_totals,omitempty"`
	CostTotals  map[string]any `json:"cost_totals,omitempty"`
}

// ToPayload converts to a generic payload map.
func (p RunCompletedPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// ParseRunCompletedPayload parses a generic payload map into RunCompletedPayload.
func ParseRunCompletedPayload(payload map[string]any) (RunCompletedPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return RunCompletedPayload{}, err
	}
	var p RunCompletedPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// ContextResetPayload is the typed payload for EventContextReset.
type ContextResetPayload struct {
	ResetIndex int             `json:"reset_index"`
	AtStep     int             `json:"at_step"`
	Persist    json.RawMessage `json:"persist,omitempty"`
}

// RunFailedPayload is the typed payload for EventRunFailed.
type RunFailedPayload struct {
	Error       string         `json:"error"`
	UsageTotals map[string]any `json:"usage_totals,omitempty"`
	CostTotals  map[string]any `json:"cost_totals,omitempty"`
}

// ToPayload converts to a generic payload map.
func (p RunFailedPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// ParseRunFailedPayload parses a generic payload map into RunFailedPayload.
func ParseRunFailedPayload(payload map[string]any) (RunFailedPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return RunFailedPayload{}, err
	}
	var p RunFailedPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// UsageDeltaPayload is the typed payload for EventUsageDelta.
type UsageDeltaPayload struct {
	Step              int            `json:"step"`
	UsageStatus       string         `json:"usage_status"`
	CostStatus        string         `json:"cost_status"`
	TurnUsage         map[string]any `json:"turn_usage,omitempty"`
	TurnCostUSD       float64        `json:"turn_cost_usd"`
	CumulativeUsage   map[string]any `json:"cumulative_usage,omitempty"`
	CumulativeCostUSD float64        `json:"cumulative_cost_usd"`
	PricingVersion    string         `json:"pricing_version,omitempty"`
}

// ToPayload converts to a generic payload map.
func (p UsageDeltaPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// ParseUsageDeltaPayload parses a generic payload map into UsageDeltaPayload.
func ParseUsageDeltaPayload(payload map[string]any) (UsageDeltaPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return UsageDeltaPayload{}, err
	}
	var p UsageDeltaPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// ToolOutputDeltaPayload is the typed payload for EventToolOutputDelta.
// It carries a single incremental chunk of output from a running tool.
type ToolOutputDeltaPayload struct {
	CallID      string `json:"call_id"`
	Tool        string `json:"tool"`
	StreamIndex int    `json:"stream_index"`
	Content     string `json:"content"`
}

// ToPayload converts to a generic payload map.
func (p ToolOutputDeltaPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// ParseToolOutputDeltaPayload parses a generic payload map into ToolOutputDeltaPayload.
func ParseToolOutputDeltaPayload(payload map[string]any) (ToolOutputDeltaPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return ToolOutputDeltaPayload{}, err
	}
	var p ToolOutputDeltaPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// ContextWindowSnapshotBreakdown is the breakdown sub-object in a context
// window snapshot payload.
type ContextWindowSnapshotBreakdown struct {
	SystemPromptTokens int  `json:"system_prompt_tokens"`
	ConversationTokens int  `json:"conversation_tokens"`
	ToolResultTokens   int  `json:"tool_result_tokens"`
	// Estimated is always true — all counts use the rune-count heuristic.
	Estimated          bool `json:"estimated"`
}

// ContextWindowSnapshotPayload is the typed payload for EventContextWindowSnapshot.
type ContextWindowSnapshotPayload struct {
	Step int `json:"step"`
	// ProviderReportedTokens is the prompt token count from the provider usage
	// response. Zero when the provider did not report usage.
	ProviderReportedTokens int `json:"provider_reported_tokens"`
	// ProviderReported is true when ProviderReportedTokens came from the provider.
	ProviderReported bool `json:"provider_reported"`
	// EstimatedTotalTokens is the best-effort estimated total using rune/4 heuristic.
	EstimatedTotalTokens int `json:"estimated_total_tokens"`
	// MaxContextTokens is the model's context window size from the provider catalog.
	// Zero when unknown.
	MaxContextTokens int `json:"max_context_tokens"`
	// UsageRatio is the fraction of the context window in use (0.0–1.0+).
	UsageRatio float64 `json:"usage_ratio"`
	// HeadroomTokens is the estimated remaining capacity. May be negative on overrun.
	HeadroomTokens int `json:"headroom_tokens"`
	Breakdown      ContextWindowSnapshotBreakdown `json:"breakdown"`
}

// ToPayload converts to a generic payload map.
func (p ContextWindowSnapshotPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// ParseContextWindowSnapshotPayload parses a generic payload map into
// ContextWindowSnapshotPayload.
func ParseContextWindowSnapshotPayload(payload map[string]any) (ContextWindowSnapshotPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return ContextWindowSnapshotPayload{}, err
	}
	var p ContextWindowSnapshotPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// ContextWindowWarningPayload is the typed payload for EventContextWindowWarning.
type ContextWindowWarningPayload struct {
	Step             int     `json:"step"`
	UsageRatio       float64 `json:"usage_ratio"`
	Threshold        float64 `json:"threshold"`
	ProviderReported bool    `json:"provider_reported"`
	TokensUsed       int     `json:"tokens_used"`
	MaxContextTokens int     `json:"max_context_tokens"`
}

// ToPayload converts to a generic payload map.
func (p ContextWindowWarningPayload) ToPayload() map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}

// ParseContextWindowWarningPayload parses a generic payload map into
// ContextWindowWarningPayload.
func ParseContextWindowWarningPayload(payload map[string]any) (ContextWindowWarningPayload, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return ContextWindowWarningPayload{}, err
	}
	var p ContextWindowWarningPayload
	err = json.Unmarshal(b, &p)
	return p, err
}

// ParseEventID parses a per-run event ID of the form "runID:seq" into its
// components. Returns an error for malformed IDs.
func ParseEventID(id string) (runID string, seq uint64, err error) {
	idx := strings.LastIndex(id, ":")
	if idx < 0 || idx == len(id)-1 {
		return "", 0, fmt.Errorf("invalid event ID %q: missing colon separator", id)
	}
	runID = id[:idx]
	if runID == "" {
		return "", 0, fmt.Errorf("invalid event ID %q: empty run ID", id)
	}
	seq, err = strconv.ParseUint(id[idx+1:], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid event ID %q: %w", id, err)
	}
	return runID, seq, nil
}
