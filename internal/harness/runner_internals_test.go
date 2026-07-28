package harness

// Direct tests for runner and registry internals that the end-to-end run tests
// only reach incidentally, if at all: the flat catalog export, profile-run
// persistence, and the changed-file extraction behind workflow recaps.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/store"
)

// --- Registry.CatalogTools --------------------------------------------

func TestRegistryCatalogTools(t *testing.T) {
	registry := NewRegistry()

	if err := registry.Register(ToolDefinition{
		Name:         "zeta",
		Description:  "last alphabetically",
		Parameters:   map[string]any{"type": "object"},
		ParallelSafe: true,
		Mutating:     false,
	}, func(context.Context, json.RawMessage) (string, error) { return "zeta-out", nil }); err != nil {
		t.Fatalf("register zeta: %v", err)
	}
	if err := registry.RegisterWithOptions(ToolDefinition{
		Name:         "alpha",
		Description:  "first alphabetically",
		Parameters:   map[string]any{"type": "object"},
		ParallelSafe: false,
		Mutating:     true,
	}, func(context.Context, json.RawMessage) (string, error) { return "alpha-out", nil },
		RegisterOptions{Tier: htools.TierDeferred, Tags: []string{"tag-a", "tag-b"}}); err != nil {
		t.Fatalf("register alpha: %v", err)
	}

	catalog := registry.CatalogTools()
	if len(catalog) != 2 {
		t.Fatalf("catalog has %d tools, want 2", len(catalog))
	}

	// Sorted by name, so the deferred "alpha" comes first.
	if catalog[0].Definition.Name != "alpha" || catalog[1].Definition.Name != "zeta" {
		t.Errorf("catalog is not sorted by name: %q, %q", catalog[0].Definition.Name, catalog[1].Definition.Name)
	}

	alpha := catalog[0]
	if alpha.Definition.Description != "first alphabetically" {
		t.Errorf("description = %q, want it carried through", alpha.Definition.Description)
	}
	if alpha.Definition.Tier != htools.TierDeferred {
		t.Errorf("tier = %q, want the registered tier", alpha.Definition.Tier)
	}
	if len(alpha.Definition.Tags) != 2 || alpha.Definition.Tags[0] != "tag-a" {
		t.Errorf("tags = %v, want the registered tags", alpha.Definition.Tags)
	}
	if !alpha.Definition.Mutating || alpha.Definition.ParallelSafe {
		t.Errorf("alpha flags wrong: mutating=%v parallelSafe=%v", alpha.Definition.Mutating, alpha.Definition.ParallelSafe)
	}
	if catalog[1].Definition.Tier != htools.TierCore {
		t.Errorf("a plainly-registered tool should be core tier, got %q", catalog[1].Definition.Tier)
	}

	// Each entry's handler must be bound to its OWN tool, not to whichever
	// one the loop variable happened to end on.
	for _, tool := range catalog {
		out, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("%s handler: %v", tool.Definition.Name, err)
		}
		if out != tool.Definition.Name+"-out" {
			t.Errorf("%s handler returned %q — handlers are mis-bound", tool.Definition.Name, out)
		}
	}

	// Mutating the returned definitions must not corrupt the registry.
	catalog[0].Definition.Description = "mutated"
	if again := registry.CatalogTools(); again[0].Definition.Description != "first alphabetically" {
		t.Error("CatalogTools must return copies callers cannot use to mutate the registry")
	}

	if len(NewRegistry().CatalogTools()) != 0 {
		t.Error("an empty registry should produce an empty catalog")
	}
}

// --- persistProfileRun ------------------------------------------------

type recordingProfileRunStore struct {
	mu      sync.Mutex
	records []store.ProfileRunRecord
	err     error
}

func (s *recordingProfileRunStore) RecordProfileRun(_ context.Context, r store.ProfileRunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	return s.err
}

func (s *recordingProfileRunStore) QueryRecentProfileRuns(context.Context, string, int) ([]store.ProfileRunRecord, error) {
	return nil, nil
}

func (s *recordingProfileRunStore) AggregateProfileStats(context.Context, string) (store.ProfileStats, error) {
	return store.ProfileStats{}, nil
}

func (s *recordingProfileRunStore) snapshot() []store.ProfileRunRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.ProfileRunRecord(nil), s.records...)
}

