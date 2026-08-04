package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/store"

	"github.com/google/uuid"
)

var (
	errCronRunIdempotencyConflict    = errors.New("idempotency key was already used for a different cron request")
	errCronRunIdempotencyUnavailable = errors.New("durable cron run idempotency is unavailable")
)

const (
	defaultCronRunDispatchLeaseDuration = 30 * time.Second
	defaultCronRunDispatchPollInterval  = 10 * time.Millisecond
)

type cronRunStartState struct {
	fingerprint string
	done        chan struct{}
	run         harness.Run
	err         error
}

type cronRunLeaseHeartbeat struct {
	runID string
	owner string
	// admitted becomes true only after the runner has accepted the identity.
	// Before then the heartbeat must renew even though GetRun cannot find it.
	admitted atomic.Bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// cronRunStartCache coalesces concurrent deliveries for one harnessd process.
// Completed entries are evicted because the durable binding is authoritative
// for sequential and restart-spanning replay.
type cronRunStartCache struct {
	mu      sync.Mutex
	entries map[string]*cronRunStartState
}

func newCronRunStartCache() *cronRunStartCache {
	return &cronRunStartCache{entries: make(map[string]*cronRunStartState)}
}

func (c *cronRunStartCache) getOrStart(ctx context.Context, tenantID, idempotencyKey, fingerprint string, start func() (harness.Run, error)) (harness.Run, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	key := tenantID + "\x00" + idempotencyKey

	c.mu.Lock()
	if existing, ok := c.entries[key]; ok {
		if existing.fingerprint != fingerprint {
			c.mu.Unlock()
			return harness.Run{}, errCronRunIdempotencyConflict
		}
		done := existing.done
		c.mu.Unlock()
		select {
		case <-done:
			return existing.run, existing.err
		case <-ctx.Done():
			return harness.Run{}, ctx.Err()
		}
	}
	state := &cronRunStartState{fingerprint: fingerprint, done: make(chan struct{})}
	c.entries[key] = state
	c.mu.Unlock()

	run, err := start()
	c.mu.Lock()
	state.run = run
	state.err = err
	close(state.done)
	delete(c.entries, key)
	c.mu.Unlock()
	return run, err
}

func (s *Server) getOrStartCronRun(
	ctx context.Context,
	req cronRunRequest,
	idempotencyKey string,
	runRequest harness.RunRequest,
) (harness.Run, error) {
	fingerprint := cronRunRequestFingerprint(req)
	return s.cronRunStartCache().getOrStart(ctx, req.TenantID, idempotencyKey, fingerprint, func() (harness.Run, error) {
		durable, ok := s.runStore.(store.CronRunStartStore)
		if !ok {
			return harness.Run{}, errCronRunIdempotencyUnavailable
		}
		binding, claimed, err := durable.ClaimCronRunStart(ctx, store.CronRunStart{
			TenantID:       req.TenantID,
			IdempotencyKey: idempotencyKey,
			Fingerprint:    fingerprint,
			RunID:          "run_" + uuid.NewString(),
			CreatedAt:      s.cronRunNow(),
		})
		if err != nil {
			return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, err)
		}
		if binding.Fingerprint != fingerprint {
			return harness.Run{}, errCronRunIdempotencyConflict
		}
		_ = claimed // The dispatch lease, not reservation creation, elects the dispatcher.
		owner := s.cronRunDispatchOwnerID()
		for {
			persisted, getErr := s.runStore.GetRun(ctx, binding.RunID)
			switch {
			case getErr == nil:
				if persisted.Status == store.RunStatusQueued {
					if active, exists := s.runner.GetRun(binding.RunID); exists {
						leased, acquired, leaseErr := s.acquireCronRunDispatchLease(ctx, durable, binding, owner)
						if leaseErr != nil {
							return harness.Run{}, leaseErr
						}
						binding = leased
						if !acquired {
							if binding.Accepted {
								return active, nil
							}
							if waitErr := s.waitForCronRunDispatch(ctx); waitErr != nil {
								return harness.Run{}, waitErr
							}
							continue
						}
						heartbeat := s.ensureCronRunDispatchLeaseHeartbeat(durable, binding, owner)
						heartbeat.admitted.Store(true)
						if markErr := durable.MarkCronRunStartAccepted(ctx, req.TenantID, idempotencyKey, owner); markErr != nil {
							return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, markErr)
						}
						return active, nil
					}
					leased, acquired, leaseErr := s.acquireCronRunDispatchLease(ctx, durable, binding, owner)
					if leaseErr != nil {
						return harness.Run{}, leaseErr
					}
					binding = leased
					if !acquired {
						if binding.Accepted {
							return cronHarnessRunFromStore(persisted), nil
						}
						if waitErr := s.waitForCronRunDispatch(ctx); waitErr != nil {
							return harness.Run{}, waitErr
						}
						continue
					}
					heartbeat := s.ensureCronRunDispatchLeaseHeartbeat(durable, binding, owner, true)
					admissionCtx, detachOwnerLoss, stopAdmission := cronRunAdmissionContext(ctx, heartbeat)
					run, resumeErr := s.runner.ResumeRunWithIDContext(admissionCtx, runRequest, binding.RunID)
					if resumeErr != nil {
						stopAdmission()
						if errors.Is(resumeErr, harness.ErrRunPersistence) {
							return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, resumeErr)
						}
						return harness.Run{}, resumeErr
					}
					detachOwnerLoss()
					heartbeat.admitted.Store(true)
					if markErr := durable.MarkCronRunStartAccepted(ctx, req.TenantID, idempotencyKey, owner); markErr != nil {
						return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, markErr)
					}
					return run, nil
				}
				if binding.Accepted {
					return cronHarnessRunFromStore(persisted), nil
				}
				leased, acquired, leaseErr := s.acquireCronRunDispatchLease(ctx, durable, binding, owner)
				if leaseErr != nil {
					return harness.Run{}, leaseErr
				}
				binding = leased
				if !acquired {
					if binding.Accepted {
						return cronHarnessRunFromStore(persisted), nil
					}
					if waitErr := s.waitForCronRunDispatch(ctx); waitErr != nil {
						return harness.Run{}, waitErr
					}
					continue
				}
				if markErr := durable.MarkCronRunStartAccepted(ctx, req.TenantID, idempotencyKey, owner); markErr != nil {
					return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, markErr)
				}
				return cronHarnessRunFromStore(persisted), nil
			case !store.IsNotFound(getErr):
				return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, getErr)
			}

			leased, acquired, leaseErr := s.acquireCronRunDispatchLease(ctx, durable, binding, owner)
			if leaseErr != nil {
				return harness.Run{}, leaseErr
			}
			binding = leased
			if !acquired {
				if waitErr := s.waitForCronRunDispatch(ctx); waitErr != nil {
					return harness.Run{}, waitErr
				}
				continue
			}

			heartbeat := s.ensureCronRunDispatchLeaseHeartbeat(durable, binding, owner, true)
			admissionCtx, detachOwnerLoss, stopAdmission := cronRunAdmissionContext(ctx, heartbeat)
			run, startErr := s.runner.StartRunWithIDContext(admissionCtx, runRequest, binding.RunID)
			if startErr != nil {
				stopAdmission()
				if errors.Is(startErr, harness.ErrRunPersistence) {
					return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, startErr)
				}
				return harness.Run{}, startErr
			}
			detachOwnerLoss()
			heartbeat.admitted.Store(true)
			if err := durable.MarkCronRunStartAccepted(ctx, req.TenantID, idempotencyKey, owner); err != nil {
				return harness.Run{}, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, err)
			}
			return run, nil
		}
	})
}

