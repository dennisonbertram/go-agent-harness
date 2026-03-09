package harness

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// EventType represents a typed SSE event name.
type EventType string

// Run lifecycle events.
const (
	EventRunStarted        EventType = "run.started"
	EventRunCompleted      EventType = "run.completed"
	EventRunFailed         EventType = "run.failed"
	EventRunWaitingForUser EventType = "run.waiting_for_user"
	EventRunResumed        EventType = "run.resumed"
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
	EventToolActivated     EventType = "tool.activated" // Deferred tool activated via find_tool
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

// Hook events.
const (
	EventHookStarted   EventType = "hook.started"
	EventHookFailed    EventType = "hook.failed"
	EventHookCompleted EventType = "hook.completed"
)

// Callback events.
const (
	EventCallbackScheduled EventType = "callback.scheduled"
	EventCallbackFired     EventType = "callback.fired"
	EventCallbackCanceled  EventType = "callback.canceled"
)

// AllEventTypes returns all known event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventRunStarted,
		EventRunCompleted,
		EventRunFailed,
		EventRunWaitingForUser,
		EventRunResumed,
		EventLLMTurnRequested,
		EventLLMTurnCompleted,
		EventAssistantMessageDelta,
		EventAssistantThinkingDelta,
		EventToolCallStarted,
		EventToolCallCompleted,
		EventToolCallDelta,
		EventToolActivated,
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
