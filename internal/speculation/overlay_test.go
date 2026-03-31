package speculation_test

import (
	"os"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/speculation"
)

// TestNewOverlay_CreatesDirectory verifies overlay dir exists after creation.
func TestNewOverlay_CreatesDirectory(t *testing.T) {
	baseDir := t.TempDir()
	overlayBase := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	cfg.OverlayDir = overlayBase

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	defer overlay.Cleanup() //nolint:errcheck

	if overlay.OverlayDir == "" {
		t.Fatal("OverlayDir: got empty string, want a directory path")
	}
	if _, statErr := os.Stat(overlay.OverlayDir); os.IsNotExist(statErr) {
		t.Errorf("OverlayDir %q does not exist after NewOverlay()", overlay.OverlayDir)
	}
}

// TestOverlay_Cleanup verifies directory is removed after Cleanup.
func TestOverlay_Cleanup(t *testing.T) {
	baseDir := t.TempDir()
	overlayBase := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	cfg.OverlayDir = overlayBase

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}

	overlayPath := overlay.OverlayDir

	if err := overlay.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	if _, statErr := os.Stat(overlayPath); !os.IsNotExist(statErr) {
		t.Errorf("OverlayDir %q still exists after Cleanup(), want removed", overlayPath)
	}
}

// TestOverlay_HasChanges_Empty verifies no changes in a fresh overlay.
func TestOverlay_HasChanges_Empty(t *testing.T) {
	baseDir := t.TempDir()
	overlayBase := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	cfg.OverlayDir = overlayBase

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	defer overlay.Cleanup() //nolint:errcheck

	changed, err := overlay.HasChanges()
	if err != nil {
		t.Fatalf("HasChanges() error: %v", err)
	}
	if changed {
		t.Error("HasChanges(): got true on fresh overlay, want false")
	}
}

// TestOverlay_HasChanges_WithFile verifies HasChanges=true after writing a file.
func TestOverlay_HasChanges_WithFile(t *testing.T) {
	baseDir := t.TempDir()
	overlayBase := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	cfg.OverlayDir = overlayBase

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	defer overlay.Cleanup() //nolint:errcheck

	// Write a file into the overlay directory to simulate a write operation
	testFile := filepath.Join(overlay.OverlayDir, "modified.txt")
	if err := os.WriteFile(testFile, []byte("modified content"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	changed, err := overlay.HasChanges()
	if err != nil {
		t.Fatalf("HasChanges() error: %v", err)
	}
	if !changed {
		t.Error("HasChanges(): got false after writing a file, want true")
	}
}

// TestOverlay_ListChanges_Empty verifies empty list for fresh overlay.
func TestOverlay_ListChanges_Empty(t *testing.T) {
	baseDir := t.TempDir()
	overlayBase := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	cfg.OverlayDir = overlayBase

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	defer overlay.Cleanup() //nolint:errcheck

	changes, err := overlay.ListChanges()
	if err != nil {
		t.Fatalf("ListChanges() error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("ListChanges(): got %d entries on fresh overlay, want 0; entries: %v", len(changes), changes)
	}
}

// TestOverlay_ListChanges_WithFiles verifies ListChanges returns written file paths.
func TestOverlay_ListChanges_WithFiles(t *testing.T) {
	baseDir := t.TempDir()
	overlayBase := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	cfg.OverlayDir = overlayBase

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	defer overlay.Cleanup() //nolint:errcheck

	// Write two files into the overlay directory
	file1 := filepath.Join(overlay.OverlayDir, "file1.go")
	file2 := filepath.Join(overlay.OverlayDir, "file2.go")
	if err := os.WriteFile(file1, []byte("content1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0600); err != nil {
		t.Fatal(err)
	}

	changes, err := overlay.ListChanges()
	if err != nil {
		t.Fatalf("ListChanges() error: %v", err)
	}
	if len(changes) != 2 {
		t.Errorf("ListChanges(): got %d entries, want 2; entries: %v", len(changes), changes)
	}
}

// TestNewOverlay_IDIsNonEmpty verifies overlay has a non-empty ID.
func TestNewOverlay_IDIsNonEmpty(t *testing.T) {
	baseDir := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	defer overlay.Cleanup() //nolint:errcheck

	if overlay.ID == "" {
		t.Error("Overlay.ID: got empty string, want non-empty unique ID")
	}
}

// TestNewOverlay_AutoTempDir verifies overlay uses $TMPDIR/speculation/ when OverlayDir is empty.
func TestNewOverlay_AutoTempDir(t *testing.T) {
	baseDir := t.TempDir()
	cfg := speculation.DefaultSpeculationConfig()
	cfg.OverlayDir = "" // auto

	overlay, err := speculation.NewOverlay(baseDir, cfg)
	if err != nil {
		t.Fatalf("NewOverlay() error: %v", err)
	}
	defer overlay.Cleanup() //nolint:errcheck

	if overlay.OverlayDir == "" {
		t.Fatal("OverlayDir: got empty string even with auto mode, want a temp path")
	}
	if _, statErr := os.Stat(overlay.OverlayDir); os.IsNotExist(statErr) {
		t.Errorf("Auto-created OverlayDir %q does not exist", overlay.OverlayDir)
	}
}
