package tools

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrCallbackCancelConflict            = errors.New("callback cannot be canceled in its current state")
	ErrCallbackRecoveryAuthorityRequired = errors.New("callback recovery requires workspace process-loss authority")
)

// Constants
const (
	MaxCallbackDelay    = 1 * time.Hour
	MinCallbackDelay    = 5 * time.Second
	MaxCallbacksPerConv = 10
)

// RunStarter is the interface for starting a new run on a conversation.
// Implemented by the runner; injected via lazy adapter to avoid circular deps.
//
// tenantID and agentID carry the originating run's scope so a callback fired
// from a tenant- or agent-scoped conversation starts its follow-up run on the
// SAME scope. Without them the follow-up run is denied access to the scoped
// conversation at fire time (a direct autonomy breaker). Both may be empty for
// the default/unscoped case.
type RunStarter interface {
	StartRun(prompt, conversationID, tenantID, agentID string) error
}

// CallbackRunStarter is the #1006 durable admission boundary. Implementations
// must reconcile reservedRunID and return the one admitted identity.
type CallbackRunStarter interface {
	StartCallback(context.Context, CallbackInfo) (string, error)
}

// CallbackStartError lets an authoritative admission adapter classify a
// failure without exposing the underlying provider/store error in durable
// callback status. Unclassified errors are treated as retryable and persisted
// as the generic safe summary "callback admission unavailable".
type CallbackStartError struct {
	Err     error
	Retry   bool
	Summary string
}

func (e *CallbackStartError) Error() string {
	if e == nil || e.Err == nil {
		return "callback admission failed"
	}
	return e.Err.Error()
}

func (e *CallbackStartError) Unwrap() error { return e.Err }

// SetRequest carries the parameters for scheduling a delayed callback. It is a
// small struct (rather than a long positional argument list) so the run scope
// (tenant + agent) can be threaded through Set -> fire -> StartRun without
// repeated signature churn.
type SetRequest struct {
	ConversationID string
	Delay          time.Duration
	Prompt         string
	// TenantID and AgentID capture the originating run's scope so the fired
	// follow-up run is started on the same tenant + agent. Both may be empty
	// for the default/unscoped case.
	TenantID string
	AgentID  string
}

// CallbackEvents is an optional sink for callback lifecycle notifications.
// The CallbackManager calls Emit for every externally observable transition.
// Event names match the harness EventType string values; the manager uses
// plain strings to avoid importing the runner event bus (no import cycle).
//
// Implementations MUST be safe for concurrent use and MUST NOT call back into
// the CallbackManager. A nil sink disables emission entirely.
type CallbackEvents interface {
	Emit(event string, info CallbackInfo)
}

// Event name constants for the callback lifecycle. These mirror the harness
// EventType string values but live here so the tools package stays free of a
// dependency on the runner.
const (
	eventCallbackScheduled   = "callback.scheduled"
	eventCallbackDispatching = "callback.dispatching"
	eventCallbackRetryWait   = "callback.retry_wait"
	eventCallbackStarted     = "callback.started"
	eventCallbackFailed      = "callback.failed"
	eventCallbackFired       = "callback.fired"
	eventCallbackCanceled    = "callback.canceled"
)

// CallbackOption configures a CallbackManager at construction time.
type CallbackOption func(*CallbackManager)

// callbackRecoveryAuthority is intentionally narrower than CallbackStore.
// Legacy/in-memory test stores retain their existing contracts, while the
// filesystem-backed SQLite implementation supplies a process-loss fence for
// recovery of expired dispatching rows.
type callbackRecoveryAuthority interface {
	AcquireCallbackRecoveryAuthority(context.Context) (func(), error)
}

func WithCallbackStore(store CallbackStore) CallbackOption {
	return func(m *CallbackManager) { m.store = store }
}

// WithEventSink wires an optional CallbackEvents sink onto the manager so
// callback lifecycle events are observable. Passing a nil sink is a no-op.
func WithEventSink(sink CallbackEvents) CallbackOption {
	return func(m *CallbackManager) {
		m.events = sink
	}
}

// CallbackState represents the lifecycle state of a callback.
type CallbackState string

const (
	CallbackStatePending     CallbackState = "pending"
	CallbackStateDispatching CallbackState = "dispatching"
	CallbackStateRetryWait   CallbackState = "retry_wait"
	CallbackStateStarted     CallbackState = "started"
	CallbackStateFailed      CallbackState = "failed"
	// Fired is retained as a legacy read value; new durable work uses started.
	CallbackStateFired    CallbackState = "fired"
	CallbackStateCanceled CallbackState = "canceled"
	// callbackStateDispatchingFenced is intentionally private and persisted.
	// Pre-#1106 binaries hard-code state='dispatching' when reclaiming an
	// expired lease, so they cannot take this current-version owner while its
	// admission is still unwinding. Public API/event reads normalize it back to
	// CallbackStateDispatching.
	callbackStateDispatchingFenced CallbackState = "dispatching_fenced"
)

