package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"go-agent-harness/internal/config"
)

// TestWorkspaceRunnerConfigRoundTrip verifies that a WorkspaceRunnerConfig
// can be serialized to TOML and parsed back, yielding identical values.
func TestWorkspaceRunnerConfigRoundTrip(t *testing.T) {
	original := config.WorkspaceRunnerConfig{
		Model:    "gpt-4.1",
		MaxSteps: 25,

		MaxCostPerRunUSD: 2.50,
		MemoryEnabled:    true,

		AutoCompactEnabled:   true,
		AutoCompactMode:      "summarize",
		AutoCompactThreshold: 0.75,
		AutoCompactKeepLast:  5,
		ModelContextWindow:   200000,

		TraceToolDecisions: true,
		DetectAntiPatterns: true,
		TraceHookMutations: false,

		CaptureRequestEnvelope: true,
		SnapshotMemorySnippet:  false,

		ErrorChainEnabled: true,
		ErrorContextDepth: 15,

		CaptureReasoning: true,

		CostAnomalyDetectionEnabled: true,
		CostAnomalyStepMultiplier:   3.0,

		AuditTrailEnabled: true,

		ContextWindowSnapshotEnabled:  true,
		ContextWindowWarningThreshold: 0.85,

		CausalGraphEnabled: true,

		RolloutDir: "/tmp/rollouts",
	}

	tomlStr, err := original.ToTOML()
	if err != nil {
		t.Fatalf("ToTOML() error: %v", err)
	}
	if tomlStr == "" {
		t.Fatal("ToTOML() returned empty string")
	}

	var parsed config.WorkspaceRunnerConfig
	if _, err := toml.Decode(tomlStr, &parsed); err != nil {
		t.Fatalf("toml.Decode() error: %v\nTOML:\n%s", err, tomlStr)
	}

	// Verify all fields round-trip correctly.
	if parsed.Model != original.Model {
		t.Errorf("Model: got %q, want %q", parsed.Model, original.Model)
	}
	if parsed.MaxSteps != original.MaxSteps {
		t.Errorf("MaxSteps: got %d, want %d", parsed.MaxSteps, original.MaxSteps)
	}
	if parsed.MaxCostPerRunUSD != original.MaxCostPerRunUSD {
		t.Errorf("MaxCostPerRunUSD: got %f, want %f", parsed.MaxCostPerRunUSD, original.MaxCostPerRunUSD)
	}
	if parsed.MemoryEnabled != original.MemoryEnabled {
		t.Errorf("MemoryEnabled: got %v, want %v", parsed.MemoryEnabled, original.MemoryEnabled)
	}
	if parsed.AutoCompactEnabled != original.AutoCompactEnabled {
		t.Errorf("AutoCompactEnabled: got %v, want %v", parsed.AutoCompactEnabled, original.AutoCompactEnabled)
	}
	if parsed.AutoCompactMode != original.AutoCompactMode {
		t.Errorf("AutoCompactMode: got %q, want %q", parsed.AutoCompactMode, original.AutoCompactMode)
	}
	if parsed.AutoCompactThreshold != original.AutoCompactThreshold {
		t.Errorf("AutoCompactThreshold: got %f, want %f", parsed.AutoCompactThreshold, original.AutoCompactThreshold)
	}
	if parsed.AutoCompactKeepLast != original.AutoCompactKeepLast {
		t.Errorf("AutoCompactKeepLast: got %d, want %d", parsed.AutoCompactKeepLast, original.AutoCompactKeepLast)
	}
	if parsed.ModelContextWindow != original.ModelContextWindow {
		t.Errorf("ModelContextWindow: got %d, want %d", parsed.ModelContextWindow, original.ModelContextWindow)
	}
	if parsed.TraceToolDecisions != original.TraceToolDecisions {
		t.Errorf("TraceToolDecisions: got %v, want %v", parsed.TraceToolDecisions, original.TraceToolDecisions)
	}
	if parsed.DetectAntiPatterns != original.DetectAntiPatterns {
		t.Errorf("DetectAntiPatterns: got %v, want %v", parsed.DetectAntiPatterns, original.DetectAntiPatterns)
	}
	if parsed.TraceHookMutations != original.TraceHookMutations {
		t.Errorf("TraceHookMutations: got %v, want %v", parsed.TraceHookMutations, original.TraceHookMutations)
	}
	if parsed.CaptureRequestEnvelope != original.CaptureRequestEnvelope {
		t.Errorf("CaptureRequestEnvelope: got %v, want %v", parsed.CaptureRequestEnvelope, original.CaptureRequestEnvelope)
	}
	if parsed.SnapshotMemorySnippet != original.SnapshotMemorySnippet {
		t.Errorf("SnapshotMemorySnippet: got %v, want %v", parsed.SnapshotMemorySnippet, original.SnapshotMemorySnippet)
	}
	if parsed.ErrorChainEnabled != original.ErrorChainEnabled {
		t.Errorf("ErrorChainEnabled: got %v, want %v", parsed.ErrorChainEnabled, original.ErrorChainEnabled)
	}
	if parsed.ErrorContextDepth != original.ErrorContextDepth {
		t.Errorf("ErrorContextDepth: got %d, want %d", parsed.ErrorContextDepth, original.ErrorContextDepth)
	}
	if parsed.CaptureReasoning != original.CaptureReasoning {
		t.Errorf("CaptureReasoning: got %v, want %v", parsed.CaptureReasoning, original.CaptureReasoning)
	}
	if parsed.CostAnomalyDetectionEnabled != original.CostAnomalyDetectionEnabled {
		t.Errorf("CostAnomalyDetectionEnabled: got %v, want %v", parsed.CostAnomalyDetectionEnabled, original.CostAnomalyDetectionEnabled)
	}
	if parsed.CostAnomalyStepMultiplier != original.CostAnomalyStepMultiplier {
		t.Errorf("CostAnomalyStepMultiplier: got %f, want %f", parsed.CostAnomalyStepMultiplier, original.CostAnomalyStepMultiplier)
	}
	if parsed.AuditTrailEnabled != original.AuditTrailEnabled {
		t.Errorf("AuditTrailEnabled: got %v, want %v", parsed.AuditTrailEnabled, original.AuditTrailEnabled)
	}
	if parsed.ContextWindowSnapshotEnabled != original.ContextWindowSnapshotEnabled {
		t.Errorf("ContextWindowSnapshotEnabled: got %v, want %v", parsed.ContextWindowSnapshotEnabled, original.ContextWindowSnapshotEnabled)
	}
	if parsed.ContextWindowWarningThreshold != original.ContextWindowWarningThreshold {
		t.Errorf("ContextWindowWarningThreshold: got %f, want %f", parsed.ContextWindowWarningThreshold, original.ContextWindowWarningThreshold)
	}
	if parsed.CausalGraphEnabled != original.CausalGraphEnabled {
		t.Errorf("CausalGraphEnabled: got %v, want %v", parsed.CausalGraphEnabled, original.CausalGraphEnabled)
	}
	if parsed.RolloutDir != original.RolloutDir {
		t.Errorf("RolloutDir: got %q, want %q", parsed.RolloutDir, original.RolloutDir)
	}
}

