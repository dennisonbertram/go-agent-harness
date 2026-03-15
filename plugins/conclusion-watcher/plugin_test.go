package conclusionwatcher_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/internal/harness"

	cw "go-agent-harness/plugins/conclusion-watcher"
)

// ============================================================
// Task 1 — ObservationLedger tests
// ============================================================

func TestLedger_RecordAndHasSeenFile(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordFileSeen("foo/bar.go")
	if !l.HasSeenFile("foo/bar.go") {
		t.Fatal("expected HasSeenFile to return true for recorded file")
	}
}

func TestLedger_UnseenFileReturnsFalse(t *testing.T) {
	l := cw.NewObservationLedger()
	if l.HasSeenFile("never/seen.go") {
		t.Fatal("expected HasSeenFile to return false for unseen file")
	}
}

func TestLedger_PathNormalization(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordFileSeen("./foo/bar.go")
	if !l.HasSeenFile("foo/bar.go") {
		t.Fatal("expected ./foo/bar.go and foo/bar.go to be the same")
	}
	if !l.HasSeenFile("./foo/bar.go") {
		t.Fatal("expected ./foo/bar.go lookup to still work")
	}
}

func TestLedger_RecordTool_RecentTools(t *testing.T) {
	l := cw.NewObservationLedger()
	tools := []string{"read_file", "grep", "glob", "bash", "write_file"}
	for i, name := range tools {
		l.RecordTool(i+1, name, "")
	}
	recent := l.RecentTools(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent tools, got %d", len(recent))
	}
	// most recent last: glob, bash, write_file
	if recent[0] != "glob" || recent[1] != "bash" || recent[2] != "write_file" {
		t.Fatalf("unexpected recent tools: %v", recent)
	}
}

func TestLedger_RecentTools_AllWhenNLessOrEqual0(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "read_file", "")
	l.RecordTool(2, "grep", "")
	recent := l.RecentTools(0)
	if len(recent) != 2 {
		t.Fatalf("expected 2 tools for n<=0, got %d", len(recent))
	}
}

func TestLedger_LastStepHadDiagnostic_True(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(5, "read_file", "")
	if !l.LastStepHadDiagnostic(5) {
		t.Fatal("expected LastStepHadDiagnostic to be true at step 5")
	}
}

func TestLedger_LastStepHadDiagnostic_False(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(3, "write_file", "") // mutating, not diagnostic
	if l.LastStepHadDiagnostic(5) {
		t.Fatal("expected LastStepHadDiagnostic to be false when no diagnostic at step 5 or 4")
	}
}

func TestLedger_LastStepHadDiagnostic_PrevStep(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(4, "grep", "") // step 4 = step-1 for step 5
	if !l.LastStepHadDiagnostic(5) {
		t.Fatal("expected diagnostic at step N-1 to count for step N")
	}
}

func TestLedger_LastStepHadExploration_True(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(2, "git_log", "")
	if !l.LastStepHadExploration(2) {
		t.Fatal("expected git_log to count as exploration")
	}
}

func TestLedger_LastStepHadExploration_False(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "bash", "") // bash is diagnostic but not exploration
	if l.LastStepHadExploration(1) {
		t.Fatal("expected bash alone to not count as exploration")
	}
}

func TestLedger_ConcurrencySafe(t *testing.T) {
	l := cw.NewObservationLedger()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.RecordTool(n, "read_file", "")
			l.RecordFileSeen("concurrent/file.go")
			l.HasSeenFile("concurrent/file.go")
			l.LastStepHadDiagnostic(n)
		}(i)
	}
	wg.Wait()
}

func TestLedger_Reset(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordFileSeen("foo.go")
	l.RecordTool(1, "read_file", "")
	l.Reset()
	if l.HasSeenFile("foo.go") {
		t.Fatal("expected file to be gone after Reset")
	}
	if len(l.RecentTools(0)) != 0 {
		t.Fatal("expected tool history to be empty after Reset")
	}
}

// ============================================================
// Task 2 — DetectHedgeAssertion tests
// ============================================================