// CallbackInfo holds metadata about a scheduled callback.
type CallbackInfo struct {
	ID             string        `json:"id"`
	ConversationID string        `json:"conversation_id"`
	Delay          string        `json:"delay"`
	Prompt         string        `json:"prompt"`
	State          CallbackState `json:"state"`
	FiresAt        time.Time     `json:"fires_at"`
	CreatedAt      time.Time     `json:"created_at"`
	// TenantID and AgentID capture the originating run's scope so the fired
	// follow-up run is started on the same tenant + agent. Both may be empty
	// for the default/unscoped case. Omitted from JSON when empty to preserve
	// the existing tool-result shape for unscoped callbacks.
	TenantID           string    `json:"tenant_id,omitempty"`
	AgentID            string    `json:"agent_id,omitempty"`
	RunID              string    `json:"run_id,omitempty"`
	Attempt            int       `json:"attempt"`
	NextAttemptAt      time.Time `json:"next_attempt_at,omitzero"`
	LastError          string    `json:"last_error,omitempty"`
	DispatchToken      string    `json:"-"`
	DispatchLeaseUntil time.Time `json:"-"`
}

type pendingCallback struct {
	info              CallbackInfo
	timer             *time.Timer
	claimRetryAttempt int
	// recoveryToken is set only for a current-version dispatch observed during
	// successful bootstrap under process-loss authority. Holding the workspace
	// lock later does not prove an in-process admission died.
	recoveryToken string
}

// CallbackManager manages delayed callbacks for agent conversations.
type CallbackManager struct {
	mu              sync.Mutex
	callbacks       map[string]*pendingCallback // keyed by callback ID
	byConv          map[string][]string         // conversation ID -> callback IDs
	starter         RunStarter
	callbackStarter CallbackRunStarter
	now             func() time.Time
	stopped         bool
	// events is an optional sink for callback lifecycle notifications. Nil by
	// default (no emission). Set via WithEventSink. Read-only after construction.
	events       CallbackEvents
	store        CallbackStore
	dispatchWG   sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	retryBase    time.Duration
	leaseTime    time.Duration
	maxAttempts  int
	claimRetries int
	claimBackoff time.Duration
	// recoveryRelease keeps the workspace process-loss fence for this
	// manager's full lifetime. It is released only after Shutdown has waited
	// for all admissions, or when Recover itself fails before ownership.
	recoveryRelease func()
}

