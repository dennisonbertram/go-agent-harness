package harness

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/store"
)

type barrierGetRunStore struct {
	*store.MemoryStore
	getCalls atomic.Int32
	release  chan struct{}
	once     sync.Once
}

type barrierConversationListStore struct {
	*store.MemoryStore
	conversationID string
	listCalls      atomic.Int32
	release        chan struct{}
	once           sync.Once
}

type countingCreateRunStore struct {
	*store.MemoryStore
	createCalls atomic.Int32
}

func (s *countingCreateRunStore) CreateRun(ctx context.Context, run *store.Run) error {
	s.createCalls.Add(1)
	return s.MemoryStore.CreateRun(ctx, run)
}

type sharedConversationListBarrier struct {
	conversationID string
	listCalls      atomic.Int32
	release        chan struct{}
	once           sync.Once
}

type sharedBarrierStore struct {
	store.Store
	barrier *sharedConversationListBarrier
}

func (s *sharedBarrierStore) ListRuns(ctx context.Context, filter store.RunFilter) ([]*store.Run, error) {
	runs, err := s.Store.ListRuns(ctx, filter)
	if err != nil || filter.ConversationID != s.barrier.conversationID {
		return runs, err
	}
	if s.barrier.listCalls.Add(1) == 2 {
		s.barrier.once.Do(func() { close(s.barrier.release) })
	}
	select {
	case <-s.barrier.release:
		return runs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *barrierConversationListStore) ListRuns(ctx context.Context, filter store.RunFilter) ([]*store.Run, error) {
	if filter.ConversationID == s.conversationID {
		// Snapshot before either caller can advance to CreateRun. This forces
		// both ownership checks to observe the same empty durable state.
		runs, err := s.MemoryStore.ListRuns(ctx, filter)
		if err != nil {
			return nil, err
		}
		if s.listCalls.Add(1) == 2 {
			s.once.Do(func() { close(s.release) })
		}
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return runs, nil
	}
	return s.MemoryStore.ListRuns(ctx, filter)
}

func TestStartRunWithIDAtomicallyClaimsNewConversationOwner(t *testing.T) {
	const conversationID = "conv-concurrent-owner-claim"
	runStore := &barrierConversationListStore{
		MemoryStore:    store.NewMemoryStore(),
		conversationID: conversationID,
		release:        make(chan struct{}),
	}
	provider := newHeldProvider()
	newRunner := func() *Runner {
		return NewRunner(provider, NewRegistry(), RunnerConfig{
			DefaultModel:        "test-model",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
			Store:               runStore,
		})
	}
	runnerA := newRunner()
	runnerB := newRunner()
	t.Cleanup(func() {
		provider.unblockAll()
		_ = runnerA.Shutdown(context.Background())
		_ = runnerB.Shutdown(context.Background())
	})

	type result struct {
		run Run
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for _, tc := range []struct {
		runner *Runner
		runID  string
		agent  string
	}{
		{runner: runnerA, runID: "run_concurrent-owner-a", agent: "agent-a"},
		{runner: runnerB, runID: "run_concurrent-owner-b", agent: "agent-b"},
	} {
		tc := tc
		go func() {
			<-start
			run, err := tc.runner.StartRunWithID(RunRequest{
				Prompt:         "claim conversation",
				TenantID:       "tenant-shared",
				AgentID:        tc.agent,
				ConversationID: conversationID,
			}, tc.runID)
			results <- result{run: run, err: err}
		}()
	}
	close(start)

	successes := 0
	denials := 0
	for i := 0; i < 2; i++ {
		got := <-results
		switch {
		case got.err == nil:
			successes++
		case errors.Is(got.err, ErrConversationAccessDenied):
			denials++
		default:
			t.Fatalf("unexpected concurrent owner claim error: %v", got.err)
		}
	}
	if successes != 1 || denials != 1 {
		t.Fatalf("concurrent owner claims: successes=%d denials=%d, want 1/1", successes, denials)
	}
	persisted, err := runStore.MemoryStore.ListRuns(context.Background(), store.RunFilter{ConversationID: conversationID})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("persisted conversation runs = %d, want 1", len(persisted))
	}
	select {
	case <-provider.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("winning owner never dispatched")
	}
	select {
	case <-provider.entered:
		t.Fatal("losing owner also dispatched")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStartRunAtomicallyClaimsNewConversationOwnerBeforeDispatch(t *testing.T) {
	const conversationID = "conv-ordinary-concurrent-owner-claim"
	runStore := &barrierConversationListStore{
		MemoryStore:    store.NewMemoryStore(),
		conversationID: conversationID,
		release:        make(chan struct{}),
	}
	provider := newHeldProvider()
	newRunner := func() *Runner {
		return NewRunner(provider, NewRegistry(), RunnerConfig{
			DefaultModel: "test-model", MaxSteps: 1, Store: runStore,
		})
	}
	runnerA := newRunner()
	runnerB := newRunner()
	t.Cleanup(func() {
		provider.unblockAll()
		_ = runnerA.Shutdown(context.Background())
		_ = runnerB.Shutdown(context.Background())
	})

	type result struct {
		run Run
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for _, tc := range []struct {
		runner *Runner
		agent  string
	}{
		{runner: runnerA, agent: "agent-a"},
		{runner: runnerB, agent: "agent-b"},
	} {
		tc := tc
		go func() {
			<-start
			run, err := tc.runner.StartRun(RunRequest{
				Prompt:         "claim ordinary conversation",
				TenantID:       "tenant-shared",
				AgentID:        tc.agent,
				ConversationID: conversationID,
			})
			results <- result{run: run, err: err}
		}()
	}
	close(start)

	successes := 0
	denials := 0
	winnerRunID := ""
	for i := 0; i < 2; i++ {
		got := <-results
		switch {
		case got.err == nil:
			successes++
			winnerRunID = got.run.ID
		case errors.Is(got.err, ErrConversationAccessDenied):
			denials++
			if got.run.ID != "" {
				t.Fatalf("denied ordinary claim returned run ID %q", got.run.ID)
			}
		default:
			t.Fatalf("unexpected ordinary owner claim error: %v", got.err)
		}
	}
	if successes != 1 || denials != 1 {
		t.Fatalf("ordinary concurrent owner claims: successes=%d denials=%d, want 1/1", successes, denials)
	}
	persisted, err := runStore.MemoryStore.ListRuns(context.Background(), store.RunFilter{ConversationID: conversationID})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != winnerRunID {
		t.Fatalf("persisted ordinary runs = %+v, want only winner %q", persisted, winnerRunID)
	}
	select {
	case <-provider.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("winning ordinary owner never dispatched")
	}
	select {
	case <-provider.entered:
		t.Fatal("losing ordinary owner also dispatched")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStartRunAttemptsInitialPersistenceExactlyOnce(t *testing.T) {
	runStore := &countingCreateRunStore{MemoryStore: store.NewMemoryStore()}
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model",
		MaxSteps:     1,
		Store:        runStore,
	})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	run, err := runner.StartRun(RunRequest{
		Prompt:         "persist once",
		TenantID:       "tenant-a",
		AgentID:        "agent-a",
		ConversationID: "conv-persist-once",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, run.ID, RunStatusCompleted)
	if got := runStore.createCalls.Load(); got != 1 {
		t.Fatalf("CreateRun calls = %d, want exactly 1", got)
	}
}

func TestStartRunWithIDAtomicallyClaimsNewConversationOwnerAcrossSQLiteRunners(t *testing.T) {
	const conversationID = "conv-sqlite-concurrent-owner-claim"
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	sqliteA, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore A: %v", err)
	}
	if err := sqliteA.Migrate(ctx); err != nil {
		t.Fatalf("Migrate A: %v", err)
	}
	sqliteB, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore B: %v", err)
	}
	if err := sqliteB.Migrate(ctx); err != nil {
		t.Fatalf("Migrate B: %v", err)
	}
	barrier := &sharedConversationListBarrier{
		conversationID: conversationID,
		release:        make(chan struct{}),
	}
	runStoreA := &sharedBarrierStore{Store: sqliteA, barrier: barrier}
	runStoreB := &sharedBarrierStore{Store: sqliteB, barrier: barrier}
	provider := newHeldProvider()
	runnerA := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model", MaxSteps: 1, Store: runStoreA,
	})
	runnerB := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel: "test-model", MaxSteps: 1, Store: runStoreB,
	})
	t.Cleanup(func() {
		provider.unblockAll()
		_ = runnerA.Shutdown(context.Background())
		_ = runnerB.Shutdown(context.Background())
		_ = sqliteA.Close()
		_ = sqliteB.Close()
	})

	errs := make(chan error, 2)
	start := make(chan struct{})
	for _, tc := range []struct {
		runner *Runner
		runID  string
		agent  string
	}{
		{runner: runnerA, runID: "run_sqlite-owner-a", agent: "agent-a"},
		{runner: runnerB, runID: "run_sqlite-owner-b", agent: "agent-b"},
	} {
		tc := tc
		go func() {
			<-start
			_, err := tc.runner.StartRunWithID(RunRequest{
				Prompt:         "claim sqlite conversation",
				TenantID:       "tenant-shared",
				AgentID:        tc.agent,
				ConversationID: conversationID,
			}, tc.runID)
			errs <- err
		}()
	}
	close(start)

	successes := 0
	denials := 0
	for i := 0; i < 2; i++ {
		err := <-errs
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConversationAccessDenied):
			denials++
		default:
			t.Fatalf("unexpected SQLite concurrent owner claim error: %v", err)
		}
	}
	if successes != 1 || denials != 1 {
		t.Fatalf("SQLite concurrent owner claims: successes=%d denials=%d, want 1/1", successes, denials)
	}
	persisted, err := sqliteA.ListRuns(ctx, store.RunFilter{ConversationID: conversationID})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("persisted SQLite conversation runs = %d, want 1", len(persisted))
	}
	select {
	case <-provider.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("winning SQLite owner never dispatched")
	}
	select {
	case <-provider.entered:
		t.Fatal("losing SQLite owner also dispatched")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestStartRunAtomicallyClaimsNewConversationOwnerAcrossSQLiteRunners(t *testing.T) {
	const conversationID = "conv-sqlite-ordinary-owner-claim"
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "runs.db")
	sqliteA, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore A: %v", err)
	}
	if err := sqliteA.Migrate(ctx); err != nil {
		t.Fatalf("Migrate A: %v", err)
	}
	sqliteB, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore B: %v", err)
	}
	if err := sqliteB.Migrate(ctx); err != nil {
		t.Fatalf("Migrate B: %v", err)
	}
	barrier := &sharedConversationListBarrier{conversationID: conversationID, release: make(chan struct{})}
	runStoreA := &sharedBarrierStore{Store: sqliteA, barrier: barrier}
	runStoreB := &sharedBarrierStore{Store: sqliteB, barrier: barrier}
	provider := newHeldProvider()
	runnerA := NewRunner(provider, NewRegistry(), RunnerConfig{DefaultModel: "test-model", MaxSteps: 1, Store: runStoreA})
	runnerB := NewRunner(provider, NewRegistry(), RunnerConfig{DefaultModel: "test-model", MaxSteps: 1, Store: runStoreB})
	t.Cleanup(func() {
		provider.unblockAll()
		_ = runnerA.Shutdown(context.Background())
		_ = runnerB.Shutdown(context.Background())
		_ = sqliteA.Close()
		_ = sqliteB.Close()
	})

	type result struct {
		run Run
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for _, tc := range []struct {
		runner *Runner
		agent  string
	}{
		{runner: runnerA, agent: "agent-a"},
		{runner: runnerB, agent: "agent-b"},
	} {
		tc := tc
		go func() {
			<-start
			run, err := tc.runner.StartRun(RunRequest{
				Prompt:         "claim ordinary sqlite conversation",
				TenantID:       "tenant-shared",
				AgentID:        tc.agent,
				ConversationID: conversationID,
			})
			results <- result{run: run, err: err}
		}()
	}
	close(start)

	successes := 0
	denials := 0
	winnerRunID := ""
	for i := 0; i < 2; i++ {
		got := <-results
		switch {
		case got.err == nil:
			successes++
			winnerRunID = got.run.ID
		case errors.Is(got.err, ErrConversationAccessDenied):
			denials++
		default:
			t.Fatalf("unexpected ordinary SQLite owner claim error: %v", got.err)
		}
	}
	if successes != 1 || denials != 1 {
		t.Fatalf("ordinary SQLite owner claims: successes=%d denials=%d, want 1/1", successes, denials)
	}
	persisted, err := sqliteA.ListRuns(ctx, store.RunFilter{ConversationID: conversationID})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != winnerRunID {
		t.Fatalf("persisted ordinary SQLite runs = %+v, want only winner %q", persisted, winnerRunID)
	}
	select {
	case <-provider.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("winning ordinary SQLite owner never dispatched")
	}
	select {
	case <-provider.entered:
		t.Fatal("losing ordinary SQLite owner also dispatched")
	case <-time.After(50 * time.Millisecond):
	}
}

