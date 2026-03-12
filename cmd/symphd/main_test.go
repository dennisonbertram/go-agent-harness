package main

import (
	"errors"
	"testing"
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
		t.Fatal("did not expect exit when run succeeds")
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
	exitFunc = func(code int) { exitCode = code }

	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}

// TestRun_InvalidConfigPath exercises run with a non-existent config path,
// which causes an early return without starting any network services.
func TestRun_InvalidConfigPath(t *testing.T) {
	err := run([]string{"-config", "/nonexistent/path/to/config.yaml"})
	if err == nil {
		t.Fatal("expected error for non-existent config path, got nil")
	}
}

// TestRun_InvalidFlag verifies that run returns an error for unknown flags.
func TestRun_InvalidFlag(t *testing.T) {
	err := run([]string{"-unknown-flag-xyz"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
}
