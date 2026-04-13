package repostructure

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestRepoRootDoesNotContainGoSource(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		t.Fatalf("read repo root: %v", err)
	}

	var goFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".go" {
			goFiles = append(goFiles, entry.Name())
		}
	}

	if len(goFiles) == 0 {
		return
	}

	slices.Sort(goFiles)
	t.Fatalf("repo root should be reserved for repo metadata and directories; found Go source files: %v", goFiles)
}

func TestPlaygroundIsIsolatedAsSeparateModule(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(repoRoot, "playground", "go.mod")); err != nil {
		t.Fatalf("playground should be isolated behind its own module boundary: %v", err)
	}
}