// TestWorkspaceRunnerConfigToTOMLContainsExpectedKeys verifies that the
// serialized TOML contains the expected TOML keys.
func TestWorkspaceRunnerConfigToTOMLContainsExpectedKeys(t *testing.T) {
	cfg := config.WorkspaceRunnerConfig{
		Model:                  "gpt-4.1",
		AutoCompactEnabled:     true,
		TraceToolDecisions:     true,
		CaptureRequestEnvelope: true,
		AuditTrailEnabled:      true,
	}

	tomlStr, err := cfg.ToTOML()
	if err != nil {
		t.Fatalf("ToTOML() error: %v", err)
	}

	expectedKeys := []string{
		"model",
		"auto_compact_enabled",
		"trace_tool_decisions",
		"capture_request_envelope",
		"audit_trail_enabled",
	}
	for _, key := range expectedKeys {
		if !strings.Contains(tomlStr, key) {
			t.Errorf("ToTOML() output missing key %q\nGot:\n%s", key, tomlStr)
		}
	}
}

// TestWorkspaceRunnerConfigFromConfig verifies that WorkspaceRunnerConfigFromConfig()
// correctly maps fields from a config.Config.
func TestWorkspaceRunnerConfigFromConfig(t *testing.T) {
	c := config.Config{
		Model:    "gpt-4o",
		MaxSteps: 10,
		Cost: config.CostConfig{
			MaxPerRunUSD: 5.0,
		},
		Memory: config.MemoryConfig{
			Enabled: false,
		},
		AutoCompact: config.AutoCompactConfig{
			Enabled:   true,
			Mode:      "strip",
			Threshold: 0.90,
			KeepLast:  3,
		},
		Forensics: config.ForensicsConfig{
			TraceToolDecisions:     true,
			CaptureRequestEnvelope: true,
			AuditTrailEnabled:      true,
		},
	}

	wc := config.WorkspaceRunnerConfigFromConfig(c)

	if wc.Model != "gpt-4o" {
		t.Errorf("Model: got %q, want %q", wc.Model, "gpt-4o")
	}
	if wc.MaxSteps != 10 {
		t.Errorf("MaxSteps: got %d, want 10", wc.MaxSteps)
	}
	if wc.MaxCostPerRunUSD != 5.0 {
		t.Errorf("MaxCostPerRunUSD: got %f, want 5.0", wc.MaxCostPerRunUSD)
	}
	if wc.MemoryEnabled {
		t.Error("MemoryEnabled: got true, want false")
	}
	if !wc.AutoCompactEnabled {
		t.Error("AutoCompactEnabled: got false, want true")
	}
	if wc.AutoCompactMode != "strip" {
		t.Errorf("AutoCompactMode: got %q, want %q", wc.AutoCompactMode, "strip")
	}
	if wc.AutoCompactThreshold != 0.90 {
		t.Errorf("AutoCompactThreshold: got %f, want 0.90", wc.AutoCompactThreshold)
	}
	if wc.AutoCompactKeepLast != 3 {
		t.Errorf("AutoCompactKeepLast: got %d, want 3", wc.AutoCompactKeepLast)
	}
	if !wc.TraceToolDecisions {
		t.Error("TraceToolDecisions: got false, want true")
	}
	if !wc.CaptureRequestEnvelope {
		t.Error("CaptureRequestEnvelope: got false, want true")
	}
	if !wc.AuditTrailEnabled {
		t.Error("AuditTrailEnabled: got false, want true")
	}
}

