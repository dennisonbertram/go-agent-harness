package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui"
)

// Issue #1407: "@cal" + Tab must complete bare relative names, not only
// paths that start with ./, / or ~. A first-time user types the file name.
func TestFilePathCompleter_BareRelativeName(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"calc.go", "calc_test.go", "go.mod"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	got := tui.FilePathCompleter("Explain what @cal")
	if len(got) != 2 {
		t.Fatalf("want the two calc files, got %v", got)
	}
	for _, c := range got {
		if c != "Explain what @calc.go" && c != "Explain what @calc_test.go" {
			t.Errorf("unexpected completion %q", c)
		}
	}
	if got := tui.FilePathCompleter("say @go.m"); len(got) != 1 || got[0] != "say @go.mod" {
		t.Errorf("single bare match must complete fully, got %v", got)
	}
}
