package deferred

import (
	"context"
	"encoding/json"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/profiles"
)

// ListProfilesTool returns a deferred tool that lists all available profiles with metadata.
// profilesDir is the user-global profiles directory; pass empty string to use defaults.
func ListProfilesTool(profilesDir string) tools.Tool {
	return ListProfilesToolWithDirs("", profilesDir)
}

// ListProfilesToolWithDirs is like ListProfilesTool but accepts explicit project and user dirs.
// This is used for testing and for the HTTP handler which may specify both dirs.
func ListProfilesToolWithDirs(projectDir, userDir string) tools.Tool {
	def := tools.Definition{
		Name:         "list_profiles",
		Description:  descriptions.Load("list_profiles"),
		Action:       tools.ActionList,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"profiles", "agent", "subagent", "discovery"},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	handler := func(_ context.Context, _ json.RawMessage) (string, error) {
		summaries, err := profiles.ListProfileSummariesFromDirs(projectDir, userDir)
		if err != nil {
			return "", err
		}
		if summaries == nil {
			summaries = []profiles.ProfileSummary{}
		}
		return tools.MarshalToolResult(map[string]any{
			"profiles": summaries,
			"count":    len(summaries),
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
