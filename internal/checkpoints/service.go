package checkpoints

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrAlreadyResolved = errors.New("checkpoint already resolved")

type Service struct {
	store             Store
	now               func() time.Time
	resolutionLocksMu sync.Mutex
	resolutionLocks   map[string]*resolutionLock
	mu                sync.Mutex
	waiters           map[string][]chan waitResult
}

type resolutionLock struct {
	token chan struct{}
	refs  int
}

type waitResult struct {
	result WaitResult
	err    error
}

func NewService(store Store, now func() time.Time) *Service {
	if store == nil {
		store = NewMemoryStore()
	}
	if now == nil {
		now = time.Now
	}
	return &Service{
		store:           store,
		now:             now,
		resolutionLocks: make(map[string]*resolutionLock),
		waiters:         make(map[string][]chan waitResult),
	}
}

func (s *Service) Store() Store {
	return s.store
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Record, error) {
	now := s.now().UTC()
	record := Record{
		ID:             "checkpoint_" + uuid.NewString(),
		Kind:           req.Kind,
		Status:         StatusPending,
		RunID:          req.RunID,
		WorkflowRunID:  req.WorkflowRunID,
		CallID:         req.CallID,
		Tool:           req.Tool,
		Args:           req.Args,
		Questions:      req.Questions,
		SuspendPayload: req.SuspendPayload,
		ResumeSchema:   req.ResumeSchema,
		DeadlineAt:     req.DeadlineAt.UTC(),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.store.Create(ctx, &record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) Get(ctx context.Context, id string) (Record, error) {
	record, err := s.store.Get(ctx, id)
	if err != nil {
		return Record{}, err
	}
	return *record, nil
}

func (s *Service) PendingByRun(ctx context.Context, runID string) (Record, bool, error) {
	record, err := s.store.PendingByRun(ctx, runID)
	if err != nil {
		return Record{}, false, err
	}
	if record == nil {
		return Record{}, false, nil
	}
	return *record, true, nil
}

func (s *Service) PendingByWorkflowRun(ctx context.Context, workflowRunID string) (Record, bool, error) {
	record, err := s.store.PendingByWorkflowRun(ctx, workflowRunID)
	if err != nil {
		return Record{}, false, err
	}
	if record == nil {
		return Record{}, false, nil
	}
	return *record, true, nil
}

func (s *Service) Wait(ctx context.Context, id string) (WaitResult, error) {
	record, err := s.store.Get(ctx, id)
	if err != nil {
		return WaitResult{}, err
	}
	if record.Status != StatusPending {
		return waitResultFromRecord(record)
	}

	ch := make(chan waitResult, 1)
	s.mu.Lock()
	s.waiters[id] = append(s.waiters[id], ch)
	s.mu.Unlock()

	record, err = s.store.Get(ctx, id)
	if err != nil {
		s.unregister(id, ch)
		return WaitResult{}, err
	}
	if record.Status != StatusPending {
		s.unregister(id, ch)
		return waitResultFromRecord(record)
	}

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case outcome := <-ch:
			return outcome.result, outcome.err
		case <-ticker.C:
			record, err := s.store.Get(ctx, id)
			if err != nil {
				// Cross-Service polling is opportunistic. Once the pending record
				// and local waiter are established, a transient read outage must not
				// discard that waiter or mask a later local/remote resolution. The
				// caller's context remains the termination boundary.
				continue
			}
			// A resolution owned by this Service notifies only after its store
			// call has fully returned. Do not let polling observe the durable
			// row early and overtake that local completion boundary. Polling is
			// the fallback only for resolutions performed by another Service.
			if record.Status != StatusPending && !s.resolutionActive(id) {
				s.unregister(id, ch)
				return waitResultFromRecord(record)
			}
		case <-ctx.Done():
			s.unregister(id, ch)
			return WaitResult{}, ctx.Err()
		}
	}
}

func (s *Service) Resume(ctx context.Context, id string, payload map[string]any) error {
	return s.resolve(ctx, id, StatusResumed, payload)
}

func (s *Service) Approve(ctx context.Context, id string) error {
	return s.resolve(ctx, id, StatusApproved, nil)
}