// newRunnerWithProfileRun builds a runner carrying a single run state suitable
// for exercising persistProfileRun directly.
func newRunnerWithProfileRun(t *testing.T, profileName string, st *recordingProfileRunStore) *Runner {
	t.Helper()
	cfg := RunnerConfig{DefaultModel: "m", ProfileRunStore: st}
	r := &Runner{runs: map[string]*runState{}, skillConstraints: NewSkillConstraintTracker()}
	r.config = cfg
	r.runs["run-1"] = &runState{
		config:      &cfg,
		profileName: profileName,
		currentStep: 4,
		run:         Run{ID: "run-1", CreatedAt: time.Now().UTC().Add(-time.Minute)},
		events: []Event{
			{Type: EventToolCallStarted, Payload: map[string]any{"tool": "read"}},
			{Type: EventToolCallStarted, Payload: map[string]any{"tool": "bash"}},
			{Type: EventToolCallStarted, Payload: map[string]any{"tool": ""}},       // ignored
			{Type: EventToolCallStarted, Payload: map[string]any{"tool": 42}},       // ignored: wrong type
			{Type: EventRunStepStarted, Payload: map[string]any{"tool": "ignored"}}, // ignored: wrong event
		},
	}
	return r
}

func TestPersistProfileRun(t *testing.T) {
	t.Run("records the run with the tools it used", func(t *testing.T) {
		st := &recordingProfileRunStore{}
		r := newRunnerWithProfileRun(t, "researcher", st)

		r.persistProfileRun("run-1", "completed", 1.25)

		got := st.snapshot()
		if len(got) != 1 {
			t.Fatalf("recorded %d runs, want 1", len(got))
		}
		rec := got[0]
		if rec.ProfileName != "researcher" || rec.RunID != "run-1" {
			t.Errorf("record identity wrong: profile=%q run=%q", rec.ProfileName, rec.RunID)
		}
		if rec.ID != "researcher:run-1" {
			t.Errorf("record ID = %q, want \"researcher:run-1\"", rec.ID)
		}
		if rec.StepCount != 4 {
			t.Errorf("step count = %d, want 4", rec.StepCount)
		}
		if rec.ToolCalls != 2 {
			t.Errorf("tool call count = %d, want 2 (only well-formed events count)", rec.ToolCalls)
		}
		if rec.CostUSD != 1.25 {
			t.Errorf("cost = %v, want 1.25", rec.CostUSD)
		}
		if rec.Status != "completed" {
			t.Errorf("status = %q, want completed", rec.Status)
		}
		// Only well-formed tool-call-started events contribute.
		joined := strings.Join(rec.TopTools, ",")
		if !strings.Contains(joined, "read") || !strings.Contains(joined, "bash") {
			t.Errorf("top tools = %q, want it to include read and bash", joined)
		}
	})

	t.Run("does nothing without a profile name", func(t *testing.T) {
		st := &recordingProfileRunStore{}
		r := newRunnerWithProfileRun(t, "", st)
		r.persistProfileRun("run-1", "completed", 1)
		if len(st.snapshot()) != 0 {
			t.Error("a run with no profile must not be recorded")
		}
	})

	t.Run("does nothing for an unknown run", func(t *testing.T) {
		st := &recordingProfileRunStore{}
		r := newRunnerWithProfileRun(t, "researcher", st)
		r.persistProfileRun("no-such-run", "completed", 1)
		if len(st.snapshot()) != 0 {
			t.Error("an unknown run must not be recorded")
		}
	})

	t.Run("a store error is swallowed rather than panicking", func(t *testing.T) {
		st := &recordingProfileRunStore{err: errors.New("disk on fire")}
		r := newRunnerWithProfileRun(t, "researcher", st)
		r.persistProfileRun("run-1", "failed", 0) // must not panic
		if len(st.snapshot()) != 1 {
			t.Error("the store should still have been called")
		}
	})

	t.Run("does nothing when no store is configured", func(t *testing.T) {
		r := newRunnerWithProfileRun(t, "researcher", nil)
		cfg := RunnerConfig{DefaultModel: "m"}
		r.config = cfg
		r.runs["run-1"].config = &cfg
		r.persistProfileRun("run-1", "completed", 1) // must not panic
	})
}

// --- changed-file extraction ------------------------------------------