func (s *Server) cronRunDispatchOwnerID() string {
	s.cronRunDispatchOwnerOnce.Do(func() {
		s.cronRunDispatchOwner = "harnessd_" + uuid.NewString()
	})
	return s.cronRunDispatchOwner
}

func (s *Server) cronRunNow() time.Time {
	if s.timeNow != nil {
		return s.timeNow().UTC()
	}
	return time.Now().UTC()
}

func (s *Server) acquireCronRunDispatchLease(ctx context.Context, durable store.CronRunStartStore, binding store.CronRunStart, owner string) (store.CronRunStart, bool, error) {
	now := s.cronRunNow()
	duration := s.cronRunDispatchLeaseDuration
	if duration <= 0 {
		duration = defaultCronRunDispatchLeaseDuration
	}
	leased, acquired, err := durable.AcquireCronRunStartDispatchLease(ctx, binding.TenantID, binding.IdempotencyKey, owner, now, now.Add(duration))
	if err != nil {
		return store.CronRunStart{}, false, fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, err)
	}
	if acquired && leased.DispatchOwner != owner {
		return store.CronRunStart{}, false, fmt.Errorf("%w: acquired cron dispatch lease returned owner %q", errCronRunIdempotencyUnavailable, leased.DispatchOwner)
	}
	return leased, acquired, nil
}

