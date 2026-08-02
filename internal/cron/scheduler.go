package cron

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	robfigcron "github.com/robfig/cron/v3"
)

// Scheduler manages scheduled jobs using robfig/cron.
type Scheduler struct {
	store          Store
	executor       Executor
	clock          Clock
	cron           *robfigcron.Cron
	addFunc        func(string, func()) (robfigcron.EntryID, error)
	sem            chan struct{} // concurrency semaphore
	wg             sync.WaitGroup
	mu             sync.Mutex
	entries        map[string]robfigcron.EntryID // jobID -> entryID
	generations    map[string]uint64             // jobID -> live callback generation
	nextGeneration uint64                        // never reused, including after pause/delete
	prepared       map[string]*PreparedJob       // jobID -> reserved replacement
	activeScopes   map[string]int                // scoped conversation keys with executing cron work
	// reconciledLeases records the durable active rows whose scope lease was
	// restored after process restart. It makes repeated/async reconciliation
	// idempotent: a second pass may observe the same run again, but can never
	// double-count its local no-overlap lease.
	reconciledLeases map[string]string // executionID -> scope key
	reconcileMu      sync.Mutex
	// lifecycleMu serializes reconciliation admission with shutdown.  It is
	// deliberately distinct from reconcileMu: Stop must be able to cancel a
	// remote observer that is currently holding reconcileMu, then wait for it
	// without deadlocking a later bind notification.
	lifecycleMu     sync.Mutex
	stopped         bool
	reconcileWG     sync.WaitGroup
	reconcileCtx    context.Context
	reconcileCancel context.CancelFunc
	// observationCtx owns only the terminal-lifecycle wait for live harness
	// runs. It deliberately does not govern dispatch: accepting a run must not
	// be cancelled merely because the scheduler is stopping.
	observationCtx    context.Context
	observationCancel context.CancelFunc
	observationWG     sync.WaitGroup
	jitterCfg         JitterConfig
	jitterCache       map[string]time.Duration // jobID|schedule -> jitter offset
	sleepFn           func(time.Duration)      // injectable sleep for testing; defaults to time.Sleep
	done              chan struct{}            // closed by Stop to interrupt in-flight jitter waits
	stopOnce          sync.Once                // guards closing done so a double Stop cannot panic
}

// JobScheduler is the live-dispatch subset required by embedded lifecycle
// adapters and their deterministic failure tests.
type JobScheduler interface {
	AddJob(Job) error
	PrepareJob(Job) (*PreparedJob, error)
	CommitJob(*PreparedJob)
	AbortJob(*PreparedJob)
	UpdateJobSchedule(Job) error
	RemoveJob(string)
}

// PreparedJob is an inert scheduler registration. It cannot dispatch until
// CommitJob makes its generation live; AbortJob removes it without disturbing
// the entry that was live when preparation began.
type PreparedJob struct {
	scheduler  *Scheduler
	jobID      string
	entryID    robfigcron.EntryID
	oldEntryID robfigcron.EntryID
	hadOld     bool
	generation uint64
	done       bool
}

// SchedulerConfig holds scheduler configuration.
type SchedulerConfig struct {
	MaxConcurrent int
	Jitter        JitterConfig
}

// NewScheduler creates a new Scheduler.
func NewScheduler(store Store, executor Executor, clock Clock, cfg SchedulerConfig) *Scheduler {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.Jitter.MinSec <= 0 && cfg.Jitter.MaxSec <= 0 {
		cfg.Jitter = DefaultJitterConfig()
	}
	c := robfigcron.New(
		robfigcron.WithLocation(time.UTC),
		robfigcron.WithParser(robfigcron.NewParser(
			robfigcron.Minute|robfigcron.Hour|robfigcron.Dom|robfigcron.Month|robfigcron.Dow,
		)),
	)
	reconcileCtx, reconcileCancel := context.WithCancel(context.Background())
	observationCtx, observationCancel := context.WithCancel(context.Background())
	return &Scheduler{
		store:             store,
		executor:          executor,
		clock:             clock,
		cron:              c,
		addFunc:           c.AddFunc,
		sem:               make(chan struct{}, cfg.MaxConcurrent),
		entries:           make(map[string]robfigcron.EntryID),
		generations:       make(map[string]uint64),
		prepared:          make(map[string]*PreparedJob),
		activeScopes:      make(map[string]int),
		reconciledLeases:  make(map[string]string),
		reconcileCtx:      reconcileCtx,
		reconcileCancel:   reconcileCancel,
		observationCtx:    observationCtx,
		observationCancel: observationCancel,
		jitterCfg:         cfg.Jitter,
		jitterCache:       make(map[string]time.Duration),
		sleepFn:           time.Sleep,
		done:              make(chan struct{}),
	}
}

