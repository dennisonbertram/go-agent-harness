package config

import (
	"bytes"

	"github.com/BurntSushi/toml"
)

// WorkspaceRunnerConfig is the serializable subset of RunnerConfig fields that
// are propagated to subagent workspaces via a TOML config file (harness.toml).
//
// Only plain value types (bool, int, float64, string) are included here —
// interface-typed fields (hooks, managers, registries) cannot be serialized
// and are intentionally excluded. Secrets (API keys) must be propagated via
// workspace.Options.Env, never written to disk.
type WorkspaceRunnerConfig struct {
	// Core
	Model    string `toml:"model"`
	MaxSteps int    `toml:"max_steps"`

	// Cost
	MaxCostPerRunUSD float64 `toml:"max_cost_per_run_usd"`

	// Memory
	MemoryEnabled bool `toml:"memory_enabled"`

	// Auto-compaction
	AutoCompactEnabled   bool    `toml:"auto_compact_enabled"`
	AutoCompactMode      string  `toml:"auto_compact_mode"`
	AutoCompactThreshold float64 `toml:"auto_compact_threshold"`
	AutoCompactKeepLast  int     `toml:"auto_compact_keep_last"`
	ModelContextWindow   int     `toml:"model_context_window"`

	// Forensics: tool decisions
	TraceToolDecisions bool `toml:"trace_tool_decisions"`
	DetectAntiPatterns bool `toml:"detect_anti_patterns"`
	TraceHookMutations bool `toml:"trace_hook_mutations"`

	// Forensics: request envelope
	CaptureRequestEnvelope bool `toml:"capture_request_envelope"`
	SnapshotMemorySnippet  bool `toml:"snapshot_memory_snippet"`

	// Forensics: error chain
	ErrorChainEnabled bool `toml:"error_chain_enabled"`
	ErrorContextDepth int  `toml:"error_context_depth"`

	// Reasoning
	CaptureReasoning bool `toml:"capture_reasoning"`

	// Cost anomaly
	CostAnomalyDetectionEnabled bool    `toml:"cost_anomaly_detection_enabled"`
	CostAnomalyStepMultiplier   float64 `toml:"cost_anomaly_step_multiplier"`

	// Audit trail
	AuditTrailEnabled bool `toml:"audit_trail_enabled"`

	// Context window
	ContextWindowSnapshotEnabled  bool    `toml:"context_window_snapshot_enabled"`
	ContextWindowWarningThreshold float64 `toml:"context_window_warning_threshold"`

	// Causal graph
	CausalGraphEnabled bool `toml:"causal_graph_enabled"`

	// Rollout
	RolloutDir string `toml:"rollout_dir"`
}

// ToTOML serializes the WorkspaceRunnerConfig to a TOML string suitable for
// writing to harness.toml in a subagent workspace.
func (w WorkspaceRunnerConfig) ToTOML() (string, error) {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(w); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// WorkspaceRunnerConfigFromConfig creates a WorkspaceRunnerConfig from a
// config.Config, mapping each propagatable field. Only value-typed fields
// from Config are mapped here; secret fields (API keys) must be propagated
// via workspace.Options.Env instead.
func WorkspaceRunnerConfigFromConfig(c Config) WorkspaceRunnerConfig {
	return WorkspaceRunnerConfig{
		Model:    c.Model,
		MaxSteps: c.MaxSteps,

		MaxCostPerRunUSD: c.Cost.MaxPerRunUSD,
		MemoryEnabled:    c.Memory.Enabled,

		AutoCompactEnabled:   c.AutoCompact.Enabled,
		AutoCompactMode:      c.AutoCompact.Mode,
		AutoCompactThreshold: c.AutoCompact.Threshold,
		AutoCompactKeepLast:  c.AutoCompact.KeepLast,
		ModelContextWindow:   c.AutoCompact.ModelContextWindow,

		TraceToolDecisions: c.Forensics.TraceToolDecisions,
		DetectAntiPatterns: c.Forensics.DetectAntiPatterns,
		TraceHookMutations: c.Forensics.TraceHookMutations,

		CaptureRequestEnvelope: c.Forensics.CaptureRequestEnvelope,
		SnapshotMemorySnippet:  c.Forensics.SnapshotMemorySnippet,

		ErrorChainEnabled: c.Forensics.ErrorChainEnabled,
		ErrorContextDepth: c.Forensics.ErrorContextDepth,

		CaptureReasoning: c.Forensics.CaptureReasoning,

		CostAnomalyDetectionEnabled: c.Forensics.CostAnomalyDetectionEnabled,
		CostAnomalyStepMultiplier:   c.Forensics.CostAnomalyStepMultiplier,

		AuditTrailEnabled: c.Forensics.AuditTrailEnabled,

		ContextWindowSnapshotEnabled:  c.Forensics.ContextWindowSnapshotEnabled,
		ContextWindowWarningThreshold: c.Forensics.ContextWindowWarningThreshold,

		CausalGraphEnabled: c.Forensics.CausalGraphEnabled,

		RolloutDir: c.Forensics.RolloutDir,
	}
}
