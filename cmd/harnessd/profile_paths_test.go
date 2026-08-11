package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUserProfilesDirUsesDefaultOnlyWhenOverrideIsAbsent(t *testing.T) {
	t.Parallel()
	configDir := filepath.Join(t.TempDir(), ".harness")
	got, err := resolveUserProfilesDir(configDir, func(string) string { return "" })
	if err != nil {
		t.Fatalf("resolve default: %v", err)
	}
	if want := filepath.Join(configDir, "profiles"); got != want {
		t.Fatalf("default profiles dir = %q, want %q", got, want)
	}
}

func TestResolveUserProfilesDirAcceptsCleanAbsoluteOverrideAndRejectsRelative(t *testing.T) {
	t.Parallel()
	configDir := filepath.Join(t.TempDir(), ".harness")
	absolute := filepath.Join(t.TempDir(), "profiles", "..", "isolated")
	got, err := resolveUserProfilesDir(configDir, func(key string) string {
		if key == "HARNESS_PROFILES_DIR" {
			return absolute
		}
		return ""
	})
	if err != nil {
		t.Fatalf("resolve absolute override: %v", err)
	}
	if want := filepath.Clean(absolute); got != want {
		t.Fatalf("absolute override = %q, want %q", got, want)
	}
	_, err = resolveUserProfilesDir(configDir, func(key string) string {
		if key == "HARNESS_PROFILES_DIR" {
			return "relative/profiles"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative override error = %v, want absolute-path rejection", err)
	}
}
