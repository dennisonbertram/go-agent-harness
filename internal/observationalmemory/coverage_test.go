package observationalmemory

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- NewService edge cases ---

func TestNewServiceModeOff(t *testing.T) {
	t.Parallel()
	mgr, err := NewService(ServiceOptions{Mode: ModeOff})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr.Mode() != ModeOff {
		t.Fatalf("expected ModeOff, got %q", mgr.Mode())
	}
}

func TestNewServiceUnsupportedMode(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	_, err = NewService(ServiceOptions{
		Mode:  "bogus_mode",
		Store: store,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported memory mode") {
		t.Fatalf("expected unsupported mode error, got %v", err)
	}
}

func TestNewServiceRequiresStore(t *testing.T) {
	t.Parallel()
	_, err := NewService(ServiceOptions{Mode: ModeLocalCoordinator, Store: nil})
	if err == nil || !strings.Contains(err.Error(), "memory store is required") {
		t.Fatalf("expected store required error, got %v", err)
	}
}

func TestNewServiceEmptyModeDefaultsToAuto(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	mgr, err := NewService(ServiceOptions{
		Mode:  "",
		Store: store,
		Now:   time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer mgr.Close()
	// Empty mode -> auto -> local_coordinator
	if mgr.Mode() != ModeLocalCoordinator {
		t.Fatalf("expected local_coordinator, got %q", mgr.Mode())
	}
}

func TestNewServiceDefaultsCoordinatorAndEstimator(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	mgr, err := NewService(ServiceOptions{
		Mode:        ModeLocalCoordinator,
		Store:       store,
		Coordinator: nil, // should default
		Estimator:   nil, // should default
		Now:         time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer mgr.Close()
}

// --- Export markdown ---

func TestExportMarkdownFormat(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		Observer:       ModelObserver{Model: modelStub{out: "Observed: test content."}},
		Reflector:      ModelReflector{Model: modelStub{out: "Reflection: test reflection."}},
		Estimator:      RuneTokenEstimator{},
		DefaultEnabled: true,
		DefaultConfig: Config{
			ObserveMinTokens:       1,
			SnippetMaxTokens:       500,
			ReflectThresholdTokens: 1, // low threshold to trigger reflection
		},
		Now: time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}

	// Observe to create data
	_, err = svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "Test message content here."},
		},
	})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}

	// Export as markdown
	result, err := svc.Export(context.Background(), scope, "markdown")
	if err != nil {
		t.Fatalf("export markdown: %v", err)
	}
	if result.Format != "markdown" {
		t.Fatalf("expected format 'markdown', got %q", result.Format)
	}
	if !strings.Contains(result.Content, "# Observational Memory Export") {
		t.Fatalf("expected markdown header, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "## Reflection") {
		t.Fatalf("expected reflection section, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "## Observations") {
		t.Fatalf("expected observations section, got %q", result.Content)
	}
}

func TestExportUnsupportedFormat(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	_, err = svc.Export(context.Background(), scope, "xml")
	if err == nil || !strings.Contains(err.Error(), "unsupported export format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}

func TestExportDefaultFormatIsJSON(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	result, err := svc.Export(context.Background(), scope, "")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.Format != "json" {
		t.Fatalf("expected default format 'json', got %q", result.Format)
	}
}

func TestExportMarkdownNoObservations(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	result, err := svc.Export(context.Background(), scope, "markdown")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(result.Content, "(none)") {
		t.Fatalf("expected '(none)' for empty observations, got %q", result.Content)
	}
}

// --- Observe edge cases ---

func TestObserveDisabledScope(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: false, // disabled by default
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	result, err := svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result.Observed {
		t.Fatalf("expected not observed when disabled")
	}
}

func TestObserveEmptyMessages(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	result, err := svc.Observe(context.Background(), ObserveRequest{
		Scope:    scope,
		RunID:    "run_1",
		Messages: nil,
	})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if result.Observed {
		t.Fatalf("expected not observed for empty messages")
	}
}

func TestObserveWithoutObserver(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		Observer:       nil, // no observer configured
		DefaultEnabled: true,
		DefaultConfig: Config{
			ObserveMinTokens:       1,
			SnippetMaxTokens:       500,
			ReflectThresholdTokens: 10000,
		},
		Now: time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	_, err = svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "some content here"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "observer is not configured") {
		t.Fatalf("expected observer not configured error, got %v", err)
	}
}

func TestObserveEmptyObserverOutput(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		Observer:       ModelObserver{Model: modelStub{out: "   "}}, // whitespace only
		DefaultEnabled: true,
		DefaultConfig: Config{
			ObserveMinTokens:       1,
			SnippetMaxTokens:       500,
			ReflectThresholdTokens: 10000,
		},
		Now: time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	_, err = svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "test content"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "observer returned empty output") {
		t.Fatalf("expected empty output error, got %v", err)
	}
}

// --- Snippet edge cases ---

func TestSnippetDisabled(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: false,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	snippet, _, err := svc.Snippet(context.Background(), scope)
	if err != nil {
		t.Fatalf("snippet: %v", err)
	}
	if snippet != "" {
		t.Fatalf("expected empty snippet when disabled, got %q", snippet)
	}
}

func TestSnippetNoData(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	snippet, _, err := svc.Snippet(context.Background(), scope)
	if err != nil {
		t.Fatalf("snippet: %v", err)
	}
	if snippet != "" {
		t.Fatalf("expected empty snippet with no observations, got %q", snippet)
	}
}

// --- ReflectNow edge cases ---

func TestReflectNowDisabled(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: false,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	// ReflectNow on disabled scope should succeed (noop)
	_, err = svc.ReflectNow(context.Background(), scope, "run_1", "call_1")
	if err != nil {
		t.Fatalf("reflect now should succeed on disabled scope, got %v", err)
	}
}

func TestReflectNowNoObservations(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		Reflector:      ModelReflector{Model: modelStub{out: "Should not be called."}},
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	_, err = svc.ReflectNow(context.Background(), scope, "run_1", "call_1")
	if err != nil {
		t.Fatalf("reflect now with no observations should succeed, got %v", err)
	}
}

func TestReflectNowEmptyReflectorOutput(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		Observer:       ModelObserver{Model: modelStub{out: "Observed: test."}},
		Reflector:      ModelReflector{Model: modelStub{out: "  "}}, // whitespace only
		DefaultEnabled: true,
		DefaultConfig: Config{
			ObserveMinTokens:       1,
			SnippetMaxTokens:       500,
			ReflectThresholdTokens: 10000,
		},
		Now: time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	_, err = svc.Observe(context.Background(), ObserveRequest{
		Scope: scope,
		RunID: "run_1",
		Messages: []TranscriptMessage{
			{Index: 0, Role: "user", Content: "some content"},
		},
	})
	if err != nil {
		t.Fatalf("observe: %v", err)
	}

	_, err = svc.ReflectNow(context.Background(), scope, "run_1", "call_1")
	if err == nil || !strings.Contains(err.Error(), "reflector returned empty output") {
		t.Fatalf("expected empty output error, got %v", err)
	}
}

// --- SetEnabled with config merge ---

func TestSetEnabledWithConfigOverride(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: false,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	cfg := Config{ObserveMinTokens: 999}
	status, err := svc.SetEnabled(context.Background(), scope, true, &cfg, "run_1", "call_1")
	if err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected enabled")
	}
}

