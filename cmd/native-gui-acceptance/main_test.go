package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	runLifecycle = func(bool) (string, error) { called = true; return "/private/tmp/artifacts", nil }
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

func TestDefaultScenarioManifestIsPreflightedWithoutLaunchingLifecycle(t *testing.T) {
	manifest, err := defaultScenarioManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Nonce) != 32 || len(manifest.Scenarios) != 3 {
		t.Fatalf("scenario manifest = %#v", manifest)
	}
}

func TestOwnedLifecycleBindsOnlyTrustedOwnerInputs(t *testing.T) {
	oldWD, oldTemp, oldPermissions, oldRun := workingDirectory, temporaryDirectory, permissionState, runOwnedOwner
	t.Cleanup(func() {
		workingDirectory, temporaryDirectory, permissionState, runOwnedOwner = oldWD, oldTemp, oldPermissions, oldRun
	})
	workingDirectory = func() (string, error) { return "/trusted/repository", nil }
	temporaryDirectory = func() string { return "/private/tmp" }
	permissionState = func(context.Context) (nativegui.PermissionReport, error) {
		return nativegui.PermissionReport{State: nativegui.PermissionAvailable, Accessibility: true, ScreenRecording: true, Source: "test"}, nil
	}
	var captured nativegui.OwnerConfig
	runOwnedOwner = func(config nativegui.OwnerConfig) error { captured = config; return nil }
	if _, err := ownedLifecycle(true); err != nil {
		t.Fatal(err)
	}
	if captured.RepositoryRoot != "/trusted/repository" || captured.TempParent != "/private/tmp" || captured.ArtifactParent != "/private/tmp" || len(captured.Nonce) != 32 || !captured.ForegroundOptIn || captured.Prepare == nil || captured.Spawn == nil || captured.Probe == nil || captured.Complete == nil {
		t.Fatalf("unexpected owner config: %#v", captured)
	}
}

func TestOwnerHelpersRejectUnknownChildAndCancelledProbe(t *testing.T) {
	if _, err := spawnOwnedChild("", nativegui.DefaultFakeProviderScenarioManifest(strings.Repeat("n", 32)))(context.Background(), nativegui.ChildSpec{Kind: "caller-driver"}); err == nil {
		t.Fatal("unknown caller child kind must fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := probeOwnedDaemon(ctx, nativegui.Attestation{Endpoint: "127.0.0.1:1", DaemonPID: 1}); err == nil {
		t.Fatal("cancelled probe must fail")
	}
}

func TestWriteFailureDiagnosticRetainsCleanupFailure(t *testing.T) {
	root := t.TempDir()
	nonce := strings.Repeat("n", 32)
	err := writeFailureDiagnostic(
		nativegui.Attestation{ArtifactRoot: root, Nonce: nonce},
		nativegui.CoreProof{ConversationID: "conversation-1", RunIDs: []string{"run-1"}},
		nativegui.CoreCleanup{Verified: false, Detail: "owned app did not stop"}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "failure.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{nonce, "conversation-1", "run-1", "cleanup was not verified", "owned app did not stop"} {
		if !strings.Contains(string(raw), marker) {
			t.Fatalf("failure diagnostic lacks %q: %s", marker, raw)
		}
	}
}
