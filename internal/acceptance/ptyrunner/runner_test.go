package ptyrunner

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/internal/acceptance/inventory"
)

func TestPTYCommandSetsExplicitGeometryBeforeCLIExec(t *testing.T) {
	cli := "/tmp/harnesscli"
	got := ptyCommandArgsForOS("darwin", cli, "http://127.0.0.1:9999", "run_source")
	want := []string{
		"-q", "/dev/null", "sh", "-c", "stty rows 30 cols 100; exec \"$@\"", "sh",
		cli, "-tui", "-resume=run_source", "-base-url=http://127.0.0.1:9999",
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("PTY arguments = %#v, want %#v", got, want)
	}
}

func TestPTYCommandUsesUtilLinuxCommandFormWithoutInterpolation(t *testing.T) {
	cli := "/tmp/harness cli'; touch /tmp/not-run"
	base := "http://127.0.0.1:9999/?quote='"
	source := "run source'; touch /tmp/not-run"
	got := ptyCommandArgsForOS("linux", cli, base, source)
	if len(got) != 4 || got[0] != "-q" || got[1] != "-c" || got[3] != "/dev/null" {
		t.Fatalf("Linux script arguments = %#v, want -q -c <single command> /dev/null", got)
	}
	want := "'sh' '-c' 'stty rows 30 cols 100; exec \"$@\"' 'sh' '/tmp/harness cli'\"'\"'; touch /tmp/not-run' '-tui' '-resume=run source'\"'\"'; touch /tmp/not-run' '-base-url=http://127.0.0.1:9999/?quote='\"'\"''"
	if got[2] != want {
		t.Fatalf("Linux script child command = %q, want %q", got[2], want)
	}
}

func TestPTYCommandLaunchesSentinelChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script PTY utility is Unix-only")
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skipf("script PTY utility unavailable: %v", err)
	}
	root := t.TempDir()
	sentinel := filepath.Join(root, "sentinel.sh")
	if err := os.WriteFile(sentinel, []byte("#!/bin/sh\nprintf 'sentinel child launched\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "script", ptyCommandArgs(sentinel, "http://127.0.0.1:9999", "run_source")...).CombinedOutput()
	if err != nil {
		t.Fatalf("script did not launch sentinel child: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "sentinel child launched") {
		t.Fatalf("script output = %q, want sentinel child output", output)
	}
}

func TestWaitForCurrentScreenTextFailsPromptlyWhenPTYExits(t *testing.T) {
	exited := make(chan error, 1)
	exited <- nil
	started := time.Now()
	err := waitForCurrentScreenText(context.Background(), filepath.Join(t.TempDir(), "terminal.txt"), "source reply", time.Second, exited)
	if err == nil || !strings.Contains(err.Error(), "PTY harnesscli exited before rendering \"source reply\"") {
		t.Fatalf("early PTY exit error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("early PTY exit took %s, want prompt failure", elapsed)
	}
}

func TestWaitForChildFailsPromptlyWhenRealPTYExitsAfterInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script PTY utility is Unix-only")
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skipf("script PTY utility unavailable: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/runs" {
			t.Fatalf("request path = %q, want /v1/runs", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{}})
	}))
	defer server.Close()

	root := t.TempDir()
	sentinel := filepath.Join(root, "exit-after-input.sh")
	readyPath := filepath.Join(root, "input-ready")
	sentinelScript := fmt.Sprintf("#!/bin/sh\nprintf ready > %s\nIFS= read -r _\nexit 0\n", shellQuote(readyPath))
	if err := os.WriteFile(sentinel, []byte(sentinelScript), 0o700); err != nil {
		t.Fatal(err)
	}
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inR.Close()
	defer inW.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pty := exec.CommandContext(ctx, "script", ptyCommandArgs(sentinel, server.URL, "source")...)
	pty.Stdin = inR
	if err := pty.Start(); err != nil {
		t.Fatalf("start real script PTY: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- pty.Wait() }()
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("PTY exited before input readiness: %v", err)
		case <-ctx.Done():
			t.Fatalf("wait for PTY input readiness: %v", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := io.WriteString(inW, "/resume source post-input\n"); err != nil {
		t.Fatalf("write post-input command: %v", err)
	}
	started := time.Now()
	_, err = waitForChild(ctx, server.Client(), server.URL, "conversation", "source", time.Second, done)
	if err == nil || !strings.Contains(err.Error(), "PTY harnesscli exited before creating child run") {
		t.Fatalf("post-input PTY exit error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("post-input PTY exit took %s, want prompt failure", elapsed)
	}
}

func TestCurrentScreenAppliesANSIAndVTUpdates(t *testing.T) {
	raw := "old text\r\x1b[2J\x1b[Hsource reply\npty continuation reply\x1b[1;1HContinuing run_source"
	screen, err := currentScreen([]byte(raw), 30, 100)
	if err != nil {
		t.Fatalf("currentScreen: %v", err)
	}
	for _, want := range []string{"Continuing run_source", "pty continuation reply"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen = %q, want %q after ANSI/VT updates", screen, want)
		}
	}
	if strings.Contains(screen, "old text") {
		t.Errorf("screen retained erased text: %q", screen)
	}
}

func TestRenderedScreenContainingRetainsLastVisibleFrameBeforeBlankRedraw(t *testing.T) {
	raw := "\x1b[?1049h\x1b[Hpty continuation reply\x1b[H\x1b[2J\x1b[H"
	screen, err := renderedScreenContaining([]byte(raw), 30, 100, "pty continuation reply")
	if err != nil {
		t.Fatalf("renderedScreenContaining: %v", err)
	}
	if !strings.Contains(screen, "pty continuation reply") {
		t.Fatalf("visible frame = %q, want continuation reply", screen)
	}
}

func TestRenderedScreenContainingPreservesUTF8WideAndCombiningGlyphsBeforeBlankRedraw(t *testing.T) {
	// This is the shape captured from the real Bubble Tea PTY: an alternate
	// screen frame, a Unicode status glyph, a wide CJK character, a combining
	// accent, then a default ED (CSI J) cleanup and a final blank redraw.
	// Cursor-column addressing is in terminal cells, not UTF-8 bytes or runes.
	raw := "\x1b[?1049h\x1b[H✓ 界e\u0301X\x1b[1;7H!\x1b[J\x1b[H\x1b[2J\x1b[H"
	screen, err := renderedScreenContaining([]byte(raw), 3, 12, "✓ 界e\u0301X!")
	if err != nil {
		t.Fatalf("renderedScreenContaining: %v", err)
	}
	if !strings.Contains(screen, "✓ 界e\u0301X!") {
		t.Fatalf("visible Unicode frame = %q, want exact glyphs and cell geometry", screen)
	}
	if strings.Contains(screen, "â") || strings.Contains(screen, "ç") {
		t.Fatalf("visible Unicode frame contains mojibake: %q", screen)
	}
}

func TestRealPTYResumeAndContinue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script PTY utility is Unix-only")
	}
	repo := repoRoot(t)
	bin := t.TempDir()
	daemon, cli := filepath.Join(bin, "harnessd"), filepath.Join(bin, "harnesscli")
	for _, target := range []struct{ out, pkg string }{{daemon, "./cmd/harnessd"}, {cli, "./cmd/harnesscli"}} {
		cmd := exec.Command("go", "build", "-o", target.out, target.pkg)
		cmd.Dir = repo
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", target.pkg, err, output)
		}
	}
	compiled, err := inventory.Compile(inventory.Input{Commands: tui.NewCommandRegistry().All()})
	if err != nil {
		t.Fatalf("compile live TUI command inventory: %v", err)
	}
	cases := []struct {
		command string
		caseDef inventory.Case
	}{
		{command: "resume", caseDef: continuationCase("tui_command:resume/canonical", "/resume")},
		{command: "continue", caseDef: continuationCase("tui_command:resume/alias:continue", "/continue")},
	}
	records := make([]inventory.Evidence, 0, len(cases))
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			artifactRoot := testArtifactRoot(t, tc.command)
			evidence, err := RunEvidence(ctx, Config{Daemon: daemon, CLI: cli, SourceRoot: repo, ArtifactRoot: artifactRoot, Command: tc.command, Timeout: 20 * time.Second}, compiled, tc.caseDef)
			if err != nil {
				t.Fatalf("%v (artifacts retained at %s)", err, artifactRoot)
			}
			if evidence.Outcome != inventory.Pass || evidence.InventoryHash != compiled.Hash {
				t.Fatalf("actual hash-bound pass evidence = %#v", evidence)
			}
			if evidence.InvocationID != tc.caseDef.InvocationID || evidence.RunID == "" || evidence.ConversationID == "" || len(evidence.EventIDs) == 0 {
				t.Fatalf("missing invocation correlation: %#v", evidence)
			}
			for _, artifact := range evidence.Artifacts {
				if !filepath.IsAbs(artifact.Path) || artifact.Redacted == nil || !*artifact.Redacted {
					t.Errorf("artifact is not canonical/redacted: %#v", artifact)
				}
				if err := verifyArtifactRefs([]inventory.ArtifactRef{artifact}); err != nil {
					t.Fatalf("artifact digest verification: %v", err)
				}
			}
			records = append(records, evidence)
		})
	}
	report, err := inventory.RenderResultMarkdown(compiled, []inventory.Case{cases[0].caseDef, cases[1].caseDef}, records)
	if err != nil {
		t.Fatalf("render actual PTY evidence: %v", err)
	}
	for _, invocationID := range []string{"tui_command:resume/canonical", "tui_command:resume/alias:continue"} {
		if !strings.Contains(report, invocationID) || !strings.Contains(report, "| pass |") {
			t.Fatalf("actual PASS not rendered for %s:\n%s", invocationID, report)
		}
	}
	pending, err := inventory.RenderResultMarkdown(compiled, []inventory.Case{cases[0].caseDef, cases[1].caseDef}, nil)
	if err != nil || strings.Count(pending, "| pending |") < 2 {
		t.Fatalf("planned cases received credit: err=%v report=%s", err, pending)
	}
}