func TestSetEnabledDisable(t *testing.T) {
	t.Parallel()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	svc, err := NewService(ServiceOptions{
		Mode:           ModeLocalCoordinator,
		Store:          store,
		DefaultEnabled: true,
		Now:            time.Now,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	status, err := svc.SetEnabled(context.Background(), scope, false, nil, "run_1", "call_1")
	if err != nil {
		t.Fatalf("set enabled: %v", err)
	}
	if status.Enabled {
		t.Fatalf("expected disabled")
	}
}

// --- Token estimator ---

func TestRuneTokenEstimatorEmptyString(t *testing.T) {
	t.Parallel()
	est := RuneTokenEstimator{}
	if got := est.EstimateTextTokens(""); got != 0 {
		t.Fatalf("expected 0 tokens for empty string, got %d", got)
	}
}

func TestRuneTokenEstimatorEstimatesTokens(t *testing.T) {
	t.Parallel()
	est := RuneTokenEstimator{}
	// 12 runes -> (12+3)/4 = 3
	if got := est.EstimateTextTokens("hello world!"); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

func TestRuneTokenEstimatorMessages(t *testing.T) {
	t.Parallel()
	est := RuneTokenEstimator{}
	msgs := []TranscriptMessage{
		{Content: "hello"},
		{Content: "world"},
	}
	total := est.EstimateMessagesTokens(msgs)
	if total <= 0 {
		t.Fatalf("expected positive token count, got %d", total)
	}
}

func TestRuneTokenEstimatorEmptyMessages(t *testing.T) {
	t.Parallel()
	est := RuneTokenEstimator{}
	if got := est.EstimateMessagesTokens(nil); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

// --- normalizeScope ---

func TestNormalizeScopeDefaults(t *testing.T) {
	t.Parallel()
	key := normalizeScope(ScopeKey{})
	if key.TenantID != "default" {
		t.Fatalf("expected default tenant, got %q", key.TenantID)
	}
	if key.AgentID != "default" {
		t.Fatalf("expected default agent, got %q", key.AgentID)
	}
	if key.ConversationID != "unknown" {
		t.Fatalf("expected unknown conversation, got %q", key.ConversationID)
	}
}

func TestNormalizeScopePreservesValues(t *testing.T) {
	t.Parallel()
	key := normalizeScope(ScopeKey{TenantID: "t1", ConversationID: "c1", AgentID: "a1"})
	if key.TenantID != "t1" || key.ConversationID != "c1" || key.AgentID != "a1" {
		t.Fatalf("expected preserved values, got %+v", key)
	}
}

// --- ScopeKey.MemoryID ---

func TestScopeKeyMemoryID(t *testing.T) {
	t.Parallel()
	key := ScopeKey{TenantID: "a", ConversationID: "b", AgentID: "c"}
	if got := key.MemoryID(); got != "a|b|c" {
		t.Fatalf("expected 'a|b|c', got %q", got)
	}
}

// --- DefaultConfig ---

func TestDefaultConfigHasPositiveValues(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.ObserveMinTokens <= 0 || cfg.SnippetMaxTokens <= 0 || cfg.ReflectThresholdTokens <= 0 {
		t.Fatalf("expected all positive default config values, got %+v", cfg)
	}
}

// --- markerPayloadJSON ---

func TestMarkerPayloadJSONEmpty(t *testing.T) {
	t.Parallel()
	if got := markerPayloadJSON(nil); got != "{}" {
		t.Fatalf("expected '{}', got %q", got)
	}
	if got := markerPayloadJSON(map[string]any{}); got != "{}" {
		t.Fatalf("expected '{}' for empty map, got %q", got)
	}
}

func TestMarkerPayloadJSONWithData(t *testing.T) {
	t.Parallel()
	got := markerPayloadJSON(map[string]any{"key": "value"})
	if !strings.Contains(got, "key") || !strings.Contains(got, "value") {
		t.Fatalf("expected key/value in JSON, got %q", got)
	}
}

// --- LocalCoordinator ---

func TestLocalCoordinatorWithinScope(t *testing.T) {
	t.Parallel()
	coord := NewLocalCoordinator()
	key := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}

	called := false
	err := coord.WithinScope(context.Background(), key, func(ctx context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("within scope: %v", err)
	}
	if !called {
		t.Fatalf("expected function to be called")
	}
}

func TestLocalCoordinatorWithinScopeReturnsError(t *testing.T) {
	t.Parallel()
	coord := NewLocalCoordinator()
	key := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}

	err := coord.WithinScope(context.Background(), key, func(ctx context.Context) error {
		return fmt.Errorf("test error")
	})
	if err == nil || !strings.Contains(err.Error(), "test error") {
		t.Fatalf("expected test error, got %v", err)
	}
}

// --- ModelObserver and ModelReflector nil model ---

func TestModelObserverNilModel(t *testing.T) {
	t.Parallel()
	obs := ModelObserver{Model: nil}
	_, err := obs.Observe(context.Background(), ScopeKey{}, Config{}, nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "observer model is required") {
		t.Fatalf("expected nil model error, got %v", err)
	}
}

func TestModelReflectorNilModel(t *testing.T) {
	t.Parallel()
	ref := ModelReflector{Model: nil}
	_, err := ref.Reflect(context.Background(), ScopeKey{}, Config{}, nil, "")
	if err == nil || !strings.Contains(err.Error(), "reflector model is required") {
		t.Fatalf("expected nil model error, got %v", err)
	}
}

// --- Prompt template builders ---

func TestBuildObservationPromptWithExistingChunks(t *testing.T) {
	t.Parallel()
	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	cfg := Config{ObserveMinTokens: 100}
	msgs := []TranscriptMessage{
		{Index: 0, Role: "user", Content: "hello"},
		{Index: 1, Role: "", Content: "empty role"},
		{Index: 2, Role: "user", Content: ""},
	}
	existing := []ObservationChunk{
		{Seq: 1, Content: "previous observation"},
	}
	result := buildObservationPrompt(scope, cfg, msgs, existing, "existing reflection")
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Fatalf("expected system role, got %q", result[0].Role)
	}
	content := result[1].Content
	if !strings.Contains(content, "Reflection:") {
		t.Fatalf("expected reflection section")
	}
	if !strings.Contains(content, "previous observation") {
		t.Fatalf("expected existing observation")
	}
	if !strings.Contains(content, "unknown:") {
		t.Fatalf("expected 'unknown' for empty role")
	}
}

func TestBuildObservationPromptNoExisting(t *testing.T) {
	t.Parallel()
	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	msgs := []TranscriptMessage{
		{Index: 0, Role: "user", Content: "test"},
	}
	result := buildObservationPrompt(scope, Config{ObserveMinTokens: 10}, msgs, nil, "")
	content := result[1].Content
	if !strings.Contains(content, "(none)") {
		t.Fatalf("expected (none) for no existing observations")
	}
}

func TestBuildReflectionPromptWithExistingReflection(t *testing.T) {
	t.Parallel()
	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	observations := []ObservationChunk{
		{Seq: 1, Content: "obs 1"},
	}
	result := buildReflectionPrompt(scope, Config{ReflectThresholdTokens: 100}, observations, "prior reflection")
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	content := result[1].Content
	if !strings.Contains(content, "prior reflection") {
		t.Fatalf("expected existing reflection in prompt")
	}
	if !strings.Contains(content, "obs 1") {
		t.Fatalf("expected observation in prompt")
	}
}

func TestBuildReflectionPromptNoExistingReflection(t *testing.T) {
	t.Parallel()
	scope := ScopeKey{TenantID: "t", ConversationID: "c", AgentID: "a"}
	result := buildReflectionPrompt(scope, Config{ReflectThresholdTokens: 100}, nil, "")
	content := result[1].Content
	if strings.Contains(content, "Existing reflection:") {
		t.Fatalf("should not contain existing reflection section when empty")
	}
}

// --- SQLiteStore edge cases ---

func TestNewSQLiteStoreEmptyPath(t *testing.T) {
	t.Parallel()
	_, err := NewSQLiteStore("")
	if err == nil || !strings.Contains(err.Error(), "sqlite path is required") {
		t.Fatalf("expected path required error, got %v", err)
	}
}

func TestSQLiteStoreCloseNil(t *testing.T) {
	t.Parallel()
	var s *SQLiteStore
	if err := s.Close(); err != nil {
		t.Fatalf("close nil store should not error, got %v", err)
	}
}

// --- OpenAI model edge cases ---

func TestNewOpenAIModelDefaults(t *testing.T) {
	t.Parallel()
	m, err := NewOpenAIModel(OpenAIConfig{APIKey: "test"})
	if err != nil {
		t.Fatalf("new openai model: %v", err)
	}
	if m.model != "gpt-5-nano" {
		t.Fatalf("expected default model, got %q", m.model)
	}
	if m.baseURL != "https://api.openai.com" {
		t.Fatalf("expected default base url, got %q", m.baseURL)
	}
}

func TestOpenAIModelCompleteNilModel(t *testing.T) {
	t.Parallel()
	var m *OpenAIModel
	_, err := m.Complete(context.Background(), ModelRequest{})
	if err == nil || !strings.Contains(err.Error(), "openai model is nil") {
		t.Fatalf("expected nil model error, got %v", err)
	}
}

// --- nextID ---

func TestNextIDIncrementsSequence(t *testing.T) {
	t.Parallel()
	id1 := nextID("test")
	id2 := nextID("test")
	if id1 == id2 {
		t.Fatalf("expected unique ids, got %q and %q", id1, id2)
	}
	if !strings.HasPrefix(id1, "test_") {
		t.Fatalf("expected prefix 'test_', got %q", id1)
	}
}
