package observationalmemory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type modelStub struct {
	out string
}

func (m modelStub) Complete(context.Context, ModelRequest) (string, error) {
	return m.out, nil
}

func TestServiceObserveSnippetAndExport(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		Coordinator:    NewLocalCoordinator(),
		Observer:       ModelObserver{Model: modelStub{out: "Observed: user wants concise updates."}},
		Reflector:      ModelReflector{Model: modelStub{out: "Reflection: concise updates preferred."}},
		Estimator:      RuneTokenEstimator{},
		DefaultEnabled: false,
		DefaultConfig: Config{
			ObserveMinTokens:       1,
			SnippetMaxTokens:       500,
			ReflectThresholdTokens: 2,
		},
		Now: time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	status, err := svc.SetEnabled(context.Background(), scope, true, nil, "run_1", "call_1")
	if err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected enabled status")
	}

	out, err := svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "Please keep responses concise and technical."},
			{Index: 1, Role: "assistant", Content: "Acknowledged."},
		},
	})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if !out.Observed {
		t.Fatalf("expected observed true")
	}

	snippet, status, err := svc.Snippet(context.Background(), scope)
	if err != nil {
		t.Fatalf("snippet: %v", err)
	}
	if !strings.Contains(snippet, "<observational-memory>") {
		t.Fatalf("expected snippet tags, got %q", snippet)
	}
	if status.ObservationCount == 0 {
		t.Fatalf("expected observations in status")
	}

	exported, err := svc.Export(context.Background(), scope, "json")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if exported.Bytes == 0 || !strings.Contains(exported.Content, "observations") {
		t.Fatalf("unexpected export payload: %q", exported.Content)
	}
}

func TestDisabledManager(t *testing.T) {
	t.Parallel()

	mgr := NewDisabledManager(ModeOff)
	scope := ScopeKey{TenantID: "default", ConversationID: "run_1", AgentID: "default"}
	status, err := mgr.Status(context.Background(), scope)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Enabled {
		t.Fatalf("expected disabled manager status")
	}
	if _, err := mgr.SetEnabled(context.Background(), scope, true, nil, "run_1", "call_1"); err == nil {
		t.Fatalf("expected set enabled error")
	}
}

func TestServiceStatusModeAndReflectNow(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc, err := NewService(ServiceOptions{
		Mode:        ModeAuto,
		Store:       store,
		Coordinator: NewLocalCoordinator(),
		Observer:    ModelObserver{Model: modelStub{out: "Observed: keep code style stable."}},
		Reflector:   ModelReflector{Model: modelStub{out: "Reflection: keep code style stable."}},
		Estimator:   RuneTokenEstimator{},
		DefaultConfig: Config{
			ObserveMinTokens:       1,
			SnippetMaxTokens:       256,
			ReflectThresholdTokens: 10_000,
		},
		DefaultEnabled: false,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	scope := ScopeKey{TenantID: "tenant", ConversationID: "conversation", AgentID: "agent"}
	if svc.Mode() != ModeLocalCoordinator {
		t.Fatalf("unexpected mode: %q", svc.Mode())
	}

	initial, err := svc.Status(context.Background(), scope)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if initial.Enabled {
		t.Fatalf("expected disabled by default")
	}

	if _, err := svc.SetEnabled(context.Background(), scope, true, nil, "run_1", "call_1"); err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if _, err := svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "Please keep responses concise and technical."},
		},
	}); err != nil {
		t.Fatalf("observe: %v", err)
	}
	status, err := svc.ReflectNow(context.Background(), scope, "run_1", "call_2")
	if err != nil {
		t.Fatalf("reflect now: %v", err)
	}
	if !status.ReflectionPresent {
		t.Fatalf("expected reflection to be present")
	}
}

func TestServiceReflectNowFailsWithoutReflector(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	svc, err := NewService(ServiceOptions{
		Mode:        ModeLocalCoordinator,
		Store:       store,
		Coordinator: NewLocalCoordinator(),
		Observer:    ModelObserver{Model: modelStub{out: "Observed: stable constraints."}},
		Estimator:   RuneTokenEstimator{},
		DefaultConfig: Config{
			ObserveMinTokens:       1,
			SnippetMaxTokens:       256,
			ReflectThresholdTokens: 10_000,
		},
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	scope := ScopeKey{TenantID: "tenant", ConversationID: "conversation", AgentID: "agent"}
	if _, err := svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "Capture one observation first."},
		},
	}); err != nil {
		t.Fatalf("observe: %v", err)
	}

	_, err = svc.ReflectNow(context.Background(), scope, "run_1", "call_1")
	if err == nil || !strings.Contains(err.Error(), "reflector is not configured") {
		t.Fatalf("expected reflector missing error, got: %v", err)
	}
}

func TestDisabledManagerAllMethods(t *testing.T) {
	t.Parallel()

	mgr := NewDisabledManager("")
	scope := ScopeKey{TenantID: "tenant", ConversationID: "conversation", AgentID: "agent"}

	if mgr.Mode() != ModeOff {
		t.Fatalf("expected mode off, got %q", mgr.Mode())
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := mgr.Observe(context.Background(), ObserveRequest{Scope: scope}); err != nil {
		t.Fatalf("observe: %v", err)
	}
	if snippet, _, err := mgr.Snippet(context.Background(), scope); err != nil || snippet != "" {
		t.Fatalf("snippet: %q err=%v", snippet, err)
	}
	if _, err := mgr.ReflectNow(context.Background(), scope, "run_1", "call_1"); err == nil {
		t.Fatalf("expected reflect now to fail for disabled manager")
	}
	if _, err := mgr.Export(context.Background(), scope, "json"); err == nil {
		t.Fatalf("expected export to fail for disabled manager")
	}
}

func TestMergeConfig(t *testing.T) {
	t.Parallel()

	current := Config{
		ObserveMinTokens:       10,
		SnippetMaxTokens:       20,
		ReflectThresholdTokens: 30,
	}
	merged := mergeConfig(current, Config{
		ObserveMinTokens:       15,
		SnippetMaxTokens:       0,
		ReflectThresholdTokens: 45,
	})
	if merged.ObserveMinTokens != 15 {
		t.Fatalf("expected observe_min_tokens 15, got %d", merged.ObserveMinTokens)
	}
	if merged.SnippetMaxTokens != 20 {
		t.Fatalf("expected snippet_max_tokens unchanged at 20, got %d", merged.SnippetMaxTokens)
	}
	if merged.ReflectThresholdTokens != 45 {
		t.Fatalf("expected reflect_threshold_tokens 45, got %d", merged.ReflectThresholdTokens)
	}
}
