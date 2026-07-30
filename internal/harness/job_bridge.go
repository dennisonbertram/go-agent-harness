package harness

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	htools "go-agent-harness/internal/harness/tools"
)

// EventBackgroundJobCompleted carries a finished background job's result onto
// the originating conversation's stream.
const EventBackgroundJobCompleted EventType = "job.completed"

// maxQueuedJobNoticeBytes bounds how much of a job's output is replayed into
// the next turn. A background job can print megabytes; the model needs to know
// what happened, not to have its context filled by one command.
const maxQueuedJobNoticeBytes = 2000

// maxQueuedJobNotices bounds how many completions accumulate for one
// conversation while no run is listening.
const maxQueuedJobNotices = 20

// JobEventBridge reports background job completions to both surfaces that
// need them: the UI, via an event on the originating conversation's live run,
// and the model, via a notice queued for its next turn.
//
// Both halves are necessary and neither is sufficient. A background job
// routinely finishes after the run that started it has ended, so at completion
// there is often no open stream to emit on — which is why completions are
// queued rather than only emitted. Equally, a queued notice alone would leave
// the UI silent for a job that finishes while the user is watching.
type JobEventBridge struct {
	mu      sync.Mutex
	runner  *Runner
	pending map[string][]htools.JobCompletion // conversation ID -> completions
}

func NewJobEventBridge() *JobEventBridge {
	return &JobEventBridge{pending: make(map[string][]htools.JobCompletion)}
}

// BindRunner attaches the Runner events are emitted through. Safe to call once
// the Runner exists; completions that arrive before then are still queued.
func (b *JobEventBridge) BindRunner(r *Runner) {
	b.mu.Lock()
	b.runner = r
	b.mu.Unlock()
}

// JobCompleted implements tools.JobEvents.
func (b *JobEventBridge) JobCompleted(c htools.JobCompletion) {
	b.mu.Lock()
	r := b.runner
	if c.ConversationID != "" {
		queue := b.pending[c.ConversationID]
		if len(queue) < maxQueuedJobNotices {
			b.pending[c.ConversationID] = append(queue, c)
		}
	}
	b.mu.Unlock()

	if r == nil {
		return
	}
	// Delivered to the conversation, not to a run. A background job routinely
	// outlives the run that started it, and the run-scoped fan-out is keyed on
	// live run state — so emitting there reached nobody in exactly the case
	// this exists for. The app already subscribes per conversation
	// (GET /v1/conversations/{id}/events), which is where a fact about the
	// conversation belongs anyway.
	r.emitToConversation(c.ConversationID, c.RunID, EventBackgroundJobCompleted, map[string]any{
		"shell_id":    c.ShellID,
		"command":     c.Command,
		"exit_code":   c.ExitCode,
		"timed_out":   c.TimedOut,
		"output":      truncateJobNotice(c.Output),
		"truncated":   c.Truncated,
		"working_dir": c.WorkingDir,
	})
}

// TakeNotices removes and returns the completions queued for a conversation.
// They are consumed rather than read so the same job is not reported to the
// model on every subsequent turn.
func (b *JobEventBridge) TakeNotices(conversationID string) []htools.JobCompletion {
	if conversationID == "" {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	queued := b.pending[conversationID]
	delete(b.pending, conversationID)
	return queued
}

// FormatJobNotices renders queued completions as a short message for the
// model. Returns "" when there is nothing to report, so callers can skip
// injecting an empty turn.
func FormatJobNotices(completions []htools.JobCompletion) string {
	if len(completions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Background jobs finished since your last turn:\n")
	for _, c := range completions {
		status := fmt.Sprintf("exit %d", c.ExitCode)
		if c.TimedOut {
			status = "timed out"
		}
		fmt.Fprintf(&b, "\n- %s (%s, %s)\n", c.Command, c.ShellID, status)
		out := truncateJobNotice(c.Output)
		if out == "" {
			b.WriteString("  (no output)\n")
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}
		if c.Truncated {
			b.WriteString("  [output truncated — use job_output for the full result]\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func truncateJobNotice(s string) string {
	if len(s) <= maxQueuedJobNoticeBytes {
		return s
	}
	return s[:maxQueuedJobNoticeBytes] + "\n[…truncated]"
}

// isLiveRun reports whether a run exists and has not terminated. A background
// job's originating run is usually gone by the time it finishes, so this is
// the check that decides between emitting now and relying on the queue.
func (r *Runner) isLiveRun(runID string) bool {
	if runID == "" {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.runs[runID]
	return ok && state != nil && !state.terminated
}

// JobNoticeSource supplies background-job completions that finished since a
// conversation's last turn.
type JobNoticeSource interface {
	TakeNotices(conversationID string) []htools.JobCompletion
}

// SetJobNotices installs the source consulted at the start of each run for
// background jobs that finished while nothing was listening.
func (r *Runner) SetJobNotices(src JobNoticeSource) {
	r.mu.Lock()
	r.jobNotices = src
	r.mu.Unlock()
}

// takeJobNoticeBlock returns a rendered notice for the conversation, or "".
// Consuming here means each completion is reported to the model exactly once.
func (r *Runner) takeJobNoticeBlock(conversationID string) string {
	r.mu.RLock()
	src := r.jobNotices
	r.mu.RUnlock()
	if src == nil {
		return ""
	}
	return FormatJobNotices(src.TakeNotices(conversationID))
}

// emitToConversation delivers an event to every conversation-scoped subscriber
// without requiring a live run.
//
// The normal emit path hangs the event off a run's state and fans out from
// there, which is right for anything a run produces. A background job's
// completion is not one of those: by the time it arrives its run has usually
// ended, so that path drops it precisely when it matters.
func (r *Runner) emitToConversation(convID, originRunID string, eventType EventType, payload map[string]any) {
	if convID == "" {
		return
	}
	r.conversationEventMu.Lock()
	defer r.conversationEventMu.Unlock()

	timestamp := time.Now().UTC()
	event := Event{
		ID:        fmt.Sprintf("%s:conversation:%s", originRunID, uuid.NewString()),
		RunID:     originRunID,
		Type:      eventType,
		Timestamp: timestamp,
		Payload:   deepClonePayload(payload),
	}
	if event.Payload == nil {
		event.Payload = make(map[string]any)
	}
	event.Payload["schema_version"] = EventSchemaVersion
	event.Payload["conversation_id"] = convID
	r.recordConversationEvent(convID, event)

	r.mu.RLock()
	subs := make([]chan Event, 0, len(r.convSubscribers[convID]))
	for ch := range r.convSubscribers[convID] {
		subs = append(subs, ch)
	}
	r.mu.RUnlock()
	for _, ch := range subs {
		eventCopy := event
		eventCopy.Payload = deepClonePayload(event.Payload)
		r.sendTerminalSubscriberEvent(ch, eventCopy)
	}
}