func TestDetectHedgeAssertion_MustBe(t *testing.T) {
	result := cw.DetectHedgeAssertion("run-1", 1, "the answer must be 42")
	if result == nil {
		t.Fatal("expected detection to fire for 'must be'")
	}
	if result.Pattern != cw.PatternHedgeAssertion {
		t.Fatalf("expected PatternHedgeAssertion, got %s", result.Pattern)
	}
}

func TestDetectHedgeAssertion_Obviously(t *testing.T) {
	result := cw.DetectHedgeAssertion("run-1", 2, "obviously this is correct")
	if result == nil {
		t.Fatal("expected detection for 'obviously'")
	}
}

func TestDetectHedgeAssertion_NoMatch(t *testing.T) {
	result := cw.DetectHedgeAssertion("run-1", 1, "The function reads from the database")
	if result != nil {
		t.Fatal("expected nil for neutral content")
	}
}

func TestDetectHedgeAssertion_CaseInsensitive(t *testing.T) {
	result := cw.DetectHedgeAssertion("run-1", 1, "CLEARLY this is the right approach")
	if result == nil {
		t.Fatal("expected detection to be case-insensitive")
	}
}

func TestDetectHedgeAssertion_Fields(t *testing.T) {
	result := cw.DetectHedgeAssertion("run-abc", 7, "I assume this is correct")
	if result == nil {
		t.Fatal("expected detection")
	}
	if result.RunID != "run-abc" {
		t.Errorf("expected RunID run-abc, got %s", result.RunID)
	}
	if result.Step != 7 {
		t.Errorf("expected Step 7, got %d", result.Step)
	}
	if result.Evidence == "" {
		t.Error("expected non-empty Evidence")
	}
}

func TestDetectHedgeAssertion_EmptyContent(t *testing.T) {
	result := cw.DetectHedgeAssertion("run-1", 1, "")
	if result != nil {
		t.Fatal("empty content must return nil")
	}
}

// ============================================================
// Task 3 — DetectUnverifiedFileClaim tests
// ============================================================

func TestDetectUnverifiedFileClaim_UnseenFile(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectUnverifiedFileClaim("run-1", 1,
		"the file foo.go must be correct", l)
	if result == nil {
		t.Fatal("expected detection when file not in ledger and assertion keyword present")
	}
	if result.Pattern != cw.PatternUnverifiedFileClaim {
		t.Fatalf("expected PatternUnverifiedFileClaim, got %s", result.Pattern)
	}
}

func TestDetectUnverifiedFileClaim_SeenFile(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordFileSeen("foo.go")
	result := cw.DetectUnverifiedFileClaim("run-1", 1,
		"the file foo.go must be correct", l)
	if result != nil {
		t.Fatal("expected nil when file is in ledger")
	}
}

func TestDetectUnverifiedFileClaim_NoAssertionKeyword(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectUnverifiedFileClaim("run-1", 1,
		"the file config.yaml is loaded here", l)
	if result != nil {
		t.Fatal("expected nil when no assertion keyword present")
	}
}

func TestDetectUnverifiedFileClaim_PathNormalization(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordFileSeen("./internal/foo.go") // recorded with ./
	result := cw.DetectUnverifiedFileClaim("run-1", 1,
		"internal/foo.go clearly exists", l) // referenced without ./
	if result != nil {
		t.Fatal("expected nil because ./internal/foo.go and internal/foo.go are the same")
	}
}

func TestDetectUnverifiedFileClaim_MultipleFiles(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordFileSeen("seen.go")
	// content references seen.go (in ledger) and unseen.go (not in ledger)
	result := cw.DetectUnverifiedFileClaim("run-1", 1,
		"seen.go is fine but unseen.go must be broken", l)
	if result == nil {
		t.Fatal("expected detection for unseen.go")
	}
	if !strings.Contains(result.Evidence, "unseen.go") {
		t.Errorf("expected Evidence to mention unseen.go, got: %s", result.Evidence)
	}
}

func TestDetectUnverifiedFileClaim_NilLedger(t *testing.T) {
	// must not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DetectUnverifiedFileClaim panicked with nil ledger: %v", r)
		}
	}()
	cw.DetectUnverifiedFileClaim("run-1", 1, "foo.go must be correct", nil)
}

// ============================================================
// Task 4 — DetectPrematureCompletion tests
// ============================================================

