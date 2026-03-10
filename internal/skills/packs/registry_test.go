package packs

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// writePackDir creates a skill pack directory with a YAML manifest and markdown instructions.
func writePackDir(t *testing.T, dir, name string, manifest string, instructions string) {
	t.Helper()
	packDir := filepath.Join(dir, name)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, name+".yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if instructions != "" {
		// Parse the instructions filename from the manifest (use name+".md" by default)
		if err := os.WriteFile(filepath.Join(packDir, name+".md"), []byte(instructions), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const validPackManifest = `name: my-pack
display_name: "My Pack"
category: testing
description: "A test skill pack for unit testing"
version: 1
instructions: my-pack.md
tags:
  - testing
  - unit-test
`

const validPackInstructions = `# My Pack Instructions

This pack helps with testing.
`

func TestRegistryLoad_Empty(t *testing.T) {
	dir := t.TempDir()
	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}
	packs := r.List()
	if len(packs) != 0 {
		t.Errorf("expected 0 packs, got %d", len(packs))
	}
}

func TestRegistryLoad_MissingDirectory(t *testing.T) {
	// Missing directory should not be an error - just empty registry
	r, err := NewPackRegistry("/nonexistent/path/packs")
	if err != nil {
		t.Fatalf("NewPackRegistry() should not error for missing dir, got: %v", err)
	}
	if r == nil {
		t.Fatal("registry should not be nil")
	}
	if len(r.List()) != 0 {
		t.Errorf("expected 0 packs for missing dir")
	}
}

func TestRegistryLoad_SinglePack(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	packs := r.List()
	if len(packs) != 1 {
		t.Fatalf("expected 1 pack, got %d", len(packs))
	}
	if packs[0].Name != "my-pack" {
		t.Errorf("Name = %q", packs[0].Name)
	}
}

func TestRegistryLoad_MultiplePacks(t *testing.T) {
	dir := t.TempDir()

	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	manifest2 := `name: other-pack
description: "Another pack"
version: 1
instructions: other-pack.md
category: dev
`
	writePackDir(t, dir, "other-pack", manifest2, "# Other Pack")

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	packs := r.List()
	if len(packs) != 2 {
		t.Fatalf("expected 2 packs, got %d", len(packs))
	}
}

func TestRegistryLoad_InvalidManifest(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "bad-pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write malformed YAML
	if err := os.WriteFile(filepath.Join(packDir, "bad-pack.yaml"), []byte("[bad yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := NewPackRegistry(dir)
	if err == nil {
		t.Fatal("expected error for invalid manifest YAML")
	}
}

func TestRegistryLoad_DirectoryWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	// Create a directory without a YAML manifest - should be silently skipped
	subDir := filepath.Join(dir, "no-manifest")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() should skip dirs without manifests, got error: %v", err)
	}
	if len(r.List()) != 0 {
		t.Errorf("expected 0 packs (no manifests found)")
	}
}

func TestRegistryFind_ByName(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	results := r.Find("my-pack")
	if len(results) == 0 {
		t.Fatal("expected to find my-pack by name")
	}
	if results[0].Name != "my-pack" {
		t.Errorf("Name = %q", results[0].Name)
	}
}

func TestRegistryFind_ByDescription(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	results := r.Find("unit testing")
	if len(results) == 0 {
		t.Fatal("expected to find pack by description keyword")
	}
}

func TestRegistryFind_ByTag(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	results := r.Find("unit-test")
	if len(results) == 0 {
		t.Fatal("expected to find pack by tag")
	}
}

func TestRegistryFind_ByCategory(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	results := r.Find("testing")
	if len(results) == 0 {
		t.Fatal("expected to find pack by category")
	}
}

func TestRegistryFind_NoMatch(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	results := r.Find("xyzzy-no-match-abcdef")
	// Must return non-nil empty slice, not nil
	if results == nil {
		t.Error("Find() should return empty slice, not nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRegistryFind_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	results := r.Find("UNIT-TEST")
	if len(results) == 0 {
		t.Fatal("Find() should be case-insensitive")
	}
}

func TestRegistryListByCategory(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	manifest2 := `name: other-pack
description: "Another pack"
version: 1
instructions: other-pack.md
category: deployment
`
	writePackDir(t, dir, "other-pack", manifest2, "# Other Pack")

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	testingPacks := r.ListByCategory("testing")
	if len(testingPacks) != 1 {
		t.Errorf("expected 1 testing pack, got %d", len(testingPacks))
	}
	if testingPacks[0].Name != "my-pack" {
		t.Errorf("expected my-pack, got %q", testingPacks[0].Name)
	}

	deployPacks := r.ListByCategory("deployment")
	if len(deployPacks) != 1 {
		t.Errorf("expected 1 deployment pack, got %d", len(deployPacks))
	}

	noPacks := r.ListByCategory("nonexistent-category")
	if noPacks == nil {
		t.Error("ListByCategory should return empty slice, not nil")
	}
	if len(noPacks) != 0 {
		t.Errorf("expected 0 packs for nonexistent category, got %d", len(noPacks))
	}
}

func TestRegistryActivate_ValidPack(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	activated, err := r.Activate("my-pack")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if activated == nil {
		t.Fatal("Activate() returned nil")
	}
	if activated.Manifest.Name != "my-pack" {
		t.Errorf("Name = %q", activated.Manifest.Name)
	}
	if activated.Instructions == "" {
		t.Error("Instructions should not be empty")
	}
	if !strings.Contains(activated.Instructions, "My Pack Instructions") {
		t.Errorf("Instructions does not contain expected content: %q", activated.Instructions)
	}
}

func TestRegistryActivate_NotFound(t *testing.T) {
	dir := t.TempDir()
	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	_, err = r.Activate("nonexistent-pack")
	if err == nil {
		t.Fatal("expected error for nonexistent pack")
	}
	if !strings.Contains(err.Error(), "nonexistent-pack") {
		t.Errorf("error should mention pack name: %v", err)
	}
}

func TestRegistryActivate_MissingCLI(t *testing.T) {
	dir := t.TempDir()
	manifest := `name: cli-pack
description: "Requires missing CLI"
version: 1
instructions: cli-pack.md
requires_cli:
  - definitely-missing-cli-xyz-123
`
	writePackDir(t, dir, "cli-pack", manifest, "# CLI Pack")

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	_, err = r.Activate("cli-pack")
	if err == nil {
		t.Fatal("expected error for missing CLI prerequisite")
	}
	if !strings.Contains(err.Error(), "definitely-missing-cli-xyz-123") {
		t.Errorf("error should mention missing CLI: %v", err)
	}
}

func TestRegistryActivate_MissingEnv(t *testing.T) {
	dir := t.TempDir()
	manifest := `name: env-pack
description: "Requires missing env var"
version: 1
instructions: env-pack.md
requires_env:
  - DEFINITELY_MISSING_ENV_VAR_XYZ_123
`
	writePackDir(t, dir, "env-pack", manifest, "# Env Pack")

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	_, err = r.Activate("env-pack")
	if err == nil {
		t.Fatal("expected error for missing env var prerequisite")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_MISSING_ENV_VAR_XYZ_123") {
		t.Errorf("error should mention missing env var: %v", err)
	}
}

func TestRegistryActivate_MissingInstructionsFile(t *testing.T) {
	dir := t.TempDir()
	// Create manifest that references a non-existent instructions file
	manifest := `name: no-instr
description: "Instructions file is missing"
version: 1
instructions: nonexistent.md
`
	packDir := filepath.Join(dir, "no-instr")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "no-instr.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	// NOTE: intentionally NOT creating the instructions file

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	_, err = r.Activate("no-instr")
	if err == nil {
		t.Fatal("expected error for missing instructions file")
	}
}

func TestRegistryActivate_AllowedTools(t *testing.T) {
	dir := t.TempDir()
	manifest := `name: limited-pack
description: "Pack with allowed tools constraint"
version: 1
instructions: limited-pack.md
allowed_tools:
  - bash
  - read
  - grep
`
	writePackDir(t, dir, "limited-pack", manifest, "# Limited Pack")

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	activated, err := r.Activate("limited-pack")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	if len(activated.Manifest.AllowedTools) != 3 {
		t.Errorf("AllowedTools = %v, expected 3 tools", activated.Manifest.AllowedTools)
	}
	// Verify constraint is preserved
	toolSet := make(map[string]bool)
	for _, tool := range activated.Manifest.AllowedTools {
		toolSet[tool] = true
	}
	if !toolSet["bash"] || !toolSet["read"] || !toolSet["grep"] {
		t.Errorf("expected bash, read, grep in AllowedTools, got %v", activated.Manifest.AllowedTools)
	}
}

func TestRegistryList_Sorted(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"charlie-pack", "alpha-pack", "bravo-pack"} {
		m := `name: ` + name + `
description: "` + name + ` description"
version: 1
instructions: ` + name + `.md
`
		writePackDir(t, dir, name, m, "# "+name)
	}

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	packs := r.List()
	if len(packs) != 3 {
		t.Fatalf("expected 3 packs, got %d", len(packs))
	}
	if packs[0].Name != "alpha-pack" || packs[1].Name != "bravo-pack" || packs[2].Name != "charlie-pack" {
		t.Errorf("packs not sorted: %v", []string{packs[0].Name, packs[1].Name, packs[2].Name})
	}
}

// --- Concurrency Tests ---

func TestRegistryConcurrentFind(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results := r.Find("testing")
			_ = results
		}()
	}
	wg.Wait()
}