func (s *barrierGetRunStore) GetRun(ctx context.Context, id string) (*store.Run, error) {
	if s.getCalls.Add(1) == 2 {
		s.once.Do(func() { close(s.release) })
	}
	select {
	case <-s.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.MemoryStore.GetRun(ctx, id)
}

func TestResumeRunWithIDCannotDispatchSameReservedRunConcurrently(t *testing.T) {
	memory := store.NewMemoryStore()
	now := time.Now().UTC()
	persisted := &store.Run{
		ID:             "run_concurrent-resume",
		ConversationID: "run_concurrent-resume",
		TenantID:       "tenant-concurrent-resume",
		AgentID:        "default",
		Model:          "test-model",
		Prompt:         "resume exactly once",
		Status:         store.RunStatusQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := memory.CreateRun(context.Background(), persisted); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	runStore := &barrierGetRunStore{
		MemoryStore: memory,
		release:     make(chan struct{}),
	}
	provider := newHeldProvider()
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:        "test-model",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            1,
		WorkerPoolSize:      1,
		Store:               runStore,
	})
	t.Cleanup(func() {
		provider.unblockAll()
		_ = runner.Shutdown(context.Background())
	})

	req := RunRequest{
		Prompt:   persisted.Prompt,
		TenantID: persisted.TenantID,
		AgentID:  persisted.AgentID,
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := runner.ResumeRunWithID(req, persisted.ID)
			errs <- err
		}()
	}
	close(start)

	successes := 0
	for i := 0; i < 2; i++ {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent resumes = %d, want exactly 1", successes)
	}
}

