package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validateWorkspaceRelativePattern(pattern string) error {
	if filepath.IsAbs(pattern) {
		return fmt.Errorf("absolute patterns are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(pattern))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("pattern %q escapes workspace", pattern)
	}
	return nil
}

func resolveWorkspacePath(workspaceRoot, relativePath string) (string, error) {
	if workspaceRoot == "" {
		return "", fmt.Errorf("workspace root is required")
	}
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	path := relativePath
	if path == "" {
		path = "."
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", path)
	}
	candidate := filepath.Clean(filepath.Join(absRoot, path))
	rel, err := filepath.Rel(absRoot, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", relativePath)
	}
	return candidate, nil
}

func normalizeRelPath(workspaceRoot, absPath string) string {
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absRoot = workspaceRoot
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return absPath
	}
	if rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}
