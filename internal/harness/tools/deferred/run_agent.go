package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/profiles"
)

// RunAgentTool returns a deferred tool that spawns a subagent using a named profile.
func RunAgentTool(manager tools.SubagentManager, profilesDir string) tools.Tool {
	def := tools.Definition{
		Name:         "run_agent",
		Description:  descriptions.Load("run_agent"),
		Action:       tools.ActionExecute,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"agent", "subagent", "profile", "delegation", "run"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The task for the subagent to complete. Be specific — it has no context from the parent conversation.",
				},
				"profile": map[string]any{
					"type":        "string",
					"description": "Optional profile name (e.g. 'github', 'researcher'). Defaults to 'full' (all tools).",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Optional model override for this call. Overrides the profile's configured model.",
				},
				"max_steps": map[string]any{
					"type":        "integer",
					"description": "Optional step override for this call. Overrides the profile's max_steps.",
				},
			},
			"required": []string{"task"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Task     string `json:"task"`
			Profile  string `json:"profile,omitempty"`
			Model    string `json:"model,omitempty"`
			MaxSteps int    `json:"max_steps,omitempty"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse run_agent args: %w", err)
		}
		if strings.TrimSpace(args.Task) == "" {
			return "", fmt.Errorf("run_agent: task is required")
		}
		if manager == nil {
			return "", fmt.Errorf("run_agent: subagent manager is not configured")
		}

		// Default profile to "full" when not specified.
		profileName := strings.TrimSpace(args.Profile)
		if profileName == "" {
			profileName = "full"
		}

		// Load the profile using the three-tier resolution.
		// Fail closed: any error (not-found, invalid name, parse failure) is
		// returned explicitly so that a typo in a profile name never silently
		// widens or narrows the child agent's capabilities.
		var p *profiles.Profile
		var loadErr error
		if profilesDir != "" {
			p, loadErr = profiles.LoadProfileFromUserDir(profileName, profilesDir)
		} else {
			p, loadErr = profiles.LoadProfile(profileName)
		}
		if loadErr != nil {
			return "", fmt.Errorf("run_agent: profile %q could not be loaded: %w; check available profiles with list_profiles or use a built-in profile (\"full\", \"fast\", \"minimal\")", profileName, loadErr)
		}

		// Apply profile values to the request, with per-call overrides on top.
		vals := p.ApplyValues()

		model := vals.Model
		if strings.TrimSpace(args.Model) != "" {
			model = strings.TrimSpace(args.Model)
		}

		maxSteps := vals.MaxSteps
		if args.MaxSteps > 0 {
			maxSteps = args.MaxSteps
		}

		req := tools.SubagentRequest{
			Prompt:       args.Task,
			Model:        model,
			SystemPrompt: vals.SystemPrompt,
			MaxSteps:     maxSteps,
			MaxCostUSD:   vals.MaxCostUSD,
			AllowedTools: vals.AllowedTools,
			ProfileName:  profileName,
		}

		result, err := manager.CreateAndWait(ctx, req)
		if err != nil {
			return "", fmt.Errorf("run_agent: subagent failed: %w", err)
		}

		response := map[string]any{
			"run_id":  result.RunID,
			"status":  result.Status,
			"profile": profileName,
			"output":  result.Output,
		}
		if result.Error != "" {
			response["error"] = result.Error
		}

		return tools.MarshalToolResult(response)
	}

	return tools.Tool{Definition: def, Handler: handler}
}