// NewCallbackManager creates a new CallbackManager. Optional CallbackOption
// values (e.g. WithEventSink) configure observability; the zero-option call
// NewCallbackManager(starter) remains valid and emits no events.
func NewCallbackManager(starter RunStarter, opts ...CallbackOption) *CallbackManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &CallbackManager{
		callbacks:    make(map[string]*pendingCallback),
		byConv:       make(map[string][]string),
		starter:      starter,
		now:          time.Now,
		ctx:          ctx,
		cancel:       cancel,
		retryBase:    time.Second,
		leaseTime:    30 * time.Second,
		maxAttempts:  3,
		claimRetries: 3,
		claimBackoff: 10 * time.Millisecond,
	}
	if durableStarter, ok := starter.(CallbackRunStarter); ok {
		m.callbackStarter = durableStarter
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// emitEvent forwards a callback lifecycle event to the configured sink, if any.
// It is always called OUTSIDE the manager lock so the sink cannot deadlock or
// re-enter the manager while it holds m.mu.
func (m *CallbackManager) emitEvent(event string, info CallbackInfo) {
	if m.events == nil {
		return
	}
	m.events.Emit(event, info)
}

// removeFromByConv removes id from the per-conversation byConv slice, freeing
// the slot for future callbacks. It prunes the map entry when the slice
// becomes empty. Callers MUST hold m.mu.
func (m *CallbackManager) removeFromByConv(convID, id string) {
	ids := m.byConv[convID]
	for i, v := range ids {
		if v == id {
			// Swap-remove: replace the found element with the last, then shorten.
			ids[i] = ids[len(ids)-1]
			ids[len(ids)-1] = "" // zero for GC
			ids = ids[:len(ids)-1]
			break
		}
	}
	if len(ids) == 0 {
		delete(m.byConv, convID)
	} else {
		m.byConv[convID] = ids
	}
}

// Set schedules a new delayed callback. The SetRequest carries the
// conversation, delay, prompt, and the originating run's scope (tenant +
// agent); the scope is stored on the callback and threaded through to StartRun
// when the callback fires so the follow-up run runs on the same tenant + agent.
func (m *CallbackManager) Set(req SetRequest) (CallbackInfo, error) {
	conversationID := req.ConversationID
	delay := req.Delay
	prompt := req.Prompt

	if delay < MinCallbackDelay {
		return CallbackInfo{}, fmt.Errorf("delay %v is less than minimum %v", delay, MinCallbackDelay)
	}
	if delay > MaxCallbackDelay {
		return CallbackInfo{}, fmt.Errorf("delay %v exceeds maximum %v", delay, MaxCallbackDelay)
	}
	if prompt == "" {
		return CallbackInfo{}, fmt.Errorf("prompt must not be empty")
	}
	if m.store != nil && m.callbackStarter == nil {
		return CallbackInfo{}, fmt.Errorf("durable callback starter is required")
	}
	// A durable manager participates in the workspace process-loss fence from
	// creation onward, not only when Recover happens at bootstrap. Otherwise a
	// normal Set/fire manager could be live but unfenced while a second newer
	// daemon mistakes its expired clock lease for a crash.
	if err := m.ensureRecoveryAuthority(context.Background()); err != nil {
		return CallbackInfo{}, err
	}

	m.mu.Lock()

	if m.stopped {
		m.mu.Unlock()
		return CallbackInfo{}, fmt.Errorf("callback manager is shut down")
	}

	// Check per-conversation limit
	if len(m.byConv[conversationID]) >= MaxCallbacksPerConv {
		m.mu.Unlock()
		return CallbackInfo{}, fmt.Errorf("conversation %s has reached the maximum of %d callbacks", conversationID, MaxCallbacksPerConv)
	}

	id := uuid.New().String()
	now := m.now().UTC()
	info := CallbackInfo{
		ID:             id,
		ConversationID: conversationID,
		Delay:          delay.String(),
		Prompt:         prompt,
		State:          CallbackStatePending,
		FiresAt:        now.Add(delay),
		CreatedAt:      now,
		TenantID:       req.TenantID,
		AgentID:        req.AgentID,
	}
	info.RunID = "run_callback_" + id

	if m.store != nil {
		if err := m.store.Create(context.Background(), info); err != nil {
			m.mu.Unlock()
			return CallbackInfo{}, err
		}
	}
	m.callbacks[id] = &pendingCallback{info: info}
	m.scheduleLocked(m.callbacks[id], info.FiresAt)
	m.byConv[conversationID] = append(m.byConv[conversationID], id)
	m.mu.Unlock()

	// Emit outside the lock so the sink cannot re-enter the manager.
	m.emitEvent(eventCallbackScheduled, info)

	return info, nil
}

// Cancel cancels a pending callback.
func (m *CallbackManager) Cancel(id string) (CallbackInfo, error) {
	m.mu.Lock()

	cb, ok := m.callbacks[id]
	if !ok && m.store == nil {
		m.mu.Unlock()
		return CallbackInfo{}, fmt.Errorf("callback %s not found", id)
	}
	if m.store != nil {
		info, err := m.store.CancelPending(context.Background(), id)
		if err != nil {
			current, getErr := m.store.Get(context.Background(), id)
			m.mu.Unlock()
			if getErr != nil {
				return CallbackInfo{}, fmt.Errorf("callback %s not found", id)
			}
			if isDispatchingCallbackState(current.State) || current.State == CallbackStateStarted || current.State == CallbackStateFailed || current.State == CallbackStateFired {
				return CallbackInfo{}, fmt.Errorf("%w: callback %s is %s", ErrCallbackCancelConflict, id, publicCallbackInfo(current).State)
			}
			return CallbackInfo{}, err
		}
		if cb == nil {
			cb = &pendingCallback{}
			m.callbacks[id] = cb
		}
		if cb.timer != nil {
			cb.timer.Stop()
		}
		cb.info = info
		m.removeFromByConv(info.ConversationID, id)
		m.mu.Unlock()
		m.emitEvent(eventCallbackCanceled, info)
		return info, nil
	}

	switch cb.info.State {
	case CallbackStateFired:
		m.mu.Unlock()
		return CallbackInfo{}, fmt.Errorf("%w: callback %s already fired", ErrCallbackCancelConflict, id)
	case CallbackStateDispatching, callbackStateDispatchingFenced, CallbackStateStarted, CallbackStateFailed:
		state := publicCallbackInfo(cb.info).State
		m.mu.Unlock()
		return CallbackInfo{}, fmt.Errorf("%w: callback %s is %s", ErrCallbackCancelConflict, id, state)
	case CallbackStateCanceled:
		m.mu.Unlock()
		return CallbackInfo{}, fmt.Errorf("callback %s already canceled", id)
	}

	info := cb.info
	info.State = CallbackStateCanceled
	if m.store != nil {
		if err := m.store.Update(context.Background(), info); err != nil {
			m.mu.Unlock()
			return CallbackInfo{}, err
		}
	}
	// Persist first: a failed durable cancellation must leave the existing
	// timer armed so pending work is not silently stranded.
	cb.timer.Stop()
	cb.info = info
	// Remove from byConv so the slot is freed for future callbacks on this
	// conversation. The callbacks map entry is kept so state can still be
	// queried by white-box tests; only the per-conversation slot matters.
	m.removeFromByConv(info.ConversationID, id)
	m.mu.Unlock()

	// Emit outside the lock so the sink cannot re-enter the manager.
	m.emitEvent(eventCallbackCanceled, info)

	return info, nil
}

// List returns all callbacks for a conversation.
func (m *CallbackManager) List(conversationID string) []CallbackInfo {
	result, err := m.ListCallbacks(context.Background(), conversationID)
	if err != nil {
		log.Printf("list durable callbacks for conversation: %v", err)
		return nil
	}
	return result
}

// ListCallbacks is the error-aware conversation-scoped listing boundary used
// by the agent tool. Durable read failures must not look like an empty callback
// list to the model.
func (m *CallbackManager) ListCallbacks(ctx context.Context, conversationID string) ([]CallbackInfo, error) {
	if m.store != nil {
		all, err := m.ListAllCallbacks(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]CallbackInfo, 0, len(all))
		for _, info := range all {
			if info.ConversationID == conversationID {
				result = append(result, info)
			}
		}
		return result, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := m.byConv[conversationID]
	result := make([]CallbackInfo, 0, len(ids))
	for _, id := range ids {
		if cb, ok := m.callbacks[id]; ok {
			result = append(result, cb.info)
		}
	}
	return result, nil
}

// ListAll returns every durable callback state across every conversation. The
// in-memory compatibility path returns only callbacks still tracked in its
// per-conversation active index.
func (m *CallbackManager) ListAll() []CallbackInfo {
	result, err := m.ListAllCallbacks(context.Background())
	if err != nil {
		log.Printf("list durable callbacks: %v", err)
		return nil
	}
	return result
}

// ListAllCallbacks is the error-aware listing boundary for API consumers that
// must never confuse a failed durable read with a complete task inventory.
// ListAll retains the legacy slice-only contract and returns nil on error.
func (m *CallbackManager) ListAllCallbacks(ctx context.Context) ([]CallbackInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if m.store != nil {
		all, err := m.store.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		for index := range all {
			all[index] = publicCallbackInfo(all[index])
		}
		return all, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]CallbackInfo, 0, len(m.callbacks))
	for _, ids := range m.byConv {
		for _, id := range ids {
			if cb, ok := m.callbacks[id]; ok {
				result = append(result, cb.info)
			}
		}
	}
	return result, nil
}

