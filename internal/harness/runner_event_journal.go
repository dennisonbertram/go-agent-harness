package harness

import (
	"context"
	"fmt"
	"time"

	"go-agent-harness/internal/forensics/redaction"
	"go-agent-harness/internal/rollout"
)

type eventDispatch struct {
	runID          string
	conversationID string
	eventType      EventType
	event          Event
	eventSeq       uint64

	dropped bool

	subscribers []subscriberDelivery

	recorderCh    chan rollout.RecordableEvent
	recorderDone  chan struct{}
	closeRecorder func()
}

type subscriberDelivery struct {
	ch    chan Event
	event Event
}

type eventJournal struct {
	runner *Runner
}

func newEventJournal(r *Runner) *eventJournal {
	return &eventJournal{runner: r}
}

func (j *eventJournal) prepareLocked(state *runState, runID string, eventType EventType, payload map[string]any) (eventDispatch, bool) {
	// Deep-clone the caller's payload so that nested maps and slices inside
	// the payload are not aliased. A shallow copy is insufficient: if the
	// caller holds a reference to a nested slice or map and mutates it after
	// emit() returns (or concurrently), the stored forensic event would
	// otherwise observe those mutations (#228).
	enriched := deepClonePayload(payload)
	if enriched == nil {
		enriched = make(map[string]any, 3)
	}
	// Inject forensic correlation fields into every event payload.
	enriched["schema_version"] = EventSchemaVersion
	enriched["conversation_id"] = state.run.ConversationID
	if _, ok := enriched["step"]; !ok {
		enriched["step"] = state.currentStep
	}

	// Seal the run for terminal events BEFORE redaction so that even if the
	// redaction pipeline drops the event, the recorder is still closed and
	// the terminated gate is still armed. Without this, a "drop" rule on
	// run.completed would leave the run unsealed forever.
	isTerminal := IsTerminalEvent(eventType)
	delivery := eventDispatch{
		runID:          runID,
		conversationID: state.run.ConversationID,
		eventType:      eventType,
	}
	if isTerminal {
		state.terminated = true
		delivery.recorderCh = state.recorderCh
		delivery.recorderDone = state.recorderDone
		delivery.closeRecorder = state.closeRecorderOnce
		state.recorderCh = nil
		state.recorderDone = nil
		state.closeRecorderOnce = nil
	} else {
		delivery.recorderCh = state.recorderCh
	}

	// Apply PII/secret redaction pipeline if configured.
	// The redaction config comes from the run's config snapshot (captured at
	// run creation) so an ApplyConfig swap mid-run cannot change redaction
	// behavior for an in-flight run. prepareLocked runs under r.mu, so it
	// reads state.config directly instead of calling configForRun.
	rc := j.runner.snapshotConfig()
	if state.config != nil {
		rc = *state.config
	}
	if rc.RedactionPipeline != nil {
		var keep bool
		enriched, keep = redaction.RedactPayload(rc.RedactionPipeline, string(eventType), enriched)
		if !keep {
			delivery.dropped = true
			return delivery, true
		}
	}

	// Deep-clone the enriched payload for immutable forensic storage.
	// This prevents any nested map/slice from being shared with subscribers,
	// the recorder, or the original caller.
	storedPayload := deepClonePayload(enriched)

	eventSeq := state.nextEventSeq
	event := Event{
		ID:        fmt.Sprintf("%s:%d", runID, eventSeq),
		RunID:     runID,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   storedPayload,
	}
	state.nextEventSeq++
	state.events = append(state.events, event)

	delivery.event = event
	delivery.eventSeq = eventSeq

	// Conversation-scoped subscribers (GET /v1/conversations/{id}/events,
	// issue #950) observe every event from every run on this conversation, in
	// addition to that run's own run-scoped subscribers. Each subscriber has
	// its own channel (allocated by Subscribe / SubscribeConversation), so a
	// caller subscribed to both never receives the same event twice on the
	// same channel.
	convSubs := j.runner.convSubscribers[state.run.ConversationID]

	// Snapshot every subscriber before releasing the runner lock, but publish
	// only after durable append. A subscriber registered after this snapshot
	// receives the event through replay instead of live fanout, preventing the
	// history+live duplicate race. Cancellation is handled by the closed-channel
	// guard in sendTerminalSubscriberEvent.
	delivery.subscribers = make([]subscriberDelivery, 0, len(state.subscribers)+len(convSubs))
	for ch := range state.subscribers {
		evCopy := event
		evCopy.Payload = deepClonePayload(storedPayload)
		delivery.subscribers = append(delivery.subscribers, subscriberDelivery{
			ch:    ch,
			event: evCopy,
		})
	}
	for ch := range convSubs {
		evCopy := event
		evCopy.Payload = deepClonePayload(storedPayload)
		delivery.subscribers = append(delivery.subscribers, subscriberDelivery{
			ch:    ch,
			event: evCopy,
		})
	}

	return delivery, true
}

func (j *eventJournal) persistTerminalEvent(delivery eventDispatch) bool {
	return j.persistTerminalEventContext(context.Background(), delivery)
}

func (j *eventJournal) persistTerminalEventContext(ctx context.Context, delivery eventDispatch) bool {
	persisted := j.runner.storeAppendEventContext(ctx, delivery.event, delivery.eventSeq)
	if persisted {
		j.runner.markTerminalEventPersisted(delivery.runID)
	}
	return persisted
}