func (s *Server) waitForCronRunDispatch(ctx context.Context) error {
	interval := s.cronRunDispatchPollInterval
	if interval <= 0 {
		interval = defaultCronRunDispatchPollInterval
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", errCronRunIdempotencyUnavailable, ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (s *Server) ensureCronRunDispatchLeaseHeartbeat(durable store.CronRunStartStore, binding store.CronRunStart, owner string, preAdmission ...bool) *cronRunLeaseHeartbeat {
	key := binding.TenantID + "\x00" + binding.IdempotencyKey
	s.cronRunLeaseHeartbeatMu.Lock()
	if s.cronRunLeaseHeartbeats == nil {
		s.cronRunLeaseHeartbeats = make(map[string]*cronRunLeaseHeartbeat)
	}
	if existing, exists := s.cronRunLeaseHeartbeats[key]; exists && existing.owner == owner {
		s.cronRunLeaseHeartbeatMu.Unlock()
		return existing
	}
	if existing := s.cronRunLeaseHeartbeats[key]; existing != nil {
		// A new durable owner supersedes an expired local owner immediately.
		existing.cancel()
	}
	heartbeatCtx, cancel := context.WithCancel(context.Background())
	heartbeat := &cronRunLeaseHeartbeat{runID: binding.RunID, owner: owner, ctx: heartbeatCtx, cancel: cancel}
	if len(preAdmission) == 0 || !preAdmission[0] {
		heartbeat.admitted.Store(true)
	}
	s.cronRunLeaseHeartbeats[key] = heartbeat
	s.cronRunLeaseHeartbeatMu.Unlock()

	go s.runCronRunDispatchLeaseHeartbeat(key, heartbeat, durable, binding)
	return heartbeat
}

// cronRunAdmissionContext is cancelled when its lease owner is lost. The
// heartbeat is started before StartRunWithID/ResumeRunWithID so slow preflight
// cannot dispatch after lease expiry or a competing owner takeover.
func cronRunAdmissionContext(parent context.Context, heartbeat *cronRunLeaseHeartbeat) (context.Context, func(), func()) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	stopOwnerLoss := context.AfterFunc(heartbeat.ctx, cancel)
	detachOwnerLoss := func() { _ = stopOwnerLoss() }
	stop := func() {
		detachOwnerLoss()
		cancel()
	}
	return ctx, detachOwnerLoss, stop
}

func (s *Server) runCronRunDispatchLeaseHeartbeat(key string, heartbeat *cronRunLeaseHeartbeat, durable store.CronRunStartStore, binding store.CronRunStart) {
	defer func() {
		s.cronRunLeaseHeartbeatMu.Lock()
		if s.cronRunLeaseHeartbeats[key] == heartbeat {
			delete(s.cronRunLeaseHeartbeats, key)
		}
		s.cronRunLeaseHeartbeatMu.Unlock()
		if s.cronRunLeaseHeartbeatStopped != nil {
			select {
			case s.cronRunLeaseHeartbeatStopped <- key:
			default:
			}
		}
	}()

	duration := s.cronRunDispatchLeaseDuration
	if duration <= 0 {
		duration = defaultCronRunDispatchLeaseDuration
	}
	interval := duration / 3
	if interval <= 0 {
		interval = time.Millisecond
	}
	ticks := s.cronRunDispatchHeartbeatTicks
	var ticker *time.Ticker
	if ticks == nil {
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case <-heartbeat.ctx.Done():
			return
		case <-s.runner.ShutdownSignal():
			heartbeat.cancel()
			return
		case <-ticks:
			if heartbeat.admitted.Load() {
				run, exists := s.runner.GetRun(heartbeat.runID)
				if !exists || (run.Status != harness.RunStatusQueued && run.Status != harness.RunStatusRunning) {
					return
				}
			}
			now := s.cronRunNow()
			renewCtx, cancel := context.WithTimeout(context.Background(), interval)
			renewedBinding, renewed, err := durable.RenewCronRunStartDispatchLease(renewCtx, binding.TenantID, binding.IdempotencyKey, heartbeat.owner, now, now.Add(duration))
			cancel()
			if err != nil {
				// A transient local-store outage is not evidence that ownership
				// changed. Retry on the next heartbeat; a later explicit
				// owner/expiry miss still stops this process from renewing.
				continue
			}
			if !renewed || renewedBinding.DispatchOwner != heartbeat.owner {
				heartbeat.cancel()
				return
			}
		}
	}
}

func cronHarnessRunFromStore(persisted *store.Run) harness.Run {
	return harness.Run{
		ID:             persisted.ID,
		Prompt:         persisted.Prompt,
		Status:         harness.RunStatus(persisted.Status),
		TenantID:       persisted.TenantID,
		ConversationID: persisted.ConversationID,
		AgentID:        persisted.AgentID,
	}
}

func cronRunRequestFingerprint(req cronRunRequest) string {
	canonical, _ := json.Marshal(struct {
		Prompt            string   `json:"prompt"`
		Model             string   `json:"model"`
		ProviderName      string   `json:"provider_name"`
		AllowFallback     bool     `json:"allow_fallback"`
		FallbackProviders []string `json:"fallback_providers"`
		TenantID          string   `json:"tenant_id"`
		AgentID           string   `json:"agent_id"`
		ConversationID    string   `json:"conversation_id"`
		JobID             string   `json:"job_id"`
		ExecutionID       string   `json:"execution_id"`
		CorrelationKey    string   `json:"correlation_key"`
	}{
		Prompt:            req.Prompt,
		Model:             req.Model,
		ProviderName:      req.ProviderName,
		AllowFallback:     req.AllowFallback,
		FallbackProviders: append([]string(nil), req.FallbackProviders...),
		TenantID:          req.TenantID,
		AgentID:           req.AgentID,
		ConversationID:    req.ConversationID,
		JobID:             req.JobID,
		ExecutionID:       req.ExecutionID,
		CorrelationKey:    req.CorrelationKey,
	})
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}