func TestRegistryConcurrentActivate(t *testing.T) {
	dir := t.TempDir()

	manifest1 := `name: pack-one
description: "First pack"
version: 1
instructions: pack-one.md
`
	manifest2 := `name: pack-two
description: "Second pack"
version: 1
instructions: pack-two.md
`
	writePackDir(t, dir, "pack-one", manifest1, "# Pack One")
	writePackDir(t, dir, "pack-two", manifest2, "# Pack Two")

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = r.Activate("pack-one")
		}()
		go func() {
			defer wg.Done()
			_, _ = r.Activate("pack-two")
		}()
	}
	wg.Wait()
}

func TestRegistryConcurrentListAndFind(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = r.List()
		}()
		go func() {
			defer wg.Done()
			_ = r.Find("my-pack")
		}()
		go func() {
			defer wg.Done()
			_ = r.ListByCategory("testing")
		}()
	}
	wg.Wait()
}

// --- Regression Tests ---

func TestRegistryWithSubdirectories_NotScanned(t *testing.T) {
	dir := t.TempDir()
	writePackDir(t, dir, "my-pack", validPackManifest, validPackInstructions)

	// Create a nested subdirectory that should NOT be scanned
	nestedManifest := `name: nested-pack
description: "Nested pack should not be loaded"
version: 1
instructions: nested-pack.md
`
	nestedDir := filepath.Join(dir, "my-pack", "nested-pack")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "nested-pack.yaml"), []byte(nestedManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	// Should only find my-pack, not nested-pack
	packs := r.List()
	if len(packs) != 1 {
		t.Errorf("expected 1 pack (nested dirs not scanned), got %d", len(packs))
	}
}

func TestRegistryActivate_InstructionsContent(t *testing.T) {
	dir := t.TempDir()
	instructions := `# My Pack Instructions

## Overview

This pack provides specialized workflows for testing.

## Usage

Run the tool with specific arguments.
`
	writePackDir(t, dir, "my-pack", validPackManifest, instructions)

	r, err := NewPackRegistry(dir)
	if err != nil {
		t.Fatalf("NewPackRegistry() error = %v", err)
	}

	activated, err := r.Activate("my-pack")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	if activated.Instructions != instructions {
		t.Errorf("Instructions mismatch\ngot:  %q\nwant: %q", activated.Instructions, instructions)
	}
}