func TestPTYEvidenceRejectsUnknownInvocationAndArtifactDrift(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Commands: tui.NewCommandRegistry().All()})
	if err != nil {
		t.Fatal(err)
	}
	bad := continuationCase("tui_command:resume/alias:missing", "/missing")
	if _, err := invocationForCase(compiled, bad); err == nil || !strings.Contains(err.Error(), "unknown invocation") {
		t.Fatalf("unknown invocation accepted: %v", err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "screen.txt")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	refs, err := checkedArtifactRefs(root, map[string]string{
		"terminal": path, "screen": path, "keystrokes": path, "sse": path, "api_store": path,
	})
	if err != nil {
		t.Fatalf("checked artifacts: %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyArtifactRefs(refs); err == nil || !strings.Contains(err.Error(), "digest changed") {
		t.Fatalf("changed artifact accepted: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := verifyArtifactRefs(refs); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing artifact accepted: %v", err)
	}
}

func TestPTYEvidenceIsBoundToTheCompiledInventoryHash(t *testing.T) {
	compiled, err := inventory.Compile(inventory.Input{Commands: tui.NewCommandRegistry().All()})
	if err != nil {
		t.Fatal(err)
	}
	caseDef := continuationCase("tui_command:resume/canonical", "/resume")
	redacted := true
	now := time.Now().UTC()
	evidence := inventory.Evidence{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash, ItemID: caseDef.ItemID, InvocationID: caseDef.InvocationID,
		Surface: inventory.SurfaceTUI, EvidenceClass: inventory.EvidenceClassConversation, Outcome: inventory.Pass,
		OrderedActions: caseDef.OrderedActions, RunID: "child-run", ConversationID: "conversation", EventIDs: []string{"event-1"},
		ExpectedPostconditions: caseDef.ExpectedPostconditions,
		ObservedPostconditions: []inventory.ProbeObservation{
			{Kind: inventory.PostconditionRenderedScreen, Probe: "pty-screen", AssertionID: "continuation-rendered", Value: "reply", Verified: true},
			{Kind: inventory.PostconditionConversationState, Probe: "api-store", AssertionID: "same-conversation", Value: "conversation", Verified: true},
		},
		Artifacts: []inventory.ArtifactRef{{Kind: inventory.ArtifactTerminalCapture, Path: "/private/tmp/pty-proof.txt", Digest: "sha256:" + strings.Repeat("0", 64), Redacted: &redacted}},
		Cleanup:   inventory.CleanupEvidence{Verified: true, Detail: "verified test cleanup"}, Timing: inventory.Timing{StartedAt: now, FinishedAt: now.Add(time.Nanosecond)},
	}
	if err := inventory.ValidateEvidence(compiled, caseDef, evidence); err != nil {
		t.Fatalf("hash-bound PTY evidence rejected: %v", err)
	}
	evidence.InventoryHash = "sha256:" + strings.Repeat("1", 64)
	if err := inventory.ValidateEvidence(compiled, caseDef, evidence); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("inventory-drifted PTY evidence accepted: %v", err)
	}
}

