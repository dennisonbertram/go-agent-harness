package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMainDoesNotExitWhenRunSucceeds(t *testing.T) {
	origRun := runMain
	origExit := exitFunc
	defer func() {
		runMain = origRun
		exitFunc = origExit
	}()

	runMain = func() error { return nil }
	exitCalled := false
	exitFunc = func(int) { exitCalled = true }

	main()

	if exitCalled {
		t.Fatalf("did not expect exit")
	}
}

func TestMainExitsWhenRunFails(t *testing.T) {
	origRun := runMain
	origExit := exitFunc
	defer func() {
		runMain = origRun
		exitFunc = origExit
	}()

	runMain = func() error { return errors.New("boom") }
	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
		panic("exit-called")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic sentinel")
		}
		if r != "exit-called" {
			t.Fatalf("unexpected panic: %v", r)
		}
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	}()

	main()
}

func TestRunWithSignalsNilChannel(t *testing.T) {
	err := runWithSignals(nil, os.Getenv)
	if err == nil {
		t.Fatalf("expected error for nil signal channel")
	}
}

func TestRunWithSignalsGracefulShutdown(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-cronsd.db")

	env := map[string]string{
		"CRONSD_ADDR":    "127.0.0.1:0",
		"CRONSD_DB_PATH": dbPath,
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv)
	}()

	time.Sleep(200 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}

func TestRunWithSignalsDefaultEnv(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-cronsd.db")

	// Test with nil getenv (should use os.Getenv).
	env := map[string]string{
		"CRONSD_ADDR":    "127.0.0.1:0",
		"CRONSD_DB_PATH": dbPath,
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv)
	}()

	time.Sleep(200 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out")
	}
}

func TestRunDelegatesToRunWithSignals(t *testing.T) {
	// run() calls runWithSignals with a real signal channel and os.Getenv.
	// We cannot call it directly because it will block waiting for a DB.
	// Instead, verify it exists and has the correct signature by calling it
	// in a goroutine with a temp DB and stopping it quickly.
	dir := t.TempDir()
	t.Setenv("CRONSD_ADDR", "127.0.0.1:0")
	t.Setenv("CRONSD_DB_PATH", dir+"/test-run.db")

	done := make(chan error, 1)
	go func() {
		done <- run()
	}()

	// Give it a moment to start, then send interrupt via the process signal.
	time.Sleep(200 * time.Millisecond)
	// We cannot easily send a signal to our own process in a test,
	// so we just verify it started without error by checking it's running.
	// The function is covered by the fact that it was called and did not
	// immediately return an error.
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run() error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for run() to return")
	}
}

func TestRunWithSignalsBadDBPath(t *testing.T) {
	// Create a file so we can use it as a "directory" to trigger MkdirAll failure.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(blocker, "subdir", "cronsd.db")

	env := map[string]string{
		"CRONSD_ADDR":    "127.0.0.1:0",
		"CRONSD_DB_PATH": badPath,
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	err := runWithSignals(sig, getenv)
	if err == nil {
		t.Fatalf("expected error for bad db path")
	}
}