// Start loads all active jobs from the store and starts the cron scheduler.
func (s *Scheduler) Start(ctx context.Context) error {
	jobs, err := s.store.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}
	// Recovery must restore durable no-overlap leases before registrations are
	// armed, but observing a recovered remote run can legitimately take minutes.
	// Do not make daemon readiness depend on that external terminal state.
	if err := s.restoreExecutionLeases(ctx, jobs); err != nil {
		return fmt.Errorf("reconcile executions: %w", err)
	}
	for _, job := range jobs {
		if job.Status != StatusActive {
			continue
		}
		if err := s.ValidateJob(job); err != nil {
			return fmt.Errorf("job %q is not ready: %w", job.ID, err)
		}
		if err := s.AddJob(job); err != nil {
			log.Printf("cron: failed to add job %s (%s): %v", job.ID, job.Name, err)
		}
	}
	s.cron.Start()
	// Every observer, including RemoteRunStarter, runs asynchronously. This is
	// also harmless before the embedded runner bridge is bound: unavailable and
	// nonterminal observation retain the restored lease, and binding schedules a
	// later retry through ReconcileAfterExecutorBound.
	s.reconcileExecutionsAsync(s.reconcileCtx, jobs)
	return nil
}

// ReconcileAfterExecutorBound retries persisted execution observation after a
// late-bound embedded runner becomes available. It returns immediately so the
// harnessd boot path cannot be held by a live scheduled conversation. The same
// asynchronous mechanism is used for remote cronsd recovery at Start.
func (s *Scheduler) ReconcileAfterExecutorBound(ctx context.Context) {
	// Reconciliation has scheduler lifetime, not caller lifetime. In
	// particular bootstrap currently passes context.Background(), which must
	// not let a remote polling goroutine outlive Scheduler.Stop/store teardown.
	// Keep the parameter for API compatibility; the scheduler-owned context is
	// the authoritative cancellation boundary.
	s.reconcileExecutionsAsync(nil, nil)
}

func (s *Scheduler) reconcileExecutionsAsync(ctx context.Context, jobs []Job) {
	s.lifecycleMu.Lock()
	if s.stopped {
		s.lifecycleMu.Unlock()
		return
	}
	ctx = s.reconcileCtx
	s.reconcileWG.Add(1)
	s.lifecycleMu.Unlock()
	go func() {
		defer s.reconcileWG.Done()
		if jobs == nil {
			loaded, err := s.store.ListJobs(ctx)
			if err != nil {
				log.Printf("cron: post-bind reconciliation could not load jobs: %v", err)
				return
			}
			jobs = loaded
		}
		if err := s.reconcileExecutions(ctx, jobs); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("cron: asynchronous execution reconciliation failed: %v", err)
		}
	}()
}

// restoreExecutionLeases imports durable active execution rows without waiting
// for a terminal observer. It is the synchronous portion of scheduler startup.
func (s *Scheduler) restoreExecutionLeases(ctx context.Context, jobs []Job) error {
	return s.reconcileExecutionRows(ctx, jobs, false)
}

// reconcileExecutions restores the no-overlap lease held by executions that
// survived a scheduler process restart. Rows with no run ID are pre-start
// failures; linked rows are terminalized immediately when a generic observer
// is available, or conservatively retain their lease until one is supplied.
func (s *Scheduler) reconcileExecutions(ctx context.Context, jobs []Job) error {
	return s.reconcileExecutionRows(ctx, jobs, true)
}

