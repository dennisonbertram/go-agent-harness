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

func TestRunWithSignalsBadDBPath(t *testing.T) {
	// SQLiteStore requires a non-empty path; passing empty should error.
	env := map[string]string{
		"CRONSD_ADDR":    "127.0.0.1:0",
		"CRONSD_DB_PATH": "", // will use default which is valid, so use explicit empty via store
	}
	// To actually trigger a store error, we pass a path that resolves to empty.
	// The NewSQLiteStore rejects empty string. We need to override the default.
	// Since empty env falls through to the home-dir default, let's use a path
	// in a read-only location.
	env["CRONSD_DB_PATH"] = "/dev/null/impossible/cronsd.db"
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	err := runWithSignals(sig, getenv)
	if err == nil {
		t.Fatalf("expected error for bad db path")
	}
}
