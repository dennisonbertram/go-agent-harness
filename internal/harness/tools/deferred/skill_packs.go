package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
	"go-agent-harness/internal/skills/packs"
)

// ManageSkillPacksTool returns a deferred tool that lists, searches, and activates
// skill packs from the given PackRegistry.
func ManageSkillPacksTool(registry *packs.PackRegistry) tools.Tool {
	def := tools.Definition{
		Name:         "manage_skill_packs",
		Description:  descriptions.Load("manage_skill_packs"),
		Action:       tools.ActionRead,
		Mutating:     false,
		ParallelSafe: true,
		Tier:         tools.TierDeferred,
		Tags:         []string{"skills", "packs", "specialization"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "search", "activate"},
					"description": "The action to perform: list all packs, search by keyword, or activate a pack by name.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Search query (required for action=search).",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Pack name to activate (required for action=activate).",
				},
			},
			"required": []string{"action"},
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Action string `json:"action"`
			Query  string `json:"query"`
			Name   string `json:"name"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse manage_skill_packs args: %w", err)
		}

		switch args.Action {
		case "list":
			return handlePackList(registry)
		case "search":
			if strings.TrimSpace(args.Query) == "" {
				return "", fmt.Errorf("action=search requires a non-empty query")
			}
			return handlePackSearch(registry, args.Query)
		case "activate":
			if strings.TrimSpace(args.Name) == "" {
				return "", fmt.Errorf("action=activate requires a non-empty name")
			}
			return handlePackActivate(registry, args.Name)
		default:
			return "", fmt.Errorf("unknown action %q: must be one of list, search, activate", args.Action)
		}
	}

	return tools.Tool{Definition: def, Handler: handler}
}

func handlePackList(registry *packs.PackRegistry) (string, error) {
	all := registry.List()
	items := make([]map[string]any, len(all))
	for i, m := range all {
		items[i] = packSummary(m)
	}
	return tools.MarshalToolResult(map[string]any{
		"count": len(items),
		"packs": items,
	})
}

func handlePackSearch(registry *packs.PackRegistry, query string) (string, error) {
	results := registry.Find(query)
	items := make([]map[string]any, len(results))
	for i, m := range results {
		items[i] = packSummary(m)
	}
	return tools.MarshalToolResult(map[string]any{
		"query":   query,
		"count":   len(items),
		"results": items,
	})
}

func handlePackActivate(registry *packs.PackRegistry, name string) (string, error) {
	activated, err := registry.Activate(name)
	if err != nil {
		return "", err
	}

	ack, err := tools.MarshalToolResult(map[string]any{
		"pack":          activated.Manifest.Name,
		"status":        "activated",
		"allowed_tools": activated.Manifest.AllowedTools,
	})
	if err != nil {
		return "", err
	}

	metaMsg := fmt.Sprintf("<skill_pack name=%q>\n%s\n</skill_pack>",
		activated.Manifest.Name, activated.Instructions)

	return tools.WrapToolResult(tools.ToolResult{
		Output: ack,
		MetaMessages: []tools.MetaMessage{
			{Content: metaMsg},
		},
	})
}

func packSummary(m *packs.SkillManifest) map[string]any {
	summary := map[string]any{
		"name":        m.Name,
		"description": m.Description,
	}
	if m.DisplayName != "" {
		summary["display_name"] = m.DisplayName
	}
	if m.Category != "" {
		summary["category"] = m.Category
	}
	if len(m.Tags) > 0 {
		summary["tags"] = m.Tags
	}
	if len(m.RequiresCLI) > 0 {
		summary["requires_cli"] = m.RequiresCLI
	}
	if len(m.RequiresEnv) > 0 {
		summary["requires_env"] = m.RequiresEnv
	}
	return summary
}