// Shutdown stops all pending callbacks and prevents new ones.
func (m *CallbackManager) Shutdown() {
	m.mu.Lock()

	m.stopped = true
	m.cancel()
	var canceled []CallbackInfo
	for _, cb := range m.callbacks {
		if cb.info.State == CallbackStatePending || cb.info.State == CallbackStateRetryWait || isDispatchingCallbackState(cb.info.State) {
			if cb.timer != nil {
				cb.timer.Stop()
			}
			if m.store == nil {
				// Preserve the historical in-memory manager contract. Durable
				// managers retain pending rows for restart recovery instead.
				cb.info.State = CallbackStateCanceled
				m.removeFromByConv(cb.info.ConversationID, cb.info.ID)
				canceled = append(canceled, cb.info)
			}
		}
	}
	m.mu.Unlock()
	for _, info := range canceled {
		m.emitEvent(eventCallbackCanceled, info)
	}
	// A fire that committed before the shutdown fence owns one dispatch. Wait
	// for it so callers never observe shutdown complete while StartRun is live.
	m.dispatchWG.Wait()
	m.mu.Lock()
	releaseRecovery := m.recoveryRelease
	m.recoveryRelease = nil
	m.mu.Unlock()
	if releaseRecovery != nil {
		releaseRecovery()
	}
}

// fire is called by the timer when a callback is ready.
func (m *CallbackManager) fire(id string) {
	m.mu.Lock()
	cb, ok := m.callbacks[id]
	if m.stopped || !ok {
		m.mu.Unlock()
		return
	}
	if m.store != nil {
		if cb.info.State != CallbackStatePending && cb.info.State != CallbackStateRetryWait && !isDispatchingCallbackState(cb.info.State) {
			m.mu.Unlock()
			return
		}
		m.dispatchWG.Add(1)
		m.mu.Unlock()
		defer m.dispatchWG.Done()
		m.dispatchDurable(id)
		return
	}
	if cb.info.State != CallbackStatePending {
		m.mu.Unlock()
		return
	}
	m.fireLegacyLocked(id, cb)
}

// fireLegacyLocked preserves the #1005 in-memory behavior for callers that
// intentionally construct a manager without durable persistence. m.mu is held
// on entry and released before the starter is called.
func (m *CallbackManager) fireLegacyLocked(id string, cb *pendingCallback) {
	info := cb.info
	info.State = CallbackStateFired
	cb.info = info
	convID := info.ConversationID
	prompt := info.Prompt
	tenantID := info.TenantID
	agentID := info.AgentID
	// Remove from byConv so the slot is freed for future callbacks on this
	// conversation. The callbacks map entry is kept so state can still be
	// queried; only the per-conversation slot counter matters for the limit.
	m.removeFromByConv(convID, id)
	m.dispatchWG.Add(1)
	m.mu.Unlock()

	// Emit outside the lock so the sink cannot re-enter the manager.
	m.emitEvent(eventCallbackFired, info)

	// Call StartRun outside the lock to avoid deadlocks. Carry the originating
	// run's tenant + agent so the follow-up run runs on the same scope.
	defer m.dispatchWG.Done()
	if err := m.starter.StartRun(prompt, convID, tenantID, agentID); err != nil {
		// Log error but callback is still marked as fired
		log.Printf("callback %s: StartRun error: %v", id, err)
	}
}