// TestWorkspaceRunnerConfigZeroIsValid verifies that a zero-value
// WorkspaceRunnerConfig round-trips without error.
func TestWorkspaceRunnerConfigZeroIsValid(t *testing.T) {
	var zero config.WorkspaceRunnerConfig
	tomlStr, err := zero.ToTOML()
	if err != nil {
		t.Fatalf("ToTOML() on zero value returned error: %v", err)
	}

	var parsed config.WorkspaceRunnerConfig
	if _, err := toml.Decode(tomlStr, &parsed); err != nil {
		t.Fatalf("toml.Decode() on zero-value TOML returned error: %v\nTOML:\n%s", err, tomlStr)
	}
	// All fields should be zero value after round-trip.
	if parsed.Model != "" {
		t.Errorf("Model: got %q, want empty", parsed.Model)
	}
	if parsed.AutoCompactEnabled {
		t.Error("AutoCompactEnabled: got true, want false")
	}
}

// TestForensicsConfigTOML verifies parsing of the [forensics] section from TOML.
func TestForensicsConfigTOML(t *testing.T) {
	tomlContent := `
[forensics]
trace_tool_decisions = true
detect_anti_patterns = true
trace_hook_mutations = true
capture_request_envelope = true
snapshot_memory_snippet = true
error_chain_enabled = true
error_context_depth = 20
capture_reasoning = true
cost_anomaly_detection_enabled = true
cost_anomaly_step_multiplier = 4.5
audit_trail_enabled = true
context_window_snapshot_enabled = true
context_window_warning_threshold = 0.75
causal_graph_enabled = true
rollout_dir = "/var/log/rollouts"
`
	tmpDir := t.TempDir()
	cfgPath := tmpDir + "/config.toml"
	if err := os.WriteFile(cfgPath, []byte(tomlContent), 0600); err != nil {
		t.Fatal(err)
	}

	opts := config.LoadOptions{
		UserConfigPath: cfgPath,
		Getenv:         func(string) string { return "" },
	}
	cfg, err := config.Load(opts)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	f := cfg.Forensics
	if !f.TraceToolDecisions {
		t.Error("Forensics.TraceToolDecisions: got false, want true")
	}
	if !f.DetectAntiPatterns {
		t.Error("Forensics.DetectAntiPatterns: got false, want true")
	}
	if !f.TraceHookMutations {
		t.Error("Forensics.TraceHookMutations: got false, want true")
	}
	if !f.CaptureRequestEnvelope {
		t.Error("Forensics.CaptureRequestEnvelope: got false, want true")
	}
	if !f.SnapshotMemorySnippet {
		t.Error("Forensics.SnapshotMemorySnippet: got false, want true")
	}
	if !f.ErrorChainEnabled {
		t.Error("Forensics.ErrorChainEnabled: got false, want true")
	}
	if f.ErrorContextDepth != 20 {
		t.Errorf("Forensics.ErrorContextDepth: got %d, want 20", f.ErrorContextDepth)
	}
	if !f.CaptureReasoning {
		t.Error("Forensics.CaptureReasoning: got false, want true")
	}
	if !f.CostAnomalyDetectionEnabled {
		t.Error("Forensics.CostAnomalyDetectionEnabled: got false, want true")
	}
	if f.CostAnomalyStepMultiplier != 4.5 {
		t.Errorf("Forensics.CostAnomalyStepMultiplier: got %f, want 4.5", f.CostAnomalyStepMultiplier)
	}
	if !f.AuditTrailEnabled {
		t.Error("Forensics.AuditTrailEnabled: got false, want true")
	}
	if !f.ContextWindowSnapshotEnabled {
		t.Error("Forensics.ContextWindowSnapshotEnabled: got false, want true")
	}
	if f.ContextWindowWarningThreshold != 0.75 {
		t.Errorf("Forensics.ContextWindowWarningThreshold: got %f, want 0.75", f.ContextWindowWarningThreshold)
	}
	if !f.CausalGraphEnabled {
		t.Error("Forensics.CausalGraphEnabled: got false, want true")
	}
	if f.RolloutDir != "/var/log/rollouts" {
		t.Errorf("Forensics.RolloutDir: got %q, want %q", f.RolloutDir, "/var/log/rollouts")
	}
}