func TestChangedFilesFromTrace(t *testing.T) {
	events := []Event{
		{Type: EventToolCallStarted, Payload: map[string]any{
			"tool": "write", "arguments": `{"path":"src/a.go"}`}},
		{Type: EventToolCallStarted, Payload: map[string]any{
			"tool": "edit", "arguments": `{"file_path":"src/b.go"}`}},
		{Type: EventToolCallStarted, Payload: map[string]any{
			"tool": "write", "arguments": `{"path":"src/a.go"}`}}, // duplicate, must collapse
		{Type: EventRunStepStarted, Payload: map[string]any{
			"tool": "write", "arguments": `{"path":"not-a-tool-call.go"}`}}, // wrong event type
	}

	got := changedFilesFromTrace(nil, events)
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "src/a.go") || !strings.Contains(joined, "src/b.go") {
		t.Errorf("changed files = %v, want both edited files", got)
	}
	if strings.Contains(joined, "not-a-tool-call.go") {
		t.Errorf("changed files = %v, must ignore non-tool-call events", got)
	}
	if strings.Count(joined, "src/a.go") != 1 {
		t.Errorf("changed files = %v, duplicates must collapse", got)
	}
}

// TestPatchFilesReadsBothPatchFormats pins the fix for an under-reporting bug:
// apply_patch accepts BOTH the harness's "*** Begin Patch" format and standard
// unified diffs, but patchFiles only recognised the former, so a plain diff
// contributed no files to the recap's changed-file list.
func TestPatchFilesReadsBothPatchFormats(t *testing.T) {
	custom := "*** Begin Patch\n*** Update File: src/a.go\n@@\n-x\n+y\n*** Add File: src/new.go\n+z\n*** Delete File: src/gone.go\n*** End Patch"
	got := patchFiles(custom)
	if len(got) != 3 {
		t.Errorf("custom format produced %v, want three paths", got)
	}

	unified := "--- a/src/b.go\n+++ b/src/b.go\n@@ -1 +1 @@\n-x\n+y\n"
	got = patchFiles(unified)
	if len(got) != 1 || got[0] != "src/b.go" {
		t.Errorf("unified diff produced %v, want [src/b.go]", got)
	}

	// A deletion's /dev/null destination is not a changed file path.
	deletion := "--- a/src/gone.go\n+++ /dev/null\n@@ -1 +0 @@\n-x\n"
	if got := patchFiles(deletion); len(got) != 0 {
		t.Errorf("/dev/null destination produced %v, want no paths", got)
	}

	// A trailing timestamp on the +++ line must not become part of the path.
	withTimestamp := "--- a/src/c.go\t2026-01-01\n+++ b/src/c.go\t2026-01-01 10:00:00\n@@ -1 +1 @@\n-x\n+y\n"
	if got := patchFiles(withTimestamp); len(got) != 1 || got[0] != "src/c.go" {
		t.Errorf("timestamped diff header produced %v, want [src/c.go]", got)
	}

	if got := patchFiles(""); len(got) != 0 {
		t.Errorf("an empty patch produced %v, want none", got)
	}
}

func TestCleanTracePath(t *testing.T) {
	for in, want := range map[string]string{
		`  src/a.go  `: "src/a.go",
		`"src/a.go"`:   "src/a.go",
		`'src/a.go'`:   "src/a.go",
		``:             "",
		`   `:          "",
		"bad\x00path":  "",
	} {
		if got := cleanTracePath(in); got != want {
			t.Errorf("cleanTracePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- small accessors --------------------------------------------------

func TestRunnerAccessors(t *testing.T) {
	broker := NewInMemoryApprovalBroker()
	r := NewRunner(&funcProvider{fn: func(context.Context, CompletionRequest) (CompletionResult, error) {
		return CompletionResult{Content: "ok"}, nil
	}}, NewRegistry(), RunnerConfig{DefaultModel: "test-model", ApprovalBroker: broker})

	if got := r.Config().DefaultModel; got != "test-model" {
		t.Errorf("Config().DefaultModel = %q, want test-model", got)
	}
	if r.ApprovalBroker() == nil {
		t.Error("ApprovalBroker() should return the configured broker")
	}

	// EmitEvent is the exported wrapper transports use to push a custom event
	// into a run's ledger; it must reach subscribers.
	run, err := r.StartRun(RunRequest{Prompt: "hi"})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if _, err := collectRunEvents(t, r, run.ID); err != nil {
		t.Fatalf("collect: %v", err)
	}
	r.EmitEvent(run.ID, EventRunStepStarted, map[string]any{"custom": true})
}
