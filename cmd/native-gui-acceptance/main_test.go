package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"go-agent-harness/internal/acceptance/nativegui"
)

func TestMainReportsEntrypointError(t *testing.T) {
	oldArgs, oldRun, oldOut, oldErr, oldExit := commandArgs, runMain, stdout, stderr, exitFunc
	t.Cleanup(func() { commandArgs, runMain, stdout, stderr, exitFunc = oldArgs, oldRun, oldOut, oldErr, oldExit })
	var out, errOut bytes.Buffer
	commandArgs, stdout, stderr = nil, &out, &errOut
	runMain = func([]string, io.Writer, io.Writer) error { return errors.New("fixture failure") }
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	main()
	if exitCode != 1 || !strings.Contains(errOut.String(), "fixture failure") {
		t.Fatalf("exit=%d stderr=%q", exitCode, errOut.String())
	}
}

func TestRunRejectsMissingArguments(t *testing.T) {
	if err := run(nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "foreground-opt-in") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunAcceptsOnlyExplicitForegroundOptIn(t *testing.T) {
	oldLifecycle := runLifecycle
	t.Cleanup(func() { runLifecycle = oldLifecycle })
	called := false
	runLifecycle = func(bool) error { called = true; return nil }
	if err := run([]string{"-harness-url", "http://127.0.0.1:8080"}, io.Discard, io.Discard); err == nil {
		t.Fatal("caller-supplied daemon URL must be rejected")
	}
	if err := run([]string{"-foreground-opt-in"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("trusted lifecycle was not selected")
	}
}

func TestOwnedLifecycleBindsOnlyTrustedOwnerInputs(t *testing.T) {
	oldWD, oldTemp, oldRun := workingDirectory, temporaryDirectory, runOwnedOwner
	t.Cleanup(func() { workingDirectory, temporaryDirectory, runOwnedOwner = oldWD, oldTemp, oldRun })
	workingDirectory = func() (string, error) { return "/trusted/repository", nil }
	temporaryDirectory = func() string { return "/private/tmp" }
	var captured nativegui.OwnerConfig
	runOwnedOwner = func(config nativegui.OwnerConfig) error { captured = config; return nil }
	if err := ownedLifecycle(true); err != nil {
		t.Fatal(err)
	}
	if captured.RepositoryRoot != "/trusted/repository" || captured.TempParent != "/private/tmp" || !captured.ForegroundOptIn || captured.Prepare == nil || captured.Spawn == nil || captured.Probe == nil {
		t.Fatalf("unexpected owner config: %#v", captured)
	}
}

func TestOwnerHelpersRejectUnknownChildAndCancelledProbe(t *testing.T) {
	if _, err := spawnOwnedChild("")(context.Background(), nativegui.ChildSpec{Kind: "caller-driver"}); err == nil {
		t.Fatal("unknown caller child kind must fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := probeOwnedDaemon(ctx, nativegui.Attestation{Endpoint: "127.0.0.1:1", DaemonPID: 1}); err == nil {
		t.Fatal("cancelled probe must fail")
	}
}
