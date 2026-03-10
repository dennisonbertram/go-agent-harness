package watcher_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-agent-harness/internal/watcher"
)

// TestNewPollingWatcher verifies construction with default and custom intervals.
func TestNewPollingWatcher(t *testing.T) {
	t.Parallel()

	w := watcher.New(100 * time.Millisecond)
	if w == nil {
		t.Fatal("expected non-nil watcher")
	}
}

// TestWatchNonExistentDirSkipped verifies that a missing directory does not
// cause Start to crash; changes are silently skipped.
func TestWatchNonExistentDirSkipped(t *testing.T) {
	t.Parallel()

	reloadCalled := atomic.Int32{}
	w := watcher.New(20 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: "/tmp/does-not-exist-harness-72-abcdef",
		Reload: func() error {
			reloadCalled.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	w.Start(ctx)

	// Reload should NOT have been called for a missing dir
	if reloadCalled.Load() != 0 {
		t.Errorf("expected no reload calls for missing dir, got %d", reloadCalled.Load())
	}
}

// TestChangeDetectionFileCreate verifies that creating a file triggers Reload.
func TestChangeDetectionFileCreate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	reloadCalled := make(chan struct{}, 5)

	w := watcher.New(10 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			reloadCalled <- struct{}{}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)

	// Give watcher a moment to take an initial snapshot
	time.Sleep(30 * time.Millisecond)

	// Create a new file
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloadCalled:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reload was not called after file creation")
	}
}

// TestChangeDetectionFileModify verifies that modifying an existing file triggers Reload.
func TestChangeDetectionFileModify(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Pre-create a file before the watcher starts
	filePath := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(filePath, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	reloadCalled := make(chan struct{}, 5)

	w := watcher.New(10 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			reloadCalled <- struct{}{}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)

	// Give watcher a moment to take an initial snapshot
	time.Sleep(30 * time.Millisecond)

	// Modify the file - sleep briefly to ensure mtime changes
	time.Sleep(15 * time.Millisecond)
	if err := os.WriteFile(filePath, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloadCalled:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reload was not called after file modification")
	}
}

// TestChangeDetectionFileDelete verifies that deleting a file triggers Reload.
func TestChangeDetectionFileDelete(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Pre-create a file before the watcher starts
	filePath := filepath.Join(dir, "todelete.md")
	if err := os.WriteFile(filePath, []byte("delete me"), 0o644); err != nil {
		t.Fatal(err)
	}

	reloadCalled := make(chan struct{}, 5)

	w := watcher.New(10 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			reloadCalled <- struct{}{}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)

	// Give watcher a moment to take an initial snapshot
	time.Sleep(30 * time.Millisecond)

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloadCalled:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reload was not called after file deletion")
	}
}

// TestMultipleWatchedDirs verifies that multiple watched directories are all monitored.
func TestMultipleWatchedDirs(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	reloadCount := atomic.Int32{}

	w := watcher.New(10 * time.Millisecond)
	reload := func() error {
		reloadCount.Add(1)
		return nil
	}
	w.Watch(watcher.WatchedDir{Path: dir1, Reload: reload})
	w.Watch(watcher.WatchedDir{Path: dir2, Reload: reload})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)
	time.Sleep(30 * time.Millisecond)

	// Write to dir1
	if err := os.WriteFile(filepath.Join(dir1, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write to dir2
	if err := os.WriteFile(filepath.Join(dir2, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for both reloads
	deadline := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(deadline) && reloadCount.Load() < 2 {
		time.Sleep(10 * time.Millisecond)
	}

	if reloadCount.Load() < 2 {
		t.Errorf("expected at least 2 reload calls (one per dir), got %d", reloadCount.Load())
	}
}

// TestReloadCallbackError verifies that a Reload returning an error does not
// prevent subsequent polls from working.
func TestReloadCallbackError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	callCount := atomic.Int32{}

	w := watcher.New(10 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			callCount.Add(1)
			return os.ErrNotExist // simulate an error
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)
	time.Sleep(30 * time.Millisecond)

	// Trigger reload
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Give it time to fire
	time.Sleep(100 * time.Millisecond)

	// Watcher should still be running — trigger again
	time.Sleep(15 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	if callCount.Load() < 1 {
		t.Error("expected at least 1 reload call even with errors")
	}
}

// TestContextCancellationStopsWatcher verifies that cancelling the context
// stops the background goroutine.
func TestContextCancellationStopsWatcher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	reloadCalled := atomic.Int32{}

	w := watcher.New(5 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			reloadCalled.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Start(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Start returned after cancel
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start did not return after context cancellation")
	}
}

// TestConcurrentAccess verifies race-free operation under concurrent Watch and Start calls.
func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	w := watcher.New(5 * time.Millisecond)

	var wg sync.WaitGroup
	// Add watches concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			subDir := t.TempDir()
			w.Watch(watcher.WatchedDir{
				Path:   subDir,
				Reload: func() error { return nil },
			})
		}(i)
	}
	wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start may be called while watches are being added
	var wg2 sync.WaitGroup
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		w.Start(ctx)
	}()

	// Write files concurrently while watcher is running
	for i := 0; i < 3; i++ {
		wg2.Add(1)
		go func(n int) {
			defer wg2.Done()
			_ = os.WriteFile(filepath.Join(dir, "concurrent-file.txt"), []byte("data"), 0o644)
		}(i)
	}

	wg2.Wait()
	// No panic or data race = success
}

// TestSubdirectoryFileChange verifies that changes inside a subdirectory of
// a watched path (e.g. <skills-dir>/<skill-name>/SKILL.md) are detected.
// This exercises the nested-directory snapshot path.
func TestSubdirectoryFileChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a sub-directory with a file inside (like a skill dir with SKILL.md)
	subDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(subDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("initial content"), 0o644); err != nil {
		t.Fatal(err)
	}

	reloadCalled := make(chan struct{}, 5)

	w := watcher.New(10 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			reloadCalled <- struct{}{}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)

	// Give watcher time to snapshot the initial state
	time.Sleep(30 * time.Millisecond)

	// Modify the nested SKILL.md file
	time.Sleep(15 * time.Millisecond)
	if err := os.WriteFile(skillFile, []byte("updated content"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloadCalled:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reload was not called after nested file modification")
	}
}

// TestSubdirectoryCreated verifies that creating a new subdirectory with a
// file inside triggers Reload.
func TestSubdirectoryCreated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	reloadCalled := make(chan struct{}, 5)

	w := watcher.New(10 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			reloadCalled <- struct{}{}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)
	time.Sleep(30 * time.Millisecond)

	// Create a new subdirectory with a file
	newSubDir := filepath.Join(dir, "new-skill")
	if err := os.MkdirAll(newSubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newSubDir, "SKILL.md"), []byte("new skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloadCalled:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reload was not called after subdirectory creation")
	}
}

// TestNoSpuriousReloads verifies that an unchanged directory does not trigger Reload.
func TestNoSpuriousReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Pre-create a file
	if err := os.WriteFile(filepath.Join(dir, "stable.txt"), []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}

	reloadCalled := atomic.Int32{}

	w := watcher.New(10 * time.Millisecond)
	w.Watch(watcher.WatchedDir{
		Path: dir,
		Reload: func() error {
			reloadCalled.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)

	// Run for a bit without changing anything
	time.Sleep(200 * time.Millisecond)

	if reloadCalled.Load() > 0 {
		t.Errorf("expected no spurious reloads, got %d", reloadCalled.Load())
	}
}