func TestDetectPrematureCompletion_DoneNoTests(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "read_file", "")
	l.RecordTool(2, "write_file", "")
	result := cw.DetectPrematureCompletion("run-1", 3, "all done, fixed it", l)
	if result == nil {
		t.Fatal("expected detection for done without test tool in last 3 steps")
	}
	if result.Pattern != cw.PatternPrematureCompletion {
		t.Fatalf("expected PatternPrematureCompletion, got %s", result.Pattern)
	}
}

func TestDetectPrematureCompletion_DoneWithTests(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "bash", `{"command":"go test ./..."}`)
	result := cw.DetectPrematureCompletion("run-1", 2, "fixed it", l)
	if result != nil {
		t.Fatal("expected nil when bash with test keyword in last 3 steps")
	}
}

func TestDetectPrematureCompletion_NoCompletionTerms(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectPrematureCompletion("run-1", 1, "I will now read the file", l)
	if result != nil {
		t.Fatal("expected nil for neutral response")
	}
}

func TestDetectPrematureCompletion_BashMustHaveTestKeyword(t *testing.T) {
	l := cw.NewObservationLedger()
	// bash without test keyword should NOT count as verification
	l.RecordTool(1, "bash", `{"command":"ls -la"}`)
	result := cw.DetectPrematureCompletion("run-1", 2, "done and complete", l)
	if result == nil {
		t.Fatal("expected detection: bash without test keyword does not count as verification")
	}
}

func TestDetectPrematureCompletion_RunTestsTool(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "run_tests", "")
	result := cw.DetectPrematureCompletion("run-1", 2, "implementation is complete", l)
	if result != nil {
		t.Fatal("expected nil when run_tests used recently")
	}
}

func TestDetectPrematureCompletion_EmptyContent(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectPrematureCompletion("run-1", 1, "", l)
	if result != nil {
		t.Fatal("empty content must return nil")
	}
}

// ============================================================
// Task 5 — DetectSkippedDiagnostic tests
// ============================================================

func TestDetectSkippedDiagnostic_WriteFileNoDiag(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectSkippedDiagnostic("run-1", 1, "write_file",
		json.RawMessage(`{"path":"foo.go","content":"x"}`), l)
	if result == nil {
		t.Fatal("expected detection for write_file with no prior diagnostic")
	}
	if result.Pattern != cw.PatternSkippedDiagnostic {
		t.Fatalf("expected PatternSkippedDiagnostic, got %s", result.Pattern)
	}
}

func TestDetectSkippedDiagnostic_WriteFileWithDiag(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "read_file", "") // diagnostic at step 1, write at step 2
	result := cw.DetectSkippedDiagnostic("run-1", 2, "write_file",
		json.RawMessage(`{"path":"foo.go","content":"x"}`), l)
	if result != nil {
		t.Fatal("expected nil when read_file in previous step")
	}
}

func TestDetectSkippedDiagnostic_BashDestructiveNoDiag(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectSkippedDiagnostic("run-1", 1, "bash",
		json.RawMessage(`{"command":"rm -rf foo"}`), l)
	if result == nil {
		t.Fatal("expected detection for destructive bash with no prior diagnostic")
	}
}

func TestDetectSkippedDiagnostic_BashNonDestructive(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectSkippedDiagnostic("run-1", 1, "bash",
		json.RawMessage(`{"command":"go build ./..."}`), l)
	if result != nil {
		t.Fatal("expected nil for non-destructive bash")
	}
}

func TestDetectSkippedDiagnostic_EditFileCurrentStepDiag(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(3, "grep", "") // current step = 3
	result := cw.DetectSkippedDiagnostic("run-1", 3, "edit_file",
		json.RawMessage(`{"path":"foo.go"}`), l)
	if result != nil {
		t.Fatal("expected nil when diagnostic in current step")
	}
}

func TestDetectSkippedDiagnostic_NilArgs(t *testing.T) {
	// must not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DetectSkippedDiagnostic panicked with nil args: %v", r)
		}
	}()
	l := cw.NewObservationLedger()
	cw.DetectSkippedDiagnostic("run-1", 1, "bash", nil, l)
}

