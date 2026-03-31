package speculation

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Overlay manages a copy-on-write directory for speculative execution.
// The OverlayDir is a temporary directory that captures any writes made
// during speculation. The BaseDir is the original workspace.
type Overlay struct {
	// ID is a unique identifier for this overlay.
	ID string

	// BaseDir is the original workspace directory (read-only reference).
	BaseDir string

	// OverlayDir is the temporary directory where speculative writes land.
	OverlayDir string

	// CreatedAt is when this overlay was created.
	CreatedAt time.Time
}

// NewOverlay creates a new speculation overlay directory.
// If cfg.OverlayDir is empty, $TMPDIR/speculation/ is used as the base.
func NewOverlay(baseDir string, cfg SpeculationConfig) (*Overlay, error) {
	id := uuid.New().String()

	overlayBase := cfg.OverlayDir
	if overlayBase == "" {
		overlayBase = filepath.Join(os.TempDir(), "speculation")
	}

	overlayDir := filepath.Join(overlayBase, id)
	if err := os.MkdirAll(overlayDir, 0700); err != nil {
		return nil, fmt.Errorf("speculation.NewOverlay: create overlay dir %q: %w", overlayDir, err)
	}

	return &Overlay{
		ID:         id,
		BaseDir:    baseDir,
		OverlayDir: overlayDir,
		CreatedAt:  time.Now(),
	}, nil
}

// Cleanup removes the overlay directory and all its contents.
func (o *Overlay) Cleanup() error {
	if o.OverlayDir == "" {
		return nil
	}
	if err := os.RemoveAll(o.OverlayDir); err != nil {
		return fmt.Errorf("speculation.Overlay.Cleanup: remove %q: %w", o.OverlayDir, err)
	}
	return nil
}

// HasChanges checks if any files were written to the overlay directory.
// Returns true if the overlay directory contains at least one file or subdirectory.
func (o *Overlay) HasChanges() (bool, error) {
	entries, err := os.ReadDir(o.OverlayDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("speculation.Overlay.HasChanges: read dir %q: %w", o.OverlayDir, err)
	}
	return len(entries) > 0, nil
}

// ListChanges returns paths of files modified in the overlay.
// Paths are relative to the OverlayDir.
func (o *Overlay) ListChanges() ([]string, error) {
	var changes []string
	err := filepath.Walk(o.OverlayDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Skip the root overlay directory itself
		if path == o.OverlayDir {
			return nil
		}
		rel, err := filepath.Rel(o.OverlayDir, path)
		if err != nil {
			return err
		}
		changes = append(changes, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("speculation.Overlay.ListChanges: walk %q: %w", o.OverlayDir, err)
	}
	return changes, nil
}