func (j *eventJournal) recordTerminalConversation(delivery eventDispatch) {
	j.runner.recordConversationEvent(delivery.conversationID, delivery.event)
}

func (j *eventJournal) fanoutTerminal(delivery eventDispatch) {
	for _, sub := range delivery.subscribers {
		j.runner.sendTerminalSubscriberEvent(sub.ch, sub.event)
	}
}

func (j *eventJournal) publishTerminal(delivery eventDispatch) {
	j.persistTerminalEvent(delivery)
	j.recordTerminalConversation(delivery)
	j.fanoutTerminal(delivery)
}

func (j *eventJournal) dispatch(delivery eventDispatch) {
	j.dispatchContext(context.Background(), delivery, false)
}

func (j *eventJournal) dispatchContext(ctx context.Context, delivery eventDispatch, requirePersistence bool) bool {
	if delivery.dropped {
		if delivery.closeRecorder != nil {
			delivery.closeRecorder()
			j.waitForRecorderDrain(delivery, j.runner.configForRun(delivery.runID))
		}
		// StorageModeNone is an intentional policy outcome, not a transient
		// publication failure. Treat it as consumed so strict publishers do not
		// retry a deliberately suppressed lifecycle event until its deadline.
		return true
	}

	// Logger comes from the run's config snapshot when available so logging
	// stays consistent with the config the run started with.
	rc := j.runner.configForRun(delivery.runID)

	// Recordable form shared by non-terminal queuing and terminal draining.
	rev := rollout.RecordableEvent{
		ID:        delivery.event.ID,
		RunID:     delivery.event.RunID,
		Type:      string(delivery.event.Type),
		Timestamp: delivery.event.Timestamp,
		Payload:   delivery.event.Payload,
		Seq:       delivery.eventSeq,
	}

	if !IsTerminalEvent(delivery.eventType) {
		persisted := j.runner.storeAppendEventContext(ctx, delivery.event, delivery.eventSeq)
		if (!persisted && requirePersistence) || ctx.Err() != nil {
			j.discardPreparedEvent(delivery)
			return false
		}
		j.runner.recordConversationEvent(delivery.conversationID, delivery.event)
		for _, sub := range delivery.subscribers {
			j.runner.sendTerminalSubscriberEvent(sub.ch, sub.event)
		}
		if delivery.recorderCh != nil {
			if !safeRecorderSend(delivery.recorderCh, rev) {
				if rc.Logger != nil {
					rc.Logger.Error("rollout recorder: channel full, event dropped",
						"run_id", delivery.runID, "event_type", string(delivery.eventType), "seq", delivery.eventSeq)
				}
				dropMarker := rollout.RecordableEvent{
					ID:        fmt.Sprintf("%s:drop:%d", delivery.runID, delivery.eventSeq),
					RunID:     delivery.runID,
					Type:      string(EventRecorderDropDetected),
					Timestamp: time.Now().UTC(),
					Seq:       delivery.eventSeq,
					Payload: map[string]any{
						"dropped_event_id":   delivery.event.ID,
						"dropped_event_type": string(delivery.eventType),
						"dropped_seq":        delivery.eventSeq,
					},
				}
				safeRecorderSend(delivery.recorderCh, dropMarker)
			}
		}
	}

	if IsTerminalEvent(delivery.eventType) {
		if delivery.recorderCh != nil {
			sendTimer := time.NewTimer(recorderDrainTimeout)
			defer sendTimer.Stop()
			select {
			case delivery.recorderCh <- rev:
			case <-sendTimer.C:
				if rc.Logger != nil {
					rc.Logger.Error("rollout recorder: terminal send timeout, JSONL may be incomplete",
						"run_id", delivery.runID, "timeout", recorderDrainTimeout)
				}
			}
			delivery.closeRecorder()
			j.waitForRecorderDrain(delivery, rc)
		}
		return true
	}

	// Non-terminal recorder events are queued above while emit still owns the
	// conversation event lock, after context-bound persistence succeeds. That
	// keeps terminal close ordered without recording an expired wait event.
	return true
}

func (j *eventJournal) discardPreparedEvent(delivery eventDispatch) {
	j.runner.mu.Lock()
	defer j.runner.mu.Unlock()
	state, ok := j.runner.runs[delivery.runID]
	if !ok {
		return
	}
	// emit still owns conversationEventMu, so no later event can have been
	// prepared while a strict append was in flight. Roll back both the final
	// ledger entry and its sequence allocation to keep event IDs contiguous;
	// run SSE reconnect treats the sequence as the visible history index.
	last := len(state.events) - 1
	if last < 0 || state.events[last].ID != delivery.event.ID {
		return
	}
	if state.nextEventSeq != delivery.eventSeq+1 {
		return
	}
	state.events = state.events[:last]
	state.nextEventSeq = delivery.eventSeq
}

func (j *eventJournal) waitForRecorderDrain(delivery eventDispatch, rc RunnerConfig) {
	if delivery.recorderDone == nil {
		return
	}
	drainTimer := time.NewTimer(recorderDrainTimeout)
	defer drainTimer.Stop()
	select {
	case <-delivery.recorderDone:
	case <-drainTimer.C:
		if rc.Logger != nil {
			rc.Logger.Error("rollout recorder: drain timeout exceeded, JSONL may be incomplete",
				"run_id", delivery.runID, "timeout", recorderDrainTimeout)
		}
	}
}