func TestDetectSkippedDiagnostic_NilLedger(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DetectSkippedDiagnostic panicked with nil ledger: %v", r)
		}
	}()
	cw.DetectSkippedDiagnostic("run-1", 1, "write_file",
		json.RawMessage(`{}`), nil)
}

// ============================================================
// Task 6 — DetectArchitectureAssumption tests
// ============================================================

func TestDetectArchitectureAssumption_PhraseNoExploration(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "bash", "") // bash not exploration
	result := cw.DetectArchitectureAssumption("run-1", 2,
		"the design is clearly a layered architecture", l)
	if result == nil {
		t.Fatal("expected detection for architecture phrase with no exploration in last 3 steps")
	}
	if result.Pattern != cw.PatternArchitectureAssumption {
		t.Fatalf("expected PatternArchitectureAssumption, got %s", result.Pattern)
	}
}

func TestDetectArchitectureAssumption_PhraseWithExploration(t *testing.T) {
	l := cw.NewObservationLedger()
	l.RecordTool(1, "read_file", "") // exploration 2 steps ago
	result := cw.DetectArchitectureAssumption("run-1", 2,
		"the flow is handled by the router", l)
	if result != nil {
		t.Fatal("expected nil when read_file in recent history")
	}
}

func TestDetectArchitectureAssumption_CaseInsensitive(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectArchitectureAssumption("run-1", 1,
		"The Flow Is managed by the dispatcher", l)
	if result == nil {
		t.Fatal("expected detection to be case-insensitive")
	}
}

func TestDetectArchitectureAssumption_EmptyContent(t *testing.T) {
	l := cw.NewObservationLedger()
	result := cw.DetectArchitectureAssumption("run-1", 1, "", l)
	if result != nil {
		t.Fatal("empty content must return nil")
	}
}

func TestDetectArchitectureAssumption_NilLedger(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DetectArchitectureAssumption panicked with nil ledger: %v", r)
		}
	}()
	cw.DetectArchitectureAssumption("run-1", 1, "the design is X", nil)
}

// ============================================================
// Task 7 — InjectValidationPrompt tests
// ============================================================

func TestInjectValidationPrompt_AppendsText(t *testing.T) {
	response := harness.CompletionResult{Content: "Original content"}
	detection := cw.DetectionResult{
		Pattern:    cw.PatternHedgeAssertion,
		Confidence: 1.0,
		Evidence:   "must be",
		Step:       1,
		RunID:      "run-1",
	}
	prompt := "VERIFY THIS"
	result := cw.InjectValidationPrompt(
		harness.PostMessageHookResult{Action: harness.HookActionContinue},
		&response,
		prompt,
		detection,
	)
	if result.MutatedResponse == nil {
		t.Fatal("expected MutatedResponse to be set")
	}
	if !strings.Contains(result.MutatedResponse.Content, "VERIFY THIS") {
		t.Errorf("expected mutated content to contain prompt, got: %s", result.MutatedResponse.Content)
	}
	if !strings.Contains(result.MutatedResponse.Content, "Original content") {
		t.Errorf("expected mutated content to contain original text")
	}
}

func TestInjectValidationPrompt_ActionIsContinue(t *testing.T) {
	response := harness.CompletionResult{Content: "hello"}
	detection := cw.DetectionResult{Pattern: cw.PatternHedgeAssertion}
	result := cw.InjectValidationPrompt(
		harness.PostMessageHookResult{},
		&response,
		"prompt",
		detection,
	)
	if result.Action != harness.HookActionContinue {
		t.Errorf("expected HookActionContinue, got %s", result.Action)
	}
}

func TestInjectValidationPrompt_MutatedResponseSet(t *testing.T) {
	response := harness.CompletionResult{Content: "hello"}
	detection := cw.DetectionResult{Pattern: cw.PatternHedgeAssertion}
	result := cw.InjectValidationPrompt(
		harness.PostMessageHookResult{},
		&response,
		"test prompt",
		detection,
	)
	if result.MutatedResponse == nil {
		t.Fatal("expected MutatedResponse to be non-nil")
	}
}

// ============================================================
// Task 8 — PauseForUser tests
// ============================================================