func continuationCase(invocationID, input string) inventory.Case {
	return inventory.Case{
		ItemID: "tui_command:resume", InvocationID: invocationID, Surfaces: []inventory.Surface{inventory.SurfaceTUI}, EvidenceClass: inventory.EvidenceClassConversation,
		OrderedActions: []inventory.Action{{Kind: "pty_input", Value: input + " <source-run-id> pty continuation prompt"}},
		ExpectedPostconditions: []inventory.Postcondition{
			{Kind: inventory.PostconditionRenderedScreen, Probe: "pty-screen", AssertionID: "continuation-rendered", Description: "the live PTY renders the continuation reply"},
			{Kind: inventory.PostconditionConversationState, Probe: "api-store", AssertionID: "same-conversation", Description: "the child run is stored with the source conversation"},
		},
		Cleanup: "stop isolated daemon and remove the verified artifact bundle in t.Cleanup",
	}
}

func TestPTYArtifactRootsUseConfiguredPortableTempDir(t *testing.T) {
	portableRoot := t.TempDir()
	t.Setenv("TMPDIR", portableRoot)

	artifactRoot := testArtifactRoot(t, "portable")
	if parent := filepath.Dir(artifactRoot); parent != portableRoot {
		t.Fatalf("artifact root parent = %q, want configured portable temp dir %q", parent, portableRoot)
	}

	sourceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "artifact.txt"), []byte("evidence"), 0o600); err != nil {
		t.Fatalf("write source artifact: %v", err)
	}
	failureBundle, err := copyFailureBundle(sourceRoot)
	if err != nil {
		t.Fatalf("copy failure bundle: %v", err)
	}
	defer os.RemoveAll(failureBundle)
	if parent := filepath.Dir(failureBundle); parent != portableRoot {
		t.Fatalf("failure bundle parent = %q, want configured portable temp dir %q", parent, portableRoot)
	}
}

func testArtifactRoot(t *testing.T, command string) string {
	t.Helper()
	root, err := os.MkdirTemp(os.TempDir(), "issue-1204-pty-"+command+"-")
	if err != nil {
		t.Fatalf("create artifact root: %v", err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			failure, err := copyFailureBundle(root)
			if err != nil {
				t.Errorf("preserve failed PTY evidence: %v", err)
			} else {
				t.Logf("retained failed PTY evidence at %s", failure)
			}
		}
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove PTY artifact root: %v", err)
			return
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Errorf("PTY artifact root cleanup was not verified: %v", err)
		}
	})
	return root
}

func copyFailureBundle(root string) (string, error) {
	destination, err := os.MkdirTemp(os.TempDir(), "issue-1204-pty-failure-")
	if err != nil {
		return "", err
	}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return os.ErrPermission
		}
		if rel == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return os.ErrInvalid
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(destination, rel), 0o700)
		}
		if !info.Mode().IsRegular() {
			return os.ErrInvalid
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	}); err != nil {
		_ = os.RemoveAll(destination)
		return "", err
	}
	return destination, nil
}

func TestCopyFailureBundleCopiesOnlyRegularEvidence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifact.txt")
	data := []byte("safe evidence")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	bundle, err := copyFailureBundle(root)
	if err != nil {
		t.Fatalf("copy failure bundle: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(bundle) })
	copied, err := os.ReadFile(filepath.Join(bundle, "artifact.txt"))
	copiedSum, dataSum := sha256.Sum256(copied), sha256.Sum256(data)
	if err != nil || copiedSum != dataSum {
		t.Fatalf("failure bundle copy is not digest-identical: %v", err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../"))
}