func (s *Scheduler) reconcileExecutionRows(ctx context.Context, jobs []Job, observe bool) error {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	active, err := s.store.ListActiveExecutions(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]Job, len(jobs))
	for _, job := range jobs {
		byID[job.ID] = job
	}
	for _, exec := range active {
		job, ok := byID[exec.JobID]
		if !ok {
			loaded, loadErr := s.store.GetJob(ctx, exec.JobID)
			if loadErr != nil {
				// A scheduler-owned cancellation and a transient store error are
				// both nonterminal. Only a definitive not-found result establishes
				// that the job was deleted; anything else must retain the durable
				// active row and its no-overlap lease for a later reconciliation.
				if ctx.Err() != nil {
					return nil
				}
				if !IsJobNotFound(loadErr) {
					return fmt.Errorf("load job %s for execution %s: %w", exec.JobID, exec.ID, loadErr)
				}
				if err := s.finishUnavailableExecution(ctx, exec); err != nil {
					return err
				}
				continue
			}
			job = loaded
		}
		key := s.acquireReconciledScope(exec.ID, scopeKey(job))
		if !observe {
			continue
		}
		// A missing RunID can be the narrow failure window after StartRun
		// succeeds but before its durable link is written. Preserve the lease
		// rather than guessing it is safe to begin a duplicate conversation run.
		if exec.RunID == "" {
			continue
		}
		observer, ok := s.executor.(executionOutcomeObserver)
		if !ok {
			continue
		}
		outcome := ExecutionOutcome{RunID: exec.RunID, OutputSummary: exec.OutputSummary}
		observation, observed, observeErr := observer.ObserveExecution(ctx, job, outcome)
		// Shutdown is nonterminal. A remote observer commonly returns a typed
		// cancellation error when its scheduler-owned context is canceled; never
		// convert that cancellation into a failed history row or release lease.
		if ctx.Err() != nil {
			return nil
		}
		// Recovery shares the live-observation contract: only an observed,
		// error-free terminal result may finalize the durable row. Transport or
		// stream errors can be transient, and an unavailable observer is not a
		// terminal result. Keep the RunID and both no-overlap leases so a later
		// reconciliation can retry; importantly, continue so one bad remote run
		// never starves a later row that already has a terminal outcome.
		if observeErr != nil || !observed {
			continue
		}
		if err := s.finishObservedExecution(ctx, job, exec, key, observation, observeErr); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) finishObservedExecution(ctx context.Context, job Job, exec Execution, key string, observation RunObservation, observeErr error) error {
	// Observation is deliberately outside lifecycleMu because a remote poll can
	// legitimately last minutes. Terminal persistence, however, must be one
	// transaction-sized critical section with Stop: either Stop seals/cancels
	// first and the recovered active row/lease remains, or this whole terminal
	// write, local release, and job tracking update commits before Stop returns.
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopped || ctx.Err() != nil {
		return nil
	}
	exec.FinishedAt = s.clock.Now()
	exec.DurationMs = exec.FinishedAt.Sub(exec.StartedAt).Milliseconds()
	if observeErr != nil {
		exec.Status = ExecStatusTimeout
		if !isTimeoutError(observeErr) {
			exec.Status = ExecStatusFailed
		}
		exec.Error = BoundedExecutionSummary(observeErr.Error())
	} else if !observation.Succeeded {
		exec.Status = ExecStatusFailed
		exec.Error = BoundedExecutionSummary(observation.Error)
		if exec.Error == "" {
			exec.Error = "harness run failed"
		}
	} else {
		exec.Status = ExecStatusSucceeded
		exec.Error = ""
	}
	exec.OutputSummary = BoundedExecutionSummary(observation.OutputSummary)
	if updateErr := s.store.UpdateExecution(ctx, exec); updateErr != nil {
		return fmt.Errorf("persist reconciled execution %s: %w", exec.ID, updateErr)
	}
	// The durable terminal transition is the point at which another process is
	// allowed to admit this conversation. Do not release a local restart lease
	// if that transition failed.
	s.releaseReconciledScope(exec.ID, key)
	nextRun := job.NextRunAt
	if next, parseErr := NextRunTime(job.Schedule, exec.FinishedAt); parseErr == nil {
		nextRun = next
	}
	if touchErr := s.store.TouchJobRun(ctx, job.ID, exec.FinishedAt, nextRun, exec.FinishedAt); touchErr != nil {
		return fmt.Errorf("touch reconciled job %s: %w", job.ID, touchErr)
	}
	return nil
}

// finishUnavailableExecution terminalizes only a definitively deleted job.
// It shares the terminal lifecycle gate so Stop cannot turn a late lookup into
// a synthetic failure after teardown begins.
func (s *Scheduler) finishUnavailableExecution(ctx context.Context, exec Execution) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopped || ctx.Err() != nil {
		return nil
	}
	exec.Status = ExecStatusFailed
	exec.FinishedAt = s.clock.Now()
	exec.DurationMs = exec.FinishedAt.Sub(exec.StartedAt).Milliseconds()
	exec.Error = "cron execution cannot be reconciled: job unavailable"
	if updateErr := s.store.UpdateExecution(ctx, exec); updateErr != nil {
		return fmt.Errorf("finalize unavailable execution %s: %w", exec.ID, updateErr)
	}
	// A deleted job cannot be touched, but a prior successful reconciliation
	// pass may already have restored its local lease. Release only after the
	// terminal row is durable.
	s.releaseReconciledScope(exec.ID, s.reconciledScope(exec.ID))
	return nil
}

