// Package watcher provides a polling-based file watcher that monitors
// directories for changes (create, modify, delete) and calls a reload
// callback when changes are detected.
//
// It deliberately avoids external dependencies (no fsnotify) by comparing
// directory snapshots on a configurable interval. This keeps the dependency
// footprint minimal and makes the behavior predictable in tests.
package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WatchedDir pairs a directory path with the function to call when changes
// are detected inside that directory (including the directory itself).
type WatchedDir struct {
	// Path is the directory to monitor. If it does not exist at poll time,
	// the poll is silently skipped (the directory may be created later).
	Path string

	// Reload is called when the watcher detects that files in Path have
	// changed (added, modified, or deleted).
	// If Reload returns an error it is logged but the watcher continues.
	Reload func() error
}

// fileState holds the minimal information needed to detect changes to a file.
type fileState struct {
	modTime time.Time
	size    int64
}

// dirSnapshot maps file name (relative to the watched dir) to its state.
type dirSnapshot map[string]fileState

// PollingWatcher monitors a set of directories by periodically comparing
// directory snapshots. It is safe for concurrent use.
type PollingWatcher struct {
	mu       sync.Mutex
	dirs     []WatchedDir
	interval time.Duration
}

// New creates a new PollingWatcher that will poll at the given interval.
// A reasonable default is 5 * time.Second for production; tests can use
// much smaller values (e.g. 10ms) for fast feedback.
func New(interval time.Duration) *PollingWatcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &PollingWatcher{interval: interval}
}

// Watch registers a directory to be monitored. Watch may be called
// concurrently with other Watch calls or even with Start.
func (w *PollingWatcher) Watch(dir WatchedDir) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.dirs = append(w.dirs, dir)
}

// Start begins polling. It blocks until ctx is cancelled, at which point it
// returns. It is safe to call Start in a goroutine.
func (w *PollingWatcher) Start(ctx context.Context) {
	// Take initial snapshots for all currently registered directories.
	snapshots := w.buildInitialSnapshots()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(snapshots)
		}
	}
}

// buildInitialSnapshots takes a snapshot of every currently-registered
// directory without triggering any reloads. This establishes the baseline
// state that future polls compare against.
func (w *PollingWatcher) buildInitialSnapshots() map[string]dirSnapshot {
	w.mu.Lock()
	dirs := make([]WatchedDir, len(w.dirs))
	copy(dirs, w.dirs)
	w.mu.Unlock()

	snaps := make(map[string]dirSnapshot, len(dirs))
	for _, d := range dirs {
		snaps[d.Path] = snapshot(d.Path)
	}
	return snaps
}

// poll runs one iteration of the check-and-reload loop.
func (w *PollingWatcher) poll(snapshots map[string]dirSnapshot) {
	w.mu.Lock()
	dirs := make([]WatchedDir, len(w.dirs))
	copy(dirs, w.dirs)
	w.mu.Unlock()

	for _, d := range dirs {
		// Ensure we have a baseline snapshot for directories added after Start.
		if _, ok := snapshots[d.Path]; !ok {
			snapshots[d.Path] = snapshot(d.Path)
			continue
		}

		current := snapshot(d.Path)
		if !snapshotsEqual(snapshots[d.Path], current) {
			snapshots[d.Path] = current
			if err := d.Reload(); err != nil {
				log.Printf("watcher: reload error for %s: %v", d.Path, err)
			}
		}
	}
}

// snapshot walks a directory (non-recursively) and records each file's
// modification time and size. If the directory does not exist or cannot
// be read, an empty snapshot is returned so that future directory creation
// will be detected as a change.
func snapshot(dir string) dirSnapshot {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dirSnapshot{} // missing or unreadable dir → empty snapshot
	}

	snap := make(dirSnapshot, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			// Walk one level of sub-directories so that nested SKILL.md
			// files are tracked (each skill lives in its own sub-dir).
			subDir := filepath.Join(dir, entry.Name())
			subEntries, err := os.ReadDir(subDir)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if sub.IsDir() {
					continue // only one extra level
				}
				info, err := sub.Info()
				if err != nil {
					continue
				}
				key := entry.Name() + "/" + sub.Name()
				snap[key] = fileState{modTime: info.ModTime(), size: info.Size()}
			}
			// Also track the sub-directory itself (for create/delete detection)
			info, err := entry.Info()
			if err != nil {
				continue
			}
			snap[entry.Name()+"/"] = fileState{modTime: info.ModTime(), size: info.Size()}
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}
		snap[entry.Name()] = fileState{modTime: info.ModTime(), size: info.Size()}
	}
	return snap
}

// snapshotsEqual returns true if both snapshots contain the same set of
// files with the same modification times and sizes.
func snapshotsEqual(a, b dirSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if va.modTime != vb.modTime || va.size != vb.size {
			return false
		}
	}
	return true
}
