package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"go-agent-harness/cmd/harnesscli/config"
)

// populated returns a config carrying every field a wipe would destroy.
func populated() *config.Config {
	return &config.Config{
		StarredModels:  []string{"gpt-4.1-mini"},
		Gateway:        "openrouter",
		APIKeys:        map[string]string{"openrouter": "sk-or-v1-notarealkey"},
		HistoryEntries: []string{"hello"},
		Theme:          "ocean",
	}
}

// TestSaveReplacesFileRatherThanTruncatingInPlace is the regression test for
// issue #1300.
//
// It asserts the mechanism rather than racing for the symptom: os.WriteFile
// truncates the existing file, keeping its inode, and a reader catching that
// window gets a partial parse. os.Rename swaps in a new file, so the inode
// changes and no reader can ever observe a half-written config.
//
// An earlier version of this test raced a writer against a reader and passed
// with the bug present — the truncate window is too small to hit reliably. This
// form fails deterministically.
func TestSaveReplacesFileRatherThanTruncatingInPlace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := config.Save(populated()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	path := filepath.Join(home, ".config", "harnesscli", "config.json")

	inodeOf := func() uint64 {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Skip("inode not available on this platform")
		}
		return uint64(st.Ino)
	}

	before := inodeOf()
	if err := config.Save(populated()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	after := inodeOf()

	if before == after {
		t.Error("Save reused the same file (same inode) — it truncates in place, so a concurrent reader can see a partial config")
	}
}

// TestLoadDistinguishesMissingFile — Load returned a nil error for a missing file,
// so a caller could not tell "no config yet" from "read fine and it was empty",
// and would happily persist an empty config over a populated one.
func TestLoadDistinguishesMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load on missing file should not be a hard error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned a nil config")
	}
	if config.Exists() {
		t.Error("Exists() reported a config file that does not exist")
	}

	if err := config.Save(populated()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !config.Exists() {
		t.Error("Exists() did not see the file just written")
	}
}

// TestSavePreservesFileMode — the file holds a plaintext API key, so the atomic
// rewrite must not widen its permissions.
func TestSavePreservesFileMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := config.Save(populated()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(filepath.Join(home, ".config", "harnesscli", "config.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %o, want 600 — it holds a plaintext API key", perm)
	}
}

// TestSaveLeavesNoTempFiles — an atomic write via rename must not litter.
func TestSaveLeavesNoTempFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for i := 0; i < 5; i++ {
		if err := config.Save(populated()); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(home, ".config", "harnesscli"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "config.json" {
			t.Errorf("leftover file after Save: %q", e.Name())
		}
	}
}

// TestRoundTripPreservesEveryField is the false-positive control: atomicity must
// not come at the cost of dropping data.
func TestRoundTripPreservesEveryField(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := populated()
	if err := config.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Errorf("round trip changed the config:\n want %s\n got  %s", a, b)
	}
}
