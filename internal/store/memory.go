package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of Store.
// Useful for unit tests that don't require SQLite.
// All operations are thread-safe.
type MemoryStore struct {
	mu                 sync.RWMutex
	runs               map[string]*Run
	messages           map[string][]*Message // keyed by runID
	events             map[string][]*Event   // keyed by runID
	nextEventCursor    int64
	apiKeys            map[string]*APIKey // keyed by key ID (issue #9)
	cronRunStarts      map[string]CronRunStart
	conversationOwners map[string]conversationRunOwner
}

type conversationRunOwner struct {
	tenantID string
	agentID  string
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs:               make(map[string]*Run),
		messages:           make(map[string][]*Message),
		events:             make(map[string][]*Event),
		apiKeys:            make(map[string]*APIKey),
		cronRunStarts:      make(map[string]CronRunStart),
		conversationOwners: make(map[string]conversationRunOwner),
	}
}

// ClaimCronRunStart atomically reserves a tenant-scoped idempotency key.
func (m *MemoryStore) ClaimCronRunStart(_ context.Context, start CronRunStart) (CronRunStart, bool, error) {
	if start.TenantID == "" || start.IdempotencyKey == "" || start.Fingerprint == "" || start.RunID == "" {
		return CronRunStart{}, false, fmt.Errorf("store: complete cron run start binding is required")
	}
	if start.CreatedAt.IsZero() {
		start.CreatedAt = time.Now().UTC()
	}
	key := start.TenantID + "\x00" + start.IdempotencyKey
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.cronRunStarts[key]; ok {
		return existing, false, nil
	}
	m.cronRunStarts[key] = start
	return start, true, nil
}

// AcquireCronRunStartDispatchLease atomically claims or renews a dispatch lease.
func (m *MemoryStore) AcquireCronRunStartDispatchLease(_ context.Context, tenantID, idempotencyKey, owner string, now, leaseUntil time.Time) (CronRunStart, bool, error) {
	if tenantID == "" || idempotencyKey == "" || owner == "" || !leaseUntil.After(now) {
		return CronRunStart{}, false, fmt.Errorf("store: valid cron run dispatch lease is required")
	}
	key := tenantID + "\x00" + idempotencyKey
	m.mu.Lock()
	defer m.mu.Unlock()
	start, ok := m.cronRunStarts[key]
	if !ok {
		return CronRunStart{}, false, fmt.Errorf("store: cron run start binding not found")
	}
	if start.DispatchOwner != "" && start.DispatchOwner != owner && start.DispatchLeaseUntil.After(now) {
		return start, false, nil
	}
	start.DispatchOwner = owner
	start.DispatchLeaseUntil = leaseUntil
	m.cronRunStarts[key] = start
	return start, true, nil
}

// RenewCronRunStartDispatchLease extends a live lease held by the same owner.
func (m *MemoryStore) RenewCronRunStartDispatchLease(_ context.Context, tenantID, idempotencyKey, owner string, now, leaseUntil time.Time) (CronRunStart, bool, error) {
	if tenantID == "" || idempotencyKey == "" || owner == "" || !leaseUntil.After(now) {
		return CronRunStart{}, false, fmt.Errorf("store: valid cron run dispatch lease is required")
	}
	key := tenantID + "\x00" + idempotencyKey
	m.mu.Lock()
	defer m.mu.Unlock()
	start, ok := m.cronRunStarts[key]
	if !ok {
		return CronRunStart{}, false, fmt.Errorf("store: cron run start binding not found")
	}
	if start.DispatchOwner != owner || !start.DispatchLeaseUntil.After(now) {
		return start, false, nil
	}
	start.DispatchLeaseUntil = leaseUntil
	m.cronRunStarts[key] = start
	return start, true, nil
}

// MarkCronRunStartAccepted records that the current lease owner dispatched the reserved run.
func (m *MemoryStore) MarkCronRunStartAccepted(_ context.Context, tenantID, idempotencyKey, owner string) error {
	key := tenantID + "\x00" + idempotencyKey
	m.mu.Lock()
	defer m.mu.Unlock()
	start, ok := m.cronRunStarts[key]
	if !ok {
		return fmt.Errorf("store: cron run start binding not found")
	}
	if owner == "" || start.DispatchOwner != owner {
		return ErrCronRunDispatchLeaseLost
	}
	start.Accepted = true
	m.cronRunStarts[key] = start
	return nil
}

