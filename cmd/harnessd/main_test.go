package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	openai "go-agent-harness/internal/provider/openai"
)

type noopProvider struct{}

func (n *noopProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	return harness.CompletionResult{Content: "ok"}, nil
}

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

func TestGetenvOrDefault(t *testing.T) {
	t.Setenv("HARNESS_TEST_VALUE", "x")
	if got := getenvOrDefault("HARNESS_TEST_VALUE", "fallback"); got != "x" {
		t.Fatalf("expected x, got %q", got)
	}
	if got := getenvOrDefault("HARNESS_TEST_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestGetenvIntOrDefault(t *testing.T) {
	t.Setenv("HARNESS_INT", "17")
	if got := getenvIntOrDefault("HARNESS_INT", 9); got != 17 {
		t.Fatalf("expected 17, got %d", got)
	}
	t.Setenv("HARNESS_INT", "bad")
	if got := getenvIntOrDefault("HARNESS_INT", 9); got != 9 {
		t.Fatalf("expected fallback 9, got %d", got)
	}
	os.Unsetenv("HARNESS_INT")
	if got := getenvIntOrDefault("HARNESS_INT", 9); got != 9 {
		t.Fatalf("expected fallback 9, got %d", got)
	}
}

func TestAskUserTimeoutEnvParsing(t *testing.T) {
	t.Setenv("HARNESS_ASK_USER_TIMEOUT_SECONDS", "45")
	if got := getenvIntOrDefault("HARNESS_ASK_USER_TIMEOUT_SECONDS", 300); got != 45 {
		t.Fatalf("expected 45, got %d", got)
	}

	t.Setenv("HARNESS_ASK_USER_TIMEOUT_SECONDS", "bad")
	if got := getenvIntOrDefault("HARNESS_ASK_USER_TIMEOUT_SECONDS", 300); got != 300 {
		t.Fatalf("expected fallback 300, got %d", got)
	}
}

func TestGetenvToolApprovalModeOrDefault(t *testing.T) {
	t.Setenv("HARNESS_TOOL_APPROVAL_MODE", "permissions")
	if got := getenvToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModeFullAuto); got != harness.ToolApprovalModePermissions {
		t.Fatalf("expected permissions, got %q", got)
	}
	t.Setenv("HARNESS_TOOL_APPROVAL_MODE", "FULL_AUTO")
	if got := getenvToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModePermissions); got != harness.ToolApprovalModeFullAuto {
		t.Fatalf("expected full_auto, got %q", got)
	}
	t.Setenv("HARNESS_TOOL_APPROVAL_MODE", "bad")
	if got := getenvToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModeFullAuto); got != harness.ToolApprovalModeFullAuto {
		t.Fatalf("expected fallback full_auto, got %q", got)
	}
}

func TestRunDelegatesToRunWithSignals(t *testing.T) {
	orig := runWithSignalsFunc
	defer func() { runWithSignalsFunc = orig }()

	called := false
	runWithSignalsFunc = func(sig <-chan os.Signal, getenv func(string) string, newProvider providerFactory) error {
		called = true
		if sig == nil {
			t.Fatalf("expected non-nil signal channel")
		}
		if getenv == nil {
			t.Fatalf("expected getenv callback")
		}
		if newProvider == nil {
			t.Fatalf("expected provider callback")
		}
		return nil
	}

	if err := run(); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected runWithSignalsFunc to be called")
	}
}

func TestRunWithSignalsMissingAPIKey(t *testing.T) {
	err := runWithSignals(make(chan os.Signal, 1), func(string) string { return "" }, func(openai.Config) (harness.Provider, error) {
		return &noopProvider{}, nil
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithSignalsProviderFailure(t *testing.T) {
	env := map[string]string{
		"OPENAI_API_KEY": "x",
		"HARNESS_ADDR":   "127.0.0.1:0",
	}
	getenv := func(key string) string { return env[key] }

	err := runWithSignals(make(chan os.Signal, 1), getenv, func(openai.Config) (harness.Provider, error) {
		return nil, errors.New("provider init failed")
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "provider init failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithSignalsGracefulShutdown(t *testing.T) {
	env := map[string]string{
		"OPENAI_API_KEY": "x",
		"HARNESS_ADDR":   "127.0.0.1:0",
	}
	getenv := func(key string) string { return env[key] }
	sig := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() {
		done <- runWithSignals(sig, getenv, func(openai.Config) (harness.Provider, error) {
			return &noopProvider{}, nil
		})
	}()

	time.Sleep(100 * time.Millisecond)
	sig <- os.Interrupt

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithSignals returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for graceful shutdown")
	}
}