// Stop stops the cron scheduler, joins dispatch work, and cancels/joins
// scheduler-owned terminal observation. An already accepted harness run stays
// durably linked and running for recovery if its observation is still live.
//
// Stop first tells robfig/cron to stop dispatching new ticks (s.cron.Stop
// returns a context that becomes Done once any invocation already in
// progress returns). It then closes s.done — BEFORE waiting on that
// context — so that any fireJob call currently blocked in its jitter wait
// is interrupted immediately and returns without executing the job. Only
// after that does Stop wait for cron's context and then for s.wg, which
// tracks the async goroutines doing the actual execution work (those are
// allowed to run to completion; only the pre-execution jitter wait is
// abandoned on shutdown). Closing done is idempotent (sync.Once) so a
// double Stop call cannot panic.
func (s *Scheduler) Stop() {
	// Seal admission before cancellation so a concurrently delivered embedded
	// runner bind cannot add a reconciliation goroutine after the Wait below.
	s.lifecycleMu.Lock()
	s.stopped = true
	if s.reconcileCancel != nil {
		s.reconcileCancel()
	}
	if s.observationCancel != nil {
		s.observationCancel()
	}
	s.lifecycleMu.Unlock()
	ctx := s.cron.Stop()
	s.stopOnce.Do(func() { close(s.done) })
	<-ctx.Done()
	// Reconciliation is independent of fireJob's execution wait group. Join it
	// before returning so callers may safely close the cron store immediately.
	s.reconcileWG.Wait()
	// Live observation workers own durable terminalization for newly-fired
	// harness runs. They are canceled and sealed above, so this join cannot
	// synthesize a terminal result or release a live scope during shutdown.
	s.observationWG.Wait()
	s.wg.Wait()
}

func (s *Scheduler) acquireReconciledScope(executionID, key string) string {
	if key == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.reconciledLeases[executionID]; ok {
		return existing
	}
	s.reconciledLeases[executionID] = key
	s.activeScopes[key]++
	return key
}

func (s *Scheduler) releaseReconciledScope(executionID, key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	if owned, ok := s.reconciledLeases[executionID]; ok && owned == key {
		delete(s.reconciledLeases, executionID)
		if s.activeScopes[key] <= 1 {
			delete(s.activeScopes, key)
		} else {
			s.activeScopes[key]--
		}
	}
	s.mu.Unlock()
}

func (s *Scheduler) reconciledScope(executionID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reconciledLeases[executionID]
}

// AddJob registers a job with the cron scheduler.
func (s *Scheduler) AddJob(job Job) error {
	if err := s.ValidateJob(job); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepared[job.ID] != nil {
		return fmt.Errorf("cron replacement is being prepared for job %s", job.ID)
	}

	// Compute a deterministic jitter offset for this job. jitterCache is
	// retained (and still populated here, under s.mu) so tests and any
	// future callers can introspect the cached value, but fireJob itself
	// no longer reads this map — the computed jitter is captured directly
	// in the closure below and passed to fireJob as a parameter. This
	// avoids the unsynchronized read that previously raced with this write
	// (fireJob ran on the robfig/cron goroutine with no lock held).
	jitter := computeJitter(s.jitterCfg, job.ID, job.Schedule)
	s.jitterCache[jitterCacheKey(job.ID, job.Schedule)] = jitter

	// Capture job and its jitter offset for the closure.
	s.nextGeneration++
	generation := s.nextGeneration
	j := job
	entryID, err := s.addFunc(job.Schedule, func() {
		s.fireJobIfCurrent(j, jitter, generation)
	})
	if err != nil {
		return fmt.Errorf("add cron entry: %w", err)
	}
	// robfig/cron owns the actual registrations. Replacing only our map entry
	// would leave the former callback firing in parallel.
	if old, ok := s.entries[job.ID]; ok {
		s.cron.Remove(old)
	}
	s.entries[job.ID] = entryID
	s.generations[job.ID] = generation
	return nil
}

