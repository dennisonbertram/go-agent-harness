package testhelpers

import (
	"os"
	"path/filepath"
	"testing"
)

// GoldenFile reads a golden file from testdata/snapshots and returns its content.
// If the file does not exist and update is true, it creates it with the given content.
func GoldenFile(t *testing.T, name string, actual string, update bool) string {
	t.Helper()
	path := filepath.Join("testdata", "snapshots", name)
	if update {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return actual
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v", path, err)
	}
	return string(data)
}
