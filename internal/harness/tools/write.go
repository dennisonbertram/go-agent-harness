package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func writeTool(workspaceRoot string) Tool {
	def := Definition{
		Name:         "write",
		Description:  "Write content to a workspace file",
		Action:       ActionWrite,
		Mutating:     true,
		ParallelSafe: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":             map[string]any{"type": "string", "description": "relative file path inside workspace"},
				"file_path":        map[string]any{"type": "string", "description": "alias of path"},
				"content":          map[string]any{"type": "string"},
				"append":           map[string]any{"type": "boolean"},
				"expected_version": map[string]any{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Path            string `json:"path"`
			FilePath        string `json:"file_path"`
			Content         string `json:"content"`
			Append          bool   `json:"append"`
			ExpectedVersion string `json:"expected_version"`
		}{}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse write args: %w", err)
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

		before := ""
		if existing, err := os.ReadFile(absPath); err == nil {
			before = string(existing)
			if args.ExpectedVersion != "" {
				version := fileVersionFromBytes(existing)
				if version != args.ExpectedVersion {
					return marshalToolResult(map[string]any{
						"error": map[string]any{
							"code":             "stale_write",
							"path":             args.Path,
							"expected_version": args.ExpectedVersion,
							"actual_version":   version,
						},
					})
				}
			}
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("read file before write: %w", err)
		} else if args.ExpectedVersion != "" {
			return marshalToolResult(map[string]any{
				"error": map[string]any{
					"code":             "stale_write",
					"path":             args.Path,
					"expected_version": args.ExpectedVersion,
					"actual_version":   "",
				},
			})
		}

		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return "", fmt.Errorf("create parent directory: %w", err)
		}

		flags := os.O_CREATE | os.O_WRONLY
		if args.Append {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		file, err := os.OpenFile(absPath, flags, 0o644)
		if err != nil {
			return "", fmt.Errorf("open file for write: %w", err)
		}
		defer file.Close()

		n, err := file.WriteString(args.Content)
		if err != nil {
			return "", fmt.Errorf("write file: %w", err)
		}

		afterBytes, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read file after write: %w", err)
		}
		after := string(afterBytes)

		result := map[string]any{
			"path":          normalizeRelPath(workspaceRoot, absPath),
			"bytes_written": n,
			"appended":      args.Append,
			"version":       fileVersionFromBytes(afterBytes),
			"diff": map[string]any{
				"before_bytes": len(before),
				"after_bytes":  len(after),
				"changed":      before != after,
			},
		}
		return marshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