// PrepareJob registers a replacement without making it runnable. The current
// live entry remains untouched until CommitJob, so a scheduler failure cannot
// turn an otherwise healthy active job into a paused durable mutation.
func (s *Scheduler) PrepareJob(job Job) (*PreparedJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepared[job.ID] != nil {
		return nil, fmt.Errorf("cron replacement is already being prepared for job %s", job.ID)
	}

	jitter := computeJitter(s.jitterCfg, job.ID, job.Schedule)
	s.jitterCache[jitterCacheKey(job.ID, job.Schedule)] = jitter
	s.nextGeneration++
	generation := s.nextGeneration
	j := job
	entryID, err := s.addFunc(job.Schedule, func() {
		s.fireJobIfCurrent(j, jitter, generation)
	})
	if err != nil {
		return nil, fmt.Errorf("prepare cron entry: %w", err)
	}
	old, hadOld := s.entries[job.ID]
	prepared := &PreparedJob{scheduler: s, jobID: job.ID, entryID: entryID, oldEntryID: old, hadOld: hadOld, generation: generation}
	s.prepared[job.ID] = prepared
	return prepared, nil
}

// CommitJob atomically publishes a prepared callback and retires the prior
// one. The generation gate makes an already-queued old callback a no-op.
func (s *Scheduler) CommitJob(prepared *PreparedJob) {
	if prepared == nil || prepared.scheduler != s {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prepared.done {
		return
	}
	s.entries[prepared.jobID] = prepared.entryID
	s.generations[prepared.jobID] = prepared.generation
	if prepared.hadOld {
		s.cron.Remove(prepared.oldEntryID)
	}
	prepared.done = true
	delete(s.prepared, prepared.jobID)
}

// AbortJob discards an inert prepared entry and leaves the prior live entry
// and its generation unchanged.
func (s *Scheduler) AbortJob(prepared *PreparedJob) {
	if prepared == nil || prepared.scheduler != s {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prepared.done {
		return
	}
	s.cron.Remove(prepared.entryID)
	prepared.done = true
	delete(s.prepared, prepared.jobID)
}

// ValidateJob checks executor readiness for a job without executing it.
// Unsupported validators are treated as compatible so existing custom
// executors remain source-compatible.
func (s *Scheduler) ValidateJob(job Job) error {
	if s == nil || s.executor == nil {
		return fmt.Errorf("cron executor is not configured")
	}
	if validator, ok := s.executor.(JobValidator); ok {
		return validator.ValidateJob(job)
	}
	return nil
}

// HasEntry reports whether jobID is currently registered with the live
// cron dispatcher — i.e. it will fire on its schedule. A paused or
// removed job is not registered. Exposed primarily so callers (including
// tests outside this package) can observe live scheduler state without
// reaching into unexported fields.
func (s *Scheduler) HasEntry(jobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.entries[jobID]
	return ok
}

// TriggerJob starts one execution of the current stored job immediately.
//
// The stored record is loaded at trigger time so callers cannot execute a
// stale or caller-supplied snapshot. Only active jobs may be triggered. The
// execution itself follows the same asynchronous dispatch path and concurrency
// limits as a scheduled fire. Stop joins dispatch but cancels a still-live
// terminal observer, retaining the accepted harness run for later recovery.
func (s *Scheduler) TriggerJob(ctx context.Context, jobID string) error {
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job %q: %w", jobID, err)
	}
	if job.Status != StatusActive {
		return fmt.Errorf("job %q is not active (status=%s)", jobID, job.Status)
	}
	s.fireJob(job, 0)
	return nil
}

// RemoveJob removes a job from the cron scheduler.
func (s *Scheduler) RemoveJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prepared := s.prepared[jobID]; prepared != nil {
		s.cron.Remove(prepared.entryID)
		prepared.done = true
		delete(s.prepared, jobID)
	}
	if entryID, ok := s.entries[jobID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, jobID)
	}
	// Advance the global identity before dropping this per-job entry. A queued
	// callback now observes zero (not its old identity), while a later resume
	// receives a distinct globally monotonic identity without retaining an
	// unbounded tombstone map.
	s.nextGeneration++
	delete(s.generations, jobID)
}

// UpdateJobSchedule registers the replacement before removing the old entry.
// A malformed replacement therefore cannot unschedule a healthy job.
func (s *Scheduler) UpdateJobSchedule(job Job) error {
	prepared, err := s.PrepareJob(job)
	if err != nil {
		return fmt.Errorf("add replacement cron entry: %w", err)
	}
	s.CommitJob(prepared)
	return nil
}

func (s *Scheduler) fireJobIfCurrent(job Job, jitter time.Duration, generation uint64) {
	s.mu.Lock()
	live := s.generations[job.ID] == generation
	s.mu.Unlock()
	if !live {
		return
	}
	s.fireJobWithScheduleGuard(job, jitter, true, generation)
}