func (m *CallbackManager) dispatchDurable(id string) {
	if err := m.ensureRecoveryAuthority(context.Background()); err != nil {
		log.Printf("callback %s: establish workspace authority: %v", id, err)
		return
	}
	token := uuid.NewString()
	info, won, _, leaseUntil, err := m.claimWithRetry(id, token, m.store.ClaimDue)
	if err != nil {
		log.Printf("callback %s: claim due: %v", id, err)
		m.mu.Lock()
		if cb := m.callbacks[id]; cb != nil && !m.stopped {
			// A single best-effort retry made a durable pending row disappear
			// from an otherwise live daemon after two contention windows. Rearm
			// with bounded exponential backoff; this does not mutate durable
			// attempt state because no claim was ever owned.
			// Keep rearming for the manager lifetime. The counter saturates only
			// the backoff exponent; it never limits the number of retries.
			if cb.claimRetryAttempt < 5 {
				cb.claimRetryAttempt++
			}
			m.scheduleLocked(cb, m.now().Add(m.claimRetryDelay(cb.claimRetryAttempt)))
		}
		m.mu.Unlock()
		return
	}
	if !won {
		// A normal contender never takes an expired dispatching lease: the
		// former live owner must return and release its exact token. A manager
		// that already holds the workspace process-loss fence is different: it
		// may be recovering a crash orphan whose persisted future lease has just
		// reached its scheduled timer deadline.
		if isDispatchingCallbackState(info.State) {
			now := m.now().UTC()
			if info.State == callbackStateDispatchingFenced && m.hasRecoveryAuthority() {
				owned, getErr := m.store.Get(context.Background(), id)
				expectedToken := m.bootstrapRecoveryToken(id)
				if getErr == nil && expectedToken != "" && owned.DispatchToken == expectedToken && (owned.DispatchLeaseUntil.IsZero() || !owned.DispatchLeaseUntil.After(now)) {
					recovered, released, recoverErr := m.store.RecoverExpiredLease(context.Background(), id, expectedToken, now)
					if recoverErr == nil && released {
						m.syncDurableState(recovered, true)
						m.emitEvent(eventCallbackRetryWait, publicCallbackInfo(recovered))
						return
					}
				}
			}
			m.refreshDurableState(id)
			m.mu.Lock()
			if cb := m.callbacks[id]; cb != nil && !m.stopped {
				retry := m.claimBackoff
				if retry <= 0 {
					retry = time.Millisecond
				}
				m.scheduleLocked(cb, m.now().Add(retry))
			}
			m.mu.Unlock()
			return
		}
		m.syncDurableState(info, true)
		return
	}
	m.mu.Lock()
	if cb := m.callbacks[id]; cb != nil {
		cb.claimRetryAttempt = 0
		cb.recoveryToken = ""
	}
	m.mu.Unlock()
	m.syncDurableState(info, false)
	m.emitEvent(eventCallbackDispatching, publicCallbackInfo(info))

	dispatchCtx, cancel := context.WithCancel(m.ctx)
	heartbeatDone := make(chan struct{})
	heartbeatExited := make(chan struct{})
	leaseDeadlineUpdates := make(chan time.Time)
	leaseDeadlineExited := make(chan struct{})
	leaseDeadlineReached := make(chan struct{})
	var leaseDeadlineOnce sync.Once
	signalLeaseDeadline := func() { leaseDeadlineOnce.Do(func() { close(leaseDeadlineReached) }) }
	// Own deadline cancellation separately from heartbeat I/O. A busy SQLite
	// renewal can block until its context expires; the admission must still be
	// canceled at the last confirmed lease deadline before another manager may
	// reclaim the durable row.
	go func() {
		defer close(leaseDeadlineExited)
		deadline := leaseUntil
		timer := time.NewTimer(callbackLeaseWait(deadline))
		defer timer.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-dispatchCtx.Done():
				return
			case <-timer.C:
				signalLeaseDeadline()
				cancel()
				return
			case renewed := <-leaseDeadlineUpdates:
				// A renewal returned after the old deadline cannot revive a
				// dispatch that was already eligible for takeover.
				if !time.Now().UTC().Before(deadline) {
					cancel()
					return
				}
				deadline = renewed
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(callbackLeaseWait(deadline))
			}
		}
	}()
	go func() {
		defer close(heartbeatExited)
		interval := m.leaseTime / 4
		if interval <= 0 {
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		lastConfirmedDeadline := leaseUntil
		for {
			select {
			case <-heartbeatDone:
				return
			case <-dispatchCtx.Done():
				return
			case tick := <-ticker.C:
				// Bound a blocked SQLite renewal by the last persisted lease. A
				// busy operation that returns after this deadline
				// cannot safely keep the local admission alive.
				extendCtx, stopExtend := context.WithDeadline(dispatchCtx, lastConfirmedDeadline)
				ok, extendErr := m.store.ExtendLease(extendCtx, id, token, tick.UTC(), tick.UTC().Add(m.leaseTime))
				stopExtend()
				if extendErr == nil && ok {
					lastConfirmedDeadline = tick.UTC().Add(m.leaseTime)
					select {
					case leaseDeadlineUpdates <- lastConfirmedDeadline:
					case <-heartbeatDone:
						return
					case <-dispatchCtx.Done():
						return
					}
					continue
				}
				// A false response is a definitive token/state loss. A database
				// error is only transient until the last successful lease deadline;
				// stopping earlier lets another manager reclaim a still-owned run.
				if extendErr == nil || !time.Now().UTC().Before(lastConfirmedDeadline) {
					signalLeaseDeadline()
					cancel()
					return
				}
			}
		}
	}()
	runID, startErr := m.startCallback(dispatchCtx, info)
	close(heartbeatDone)
	admissionCanceled := dispatchCtx.Err() != nil
	deadlineCanceled := false
	if admissionCanceled && startErr != nil {
		select {
		case <-leaseDeadlineReached:
			deadlineCanceled = true
		default:
		}
	}
	// The heartbeat can still be blocked inside SQLite after StartCallback has
	// observed cancellation.  Release the owner token before waiting for that
	// I/O goroutine: its stale ExtendLease is token-fenced, while delaying this
	// write would make a live contender wait on unrelated database cleanup.
	if deadlineCanceled {
		if info.Attempt >= m.maxAttempts {
			// Deadline cancellation is an admission failure, not a new ownership
			// epoch. Once the persisted attempt budget is exhausted, terminalize
			// under the current token instead of repeatedly releasing/reclaiming
			// the same reserved callback forever.
			const summary = "callback admission unavailable"
			if err := m.store.MarkFailed(context.Background(), id, token, summary); err != nil {
				<-heartbeatExited
				<-leaseDeadlineExited
				m.refreshDurableState(id)
				return
			}
			updated := info
			updated.State = CallbackStateFailed
			updated.NextAttemptAt = time.Time{}
			updated.LastError = summary
			updated.DispatchToken = ""
			updated.DispatchLeaseUntil = time.Time{}
			m.syncDurableState(updated, false)
			m.emitEvent(eventCallbackFailed, publicCallbackInfo(updated))
		} else {
			next := m.now().UTC().Add(m.retryDelay(info.Attempt))
			const summary = "callback admission unavailable"
			if err := m.store.ReleaseLease(context.Background(), id, token, next, summary); err != nil {
				<-heartbeatExited
				<-leaseDeadlineExited
				m.refreshDurableState(id)
				return
			}
			updated := info
			updated.State = CallbackStateRetryWait
			updated.NextAttemptAt = next
			updated.LastError = summary
			updated.DispatchToken = ""
			updated.DispatchLeaseUntil = time.Time{}
			// The same manager is still the only workspace owner. Rearm the
			// released retry under the persisted exponential backoff.
			m.syncDurableState(updated, true)
			m.emitEvent(eventCallbackRetryWait, publicCallbackInfo(updated))
		}
	}
	<-heartbeatExited
	<-leaseDeadlineExited
	cancel()

	if admissionCanceled && startErr != nil {
		if !deadlineCanceled {
			// Shutdown or a definitive ownership loss leaves the durable claim
			// intact for an explicit process-recovery pass.
			m.syncDurableState(info, true)
		}
		return
	}
	if startErr == nil && runID != info.RunID {
		startErr = &CallbackStartError{Err: fmt.Errorf("admission returned run %q for reserved run %q", runID, info.RunID), Retry: false, Summary: "callback run identity conflict"}
	}
	if startErr != nil {
		retry, summary := classifyCallbackStartError(startErr)
		if retry && info.Attempt < m.maxAttempts {
			next := m.now().UTC().Add(m.retryDelay(info.Attempt))
			if err := m.store.MarkRetry(context.Background(), id, token, next, summary); err != nil {
				m.refreshDurableState(id)
				return
			}
			updated := info
			updated.State = CallbackStateRetryWait
			updated.NextAttemptAt = next
			updated.LastError = summary
			updated.DispatchToken = ""
			updated.DispatchLeaseUntil = time.Time{}
			m.syncDurableState(updated, true)
			m.emitEvent(eventCallbackRetryWait, publicCallbackInfo(updated))
			return
		}
		if err := m.store.MarkFailed(context.Background(), id, token, summary); err != nil {
			m.refreshDurableState(id)
			return
		}
		updated := info
		updated.State = CallbackStateFailed
		updated.NextAttemptAt = time.Time{}
		updated.LastError = summary
		updated.DispatchToken = ""
		updated.DispatchLeaseUntil = time.Time{}
		m.syncDurableState(updated, false)
		m.emitEvent(eventCallbackFailed, publicCallbackInfo(updated))
		return
	}
	if err := m.store.MarkStarted(context.Background(), id, token, runID); err != nil {
		m.refreshDurableState(id)
		return
	}
	updated := info
	updated.State = CallbackStateStarted
	updated.RunID = runID
	updated.NextAttemptAt = time.Time{}
	updated.DispatchToken = ""
	updated.DispatchLeaseUntil = time.Time{}
	m.syncDurableState(updated, false)
	m.emitEvent(eventCallbackStarted, publicCallbackInfo(updated))
	// Preserve the existing event for consumers while started becomes the
	// truthful durable terminal state for dispatch.
	m.emitEvent(eventCallbackFired, publicCallbackInfo(updated))
}

