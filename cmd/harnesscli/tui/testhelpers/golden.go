package testhelpers

import (
	"os"
	"path/filepath"
	"testing"
)

// WriteGolden writes content to a golden file at <baseDir>/snapshots/<name>.txt.
// It creates the directory structure if needed.
func WriteGolden(t *testing.T, baseDir, name, content string) {
	t.Helper()
	dir := filepath.Join(baseDir, "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating golden dir: %v", err)
	}
	path := filepath.Join(dir, name+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing golden file: %v", err)
	}
}

// ReadGolden reads a golden file from <baseDir>/snapshots/<name>.txt.
func ReadGolden(t *testing.T, baseDir, name string) string {
	t.Helper()
	path := filepath.Join(baseDir, "snapshots", name+".txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v", path, err)
	}
	return string(data)
}

// AssertGolden compares actual content against a golden file.
// If update is true, it writes/overwrites the golden file with actual content.
// If update is false, it reads the golden file and compares.
func AssertGolden(t *testing.T, baseDir, name, actual string, update bool) {
	t.Helper()
	if update {
		WriteGolden(t, baseDir, name, actual)
		return
	}
	expected := ReadGolden(t, baseDir, name)
	if expected != actual {
		t.Errorf("golden mismatch for %q:\n  got:  %q\n  want: %q", name, actual, expected)
	}
}

// GoldenFile reads a golden file from testdata/snapshots and returns its content.
// If the file does not exist and update is true, it creates it with the given content.
// Deprecated: prefer WriteGolden/ReadGolden/AssertGolden with explicit baseDir.
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