// fireJob executes a job: creates an execution record, runs the executor,
// and updates the execution and job records.
//
// jitter is the base jitter offset computed once by AddJob at registration
// time. fireJob deliberately does NOT read s.jitterCache itself: fireJob
// runs on the robfig/cron dispatch goroutine outside of s.mu, and reading
// the cache concurrently with AddJob's locked write previously caused a
// fatal, unrecoverable "concurrent map read and map write" runtime error.
func (s *Scheduler) fireJob(job Job, jitter time.Duration) {
	s.fireJobWithScheduleGuard(job, jitter, false, 0)
}

func (s *Scheduler) fireJobWithScheduleGuard(job Job, jitter time.Duration, requireMatchingSchedule bool, generation uint64) {
	ctx := context.Background()
	now := s.clock.Now()

	// Apply jitter delay before execution work.
	// The base jitter offset is computed deterministically at registration time.
	// Minute-mark avoidance is applied now using the actual fire time.
	baseJitter := jitter
	if baseJitter > 0 {
		jitterOffset := avoidMinuteMarks(baseJitter, now, s.jitterCfg.AvoidMarks)
		if s.jitterCfg.LogJitteredTimes {
			log.Printf("cron: job %s jittered by %v (original schedule: %s, base jitter: %v)",
				job.ID, jitterOffset, job.Schedule, baseJitter)
		}

		// The jitter wait must be interruptible by Stop(), otherwise
		// shutdown blocks for up to the full jitter window (which can be
		// minutes with the default jitter config) before the HTTP server
		// even begins draining. s.sleepFn is run in a background
		// goroutine so tests can keep injecting a fast/no-op sleep, while
		// production (sleepFn == time.Sleep) still returns from fireJob
		// promptly on shutdown: if s.done closes first, fireJob abandons
		// this fire entirely rather than executing the job mid-shutdown.
		sleepDone := make(chan struct{})
		go func() {
			s.sleepFn(jitterOffset)
			close(sleepDone)
		}()
		select {
		case <-sleepDone:
		case <-s.done:
			log.Printf("cron: shutdown in progress, skipping fire for job %s", job.ID)
			return
		}
	}

	// Re-read the job's current state from the store before firing. The
	// job value captured in AddJob's closure can be arbitrarily stale by
	// the time the timer/jitter wait elapses: the schedule, execution
	// config, timeout, tags, or status may have changed (e.g. the job may
	// have been paused or deleted). Firing based on the stale snapshot
	// would ignore those edits and could resurrect a paused job.
	current, err := s.store.GetJob(ctx, job.ID)
	if err != nil {
		log.Printf("cron: skipping fire for job %s: failed to reload current state: %v", job.ID, err)
		return
	}
	if current.Status != StatusActive {
		log.Printf("cron: skipping fire for job %s: no longer active (status=%s)", job.ID, current.Status)
		return
	}
	// A replacement can commit durable state just before it retires the old
	// robfig callback. Do not let that stale tick execute the replacement's
	// configuration at the old schedule; the new callback owns the new schedule.
	if requireMatchingSchedule && current.Schedule != job.Schedule {
		log.Printf("cron: skipping stale schedule fire for job %s", job.ID)
		return
	}
	job = current
	exec, admitted, scopeKey, err := s.admitExecution(ctx, job, now, generation, requireMatchingSchedule)
	if err != nil {
		log.Printf("cron: failed to create execution for job %s: %v", job.ID, err)
		return
	}
	if !admitted {
		log.Printf("cron: skipping stale registration fire for job %s", job.ID)
		return
	}

	// Acquire semaphore to limit concurrency.
	s.sem <- struct{}{}
	s.wg.Add(1)

	go func() {
		releaseLease := true
		defer func() {
			if releaseLease {
				s.releaseScope(scopeKey)
			}
			<-s.sem
			s.wg.Done()
		}()

		// A structured executor has a distinct start boundary. Persist it as
		// starting before asking a harness to create a run; shell/legacy paths
		// retain their historical direct-running behavior.
		_, structured := s.executor.(executionOutcomeExecutor)
		if structured {
			exec.Status = ExecStatusStarting
		} else {
			exec.Status = ExecStatusRunning
		}
		if updateErr := s.store.UpdateExecution(ctx, exec); updateErr != nil {
			log.Printf("cron: failed to update execution %s to active state: %v", exec.ID, updateErr)
		}

		startTime := s.clock.Now()
		var outcome ExecutionOutcome
		var execErr error
		if aware, ok := s.executor.(executionOutcomeExecutor); ok {
			outcome, execErr = aware.ExecuteOutcomeWithID(ctx, job, exec.ID)
		} else if aware, ok := s.executor.(executionAwareExecutor); ok {
			outcome.OutputSummary, execErr = aware.ExecuteWithID(ctx, job, exec.ID)
		} else {
			outcome.OutputSummary, execErr = s.executor.Execute(ctx, job)
		}

		// Successful harness admission must be durable independently of its
		// display string. A remote transport can return any summary text, but
		// history always has the exact run ID used by clients to observe it.
		if outcome.RunID != "" {
			exec.RunID = outcome.RunID
			exec.OutputSummary = outcome.OutputSummary
			exec.Status = ExecStatusRunning
			if updateErr := s.persistRunLink(ctx, exec); updateErr != nil {
				// Never reconstruct this identity from output text or continue to
				// terminalize an execution whose linked harness run is not durable.
				// Retaining the local lease complements the durable active row.
				log.Printf("cron: failed to durably persist run link for execution %s; retaining scope lease: %v", exec.ID, updateErr)
				releaseLease = false
				return
			}
		}
		// Retain the per-conversation lease until an embedded/remote observer
		// reports a terminal outcome. Observation is deliberately detached from
		// dispatch and uses a scheduler-owned cancellable context: Stop must be
		// able to join a live SSE subscription or remote poll without cancelling
		// StartRun/shell dispatch itself.
		if outcome.RunID != "" {
			if observer, ok := s.executor.(executionOutcomeObserver); ok {
				releaseLease = false
				s.observeLiveExecution(job, exec, scopeKey, outcome, observer)
				return
			}
		}
		endTime := s.clock.Now()

		exec.FinishedAt = endTime
		exec.DurationMs = endTime.Sub(startTime).Milliseconds()
		exec.OutputSummary = BoundedExecutionSummary(outcome.OutputSummary)

		if execErr != nil {
			exec.Error = BoundedExecutionSummary(execErr.Error())
			if isTimeoutError(execErr) {
				exec.Status = ExecStatusTimeout
			} else {
				exec.Status = ExecStatusFailed
			}
		} else {
			exec.Status = ExecStatusSuccess
		}

		if updateErr := s.store.UpdateExecution(ctx, exec); updateErr != nil {
			log.Printf("cron: failed to update execution %s: %v", exec.ID, updateErr)
		}

		// Record last_run_at and recompute next_run_at using a targeted
		// update that only touches run-tracking columns. This must NOT be
		// a full-object UpdateJob write: job is still the state read at
		// the top of fireJob, and by the time execution finishes it may
		// again be stale relative to concurrent edits. TouchJobRun never
		// clobbers schedule, execution config, status, timeout, or tags.
		nextRun := job.NextRunAt
		if next, parseErr := NextRunTime(job.Schedule, endTime); parseErr == nil {
			nextRun = next
		}
		// On schedule parse error, NextRunAt is left unchanged.
		if touchErr := s.store.TouchJobRun(ctx, job.ID, endTime, nextRun, endTime); touchErr != nil {
			log.Printf("cron: failed to touch job %s last_run_at: %v", job.ID, touchErr)
		}
	}()
}