// ApproveWithPayload resolves the checkpoint as approved and records payload
// (e.g. the operator's selected plan approach option) as the resume payload
// returned to the waiter.
func (s *Service) ApproveWithPayload(ctx context.Context, id string, payload map[string]any) error {
	return s.resolve(ctx, id, StatusApproved, payload)
}

func (s *Service) Deny(ctx context.Context, id string) error {
	return s.resolve(ctx, id, StatusDenied, nil)
}

func (s *Service) Expire(ctx context.Context, id string) error {
	return s.resolve(ctx, id, StatusExpired, nil)
}

// ExpirePending atomically expires an unresolved checkpoint. It returns false
// when another resolution already won, without overwriting that result.
func (s *Service) ExpirePending(ctx context.Context, id string) (bool, error) {
	unlock, err := s.acquireResolution(ctx, id)
	if err != nil {
		return false, err
	}
	defer unlock()
	record, won, err := s.store.ResolvePending(
		ctx,
		id,
		StatusExpired,
		"",
		s.now().UTC(),
	)
	if err != nil {
		return false, err
	}
	if !won {
		return false, nil
	}
	result, resultErr := waitResultFromRecord(record)
	s.notify(id, waitResult{result: result, err: resultErr})
	return true, resultErr
}

func (s *Service) resolve(ctx context.Context, id string, status Status, payload map[string]any) error {
	resumePayload := ""
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal checkpoint payload: %w", err)
		}
		resumePayload = string(raw)
	}
	unlock, err := s.acquireResolution(ctx, id)
	if err != nil {
		return err
	}
	defer unlock()
	record, won, err := s.store.ResolvePending(
		ctx,
		id,
		status,
		resumePayload,
		s.now().UTC(),
	)
	if err != nil {
		return err
	}
	if !won {
		return fmt.Errorf("%w: id=%s status=%s", ErrAlreadyResolved, id, record.Status)
	}
	result, err := waitResultFromRecord(record)
	s.notify(id, waitResult{result: result, err: err})
	return err
}

func (s *Service) acquireResolution(ctx context.Context, id string) (func(), error) {
	s.resolutionLocksMu.Lock()
	lock := s.resolutionLocks[id]
	if lock == nil {
		lock = &resolutionLock{token: make(chan struct{}, 1)}
		lock.token <- struct{}{}
		s.resolutionLocks[id] = lock
	}
	lock.refs++
	s.resolutionLocksMu.Unlock()

	select {
	case <-lock.token:
		return func() {
			lock.token <- struct{}{}
			s.releaseResolutionRef(id, lock)
		}, nil
	case <-ctx.Done():
		s.releaseResolutionRef(id, lock)
		return nil, ctx.Err()
	}
}

func (s *Service) releaseResolutionRef(id string, lock *resolutionLock) {
	s.resolutionLocksMu.Lock()
	lock.refs--
	if lock.refs == 0 && s.resolutionLocks[id] == lock {
		delete(s.resolutionLocks, id)
	}
	s.resolutionLocksMu.Unlock()
}

func (s *Service) resolutionActive(id string) bool {
	s.resolutionLocksMu.Lock()
	defer s.resolutionLocksMu.Unlock()
	return s.resolutionLocks[id] != nil
}

func waitResultFromRecord(record *Record) (WaitResult, error) {
	result := WaitResult{Status: record.Status}
	if record.ResumePayload == "" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(record.ResumePayload), &result.Payload); err != nil {
		return WaitResult{}, fmt.Errorf("decode resume payload: %w", err)
	}
	return result, nil
}

func (s *Service) notify(id string, outcome waitResult) {
	s.mu.Lock()
	waiters := append([]chan waitResult(nil), s.waiters[id]...)
	delete(s.waiters, id)
	s.mu.Unlock()
	for _, ch := range waiters {
		ch <- outcome
	}
}

func (s *Service) unregister(id string, target chan waitResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	waiters := s.waiters[id]
	if len(waiters) == 0 {
		return
	}
	filtered := waiters[:0]
	for _, ch := range waiters {
		if ch != target {
			filtered = append(filtered, ch)
		}
	}
	if len(filtered) == 0 {
		delete(s.waiters, id)
		return
	}
	s.waiters[id] = filtered
}
