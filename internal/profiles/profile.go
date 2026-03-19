// Package profiles implements the named agent profile system for the harness.
//
// A profile is a reusable subagent configuration: tool allowlist, model,
// max_steps, system prompt, and cost ceiling. Profiles are stored as TOML
// files with a defined [meta], [runner], [tools], and [mcp_servers] section.
//
// Resolution order (highest priority first):
//  1. Project-level: .harness/profiles/<name>.toml
//  2. User-global:   ~/.harness/profiles/<name>.toml
//  3. Built-in:      embedded in the binary
package profiles

import (
	"time"

	"go-agent-harness/internal/config"
)

// Profile holds the full configuration for a named agent profile.
type Profile struct {
	Meta       ProfileMeta                   `toml:"meta"`
	Runner     ProfileRunner                 `toml:"runner"`
	Tools      ProfileTools                  `toml:"tools"`
	MCPServers map[string]config.MCPServerConfig `toml:"mcp_servers,omitempty"`
}

// ProfileMeta holds profile metadata.
type ProfileMeta struct {
	Name            string  `toml:"name"`
	Description     string  `toml:"description"`
	Version         int     `toml:"version"`
	CreatedAt       string  `toml:"created_at"`
	CreatedBy       string  `toml:"created_by"` // "built-in" | "agent" | "user"
	EfficiencyScore float64 `toml:"efficiency_score"`
	ReviewCount     int     `toml:"review_count"`
	ReviewEligible  bool    `toml:"review_eligible"` // false for built-ins
}

// ProfileRunner holds runner configuration for the profile.
type ProfileRunner struct {
	Model        string  `toml:"model"`
	MaxSteps     int     `toml:"max_steps"`
	MaxCostUSD   float64 `toml:"max_cost_usd"`
	SystemPrompt string  `toml:"system_prompt"`
}

// ProfileTools holds tool configuration for the profile.
type ProfileTools struct {
	// Allow is the list of tool names permitted for this profile.
	// An empty or nil slice means all tools are allowed.
	Allow []string `toml:"allow"`
}

// ApplyToRunRequest merges profile fields into a RunRequest-compatible struct.
// It returns the values that should be applied, in priority order.
// Fields already set in the destination (non-zero) are NOT overridden by the
// profile — the caller is responsible for applying these only as defaults.
func (p *Profile) ApplyValues() ProfileValues {
	return ProfileValues{
		Model:        p.Runner.Model,
		MaxSteps:     p.Runner.MaxSteps,
		MaxCostUSD:   p.Runner.MaxCostUSD,
		SystemPrompt: p.Runner.SystemPrompt,
		AllowedTools: append([]string(nil), p.Tools.Allow...),
	}
}

// ProfileValues holds the resolved field values from a profile,
// ready to be applied to a run request.
type ProfileValues struct {
	Model        string
	MaxSteps     int
	MaxCostUSD   float64
	SystemPrompt string
	AllowedTools []string
}

// EfficiencyReport holds the result of a post-run efficiency analysis.
type EfficiencyReport struct {
	RunID                string            `json:"run_id"`
	ProfileName          string            `json:"profile_name"`
	EfficiencyScore      float64           `json:"efficiency_score"`
	ToolRedundancy       []string          `json:"tool_redundancy"`
	UnusedTools          []string          `json:"unused_tools"`
	MissingTools         []string          `json:"missing_tools"`
	SuggestedRefinements ProfileRefinements `json:"suggested_refinements"`
	ReviewerRunID        string            `json:"reviewer_run_id,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
}

// ProfileRefinements holds suggested changes to a profile based on efficiency analysis.
type ProfileRefinements struct {
	RemoveTools          []string `json:"remove_tools,omitempty"`
	AddTools             []string `json:"add_tools,omitempty"`
	SystemPromptAddition string   `json:"system_prompt_addition,omitempty"`
	MaxStepsSuggestion   int      `json:"max_steps_suggestion,omitempty"`
}