func callbackLeaseWait(deadline time.Time) time.Duration {
	wait := time.Until(deadline)
	if wait < 0 {
		return 0
	}
	return wait
}

type callbackClaimFunc func(context.Context, string, string, time.Time, time.Time) (CallbackInfo, bool, error)

// claimWithRetry absorbs bounded transient SQLite contention before a manager
// has claimed anything. The retry never changes the callback's reserved run
// identity and leaves durable recovery responsible after the bounded window.
func (m *CallbackManager) claimWithRetry(id, token string, claim callbackClaimFunc) (CallbackInfo, bool, time.Time, time.Time, error) {
	attempts := m.claimRetries
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		now := m.now().UTC()
		until := now.Add(m.leaseTime)
		info, won, err := claim(m.ctx, id, token, now, until)
		if err == nil {
			return info, won, now, until, nil
		}
		lastErr = err
		if attempt+1 == attempts {
			return CallbackInfo{}, false, now, until, lastErr
		}
		backoff := m.claimBackoff
		if backoff <= 0 {
			backoff = time.Millisecond
		}
		timer := time.NewTimer(backoff)
		select {
		case <-m.ctx.Done():
			timer.Stop()
			return CallbackInfo{}, false, now, until, m.ctx.Err()
		case <-timer.C:
		}
	}
	return CallbackInfo{}, false, time.Time{}, time.Time{}, lastErr
}

func (m *CallbackManager) startCallback(ctx context.Context, info CallbackInfo) (string, error) {
	if m.callbackStarter != nil {
		return m.callbackStarter.StartCallback(ctx, info)
	}
	return "", &CallbackStartError{Err: errors.New("callback runner is not configured"), Retry: false, Summary: "callback runner unavailable"}
}