// CreateRun persists a new run record.
func (m *MemoryStore) CreateRun(_ context.Context, run *Run) error {
	if run.ID == "" {
		return fmt.Errorf("store: run ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.runs[run.ID]; exists {
		return fmt.Errorf("store: run %q already exists", run.ID)
	}
	if run.ConversationID != "" {
		tenantID, agentID := normalizeConversationOwner(run.TenantID, run.AgentID)
		if owner, exists := m.conversationOwners[run.ConversationID]; exists {
			if owner.tenantID != tenantID || owner.agentID != agentID {
				return fmt.Errorf("store: conversation %q: %w", run.ConversationID, ErrConversationOwnerConflict)
			}
		} else {
			// All remaining in-memory operations are non-failing, so this claim
			// and the run insertion form one lock-scoped atomic operation.
			m.conversationOwners[run.ConversationID] = conversationRunOwner{tenantID: tenantID, agentID: agentID}
		}
	}
	cp := copyRun(run)
	m.runs[run.ID] = cp
	return nil
}

// UpdateRun overwrites an existing run record.
func (m *MemoryStore) UpdateRun(_ context.Context, run *Run) error {
	if run.ID == "" {
		return fmt.Errorf("store: run ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := copyRun(run)
	m.runs[run.ID] = cp
	return nil
}

// GetRun retrieves a run by ID.
func (m *MemoryStore) GetRun(_ context.Context, id string) (*Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.runs[id]
	if !ok {
		return nil, &NotFoundError{ID: id}
	}
	cp := copyRun(run)
	return cp, nil
}

// ListRuns returns runs matching filter, ordered by created_at DESC.
func (m *MemoryStore) ListRuns(_ context.Context, filter RunFilter) ([]*Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Run
	for _, run := range m.runs {
		if filter.ConversationID != "" && run.ConversationID != filter.ConversationID {
			continue
		}
		if filter.TenantID != "" && run.TenantID != filter.TenantID {
			continue
		}
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		result = append(result, copyRun(run))
	}

	// Sort by created_at DESC (newest first).
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	if result == nil {
		result = []*Run{}
	}
	return result, nil
}

// AppendMessage appends a message to a run's message log.
// Returns an error if the run does not exist or if (run_id, seq) is already present.
func (m *MemoryStore) AppendMessage(_ context.Context, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[msg.RunID]; !ok {
		return fmt.Errorf("store: run %q not found", msg.RunID)
	}
	for _, existing := range m.messages[msg.RunID] {
		if existing.Seq == msg.Seq {
			return fmt.Errorf("store: message (run_id=%s, seq=%d) already exists", msg.RunID, msg.Seq)
		}
	}
	cp := copyMessage(msg)
	m.messages[msg.RunID] = append(m.messages[msg.RunID], cp)
	return nil
}

// GetMessages returns all messages for a run, ordered by seq ASC.
func (m *MemoryStore) GetMessages(_ context.Context, runID string) ([]*Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := m.messages[runID]
	result := make([]*Message, len(msgs))
	for i, msg := range msgs {
		result[i] = copyMessage(msg)
	}
	// Sort by seq (they may have been inserted out of order in concurrent tests).
	sort.Slice(result, func(i, j int) bool {
		return result[i].Seq < result[j].Seq
	})
	return result, nil
}

// AppendEvent appends an event to a run's event log.
// Returns an error if the run does not exist or if (run_id, seq) is already present.
func (m *MemoryStore) AppendEvent(_ context.Context, event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[event.RunID]; !ok {
		return fmt.Errorf("store: run %q not found", event.RunID)
	}
	for _, existing := range m.events[event.RunID] {
		if existing.Seq == event.Seq {
			return fmt.Errorf("store: event (run_id=%s, seq=%d) already exists", event.RunID, event.Seq)
		}
	}
	cp := copyEvent(event)
	m.nextEventCursor++
	cp.Cursor = m.nextEventCursor
	event.Cursor = cp.Cursor
	m.events[event.RunID] = append(m.events[event.RunID], cp)
	return nil
}

// GetEvents returns events for a run with seq > afterSeq, ordered by seq ASC.
func (m *MemoryStore) GetEvents(_ context.Context, runID string, afterSeq int) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := m.events[runID]

	var result []*Event
	for _, e := range all {
		if e.Seq > afterSeq {
			result = append(result, copyEvent(e))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Seq < result[j].Seq
	})
	if result == nil {
		result = []*Event{}
	}
	return result, nil
}

// GetConversationEvents returns a globally ordered, tenant-scoped page across
// every run on one conversation.
func (m *MemoryStore) GetConversationEvents(
	_ context.Context, filter ConversationEventFilter,
) (ConversationEventPage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	page := ConversationEventPage{CursorFound: filter.AfterEventID == ""}
	var scoped []*Event
	var afterCursor int64
	for runID, events := range m.events {
		run := m.runs[runID]
		if run == nil || run.ConversationID != filter.ConversationID {
			continue
		}
		if filter.TenantID != "" && run.TenantID != filter.TenantID {
			continue
		}
		for _, event := range events {
			if filter.AfterEventID != "" && event.EventID == filter.AfterEventID {
				afterCursor = event.Cursor
				page.CursorFound = true
			}
			scoped = append(scoped, copyEvent(event))
		}
	}

	sort.Slice(scoped, func(i, j int) bool {
		return scoped[i].Cursor < scoped[j].Cursor
	})
	if filter.AfterEventID != "" && page.CursorFound {
		filtered := scoped[:0]
		for _, event := range scoped {
			if event.Cursor > afterCursor {
				filtered = append(filtered, event)
			}
		}
		scoped = filtered
	}

	if filter.Limit > 0 && len(scoped) > filter.Limit {
		page.Truncated = true
		scoped = scoped[:filter.Limit]
	}
	if scoped == nil {
		scoped = []*Event{}
	}
	page.Events = scoped
	return page, nil
}

// Close is a no-op for the in-memory store.
func (m *MemoryStore) Close() error { return nil }

// --- copy helpers to prevent aliasing ---

func copyRun(r *Run) *Run {
	if r == nil {
		return nil
	}
	cp := *r
	if r.Recap != nil {
		recap := *r.Recap
		recap.ChangedFiles = append([]string(nil), r.Recap.ChangedFiles...)
		recap.TestsRun = append([]string(nil), r.Recap.TestsRun...)
		recap.UsefulCommands = append([]string(nil), r.Recap.UsefulCommands...)
		cp.Recap = &recap
	}
	return &cp
}

func copyMessage(m *Message) *Message {
	if m == nil {
		return nil
	}
	cp := *m
	return &cp
}

func copyEvent(e *Event) *Event {
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}
