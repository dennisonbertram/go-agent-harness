package packs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestParsing_Valid(t *testing.T) {
	data := []byte(`
name: railway-deploy
display_name: "Railway Deployment"
category: deployment
description: "Deploy applications to Railway"
version: 1
tools:
  - bash
  - web_fetch
requires_cli:
  - railway
requires_env:
  - RAILWAY_TOKEN
instructions: railway_deploy.md
tags:
  - deploy
  - railway
allowed_tools:
  - bash
  - read
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.Name != "railway-deploy" {
		t.Errorf("Name = %q, want railway-deploy", m.Name)
	}
	if m.DisplayName != "Railway Deployment" {
		t.Errorf("DisplayName = %q", m.DisplayName)
	}
	if m.Category != "deployment" {
		t.Errorf("Category = %q", m.Category)
	}
	if m.Description != "Deploy applications to Railway" {
		t.Errorf("Description = %q", m.Description)
	}
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if len(m.Tools) != 2 || m.Tools[0] != "bash" || m.Tools[1] != "web_fetch" {
		t.Errorf("Tools = %v", m.Tools)
	}
	if len(m.RequiresCLI) != 1 || m.RequiresCLI[0] != "railway" {
		t.Errorf("RequiresCLI = %v", m.RequiresCLI)
	}
	if len(m.RequiresEnv) != 1 || m.RequiresEnv[0] != "RAILWAY_TOKEN" {
		t.Errorf("RequiresEnv = %v", m.RequiresEnv)
	}
	if m.Instructions != "railway_deploy.md" {
		t.Errorf("Instructions = %q", m.Instructions)
	}
	if len(m.Tags) != 2 {
		t.Errorf("Tags = %v", m.Tags)
	}
	if len(m.AllowedTools) != 2 {
		t.Errorf("AllowedTools = %v", m.AllowedTools)
	}
}

func TestManifestParsing_Minimal(t *testing.T) {
	data := []byte(`
name: simple-pack
description: "A minimal pack"
version: 1
instructions: simple.md
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.Name != "simple-pack" {
		t.Errorf("Name = %q", m.Name)
	}
	if len(m.Tools) != 0 {
		t.Errorf("Tools should be empty, got %v", m.Tools)
	}
	if len(m.RequiresCLI) != 0 {
		t.Errorf("RequiresCLI should be empty")
	}
	if len(m.RequiresEnv) != 0 {
		t.Errorf("RequiresEnv should be empty")
	}
}

func TestManifestParsing_Invalid_MalformedYAML(t *testing.T) {
	data := []byte(`
name: [bad yaml
`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestManifestValidation_MissingName(t *testing.T) {
	data := []byte(`
description: "No name"
version: 1
instructions: foo.md
`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestManifestValidation_MissingDescription(t *testing.T) {
	data := []byte(`
name: my-pack
version: 1
instructions: foo.md
`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestManifestValidation_MissingInstructions(t *testing.T) {
	data := []byte(`
name: my-pack
description: "A pack"
version: 1
`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for missing instructions")
	}
}

func TestManifestValidation_MissingVersion(t *testing.T) {
	data := []byte(`
name: my-pack
description: "A pack"
instructions: foo.md
`)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for missing version (version == 0)")
	}
}

func TestManifestParsing_ExtraFields(t *testing.T) {
	// Unknown fields should not cause errors (forward compat)
	data := []byte(`
name: future-pack
description: "Has unknown fields"
version: 1
instructions: foo.md
future_field: "some value"
another_new_field: 42
`)
	_, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() should ignore extra fields, got error: %v", err)
	}
}

func TestManifestParsing_UnicodeNames(t *testing.T) {
	data := []byte(`
name: unicode-pack
display_name: "日本語パック"
description: "Contains unicode: 中文, العربية, 한국어"
version: 1
instructions: foo.md
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if m.DisplayName != "日本語パック" {
		t.Errorf("DisplayName = %q", m.DisplayName)
	}
}

func TestLoadManifestFromFile(t *testing.T) {
	dir := t.TempDir()
	content := `name: file-pack
description: "Loaded from file"
version: 1
instructions: file.md
category: testing
`
	path := filepath.Join(dir, "file-pack.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifestFromFile(path)
	if err != nil {
		t.Fatalf("LoadManifestFromFile() error = %v", err)
	}
	if m.Name != "file-pack" {
		t.Errorf("Name = %q", m.Name)
	}
}

func TestLoadManifestFromFile_NotFound(t *testing.T) {
	_, err := LoadManifestFromFile("/nonexistent/path/pack.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