// observeLiveExecution owns terminal observation for a run that was accepted
// during this scheduler lifetime. The admission lease remains held until a
// durable terminal transition succeeds. Observer transport errors and an
// unavailable observer are intentionally nonterminal: reconciliation can retry
// them after restart/bind, whereas a terminal run result is represented by
// observed=true with a nil error.
func (s *Scheduler) observeLiveExecution(job Job, exec Execution, key string, outcome ExecutionOutcome, observer executionOutcomeObserver) {
	s.lifecycleMu.Lock()
	if s.stopped {
		s.lifecycleMu.Unlock()
		return
	}
	ctx := s.observationCtx
	s.observationWG.Add(1)
	s.lifecycleMu.Unlock()
	go func() {
		defer s.observationWG.Done()
		observation, observed, observeErr := observer.ObserveExecution(ctx, job, outcome)
		if ctx.Err() != nil || observeErr != nil || !observed {
			// Cancellation, remote 5xx/stream closure, and an unbound observer
			// all preserve the active row, run link, and no-overlap lease.
			return
		}
		s.finishLiveObservedExecution(ctx, job, exec, key, observation)
	}()
}

// finishLiveObservedExecution is the terminal gate for an execution started
// by this process. lifecycleMu serializes its terminal persistence with Stop:
// either the terminal update wins and releases/touches once, or Stop seals the
// scheduler first and the linked active row/lease remain for recovery.
func (s *Scheduler) finishLiveObservedExecution(ctx context.Context, job Job, exec Execution, key string, observation RunObservation) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopped || ctx.Err() != nil {
		return
	}
	exec.FinishedAt = s.clock.Now()
	exec.DurationMs = exec.FinishedAt.Sub(exec.StartedAt).Milliseconds()
	exec.OutputSummary = BoundedExecutionSummary(observation.OutputSummary)
	if observation.Succeeded {
		exec.Status = ExecStatusSuccess
		exec.Error = ""
	} else {
		exec.Status = ExecStatusFailed
		exec.Error = BoundedExecutionSummary(observation.Error)
		if exec.Error == "" {
			exec.Error = "harness run failed"
		}
	}
	if err := s.store.UpdateExecution(ctx, exec); err != nil {
		log.Printf("cron: failed to persist terminal live execution %s; retaining scope lease: %v", exec.ID, err)
		return
	}
	s.releaseScope(key)
	nextRun := job.NextRunAt
	if next, err := NextRunTime(job.Schedule, exec.FinishedAt); err == nil {
		nextRun = next
	}
	if err := s.store.TouchJobRun(ctx, job.ID, exec.FinishedAt, nextRun, exec.FinishedAt); err != nil {
		log.Printf("cron: failed to touch terminal live execution job %s: %v", job.ID, err)
	}
}

