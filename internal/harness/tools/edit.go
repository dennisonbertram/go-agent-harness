package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func editTool(workspaceRoot string) Tool {
	def := Definition{
		Name:         "edit",
		Description:  descriptions.Load("edit"),
		Action:       ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Tags:         []string{"edit", "modify", "change", "replace", "patch", "update"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":             map[string]any{"type": "string", "description": "relative file path inside workspace"},
				"file_path":        map[string]any{"type": "string", "description": "alias of path"},
				"old_text":         map[string]any{"type": "string"},
				"new_text":         map[string]any{"type": "string"},
				"replace_all":      map[string]any{"type": "boolean"},
				"expected_version": map[string]any{"type": "string"},
				"start_line_hash":  map[string]any{"type": "string", "description": "12-char hash of the first line of old_text — if provided, validates that old_text starts at the hashed line"},
				"end_line_hash":    map[string]any{"type": "string", "description": "12-char hash of the last line of old_text — if provided, validates that old_text ends at the hashed line"},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Path            string `json:"path"`
			FilePath        string `json:"file_path"`
			OldText         string `json:"old_text"`
			NewText         string `json:"new_text"`
			ReplaceAll      bool   `json:"replace_all"`
			ExpectedVersion string `json:"expected_version"`
			StartLineHash   string `json:"start_line_hash"`
			EndLineHash     string `json:"end_line_hash"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse edit args: %w", err)
		}
		if args.Path == "" {
			args.Path = args.FilePath
		}
		if args.Path == "" {
			return "", fmt.Errorf("path is required")
		}
		if args.OldText == "" {
			return "", fmt.Errorf("old_text is required")
		}

		absPath, err := ResolveWorkspacePath(workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read file for edit: %w", err)
		}
		original := string(content)

		// Hash-based addressing: validate start_line_hash and end_line_hash before editing.
		if args.StartLineHash != "" || args.EndLineHash != "" {
			fileLines := strings.Split(original, "\n")
			if args.StartLineHash != "" {
				found := false
				for _, line := range fileLines {
					if lineHash(line) == args.StartLineHash {
						found = true
						break
					}
				}
				if !found {
					return "", fmt.Errorf("start_line_hash %s not found in file", args.StartLineHash)
				}
				// Verify old_text actually starts at the hashed line.
				firstLine := strings.SplitN(args.OldText, "\n", 2)[0]
				if lineHash(firstLine) != args.StartLineHash {
					return "", fmt.Errorf("start_line_hash %s does not match first line of old_text", args.StartLineHash)
				}
			}
			if args.EndLineHash != "" {
				found := false
				for _, line := range fileLines {
					if lineHash(line) == args.EndLineHash {
						found = true
						break
					}
				}
				if !found {
					return "", fmt.Errorf("end_line_hash %s not found in file", args.EndLineHash)
				}
			}
		}

		if args.ExpectedVersion != "" {
			actual := FileVersionFromBytes(content)
			if actual != args.ExpectedVersion {
				return MarshalToolResult(map[string]any{
					"error": map[string]any{
						"code":             "stale_write",
						"path":             args.Path,
						"expected_version": args.ExpectedVersion,
						"actual_version":   actual,
					},
				})
			}
		}

		replacements := 0
		updated := original
		if args.ReplaceAll {
			replacements = strings.Count(original, args.OldText)
			updated = strings.ReplaceAll(original, args.OldText, args.NewText)
		} else {
			if strings.Contains(original, args.OldText) {
				replacements = 1
				updated = strings.Replace(original, args.OldText, args.NewText, 1)
			}
		}
		if replacements == 0 {
			return "", fmt.Errorf("old_text not found in %s", args.Path)
		}

		if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
			return "", fmt.Errorf("write edited file: %w", err)
		}
		version := FileVersionFromBytes([]byte(updated))

		result := map[string]any{
			"path":         NormalizeRelPath(workspaceRoot, absPath),
			"replacements": replacements,
			"version":      version,
			"diff": map[string]any{
				"before_bytes": len(original),
				"after_bytes":  len(updated),
				"changed":      original != updated,
			},
		}
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