func TestResumeRunWithIDExecutesWithPersistedModel(t *testing.T) {
	runStore := store.NewMemoryStore()
	now := time.Now().UTC()
	persisted := &store.Run{
		ID:             "run_persisted-model",
		ConversationID: "run_persisted-model",
		TenantID:       "tenant-persisted-model",
		AgentID:        "default",
		Model:          "persisted-model",
		Prompt:         "use the durable model",
		Status:         store.RunStatusQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runStore.CreateRun(context.Background(), persisted); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	provider := &capturingProvider{turns: []CompletionResult{{Content: "done"}}}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{
		DefaultModel:        "new-process-default",
		DefaultSystemPrompt: "You are helpful.",
		MaxSteps:            1,
		Store:               runStore,
	})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })

	if _, err := runner.ResumeRunWithID(RunRequest{
		Prompt:   persisted.Prompt,
		TenantID: persisted.TenantID,
		AgentID:  persisted.AgentID,
	}, persisted.ID); err != nil {
		t.Fatalf("ResumeRunWithID: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		provider.mu.Lock()
		if len(provider.calls) > 0 {
			model := provider.calls[0].Model
			provider.mu.Unlock()
			if model != persisted.Model {
				t.Fatalf("completion model = %q, want persisted %q", model, persisted.Model)
			}
			break
		}
		provider.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("resumed run never reached provider")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestResumeRunWithIDLoadsPersistedModelBeforeAttachmentPreflight(t *testing.T) {
	runStore := store.NewMemoryStore()
	now := time.Now().UTC()
	persisted := &store.Run{
		ID: "run_persisted-model-before-preflight", ConversationID: "run_persisted-model-before-preflight",
		TenantID: "tenant-persisted-model", AgentID: "default", Model: "gpt-4.1", Prompt: "use vision",
		Status: store.RunStatusQueued, CreatedAt: now, UpdatedAt: now,
	}
	if err := runStore.CreateRun(context.Background(), persisted); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel: "claude-sonnet-4-6", MaxSteps: 1, Store: runStore,
		ProviderRegistry: catalog.NewProviderRegistry(attachmentTestCatalog(t)),
	})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	if _, err := runner.ResumeRunWithID(RunRequest{
		Prompt: persisted.Prompt, TenantID: persisted.TenantID, AgentID: persisted.AgentID,
		Attachments: []ContentBlock{imageAttachment()},
	}, persisted.ID); err != nil {
		t.Fatalf("ResumeRunWithID must preflight with persisted vision model: %v", err)
	}
}

func TestStartRunWithIDHonorsTerminalDurabilityCapacity(t *testing.T) {
	runStore := &recoveringTerminalStatusStore{Store: store.NewMemoryStore()}
	runStore.fail.Store(true)
	runner := NewRunner(staticContentProvider{content: "done"}, NewRegistry(), RunnerConfig{
		DefaultModel:          "test-model",
		DefaultSystemPrompt:   "You are helpful.",
		MaxSteps:              1,
		MaxCompletedRetention: 1,
		Store:                 runStore,
	})
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })

	first, err := runner.StartRun(RunRequest{Prompt: "leave terminal durability pending"})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	waitForStatus(t, runner, first.ID, RunStatusCompleted)

	reservedID := "run_reserved-terminal-capacity"
	_, err = runner.StartRunWithID(RunRequest{Prompt: "must not persist or dispatch"}, reservedID)
	requireTerminalDurabilityBackpressure(t, err, 1, 1)
	if _, err := runStore.GetRun(context.Background(), reservedID); err == nil {
		t.Fatalf("reserved run %q persisted despite terminal durability backpressure", reservedID)
	}
}