func classifyCallbackStartError(err error) (bool, string) {
	var classified *CallbackStartError
	if errors.As(err, &classified) {
		summary := SafeCallbackErrorSummary(classified.Summary)
		if summary == "" {
			summary = "callback admission failed"
		}
		return classified.Retry, summary
	}
	return true, "callback admission unavailable"
}

// SafeCallbackErrorSummary returns only summaries owned by the callback
// admission boundary. Provider, database, and transport errors can contain
// credentials or customer data, so arbitrary text must never be persisted or
// exposed through tasks/SSE. An empty durable value remains empty; every
// unknown non-empty value collapses to the generic terminal summary.
func SafeCallbackErrorSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	switch summary {
	case "callback admission unavailable",
		"callback admission failed",
		"invalid callback scope",
		"callback run identity conflict",
		"callback runner unavailable":
		return summary
	default:
		return "callback admission failed"
	}
}

func (m *CallbackManager) retryDelay(attempt int) time.Duration {
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 4 {
		shift = 4
	}
	return m.retryBase * time.Duration(1<<shift)
}

func (m *CallbackManager) claimRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 5 {
		attempt = 5
	}
	base := m.claimBackoff
	if base <= 0 {
		base = time.Millisecond
	}
	return base * time.Duration(1<<(attempt-1))
}

func publicCallbackInfo(info CallbackInfo) CallbackInfo {
	if info.State == callbackStateDispatchingFenced {
		info.State = CallbackStateDispatching
	}
	info.DispatchToken = ""
	info.DispatchLeaseUntil = time.Time{}
	info.LastError = SafeCallbackErrorSummary(info.LastError)
	return info
}

func (m *CallbackManager) refreshDurableState(id string) {
	info, err := m.store.Get(context.Background(), id)
	if err == nil {
		m.syncDurableState(info, true)
	}
}

func (m *CallbackManager) hasRecoveryAuthority() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recoveryRelease != nil
}

func (m *CallbackManager) bootstrapRecoveryToken(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cb := m.callbacks[id]; cb != nil {
		return cb.recoveryToken
	}
	return ""
}

// ensureRecoveryAuthority keeps the filesystem process-loss fence for this
// manager's lifetime. SQLite-backed managers acquire it before every durable
// creation/dispatch path; lightweight fake stores remain usable in unit tests
// and cannot opt into Recover's crash-reclaim behavior.
func (m *CallbackManager) ensureRecoveryAuthority(ctx context.Context) error {
	if m.store == nil {
		return nil
	}
	authority, ok := m.store.(callbackRecoveryAuthority)
	if !ok {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return fmt.Errorf("callback manager is shut down")
	}
	if m.recoveryRelease != nil {
		return nil
	}
	// Serialize the store acquisition with Shutdown and other Set/Recover
	// callers. A check-then-acquire gap lets two goroutines both reach the same
	// store; the loser then fails a valid concurrent Set even though this manager
	// already owns the required authority.
	release, err := authority.AcquireCallbackRecoveryAuthority(ctx)
	if err != nil {
		return fmt.Errorf("acquire callback recovery authority: %w", err)
	}
	m.recoveryRelease = release
	return nil
}

func (m *CallbackManager) syncDurableState(info CallbackInfo, schedule bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cb := m.callbacks[info.ID]
	if cb == nil {
		cb = &pendingCallback{}
		m.callbacks[info.ID] = cb
	}
	cb.info = info
	if info.State != callbackStateDispatchingFenced || cb.recoveryToken != info.DispatchToken {
		cb.recoveryToken = ""
	}
	if isActiveCallbackState(info.State) {
		m.addToByConvLocked(info.ConversationID, info.ID)
		if schedule && !m.stopped {
			m.scheduleLocked(cb, callbackDueAt(info, m.now()))
		}
		return
	}
	if cb.timer != nil {
		cb.timer.Stop()
	}
	m.removeFromByConv(info.ConversationID, info.ID)
}

func isActiveCallbackState(state CallbackState) bool {
	return state == CallbackStatePending || state == CallbackStateRetryWait || isDispatchingCallbackState(state)
}

func isDispatchingCallbackState(state CallbackState) bool {
	return state == CallbackStateDispatching || state == callbackStateDispatchingFenced
}

func callbackDueAt(info CallbackInfo, fallback time.Time) time.Time {
	switch info.State {
	case CallbackStateRetryWait:
		if !info.NextAttemptAt.IsZero() {
			return info.NextAttemptAt
		}
	case CallbackStateDispatching, callbackStateDispatchingFenced:
		if !info.DispatchLeaseUntil.IsZero() {
			return info.DispatchLeaseUntil
		}
	case CallbackStatePending:
		if !info.FiresAt.IsZero() {
			return info.FiresAt
		}
	}
	return fallback
}

func (m *CallbackManager) addToByConvLocked(conversationID, id string) {
	for _, existing := range m.byConv[conversationID] {
		if existing == id {
			return
		}
	}
	m.byConv[conversationID] = append(m.byConv[conversationID], id)
}

func (m *CallbackManager) scheduleLocked(cb *pendingCallback, at time.Time) {
	if cb.timer != nil {
		cb.timer.Stop()
	}
	delay := at.Sub(m.now())
	if delay < 0 {
		delay = 0
	}
	id := cb.info.ID
	cb.timer = time.AfterFunc(delay, func() { m.fire(id) })
}

