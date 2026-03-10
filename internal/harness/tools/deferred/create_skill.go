package deferred

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/descriptions"
)

var validSkillName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// CreateSkillTool returns a deferred tool that lets the agent create validated
// SKILL.md files in the configured skills directory.
func CreateSkillTool(skillsDir string) tools.Tool {
	def := tools.Definition{
		Name:         "create_skill",
		Description:  descriptions.Load("create_skill"),
		Action:       tools.ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"skills", "specialization", "create", "write"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Machine-readable kebab-case name (e.g. 'code-review', 'deploy'). Must be lowercase letters, digits, and hyphens only.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "What the skill does. Shown in the skill catalog.",
				},
				"trigger": map[string]any{
					"type":        "string",
					"description": "When to activate the skill (e.g. 'When user asks to deploy', 'Use when reviewing code').",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full markdown body of the skill. This becomes the skill instructions. Do not include YAML frontmatter — it is generated automatically.",
				},
				"tags": map[string]any{
					"type":        "array",
					"description": "Optional tags for categorization.",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required":             []string{"name", "description", "trigger", "content"},
			"additionalProperties": false,
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Trigger     string   `json:"trigger"`
			Content     string   `json:"content"`
			Tags        []string `json:"tags"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse create_skill args: %w", err)
		}

		// Validate required fields
		name := strings.TrimSpace(args.Name)
		if name == "" {
			return "", fmt.Errorf("name is required")
		}
		if !validSkillName.MatchString(name) {
			return "", fmt.Errorf("name %q must be kebab-case (lowercase letters, digits, hyphens only)", name)
		}
		if strings.TrimSpace(args.Description) == "" {
			return "", fmt.Errorf("description is required")
		}
		if strings.TrimSpace(args.Trigger) == "" {
			return "", fmt.Errorf("trigger is required")
		}
		if args.Content == "" {
			return "", fmt.Errorf("content is required")
		}
		if skillsDir == "" {
			return "", fmt.Errorf("no skills directory configured; set HARNESS_SKILLS_DIR or enable skills")
		}

		// Duplicate detection
		skillDir := filepath.Join(skillsDir, name)
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			return "", fmt.Errorf("skill %q already exists at %s", name, skillFile)
		}

		// Build YAML frontmatter
		var fm strings.Builder
		fm.WriteString("---\n")
		fmt.Fprintf(&fm, "name: %s\n", name)
		fmt.Fprintf(&fm, "description: %s\n", quoteYAMLString(args.Description))
		if strings.TrimSpace(args.Trigger) != "" {
			fmt.Fprintf(&fm, "trigger: %s\n", quoteYAMLString(args.Trigger))
		}
		fm.WriteString("version: 1\n")
		fm.WriteString("---\n")

		fullContent := fm.String() + "\n" + strings.TrimSpace(args.Content) + "\n"

		// Create directory and write file
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return "", fmt.Errorf("create skill directory %s: %w", skillDir, err)
		}
		if err := os.WriteFile(skillFile, []byte(fullContent), 0o644); err != nil {
			return "", fmt.Errorf("write skill file %s: %w", skillFile, err)
		}

		return tools.MarshalToolResult(map[string]any{
			"status": "created",
			"name":   name,
			"path":   skillFile,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}

// quoteYAMLString wraps a string in double-quotes if it contains special YAML
// characters. For simple strings it returns the value as-is.
func quoteYAMLString(s string) string {
	// If the string contains characters that need quoting in YAML, use double-quotes.
	needsQuote := strings.ContainsAny(s, ":#{}[]|>&*!,'\"\\%@`\n\r\t")
	if !needsQuote {
		return s
	}
	// Escape double-quotes within the value.
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
