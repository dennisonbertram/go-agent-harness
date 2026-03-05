package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type multiEdit struct {
	OldText    string `json:"old_text"`
	OldString  string `json:"old_string"`
	NewText    string `json:"new_text"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

func applyPatchTool(workspaceRoot string) Tool {
	def := Definition{
		Name:         "apply_patch",
		Description:  "Apply a find/replace patch to a file in workspace",
		Action:       ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":        map[string]any{"type": "string", "description": "relative file path inside workspace"},
				"file_path":   map[string]any{"type": "string", "description": "alias of path"},
				"find":        map[string]any{"type": "string"},
				"replace":     map[string]any{"type": "string"},
				"replace_all": map[string]any{"type": "boolean"},
				"edits": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_text":    map[string]any{"type": "string"},
							"old_string":  map[string]any{"type": "string"},
							"new_text":    map[string]any{"type": "string"},
							"new_string":  map[string]any{"type": "string"},
							"replace_all": map[string]any{"type": "boolean"},
						},
					},
				},
				"expected_version": map[string]any{"type": "string"},
			},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Path            string      `json:"path"`
			FilePath        string      `json:"file_path"`
			Find            string      `json:"find"`
			Replace         string      `json:"replace"`
			ReplaceAll      bool        `json:"replace_all"`
			Edits           []multiEdit `json:"edits"`
			ExpectedVersion string      `json:"expected_version"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse apply_patch args: %w", err)
		}
		if args.Path == "" {
			args.Path = args.FilePath
		}
		if args.Path == "" {
			return "", fmt.Errorf("path is required")
		}

		absPath, err := resolveWorkspacePath(workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read patch file: %w", err)
		}
		original := string(content)
		if args.ExpectedVersion != "" {
			actual := fileVersionFromBytes(content)
			if actual != args.ExpectedVersion {
				return marshalToolResult(map[string]any{
					"error": map[string]any{
						"code":             "stale_write",
						"path":             args.Path,
						"expected_version": args.ExpectedVersion,
						"actual_version":   actual,
					},
				})
			}
		}

		updated := original
		totalReplacements := 0

		if len(args.Edits) > 0 {
			failed := make([]map[string]any, 0)
			applied := 0
			for i, e := range args.Edits {
				oldText := e.OldText
				if oldText == "" {
					oldText = e.OldString
				}
				newText := e.NewText
				if e.NewString != "" || (e.NewString == "" && e.NewText == "") {
					newText = e.NewString
				}

				if oldText == "" {
					failed = append(failed, map[string]any{"index": i, "error": "old_text is required"})
					continue
				}
				replacements := 0
				if e.ReplaceAll {
					replacements = strings.Count(updated, oldText)
					updated = strings.ReplaceAll(updated, oldText, newText)
				} else {
					if strings.Contains(updated, oldText) {
						replacements = 1
						updated = strings.Replace(updated, oldText, newText, 1)
					}
				}
				if replacements == 0 {
					failed = append(failed, map[string]any{"index": i, "error": "old_text not found"})
					continue
				}
				applied++
				totalReplacements += replacements
			}
			if totalReplacements > 0 {
				if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
					return "", fmt.Errorf("write patched file: %w", err)
				}
			}
			result := map[string]any{
				"path":          normalizeRelPath(workspaceRoot, absPath),
				"replacements":  totalReplacements,
				"applied_edits": applied,
				"failed_edits":  failed,
				"partial":       len(failed) > 0,
				"version":       fileVersionFromBytes([]byte(updated)),
				"diff":          map[string]any{"before_bytes": len(original), "after_bytes": len(updated), "changed": original != updated},
			}
			return marshalToolResult(result)
		}

		if args.Find == "" {
			return "", fmt.Errorf("find is required")
		}
		if args.ReplaceAll {
			totalReplacements = strings.Count(updated, args.Find)
			updated = strings.ReplaceAll(updated, args.Find, args.Replace)
		} else {
			if strings.Contains(updated, args.Find) {
				totalReplacements = 1
				updated = strings.Replace(updated, args.Find, args.Replace, 1)
			}
		}
		if totalReplacements == 0 {
			return "", fmt.Errorf("find text not present in %s", args.Path)
		}

		if err := os.WriteFile(absPath, []byte(updated), 0o644); err != nil {
			return "", fmt.Errorf("write patched file: %w", err)
		}

		result := map[string]any{
			"path":         normalizeRelPath(workspaceRoot, absPath),
			"replacements": totalReplacements,
			"version":      fileVersionFromBytes([]byte(updated)),
			"diff": map[string]any{
				"before_bytes": len(original),
				"after_bytes":  len(updated),
				"changed":      original != updated,
			},
		}
		return marshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
