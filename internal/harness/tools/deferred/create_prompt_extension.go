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

// validNameRe matches a safe extension name: lowercase letters, digits, hyphens.
var validNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// sanitizeExtensionName lowercases, replaces unsafe characters with hyphens, and trims
// leading/trailing hyphens. Returns an empty string if the result is empty.
func sanitizeExtensionName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)

	// Replace any character that is not a letter, digit, or hyphen with a hyphen.
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	// Collapse runs of hyphens to a single hyphen.
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

// CreatePromptExtensionTool returns a deferred tool that writes new behavior or talent
// markdown files to the system prompt extensions directory.
func CreatePromptExtensionTool(dirs tools.PromptExtensionDirs) tools.Tool {
	def := tools.Definition{
		Name:         "create_prompt_extension",
		Description:  descriptions.Load("create_prompt_extension"),
		Action:       tools.ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tier:         tools.TierDeferred,
		Tags:         []string{"prompt", "extension", "behavior", "talent"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"extension_type": map[string]any{
					"type":        "string",
					"enum":        []string{"behavior", "talent"},
					"description": `Type of extension: "behavior" (modifies agent behavior) or "talent" (adds domain expertise).`,
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Machine-readable identifier, lowercase letters and hyphens only (e.g., \"prefer-short-answers\").",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Human-readable title for the extension.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Markdown content of the extension.",
				},
				"overwrite": map[string]any{
					"type":        "boolean",
					"description": "If true, overwrite an existing file with the same name. Defaults to false.",
				},
			},
			"required":             []string{"extension_type", "name", "content"},
			"additionalProperties": false,
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			ExtensionType string `json:"extension_type"`
			Name          string `json:"name"`
			Title         string `json:"title"`
			Content       string `json:"content"`
			Overwrite     bool   `json:"overwrite"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse create_prompt_extension args: %w", err)
		}

		// Validate extension_type first.
		args.ExtensionType = strings.TrimSpace(args.ExtensionType)
		if args.ExtensionType != "behavior" && args.ExtensionType != "talent" {
			return "", fmt.Errorf("extension_type must be \"behavior\" or \"talent\", got %q", args.ExtensionType)
		}

		// Determine target directory.
		var targetDir string
		switch args.ExtensionType {
		case "behavior":
			targetDir = dirs.BehaviorsDir
		case "talent":
			targetDir = dirs.TalentsDir
		}
		if strings.TrimSpace(targetDir) == "" {
			return "", fmt.Errorf("prompt extension directories are not configured: cannot write %s extension", args.ExtensionType)
		}

		// Validate and sanitize name.
		rawName := strings.TrimSpace(args.Name)
		if rawName == "" {
			return "", fmt.Errorf("name is required")
		}
		sanitized := sanitizeExtensionName(rawName)
		if sanitized == "" {
			return "", fmt.Errorf("name %q sanitizes to an empty string; use lowercase letters, digits, and hyphens", rawName)
		}

		// Reject names with path separators before sanitization (path traversal guard).
		if strings.ContainsAny(rawName, "/\\") {
			return "", fmt.Errorf("name %q contains path separators; use a simple name without slashes", rawName)
		}

		// Validate content.
		if strings.TrimSpace(args.Content) == "" {
			return "", fmt.Errorf("content is required")
		}

		filename := sanitized + ".md"
		absPath := filepath.Join(targetDir, filename)

		// Ensure the resolved path stays within the target directory (defense-in-depth).
		cleanTarget := filepath.Clean(targetDir)
		cleanPath := filepath.Clean(absPath)
		if !strings.HasPrefix(cleanPath, cleanTarget+string(os.PathSeparator)) && cleanPath != cleanTarget {
			return "", fmt.Errorf("resolved path %q is outside the target directory", cleanPath)
		}

		// Check for existing file.
		if _, err := os.Stat(absPath); err == nil {
			if !args.Overwrite {
				return "", fmt.Errorf("extension file %q already exists; set overwrite=true to replace it", filename)
			}
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("check existing extension: %w", err)
		}

		// Write the file.
		if err := os.WriteFile(absPath, []byte(args.Content), 0o644); err != nil {
			return "", fmt.Errorf("write extension file: %w", err)
		}

		title := strings.TrimSpace(args.Title)
		if title == "" {
			title = sanitized
		}

		return tools.MarshalToolResult(map[string]any{
			"status":         "created",
			"extension_type": args.ExtensionType,
			"name":           sanitized,
			"title":          title,
			"path":           absPath,
			"filename":       filename,
			"overwritten":    args.Overwrite,
		})
	}

	return tools.Tool{Definition: def, Handler: handler}
}
