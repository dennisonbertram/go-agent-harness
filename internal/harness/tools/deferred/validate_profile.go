package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/profiles"
)

// ValidateProfileTool returns a deferred tool that parses and validates a profile
// TOML string without writing any files.
func ValidateProfileTool(_ string) tools.Tool {
	def := tools.Definition{
		Name:         "validate_profile",
		Description:  descriptions.Load("validate_profile"),
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profile", "agent", "validate", "dry-run"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"toml": map[string]any{
					"type":        "string",
					"description": "Full TOML content of the profile to validate. Not written to disk.",
				},
			},
			"required":             []string{"toml"},
			"additionalProperties": false,
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			TOML string `json:"toml"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse validate_profile args: %w", err)
		}

		tomlContent := strings.TrimSpace(args.TOML)
		if tomlContent == "" {
			return "", fmt.Errorf("validate_profile: toml content is required")
		}

		var p profiles.Profile
		if _, err := toml.Decode(tomlContent, &p); err != nil {
			return "", fmt.Errorf("validate_profile: parse TOML: %w", err)
		}

		if err := profiles.ValidateProfile(&p); err != nil {
			return "", fmt.Errorf("validate_profile: %w", err)
		}

		return tools.MarshalToolResult(map[string]any{
			"status": "valid",
			"name":   p.Meta.Name,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