func (m *CallbackManager) Recover(ctx context.Context) error {
	if m.store == nil {
		return nil
	}
	if m.callbackStarter == nil {
		return fmt.Errorf("durable callback starter is required")
	}
	acquiredRecoveryAuthority := false
	defer func() {
		if !acquiredRecoveryAuthority {
			return
		}
		// A failed bootstrap must not strand its process-lifetime fence. Keep
		// it only after the durable snapshot has been fully installed and its
		// timers are armed below.
		m.mu.Lock()
		release := m.recoveryRelease
		m.recoveryRelease = nil
		m.mu.Unlock()
		if release != nil {
			release()
		}
	}()
	// A callback lease expiration alone is not evidence that its owner process
	// died. Hold a kernel-released workspace fence for this manager's lifetime
	// before doing any dispatch recovery, so a second harnessd bootstrap cannot
	// reclaim a live owner's expired row merely because it shares the database.
	if _, ok := m.store.(callbackRecoveryAuthority); !ok {
		return fmt.Errorf("%w: callback store does not expose a workspace fence", ErrCallbackRecoveryAuthorityRequired)
	}
	m.mu.Lock()
	alreadyHeld := m.recoveryRelease != nil
	m.mu.Unlock()
	if !alreadyHeld {
		if err := m.ensureRecoveryAuthority(ctx); err != nil {
			return err
		}
		acquiredRecoveryAuthority = true
	}
	// Read every state, not only timer-active rows. The durable callback store
	// is also the restart source for conversation lifecycle visibility: terminal
	// started/failed/canceled state must be republished after process memory is
	// lost, while only active rows are re-armed below.
	rows, err := m.store.ListAll(ctx)
	if err != nil {
		return err
	}
	// Recover is called at the harness bootstrap boundary, after shutdown has
	// established that the previous owner process is gone.  Only there may an
	// expired dispatching token be converted to retry_wait.  Ordinary timer
	// dispatches never steal that row: a live owner first returns from its
	// canceled admission and calls ReleaseLease itself.
	now := m.now().UTC()
	recoveryTokens := make(map[string]string)
	for index, info := range rows {
		if !isDispatchingCallbackState(info.State) {
			continue
		}
		// ListAll deliberately redacts private lease/token fields for callers.
		// Recovery alone re-reads a dispatching row through the internal store
		// boundary so it can distinguish a live lease from an abandoned one.
		owned, err := m.store.Get(ctx, info.ID)
		if err != nil {
			return fmt.Errorf("recover callback %s dispatch state: %w", info.ID, err)
		}
		locallyOwned := m.locallyOwnsDispatch(info.ID, owned.DispatchToken)
		if !owned.DispatchLeaseUntil.IsZero() && owned.DispatchLeaseUntil.After(now) {
			if owned.State == callbackStateDispatchingFenced && !locallyOwned {
				recoveryTokens[info.ID] = owned.DispatchToken
			}
			continue
		}
		if locallyOwned {
			continue
		}
		if owned.State != callbackStateDispatchingFenced {
			// A pre-fence/older manager may still be alive. Its expired lease has
			// no kernel-backed ownership proof, so fail closed rather than create
			// a second visible continuation. A confirmed current-version crash is
			// fenced and continues through the transition below.
			continue
		}
		recovered, released, err := m.store.RecoverExpiredLease(ctx, info.ID, owned.DispatchToken, now)
		if err != nil {
			return fmt.Errorf("recover callback %s expired dispatch: %w", info.ID, err)
		}
		if released {
			rows[index] = recovered
		}
	}
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return fmt.Errorf("callback manager is shut down")
	}
	for _, info := range rows {
		if !isActiveCallbackState(info.State) {
			continue
		}
		cb := m.callbacks[info.ID]
		if cb == nil {
			id := info.ID
			cb = &pendingCallback{info: info}
			m.callbacks[id] = cb
			m.addToByConvLocked(info.ConversationID, id)
		}
		if token := recoveryTokens[info.ID]; token != "" {
			cb.recoveryToken = token
		}
	}
	m.mu.Unlock()
	for _, info := range rows {
		if event := callbackRecoveryEvent(info.State); event != "" {
			m.emitEvent(event, publicCallbackInfo(info))
		}
	}
	// Publish the recovered durable snapshot before any overdue timer may move
	// it forward, preserving lifecycle order for a reconnecting conversation.
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return fmt.Errorf("callback manager is shut down")
	}
	for _, info := range rows {
		if cb := m.callbacks[info.ID]; cb != nil && isActiveCallbackState(cb.info.State) {
			m.scheduleLocked(cb, callbackDueAt(cb.info, m.now()))
		}
	}
	acquiredRecoveryAuthority = false
	return nil
}

func (m *CallbackManager) locallyOwnsDispatch(id, token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cb := m.callbacks[id]
	return token != "" && cb != nil && cb.info.State == callbackStateDispatchingFenced && cb.info.DispatchToken == token
}

func callbackRecoveryEvent(state CallbackState) string {
	switch state {
	case CallbackStatePending:
		return eventCallbackScheduled
	case CallbackStateDispatching, callbackStateDispatchingFenced:
		return eventCallbackDispatching
	case CallbackStateRetryWait:
		return eventCallbackRetryWait
	case CallbackStateStarted:
		return eventCallbackStarted
	case CallbackStateFailed:
		return eventCallbackFailed
	case CallbackStateFired:
		return eventCallbackFired
	case CallbackStateCanceled:
		return eventCallbackCanceled
	default:
		return ""
	}
}

// --- Tool Constructors ---
