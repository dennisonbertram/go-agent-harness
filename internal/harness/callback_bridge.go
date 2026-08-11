package harness

import (
	"sync"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

// CallbackEventBridge forwards delayed-callback lifecycle events from a
// tools.CallbackManager into the owning conversation so they are observable
// through live SSE and conversation replay.
//
// Observability semantics:
//
//   - callback.scheduled is emitted synchronously while the agent's
//     set_delayed_callback tool call is executing, i.e. DURING the originating
//     run. The run is live, so the bridge resolves it by conversation ID and
//     the event is observable on that run's SSE stream in real time.
//   - Later dispatch/retry/failure/cancel events may occur after the originating
//     run has ended. They are conversation-owned and must not be appended after
//     a terminal run event. Recovery republishes the current durable lifecycle
//     snapshot before active callback timers are re-armed.
//
// The runner is bound lazily because the CallbackManager is constructed before
// the Runner exists (the same chicken-and-egg the callbackRunStarter solves).
// A nil runner makes Emit a no-op.
type CallbackEventBridge struct {
	mu     sync.RWMutex
	runner *Runner
}

// NewCallbackEventBridge returns an unbound bridge. Call BindRunner once the
// Runner has been constructed.
func NewCallbackEventBridge() *CallbackEventBridge {
	return &CallbackEventBridge{}
}

// NewCallbackManager builds a tools.CallbackManager whose lifecycle events are,
// by default, bridged onto the originating run's SSE stream via this Runner.
// Use this when the Runner already exists. When the manager must be constructed
// before the Runner (as in harnessd's startup), construct a
// CallbackEventBridge, pass it via tools.WithEventSink, and call BindRunner
// once the Runner is built.
func (r *Runner) NewCallbackManager(starter htools.RunStarter, opts ...htools.CallbackOption) *htools.CallbackManager {
	bridge := NewCallbackEventBridge()
	bridge.BindRunner(r)
	all := make([]htools.CallbackOption, 0, len(opts)+1)
	all = append(all, htools.WithEventSink(bridge))
	all = append(all, opts...)
	return htools.NewCallbackManager(starter, all...)
}

// BindRunner attaches the Runner the bridge forwards events to. Safe to call
// once, after the Runner is built.
func (b *CallbackEventBridge) BindRunner(r *Runner) {
	b.mu.Lock()
	b.runner = r
	b.mu.Unlock()
}

// Emit implements tools.CallbackEvents. It resolves a live run for the
// callback's conversation and emits the event there. If no runner is bound or
// no live run exists for the conversation, Emit is a no-op (the event has no
// open SSE stream to be delivered on).
func (b *CallbackEventBridge) Emit(event string, info htools.CallbackInfo) {
	b.mu.RLock()
	r := b.runner
	b.mu.RUnlock()
	if r == nil {
		return
	}
	r.emitCallbackEvent(event, info)
}

// emitCallbackEvent keeps scheduled attached to its live scheduling run for
// run-scoped compatibility. Later callback transitions are conversation-owned
// work: publish them directly to the conversation journal so completion of the
// scheduling run cannot drop retry/failure/start linkage, and never append a
// post-terminal event to that sealed run.
func (r *Runner) emitCallbackEvent(event string, info htools.CallbackInfo) {
	if event == string(EventCallbackScheduled) {
		if runID, ok := r.liveRunForConversation(info.ConversationID); ok {
			r.emit(runID, EventType(event), callbackEventPayload(info))
			return
		}
	}
	originRunID := info.RunID
	if originRunID == "" {
		originRunID = "callback_" + info.ID
	}
	r.emitToConversation(info.ConversationID, originRunID, EventType(event), callbackEventPayload(info))
}

// liveRunForConversation returns the ID of the most-recently-created
// non-terminated run whose conversation matches convID, if one exists.
// When multiple live runs share a conversation (rare), picking the newest
// ensures the event is delivered to the run that scheduled the callback —
// the set_delayed_callback tool call always originates from the most-recent
// run on the conversation.  When there is 0 or 1 match the result is
// identical to the previous unordered iteration.
func (r *Runner) liveRunForConversation(convID string) (string, bool) {
	if convID == "" {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var (
		bestID string
		bestAt time.Time
	)
	for id, state := range r.runs {
		if state == nil || state.terminated {
			continue
		}
		if state.run.ConversationID != convID {
			continue
		}
		if bestID == "" || state.run.CreatedAt.After(bestAt) {
			bestID = id
			bestAt = state.run.CreatedAt
		}
	}
	return bestID, bestID != ""
}

// callbackEventPayload builds the SSE payload for a callback lifecycle event.
func callbackEventPayload(info htools.CallbackInfo) map[string]any {
	payload := map[string]any{
		"callback_id":     info.ID,
		"conversation_id": info.ConversationID,
		"state":           string(info.State),
		"delay":           info.Delay,
		"prompt":          info.Prompt,
		"fires_at":        info.FiresAt,
		"created_at":      info.CreatedAt,
	}
	if info.RunID != "" {
		payload["run_id"] = info.RunID
	}
	if info.Attempt > 0 {
		payload["attempt"] = info.Attempt
	}
	if !info.NextAttemptAt.IsZero() {
		payload["next_attempt_at"] = info.NextAttemptAt
	}
	if summary := htools.SafeCallbackErrorSummary(info.LastError); summary != "" {
		payload["last_error"] = summary
	}
	return payload
}
