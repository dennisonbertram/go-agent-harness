package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
)

func readTool(workspaceRoot string) Tool {
	def := Definition{
		Name:         "read",
		Description:  descriptions.Load("read"),
		Action:       ActionRead,
		ParallelSafe: true,
		Tags:         []string{"read", "file", "view", "inspect", "contents"},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":      map[string]any{"type": "string", "description": "relative file path inside workspace"},
				"file_path": map[string]any{"type": "string", "description": "alias of path"},
				"max_bytes": map[string]any{"type": "integer", "minimum": 1, "maximum": 1048576},
				"offset":    map[string]any{"type": "integer", "minimum": 0, "description": "line offset"},
				"limit":     map[string]any{"type": "integer", "minimum": 1, "description": "max lines"},
			},
			"required": []string{"path"},
		},
	}

	handler := func(_ context.Context, raw json.RawMessage) (string, error) {
		args := struct {
			Path     string `json:"path"`
			FilePath string `json:"file_path"`
			MaxBytes int    `json:"max_bytes"`
			Offset   int    `json:"offset"`
			Limit    int    `json:"limit"`
		}{MaxBytes: 16 * 1024}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("parse read args: %w", err)
		}
		if args.Path == "" {
			args.Path = args.FilePath
		}
		if args.Path == "" {
			return "", fmt.Errorf("path is required")
		}
		if args.MaxBytes <= 0 {
			args.MaxBytes = 16 * 1024
		}
		if args.MaxBytes > 1024*1024 {
			args.MaxBytes = 1024 * 1024
		}
		if args.Offset < 0 {
			args.Offset = 0
		}
		if args.Limit < 0 {
			args.Limit = 0
		}

		absPath, err := ResolveWorkspacePath(workspaceRoot, args.Path)
		if err != nil {
			return "", err
		}

		file, err := os.Open(absPath)
		if err != nil {
			return "", fmt.Errorf("open file: %w", err)
		}
		defer file.Close()

		content, err := io.ReadAll(io.LimitReader(file, int64(args.MaxBytes+1)))
		if err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		truncated := len(content) > args.MaxBytes
		if truncated {
			content = content[:args.MaxBytes]
		}

		text := string(content)
		lineObjects := make([]map[string]any, 0)
		if args.Offset > 0 || args.Limit > 0 {
			lines := strings.Split(text, "\n")
			start := args.Offset
			if start > len(lines) {
				start = len(lines)
			}
			end := len(lines)
			if args.Limit > 0 && start+args.Limit < end {
				end = start + args.Limit
			}
			for i := start; i < end; i++ {
				lineObjects = append(lineObjects, map[string]any{"line_number": i + 1, "text": lines[i]})
			}
			text = strings.Join(lines[start:end], "\n")
		}

		version, err := ReadFileVersion(absPath)
		if err != nil {
			return "", err
		}

		result := map[string]any{
			"path":      NormalizeRelPath(workspaceRoot, absPath),
			"content":   text,
			"truncated": truncated,
			"version":   version,
		}
		if len(lineObjects) > 0 {
			result["lines"] = lineObjects
		}
		return MarshalToolResult(result)
	}

	return Tool{Definition: def, Handler: handler}
}
