package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// resolveUserProfilesDir returns the one user-writable profile directory for a
// daemon instance. HARNESS_PROFILES_DIR is deliberately opt-in and absolute:
// accepting a relative path would make the write target depend on launch cwd,
// while changing HOME would redirect unrelated process state. When omitted,
// the longstanding ~/.harness/profiles default remains unchanged.
func resolveUserProfilesDir(harnessConfigDir string, getenv func(string) string) (string, error) {
	if getenv == nil {
		return filepath.Join(harnessConfigDir, "profiles"), nil
	}
	override := strings.TrimSpace(getenv("HARNESS_PROFILES_DIR"))
	if override == "" {
		return filepath.Join(harnessConfigDir, "profiles"), nil
	}
	if !filepath.IsAbs(override) {
		return "", fmt.Errorf("HARNESS_PROFILES_DIR must be an absolute path")
	}
	return filepath.Clean(override), nil
}
