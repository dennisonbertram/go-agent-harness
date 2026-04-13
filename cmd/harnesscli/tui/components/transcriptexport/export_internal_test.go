package transcriptexport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectRuntimeSafeOutputDirSkipsUnwritableCandidates(t *testing.T) {
	blockedParent := t.TempDir()
	blockedDir := filepath.Join(blockedParent, "blocked")
	if err := os.MkdirAll(blockedDir, 0o755); err != nil {
		t.Fatalf("mkdir blocked dir: %v", err)
	}
	if err := os.Chmod(blockedDir, 0o555); err != nil {
		t.Fatalf("chmod blocked dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(blockedDir, 0o755)
	})

	fallbackRoot := t.TempDir()
	fallbackDir := filepath.Join(fallbackRoot, "exports")

	got := selectRuntimeSafeOutputDir([]string{blockedDir, fallbackDir})
	want, err := filepath.Abs(fallbackDir)
	if err != nil {
		t.Fatalf("abs fallback dir: %v", err)
	}
	if got != want {
		t.Fatalf("selected output dir: got %q, want %q", got, want)
	}

	testFile := filepath.Join(got, "write-check.md")
	if err := os.WriteFile(testFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write fallback file: %v", err)
	}
}