const runLinkPersistenceAttempts = 3

// persistRunLink retries the durable boundary after StartRun accepts a run.
// The caller must not observe or finalize it until the exact RunID is stored.
func (s *Scheduler) persistRunLink(ctx context.Context, exec Execution) error {
	var err error
	for attempt := 0; attempt < runLinkPersistenceAttempts; attempt++ {
		err = s.store.UpdateExecution(ctx, exec)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("persist run link after %d attempts: %w", runLinkPersistenceAttempts, err)
}

// admitExecution is the linearization point between a registered callback and
// pause/delete/replacement. Its final identity check and execution-row create
// share s.mu with Prepare/Commit/Remove. The lock is released before executor
// work begins. TriggerJob bypasses the identity guard and keeps its prior path.
func (s *Scheduler) admitExecution(ctx context.Context, job Job, now time.Time, generation uint64, requireCurrent bool) (Execution, bool, string, error) {
	exec := Execution{
		ID:        uuid.New().String(),
		JobID:     job.ID,
		StartedAt: now,
		Status:    ExecStatusPending,
	}
	if !requireCurrent {
		return s.admitScopedExecution(ctx, job, exec)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generations[job.ID] != generation {
		return Execution{}, false, "", nil
	}
	return s.admitScopedExecutionLocked(ctx, job, exec)
}

func scopeKey(job Job) string {
	if job.TenantID == "" || job.AgentID == "" || job.ConversationID == "" {
		return ""
	}
	return job.TenantID + "\x00" + job.AgentID + "\x00" + job.ConversationID
}

func (s *Scheduler) admitScopedExecution(ctx context.Context, job Job, exec Execution) (Execution, bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.admitScopedExecutionLocked(ctx, job, exec)
}

func (s *Scheduler) admitScopedExecutionLocked(ctx context.Context, job Job, exec Execution) (Execution, bool, string, error) {
	key := scopeKey(job)
	if key != "" {
		if s.activeScopes[key] > 0 {
			exec.Status = ExecStatusSkipped
			exec.FinishedAt = exec.StartedAt
			exec.Error = ErrExecutionSkippedOverlap.Error()
			created, err := s.store.CreateExecution(ctx, exec)
			return created, false, "", err
		}
	}
	created, admitted, err := s.store.AdmitExecution(ctx, job, exec)
	if err != nil {
		return Execution{}, false, "", err
	}
	if key != "" && admitted {
		s.activeScopes[key]++
	}
	if !admitted {
		return created, false, "", nil
	}
	return created, true, key, nil
}

func (s *Scheduler) releaseScope(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	if s.activeScopes[key] <= 1 {
		delete(s.activeScopes, key)
	} else {
		s.activeScopes[key]--
	}
	s.mu.Unlock()
}

// isTimeoutError checks if an error message indicates a timeout.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var remoteErr *RemoteRunError
	if errors.As(err, &remoteErr) && remoteErr.Code == "timeout" {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return len(msg) >= 7 && containsTimeout(msg)
}

func containsTimeout(s string) bool {
	for i := 0; i <= len(s)-7; i++ {
		if s[i:i+7] == "timed o" {
			return true
		}
	}
	return false
}
