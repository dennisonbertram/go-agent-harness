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

// Error chain events.
const (
	// EventErrorContext is emitted immediately before run.failed when
	// ErrorChainEnabled is set in RunnerConfig. It carries an error
	// classification, a context snapshot of the last N tool calls and
	// messages, and an optional cause chain.
	EventErrorContext EventType = "error.context"
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