func TestPauseForUser_ActionIsBlock(t *testing.T) {
	detection := cw.DetectionResult{
		Pattern:  cw.PatternSkippedDiagnostic,
		Evidence: "write_file without prior read",
	}
	result := cw.PauseForUser(detection)
	if result.Action != harness.HookActionBlock {
		t.Errorf("expected HookActionBlock, got %s", result.Action)
	}
}

func TestPauseForUser_ReasonContainsEvidence(t *testing.T) {
	detection := cw.DetectionResult{
		Pattern:  cw.PatternSkippedDiagnostic,
		Evidence: "rm -rf called without reading first",
	}
	result := cw.PauseForUser(detection)
	if !strings.Contains(result.Reason, detection.Evidence) {
		t.Errorf("expected Reason to contain Evidence, got: %s", result.Reason)
	}
}

// ============================================================
// Task 9 — RequestCritique tests
// ============================================================

type mockCritiqueProvider struct {
	critique string
	err      error
	called   bool
	gotCtx   context.Context
}

func (m *mockCritiqueProvider) Critique(ctx context.Context, content string) (string, error) {
	m.called = true
	m.gotCtx = ctx
	return m.critique, m.err
}

func TestRequestCritique_InjectsCritique(t *testing.T) {
	provider := &mockCritiqueProvider{critique: "X is wrong here"}
	response := harness.CompletionResult{Content: "I assume this is correct"}
	detection := cw.DetectionResult{Pattern: cw.PatternHedgeAssertion, Evidence: "I assume"}
	result, err := cw.RequestCritique(
		context.Background(),
		harness.PostMessageHookResult{Action: harness.HookActionContinue},
		&response,
		detection,
		provider,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MutatedResponse == nil {
		t.Fatal("expected MutatedResponse to be set")
	}
	if !strings.Contains(result.MutatedResponse.Content, "X is wrong here") {
		t.Errorf("expected critique injected, got: %s", result.MutatedResponse.Content)
	}
}

func TestRequestCritique_ProviderError(t *testing.T) {
	provider := &mockCritiqueProvider{err: context.DeadlineExceeded}
	response := harness.CompletionResult{Content: "some content"}
	detection := cw.DetectionResult{Pattern: cw.PatternHedgeAssertion}
	_, err := cw.RequestCritique(
		context.Background(),
		harness.PostMessageHookResult{},
		&response,
		detection,
		provider,
	)
	if err == nil {
		t.Fatal("expected error when provider fails")
	}
}

func TestRequestCritique_ContextPropagated(t *testing.T) {
	provider := &mockCritiqueProvider{critique: "ok"}
	response := harness.CompletionResult{Content: "content"}
	detection := cw.DetectionResult{Pattern: cw.PatternHedgeAssertion}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := cw.RequestCritique(ctx,
		harness.PostMessageHookResult{},
		&response,
		detection,
		provider,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.gotCtx != ctx {
		t.Error("expected context to be propagated to provider")
	}
}

// ============================================================
// Task 10 — ConclusionWatcher core + Register tests
// ============================================================

func TestNew_Defaults(t *testing.T) {
	w := cw.New(cw.WatcherConfig{})
	if w == nil {
		t.Fatal("New must not return nil")
	}
	if w.InterventionCount() != 0 {
		t.Fatal("expected InterventionCount 0 on new watcher")
	}
	if len(w.Detections()) != 0 {
		t.Fatal("expected empty Detections on new watcher")
	}
}

func TestRegister_AppendsHooks(t *testing.T) {
	w := cw.New(cw.WatcherConfig{})
	cfg := &harness.RunnerConfig{}
	prePre := len(cfg.PreToolUseHooks)
	postPre := len(cfg.PostMessageHooks)
	postToolPre := len(cfg.PostToolUseHooks)
	w.Register(cfg)
	if len(cfg.PreToolUseHooks) != prePre+1 {
		t.Errorf("expected PreToolUseHooks to grow by 1, got %d", len(cfg.PreToolUseHooks))
	}
	if len(cfg.PostMessageHooks) != postPre+1 {
		t.Errorf("expected PostMessageHooks to grow by 1, got %d", len(cfg.PostMessageHooks))
	}
	if len(cfg.PostToolUseHooks) != postToolPre+1 {
		t.Errorf("expected PostToolUseHooks to grow by 1, got %d", len(cfg.PostToolUseHooks))
	}
}

func TestRegister_DoesNotReplaceExistingHooks(t *testing.T) {
	w := cw.New(cw.WatcherConfig{})
	existingPre := &noopPreToolUseHook{}
	existingPost := &noopPostMessageHook{}
	cfg := &harness.RunnerConfig{
		PreToolUseHooks:  []harness.PreToolUseHook{existingPre},
		PostMessageHooks: []harness.PostMessageHook{existingPost},
	}
	w.Register(cfg)
	if cfg.PreToolUseHooks[0] != existingPre {
		t.Error("existing PreToolUseHook was replaced")
	}
	if cfg.PostMessageHooks[0] != existingPost {
		t.Error("existing PostMessageHook was replaced")
	}
}

func TestWatcher_PostMessageHook_FiresDetection(t *testing.T) {
	var emitted []string
	w := cw.New(cw.WatcherConfig{
		EventEmitter: func(eventType, runID string, payload map[string]any) {
			emitted = append(emitted, eventType)
		},
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	_, err := hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID: "run-1",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "obviously this must be correct",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.Detections()) == 0 {
		t.Fatal("expected at least one detection")
	}
	if len(emitted) == 0 {
		t.Fatal("expected EventEmitter to be called")
	}
}

func TestWatcher_PreToolUseHook_FiresOnWriteFile(t *testing.T) {
	w := cw.New(cw.WatcherConfig{Mode: cw.InterventionPauseForUser})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PreToolUseHooks[0]
	result, err := hook.PreToolUse(context.Background(), harness.PreToolUseEvent{
		RunID:    "run-1",
		ToolName: "write_file",
		Args:     json.RawMessage(`{"path":"foo.go","content":"x"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for write_file with no prior diagnostic")
	}
	if result.Decision != harness.ToolHookDeny {
		t.Errorf("expected ToolHookDeny, got %d", result.Decision)
	}
}

func TestWatcher_PostToolUseHook_UpdatesLedger(t *testing.T) {
	w := cw.New(cw.WatcherConfig{})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostToolUseHooks[0]
	_, err := hook.PostToolUse(context.Background(), harness.PostToolUseEvent{
		RunID:    "run-1",
		ToolName: "read_file",
		Args:     json.RawMessage(`{"path":"internal/foo.go"}`),
		Result:   "file content here",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Now a subsequent detection should not fire for internal/foo.go
	result := cw.DetectUnverifiedFileClaim("run-1", 2,
		"internal/foo.go must be correct", w.Ledger())
	if result != nil {
		t.Fatal("expected nil because internal/foo.go was recorded by PostToolUse")
	}
}

func TestWatcher_InterventionCount(t *testing.T) {
	w := cw.New(cw.WatcherConfig{})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	for i := 0; i < 3; i++ {
		_, _ = hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
			RunID:    "run-1",
			Step:     i + 1,
			Response: harness.CompletionResult{Content: "obviously must be correct"},
		})
	}
	if w.InterventionCount() < 1 {
		t.Fatal("expected interventions to be counted")
	}
}

func TestWatcher_MaxInterventionsRespected(t *testing.T) {
	w := cw.New(cw.WatcherConfig{
		MaxInterventionsPerRun: 2,
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	// fire 5 detections
	for i := 0; i < 5; i++ {
		_, _ = hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
			RunID:    "run-1",
			Step:     i + 1,
			Response: harness.CompletionResult{Content: "obviously must be correct"},
		})
	}
	if w.InterventionCount() > 2 {
		t.Errorf("expected at most 2 interventions, got %d", w.InterventionCount())
	}
	// But all detections should still be recorded
	if len(w.Detections()) < 5 {
		t.Errorf("expected at least 5 detections recorded, got %d", len(w.Detections()))
	}
}

func TestWatcher_EventEmitterCalled(t *testing.T) {
	var mu sync.Mutex
	var eventTypes []string
	w := cw.New(cw.WatcherConfig{
		EventEmitter: func(eventType, runID string, payload map[string]any) {
			mu.Lock()
			eventTypes = append(eventTypes, eventType)
			mu.Unlock()
		},
	})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	_, _ = hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID:    "run-1",
		Step:     1,
		Response: harness.CompletionResult{Content: "obviously this must be"},
	})

	mu.Lock()
	defer mu.Unlock()
	if len(eventTypes) == 0 {
		t.Fatal("expected EventEmitter to be called")
	}
	for _, et := range eventTypes {
		if et != cw.EventConclusionDetected && et != cw.EventConclusionIntervened {
			t.Errorf("unexpected event type: %s", et)
		}
	}
}

func TestWatcher_ConcurrencySafe(t *testing.T) {
	w := cw.New(cw.WatcherConfig{})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
				RunID:    "run-concurrent",
				Step:     n,
				Response: harness.CompletionResult{Content: "obviously must be correct"},
			})
		}(i + 1)
	}
	wg.Wait()
	// should not panic or race
	_ = w.Detections()
	_ = w.InterventionCount()
}

func TestWatcher_Detections_ReturnsCopy(t *testing.T) {
	w := cw.New(cw.WatcherConfig{})
	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	hook := cfg.PostMessageHooks[0]
	_, _ = hook.AfterMessage(context.Background(), harness.PostMessageHookInput{
		RunID:    "run-1",
		Step:     1,
		Response: harness.CompletionResult{Content: "obviously this must be"},
	})

	d1 := w.Detections()
	if len(d1) == 0 {
		t.Skip("no detections fired, skip copy test")
	}
	// Mutate the returned slice
	d1[0].RunID = "mutated"
	// Internal state must not change
	d2 := w.Detections()
	if d2[0].RunID == "mutated" {
		t.Fatal("Detections() must return a copy, not the internal slice")
	}
}

// ============================================================
// Task 11 — Integration smoke test
// ============================================================

func TestWatcher_EndToEndInjectMode(t *testing.T) {
	var emittedEvents []string
	var emittedPayloads []map[string]any
	var mu sync.Mutex

	w := cw.New(cw.WatcherConfig{
		Mode: cw.InterventionInjectPrompt,
		EventEmitter: func(eventType, runID string, payload map[string]any) {
			mu.Lock()
			emittedEvents = append(emittedEvents, eventType)
			emittedPayloads = append(emittedPayloads, payload)
			mu.Unlock()
		},
	})

	cfg := &harness.RunnerConfig{}
	w.Register(cfg)

	// Synthetic hook input: response contains hedge language + unverified file claim
	in := harness.PostMessageHookInput{
		RunID: "integration-run",
		Step:  1,
		Response: harness.CompletionResult{
			Content: "The file internal/runner.go obviously must be the entry point.",
		},
	}

	hook := cfg.PostMessageHooks[0]
	result, err := hook.AfterMessage(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have fired at least one detection
	if len(w.Detections()) == 0 {
		t.Fatal("expected at least one detection in end-to-end test")
	}

	// In inject mode, MutatedResponse should be set
	if result.MutatedResponse == nil {
		t.Fatal("expected MutatedResponse to be set in inject mode")
	}
	if !strings.Contains(result.MutatedResponse.Content, in.Response.Content) {
		t.Error("expected MutatedResponse to contain original content")
	}

	// EventEmitter should have been called with EventConclusionDetected
	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, et := range emittedEvents {
		if et == cw.EventConclusionDetected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected EventConclusionDetected to be emitted, got: %v", emittedEvents)
	}
}

// ============================================================
// Helper types for tests
// ============================================================

type noopPreToolUseHook struct{}

func (h *noopPreToolUseHook) Name() string { return "noop-pre" }
func (h *noopPreToolUseHook) PreToolUse(_ context.Context, _ harness.PreToolUseEvent) (*harness.PreToolUseResult, error) {
	return nil, nil
}

type noopPostMessageHook struct{}

func (h *noopPostMessageHook) Name() string { return "noop-post" }
func (h *noopPostMessageHook) AfterMessage(_ context.Context, _ harness.PostMessageHookInput) (harness.PostMessageHookResult, error) {
	return harness.PostMessageHookResult{Action: harness.HookActionContinue}, nil
}