// TestAutoCompactConfigTOML verifies parsing of the [auto_compact] section from TOML.
func TestAutoCompactConfigTOML(t *testing.T) {
	tomlContent := `
[auto_compact]
enabled = true
mode = "summarize"
threshold = 0.70
keep_last = 12
model_context_window = 64000
`
	tmpDir := t.TempDir()
	cfgPath := tmpDir + "/config.toml"
	if err := os.WriteFile(cfgPath, []byte(tomlContent), 0600); err != nil {
		t.Fatal(err)
	}

	opts := config.LoadOptions{
		UserConfigPath: cfgPath,
		Getenv:         func(string) string { return "" },
	}
	cfg, err := config.Load(opts)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	ac := cfg.AutoCompact
	if !ac.Enabled {
		t.Error("AutoCompact.Enabled: got false, want true")
	}
	if ac.Mode != "summarize" {
		t.Errorf("AutoCompact.Mode: got %q, want %q", ac.Mode, "summarize")
	}
	if ac.Threshold != 0.70 {
		t.Errorf("AutoCompact.Threshold: got %f, want 0.70", ac.Threshold)
	}
	if ac.KeepLast != 12 {
		t.Errorf("AutoCompact.KeepLast: got %d, want 12", ac.KeepLast)
	}
	if ac.ModelContextWindow != 64000 {
		t.Errorf("AutoCompact.ModelContextWindow: got %d, want 64000", ac.ModelContextWindow)
	}
}
