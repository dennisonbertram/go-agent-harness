package symphd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	content := `
addr: ":9999"
workspace_type: "worktree"
max_concurrent_agents: 5
poll_interval_ms: 2000
harness_url: "http://localhost:9090"
base_dir: "/tmp/test"
`
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Addr != ":9999" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.WorkspaceType != "worktree" {
		t.Errorf("WorkspaceType = %q", cfg.WorkspaceType)
	}
	if cfg.MaxConcurrentAgents != 5 {
		t.Errorf("MaxConcurrentAgents = %d", cfg.MaxConcurrentAgents)
	}
	if cfg.PollIntervalMs != 2000 {
		t.Errorf("PollIntervalMs = %d", cfg.PollIntervalMs)
	}
	if cfg.HarnessURL != "http://localhost:9090" {
		t.Errorf("HarnessURL = %q", cfg.HarnessURL)
	}
	if cfg.BaseDir != "/tmp/test" {
		t.Errorf("BaseDir = %q", cfg.BaseDir)
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Addr != ":8888" {
		t.Errorf("default Addr = %q", cfg.Addr)
	}
	if cfg.WorkspaceType != "local" {
		t.Errorf("default WorkspaceType = %q", cfg.WorkspaceType)
	}
	if cfg.MaxConcurrentAgents != 10 {
		t.Errorf("default MaxConcurrentAgents = %d", cfg.MaxConcurrentAgents)
	}
	if cfg.PollIntervalMs != 5000 {
		t.Errorf("default PollIntervalMs = %d", cfg.PollIntervalMs)
	}
	if cfg.HarnessURL != "http://localhost:8080" {
		t.Errorf("default HarnessURL = %q", cfg.HarnessURL)
	}
	if cfg.BaseDir == "" {
		t.Error("default BaseDir should not be empty")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("addr: [bad yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Addr != ":8888" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.MaxConcurrentAgents != 10 {
		t.Errorf("MaxConcurrentAgents = %d", cfg.MaxConcurrentAgents)
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	if err := os.WriteFile(path, []byte("addr: \":7777\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Addr != ":7777" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	// Unset fields should get defaults
	if cfg.WorkspaceType != "local" {
		t.Errorf("WorkspaceType = %q", cfg.WorkspaceType)
	}
	if cfg.HarnessURL != "http://localhost:8080" {
		t.Errorf("HarnessURL = %q", cfg.HarnessURL)
	}
}

func TestLoad_ErrorContainsPath(t *testing.T) {
	_, err := Load("/no/such/file.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "/no/such/file.yaml") {
		t.Errorf("error should contain path, got: %v", err)
	}
}
