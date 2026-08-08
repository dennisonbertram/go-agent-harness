package ptyrunner

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"go-agent-harness/cmd/harnesscli/tui"
	"go-agent-harness/internal/acceptance/inventory"
	"go-agent-harness/internal/acceptance/scheduledlifecycle"
)

func TestRunAttachedRejectsInvalidLifecycleBeforeCLIStart(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cli-started")
	cli := filepath.Join(t.TempDir(), "cli-marker.sh")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\nprintf started > \"$MARKER\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARKER", marker)
	_, err := RunAttached(context.Background(), AttachedConfig{
		CLI:          cli,
		ArtifactRoot: t.TempDir(),
		Attachment: scheduledlifecycle.PTYAttachment{
			Workspace: "/owned/workspace",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "attachment") {
		t.Fatalf("RunAttached error = %v, want invalid attachment", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("invalid attachment launched CLI: stat marker = %v", statErr)
	}
}

func TestAttachedConfigRequiresLifecycleSourceSHA(t *testing.T) {
	err := validateAttachedConfig(AttachedConfig{
		CLI:          "/tmp/harnesscli",
		ArtifactRoot: "/tmp/artifacts",
		Attachment: scheduledlifecycle.PTYAttachment{
			BaseURL: "http://127.0.0.1:12345", Workspace: "/tmp/workspace",
			ConversationDB: "/tmp/conversations.db", RunDB: "/tmp/runs.db",
			CronDB: "/tmp/cron.db", CallbackDB: "/tmp/callbacks.db",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "source SHA") {
		t.Fatalf("validateAttachedConfig error = %v, want missing source SHA", err)
	}
}

func TestRunAttachedSealsTwoMessageFramesAgainstOneLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY acceptance is Unix-only")
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	artifactRoot := testArtifactRoot(t, "attached-lifecycle")
	turnsPath := filepath.Join(artifactRoot, "turns.json")
	if err := os.WriteFile(turnsPath, []byte(`[{"content":"ATTACHED_FIRST_REPLY","deltas":[{"content":"ATTACHED_FIRST_REPLY"}]},{"content":"ATTACHED_SECOND_REPLY","deltas":[{"content":"ATTACHED_SECOND_REPLY"}]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	lifecycle, err := scheduledlifecycle.Start(ctx, scheduledlifecycle.Config{
		Command: daemon, SourceRoot: repo, ArtifactRoot: artifactRoot, Timeout: 15 * time.Second,
		Environment: []string{
			"HARNESS_PROVIDER=fake", "HARNESS_FAKE_TURNS=" + turnsPath,
			"HARNESS_AUTH_DISABLED=true", "HARNESS_PROMPTS_DIR=" + filepath.Join(repo, "prompts"),
			"HOME=" + filepath.Join(artifactRoot, "home"),
		},
	})
	if err != nil {
		t.Fatalf("start owned lifecycle: %v", err)
	}
	t.Cleanup(func() { _ = lifecycle.Close() })

	session, err := RunAttached(ctx, AttachedConfig{Attachment: lifecycle.PTY(), CLI: cli, ArtifactRoot: filepath.Join(artifactRoot, "attached"), Timeout: 15 * time.Second})
	if err != nil {
		t.Fatalf("attach PTY: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := &http.Client{Timeout: 5 * time.Second}
	first, err := session.BeginAction("first_prompt", "attached first prompt\r")
	if err != nil {
		t.Fatal(err)
	}
	firstRun, conversationID, err := waitForCompletedPromptRun(ctx, client, lifecycle.PublicURL, "attached first prompt", "", 15*time.Second, session.ptyDone)
	if err != nil {
		t.Fatal(err)
	}
	firstFrame, err := session.SealAction(ctx, first, AttachedIdentity{ConversationID: conversationID, RunID: firstRun}, "ATTACHED_FIRST_REPLY")
	if err != nil {
		t.Fatal(err)
	}
	second, err := session.BeginAction("manual_follow_up", "attached second prompt\r")
	if err != nil {
		t.Fatal(err)
	}
	secondRun, secondConversationID, err := waitForCompletedPromptRun(ctx, client, lifecycle.PublicURL, "attached second prompt", firstRun, 15*time.Second, session.ptyDone)
	if err != nil {
		t.Fatal(err)
	}
	if secondConversationID != conversationID {
		t.Fatalf("manual follow-up conversation = %q, want %q", secondConversationID, conversationID)
	}
	secondFrame, err := session.SealAction(ctx, second, AttachedIdentity{ConversationID: conversationID, RunID: secondRun}, "ATTACHED_SECOND_REPLY")
	if err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"first": firstFrame, "second": secondFrame} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s frame: %v", name, err)
		}
		var frame freshFrameRecord
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("decode %s frame: %v", name, err)
		}
		if frame.ConversationID != conversationID || frame.RunID == "" {
			t.Fatalf("%s frame identity = %#v, want conversation %q and run", name, frame, conversationID)
		}
	}
	result := session.Result()
	identity, err := os.ReadFile(result.IdentityPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{lifecycle.Provenance.SourceSHA, `"Rows": 30`, `"Cols": 100`, "first_prompt", "manual_follow_up"} {
		if !strings.Contains(string(identity), expected) {
			t.Fatalf("identity artifact missing %q: %s", expected, identity)
		}
	}
}

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

func TestPTYFreshCommandSetsExplicitGeometryBeforeCLIExec(t *testing.T) {
	cli := "/tmp/harnesscli"
	base := "http://127.0.0.1:9999"
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			got := ptyFreshCommandArgsForOS(goos, cli, base)
			joined := strings.Join(got, "\x00")
			if !strings.Contains(joined, "stty rows 30 cols 100; exec \"$@\"") {
				t.Fatalf("fresh PTY arguments = %#v, want explicit geometry before CLI exec", got)
			}
			if !strings.Contains(joined, cli) || !strings.Contains(joined, "-tui") || !strings.Contains(joined, "-base-url="+base) {
				t.Fatalf("fresh PTY arguments = %#v, want CLI TUI invocation", got)
			}
		})
	}
}

func TestPTYFreshCommandUsesPlatformLauncher(t *testing.T) {
	if got := ptyFreshCommandArgs("/tmp/harnesscli", "http://127.0.0.1:9999"); len(got) == 0 || !strings.Contains(strings.Join(got, "\x00"), "stty rows 30 cols 100") {
		t.Fatalf("fresh launcher = %#v", got)
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
	// The full race matrix can compile and launch neighboring real-PTY cases;
	// retain an execution bound without treating host scheduling contention as
	// a sentinel-launch failure.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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

func TestWaitForCurrentScreenWithoutTextObservesDismissal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screen.txt")
	if err := os.WriteFile(path, []byte("visible reply"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := waitForCurrentScreenWithoutText(context.Background(), path, "Search:", time.Second, make(chan error)); err != nil {
		t.Fatal(err)
	}
}

func TestFreshFrameCollectorSealsMonotonicImmutablePrefixes(t *testing.T) {
	root := t.TempDir()
	collector := freshFrameCollector{artifactRoot: root}
	first := []byte("one")
	firstScreen, firstFrame, err := collector.seal(first, "FIRST_REPLY", freshFrameSpec{Sequence: 1, Action: "first_prompt", Input: "first\r", Expected: "FIRST_REPLY", ConversationID: "conv", RunID: "run-1", Artifact: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := collector.seal(first, "again", freshFrameSpec{Sequence: 2, Action: "search", Input: "search\r", Artifact: "duplicate"}); err == nil || !strings.Contains(err.Error(), "no new typescript bytes") {
		t.Fatalf("non-growing prefix seal error = %v, want rejected", err)
	}
	second := append(append([]byte(nil), first...), []byte("two")...)
	_, secondFrame, err := collector.seal(second, "SECOND_REPLY", freshFrameSpec{Sequence: 2, Action: "second_prompt", Input: "second\r", Expected: "SECOND_REPLY", ConversationID: "conv", RunID: "run-2", Artifact: "second"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{firstScreen, firstFrame, secondFrame} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("sealed artifact %s: %v", path, err)
		}
	}
	raw, err := os.ReadFile(firstFrame)
	if err != nil {
		t.Fatal(err)
	}
	var record freshFrameRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record.Start != 0 || record.End != len(first) || record.InputSHA256 != digestBytes([]byte("first\r")) || record.PrefixSHA256 != digestBytes(first) {
		t.Fatalf("first seal = %#v", record)
	}
}

func TestVTExactWidthCRLFAdvancesOneRow(t *testing.T) {
	raw := []byte(strings.Repeat("x", 100) + "\r\nFIRST_REPLY")
	screen, err := currentScreen(raw, 3, 100)
	if err != nil || !strings.Contains(strings.Split(screen, "\n")[1], "FIRST_REPLY") {
		t.Fatalf("exact-width CRLF screen = %q, err=%v", screen, err)
	}
}

func TestVTWrapPendingTransitions(t *testing.T) {
	fill := strings.Repeat("x", 100)
	for _, tc := range []struct {
		name, transition string
		wantRow          int
	}{
		{"cursor-up", "\x1b[A", 0}, {"cursor-down", "\x1b[B", 1}, {"cursor-right", "\x1b[C", 0}, {"cursor-left", "\x1b[D", 0}, {"column", "\x1b[1G", 0}, {"cup", "\x1b[1;1H", 0}, {"cup-f", "\x1b[1;1f", 0}, {"clamped-cursor", "\x1b[999C", 0}, {"clamped-cup", "\x1b[2;999H", 1},
		{"erase-display", "\x1b[0J", 0}, {"erase-line", "\x1b[0K", 0}, {"backspace", "\b", 0},
		{"tab-preserves", "\t", 1}, {"sgr-preserves", "\x1b[31m", 1}, {"combining-preserves", "\u0301", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			screen, err := currentScreen([]byte(fill+tc.transition+"Z"), 3, 100)
			if err != nil || !strings.Contains(strings.Split(screen, "\n")[tc.wantRow], "Z") {
				t.Fatalf("screen=%q err=%v", screen, err)
			}
		})
	}
}

func TestVTWrapPendingJ3AndAlternateBuffers(t *testing.T) {
	fill := strings.Repeat("x", 100)
	before, err := currentScreen([]byte(fill+"\x1b[3J"), 2, 100)
	if err != nil || !strings.Contains(strings.Split(before, "\n")[0], "x") {
		t.Fatalf("J3 mutated grid: %q %v", before, err)
	}
	primary, err := currentScreen([]byte(fill+"\x1b[?1049hA\x1b[?1049lB"), 3, 100)
	if err != nil || !strings.Contains(strings.Split(primary, "\n")[1], "B") {
		t.Fatalf("1049 primary pending not restored: %q %v", primary, err)
	}
	alt, err := currentScreen([]byte("\x1b[?47hA\x1b[?47l\x1b[?47hB"), 2, 100)
	if err != nil || !strings.Contains(alt, "A") || !strings.Contains(alt, "B") {
		t.Fatalf("47 state not independent: %q %v", alt, err)
	}
}

func TestFreshFailureShapeRendersFirstReply(t *testing.T) {
	raw := append([]byte("\x1b[?1049h"+strings.Repeat("x", 100)+"\r\n"), []byte("fresh first prompt\r\nFIRST_REPLY\x1b[28;H")...)
	if _, err := renderedScreenContaining(raw, 30, 100, "FIRST_REPLY"); err != nil {
		t.Fatalf("retained failure shape lost reply: %v", err)
	}
}

func TestFreshCollectorActionDeadlineBeatsParentContext(t *testing.T) {
	collector := freshFrameCollector{raw: []byte("not the reply"), updates: make(chan struct{}), artifactRoot: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started := time.Now()
	_, _, err := collector.waitAndSealText(ctx, make(chan error), 25*time.Millisecond, freshFrameSpec{Sequence: 1, Expected: "FIRST_REPLY", Artifact: "deadline"})
	if err == nil || !strings.Contains(err.Error(), "action 1") || time.Since(started) > 250*time.Millisecond {
		t.Fatalf("action deadline error = %v after %s", err, time.Since(started))
	}
}

func TestFreshCollectorReadsGrowingPrefix(t *testing.T) {
	collector := freshFrameCollector{raw: []byte("abc")}
	if got, err := collector.readGrowingPrefix(); err != nil || string(got) != "abc" {
		t.Fatalf("growing prefix = %q, %v", got, err)
	}
	collector.lastEnd = 3
	if _, err := collector.readGrowingPrefix(); err == nil {
		t.Fatal("unchanged prefix accepted")
	}
}

func TestRenderedScreenContainingAfterRejectsHistoricalExpectedText(t *testing.T) {
	raw := []byte("\x1b[HOLD_LABEL")
	barrier := len(raw)
	raw = append(raw, []byte("\x1b[H\x1b[2J\x1b[Hunrelated redraw")...)
	if !renderedScreenAbsentAfter(raw, barrier, "OLD_LABEL") {
		t.Fatal("post-barrier redraw never cleared the historical label")
	}
}

func TestFreshFrameRecordIncludesActionAndMatchProvenance(t *testing.T) {
	collector := freshFrameCollector{artifactRoot: t.TempDir(), raw: []byte("baseline")}
	barrier := collector.beginAction()
	raw := append([]byte("baseline"), []byte("\x1b[HNEW_LABEL")...)
	_, frame, err := collector.sealAt(raw, "NEW_LABEL", freshFrameSpec{Sequence: 1, Action: "post_barrier", Input: "x", Expected: "NEW_LABEL", Artifact: "post", Barrier: barrier}, len(raw), 7)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(frame)
	if err != nil {
		t.Fatal(err)
	}
	var record freshFrameRecord
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	if record.ActionStartOffset != len("baseline") || record.MatchEnd != len(raw) || record.MatchVersion != 7 {
		t.Fatalf("action provenance = %#v", record)
	}
}

func TestPostBarrierMatcherUsesOnlySemanticVTBoundariesAndCompleteComposite(t *testing.T) {
	raw := []byte("prefix\x1b[Hone\x1b[Htwo\x1b[Hone two")
	start := len("prefix")
	screen, end, err := renderedScreenContainingAfter(raw, start, ptyRows, ptyCols, []string{"one", "two"}, false)
	if err != nil || end != len(raw) || !strings.Contains(screen, "one two") {
		t.Fatalf("composite candidate = %q end=%d err=%v", screen, end, err)
	}
	boundaries := semanticVTBoundaries([]byte("x\x1b[Hy\x1b[?1049lz"), 1)
	if len(boundaries) != 3 || boundaries[2] != len("x\x1b[Hy\x1b[?1049lz") {
		t.Fatalf("semantic boundaries = %#v", boundaries)
	}
}

func TestPostBarrierRepeatedLabelRequiresDismissalThenReappearance(t *testing.T) {
	raw := []byte("\x1b[HREPEATED")
	start := len(raw)
	raw = append(raw, []byte("\x1b[HREPEATED")...)
	if _, _, err := renderedScreenContainingAfter(raw, start, ptyRows, ptyCols, []string{"REPEATED"}, true); err == nil {
		t.Fatal("repeated label passed without false-to-true transition")
	}
	raw = append(raw, []byte("\x1b[2J\x1b[H\x1b[HREPEATED")...)
	if _, _, err := renderedScreenContainingAfter(raw, start, ptyRows, ptyCols, []string{"REPEATED"}, true); err != nil {
		t.Fatalf("repeated label did not pass after dismissal/reappearance: %v", err)
	}
}

func TestPostBarrierDismissalRequiresVisibleBaselineAndFalseTransition(t *testing.T) {
	baseline := []byte("\x1b[Hoverlay")
	start := len(baseline)
	if _, _, err := renderedScreenAbsentAfterCandidate(baseline, start, ptyRows, ptyCols, []string{"overlay"}); err == nil {
		t.Fatal("dismissal passed without post-barrier candidate")
	}
	raw := append(baseline, []byte("\x1b[2J\x1b[H")...)
	if _, _, err := renderedScreenAbsentAfterCandidate(raw, start, ptyRows, ptyCols, []string{"overlay"}); err != nil {
		t.Fatalf("dismissal candidate = %v", err)
	}
}

func TestFreshCollectorRetainsFinalBytesAndNormalizesWrappedEIOAsEOF(t *testing.T) {
	terminal, err := os.CreateTemp(t.TempDir(), "terminal-*")
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	collector := &freshFrameCollector{terminal: terminal, updates: make(chan struct{}), read: func(buf []byte) (int, error) {
		called++
		if called == 1 {
			copy(buf, "final bytes")
			return len("final bytes"), syscall.EIO
		}
		return 0, io.EOF
	}}
	collector.collect()
	raw, _, eof, readErr := collector.snapshot()
	if !eof || readErr != nil || string(raw) != "final bytes" {
		t.Fatalf("collector state raw=%q eof=%v err=%v", raw, eof, readErr)
	}
	if err := collector.waitEOF(context.Background()); err != nil {
		t.Fatalf("waitEOF: %v", err)
	}
	if stored, err := os.ReadFile(terminal.Name()); err != nil || string(stored) != "final bytes" {
		t.Fatalf("terminal=%q err=%v", stored, err)
	}
}

func TestFreshCollectorKeepsArbitraryReadErrorFatal(t *testing.T) {
	terminal, err := os.CreateTemp(t.TempDir(), "terminal-*")
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("master broke")
	collector := &freshFrameCollector{terminal: terminal, updates: make(chan struct{}), read: func([]byte) (int, error) { return 0, want }}
	collector.collect()
	if err := collector.waitEOF(context.Background()); !errors.Is(err, want) {
		t.Fatalf("waitEOF error = %v, want %v", err, want)
	}
}

func TestFreshCollectorEIOBeforeQualifyingFrameCannotSealPendingAction(t *testing.T) {
	collector := freshFrameCollector{raw: []byte("not the reply"), eof: true, updates: make(chan struct{}), artifactRoot: t.TempDir()}
	_, _, err := collector.waitAndSealText(context.Background(), make(chan error), 20*time.Millisecond, freshFrameSpec{Sequence: 1, Expected: "FIRST_REPLY", Artifact: "pending", Barrier: collector.beginAction()})
	if err == nil || strings.Contains(err.Error(), "sealed") {
		t.Fatalf("pending action incorrectly sealed after EIO/EOF: %v", err)
	}
}

func TestCloseMasterBeforeProcessCleanup(t *testing.T) {
	order := make([]string, 0, 2)
	master := closeFunc(func() error {
		order = append(order, "master")
		return nil
	})
	closeMasterBeforeProcessCleanup(master, func() {
		order = append(order, "process")
	})
	if got, want := strings.Join(order, ","), "master,process"; got != want {
		t.Fatalf("cleanup order = %q, want %q", got, want)
	}
}

type closeFunc func() error

func (f closeFunc) Close() error { return f() }

func TestLinuxPTYSlaveCloseDrainsAsCleanEOF(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux PTY EIO lifecycle")
	}
	cmd := exec.Command("sh", "-c", "printf final-linux-pty")
	master, err := pty.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := master.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close PTY master: %v", err)
		}
	}()
	collector, err := startFreshMasterCollector(master, filepath.Join(t.TempDir(), "terminal.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := collector.waitEOF(context.Background()); err != nil {
		t.Fatalf("Linux PTY EOF = %v", err)
	}
	raw, _, _, _ := collector.snapshot()
	if !strings.Contains(string(raw), "final-linux-pty") {
		t.Fatalf("final bytes = %q", raw)
	}
}

func TestCaptureScreenContainingWritesRenderedArtifact(t *testing.T) {
	root := t.TempDir()
	terminal := filepath.Join(root, "terminal.txt")
	if err := os.WriteFile(terminal, []byte("FIRST_REPLY"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := captureScreenContaining(terminal, root, "screen.txt", "FIRST_REPLY")
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(path); err != nil || !strings.Contains(string(raw), "FIRST_REPLY") {
		t.Fatalf("artifact = %q, %v", raw, err)
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

func TestRenderedScreenContainingRetainsFrameBeforeAlternateBufferExit(t *testing.T) {
	// Bubble Tea may write its final incremental transcript without another CUP
	// home, then leave the alternate buffer. The evidence reader must snapshot
	// immediately before that mode transition rather than returning the blank
	// primary screen after it.
	raw := "\x1b[?1049h\x1b[H\r\n⏺ SECOND_REPLY\x1b[K\r\n\x1b[?1049l"
	screen, err := renderedScreenContaining([]byte(raw), ptyRows, ptyCols, "SECOND_REPLY")
	if err != nil {
		t.Fatalf("renderedScreenContaining: %v", err)
	}
	if !strings.Contains(screen, "SECOND_REPLY") {
		t.Fatalf("visible pre-exit frame = %q, want SECOND_REPLY", screen)
	}
}

func TestRenderedScreenContainingRetainsIncrementalFrameAfterLatestHome(t *testing.T) {
	// The real TUI's final redraw starts at CUP home but does not erase first.
	// A cumulative replay can scroll that transcript out of its synthetic grid;
	// replaying the latest frame itself preserves what the user saw.
	raw := "\x1b[?1049h\x1b[Hold transcript\x1b[28;H\x1b[H\n\n\n\n\n\n\n\n\n\n\n\n\n\n⏺ SECOND_REPLY\x1b[K\r\n\x1b[?1049l"
	screen, err := renderedScreenContaining([]byte(raw), ptyRows, ptyCols, "SECOND_REPLY")
	if err != nil {
		t.Fatalf("renderedScreenContaining: %v", err)
	}
	if !strings.Contains(screen, "SECOND_REPLY") {
		t.Fatalf("visible incremental frame = %q, want SECOND_REPLY", screen)
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

func TestRealPTYFreshConversationSearchAndSecondTurn(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	artifactRoot := testArtifactRoot(t, "fresh-conversation")
	result, err := RunFreshConversation(ctx, Config{
		Daemon: daemon, CLI: cli, SourceRoot: repo, ArtifactRoot: artifactRoot, Timeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("fresh conversation PTY evidence: %v (artifacts retained at %s)", err, artifactRoot)
	}
	if result.FirstRunID == "" || result.SecondRunID == "" || result.FirstRunID == result.SecondRunID || result.ConversationID == "" {
		t.Fatalf("fresh run identities = %#v", result)
	}
	for _, name := range []string{"terminal", "first_screen", "first_frame", "search_screen", "search_frame", "search_exit_screen", "search_exit_frame", "second_screen", "second_frame", "final_screen", "final_frame", "keystrokes", "sse", "api_store"} {
		path := result.ArtifactPaths[name]
		if path == "" {
			t.Fatalf("missing %s artifact: %#v", name, result.ArtifactPaths)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s artifact: %v", name, err)
		}
	}
	for _, screenName := range []struct{ artifact, want string }{
		{"first_screen", "FIRST_REPLY"},
		{"search_screen", "Search: FIRST_REPLY (1 result)"},
		{"second_screen", "SECOND_REPLY"},
	} {
		raw, err := os.ReadFile(result.ArtifactPaths[screenName.artifact])
		if err != nil || !strings.Contains(string(raw), screenName.want) {
			t.Fatalf("%s = %q, err=%v, want %q", screenName.artifact, raw, err, screenName.want)
		}
	}
	exitScreen, err := os.ReadFile(result.ArtifactPaths["search_exit_screen"])
	if err != nil || strings.Contains(string(exitScreen), "Search: FIRST_REPLY (1 result)") {
		t.Fatalf("search exit screen = %q, err=%v, want search dismissed", exitScreen, err)
	}
	terminal, err := os.ReadFile(result.ArtifactPaths["terminal"])
	if err != nil {
		t.Fatal(err)
	}
	frames := []struct {
		artifact, action, expected, run string
	}{
		{"first_frame", "first_prompt", "FIRST_REPLY", result.FirstRunID},
		{"search_frame", "search", "Search: FIRST_REPLY (1 result)", result.FirstRunID},
		{"search_exit_frame", "escape", "Search: FIRST_REPLY (1 result)", result.FirstRunID},
		{"second_frame", "second_prompt", "SECOND_REPLY", result.SecondRunID},
		{"final_frame", "quit", "", result.SecondRunID},
	}
	previousEnd := 0
	for sequence, want := range frames {
		raw, err := os.ReadFile(result.ArtifactPaths[want.artifact])
		if err != nil {
			t.Fatal(err)
		}
		var frame freshFrameRecord
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("decode %s: %v", want.artifact, err)
		}
		if frame.Sequence != sequence+1 || frame.Action != want.action || frame.Expected != want.expected || frame.RunID != want.run || frame.ConversationID != result.ConversationID {
			t.Fatalf("%s record = %#v, want sequence/action/expected/run/conversation", want.artifact, frame)
		}
		if frame.Start != previousEnd || frame.End < frame.Start || (sequence < len(frames)-1 && frame.End == frame.Start) || frame.End > len(terminal) {
			t.Fatalf("%s offsets = [%d,%d), previous=%d terminal=%d", want.artifact, frame.Start, frame.End, previousEnd, len(terminal))
		}
		if frame.InputSHA256 == "" || frame.PrefixSHA256 != digestBytes(terminal[:frame.End]) || frame.RenderSHA256 == "" {
			t.Fatalf("%s hashes = %#v, want input and matching prefix/render hashes", want.artifact, frame)
		}
		previousEnd = frame.End
	}
	probe, err := os.ReadFile(result.ArtifactPaths["api_store"])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{result.FirstRunID, result.SecondRunID, result.ConversationID, "FIRST_REPLY", "SECOND_REPLY"} {
		if !strings.Contains(string(probe), want) {
			t.Fatalf("API/store probe missing %q: %s", want, probe)
		}
	}
}

// TestRealPTYNonMutatingCommandBatch is intentionally a user-realistic
// regression: every key is sent to an owned 30x100 terminal, and the scenario
// must seal its rendered frame before it advances to the next command.
func TestRealPTYNonMutatingCommandBatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("direct PTY acceptance is Unix-only")
	}
	repo := repoRoot(t)
	bin := t.TempDir()
	daemon, cli := filepath.Join(bin, "harnessd"), filepath.Join(bin, "harnesscli")
	for _, target := range []struct{ out, pkg string }{{daemon, "./cmd/harnessd"}, {cli, "./cmd/harnesscli"}} {
		cmd := exec.Command("go", "build", "-o", target.out, target.pkg)
		cmd.Dir = repo
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\\n%s", target.pkg, err, output)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	artifactRoot := testArtifactRoot(t, "nonmutating-command-batch")
	result, err := RunNonMutatingCommandBatch(ctx, Config{
		Daemon: daemon, CLI: cli, SourceRoot: repo, ArtifactRoot: artifactRoot, Timeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("non-mutating PTY batch: %v (artifacts retained at %s)", err, artifactRoot)
	}
	if result.SourceRunID == "" || result.ResumeRunID == "" || result.ContinueRunID == "" || result.ConversationID == "" {
		t.Fatalf("missing run/conversation correlation: %#v", result)
	}
	if result.ResumeRunID == result.ContinueRunID || result.SourceRunID == result.ResumeRunID || result.SourceRunID == result.ContinueRunID {
		t.Fatalf("run identities must be distinct: %#v", result)
	}
	if result.ContinueTargetRunID != result.ResumeRunID {
		t.Fatalf("/continue target=%q, want completed /resume child %q", result.ContinueTargetRunID, result.ResumeRunID)
	}
	keystrokes, err := os.ReadFile(result.ArtifactPaths["keystrokes"])
	if err != nil || !strings.Contains(string(keystrokes), "/continue "+result.ResumeRunID+" continue continuation prompt\r") {
		t.Fatalf("keystrokes=%q err=%v, want /continue target to be completed /resume child", keystrokes, err)
	}
	for _, action := range []string{"first_prompt", "help", "cost", "stats", "config", "context", "doctor", "permissions", "search", "search_escape", "unknown", "resume", "continue", "quit"} {
		if result.ActionFrames[action] == "" {
			t.Fatalf("missing causal frame for %s: %#v", action, result.ActionFrames)
		}
	}
	terminal, err := os.ReadFile(result.ArtifactPaths["terminal"])
	if err != nil {
		t.Fatal(err)
	}
	orderedActions := []string{
		"first_prompt", "help", "help_escape", "cost", "cost_escape", "stats", "stats_escape",
		"config", "config_escape", "context", "context_escape", "doctor", "permissions",
		"permissions_escape", "search", "search_escape", "unknown", "title", "dashboard", "dashboard_escape",
		"workflow", "workflow_escape", "tasks", "tasks_escape", "undo", "undo_escape", "plugins", "plugins_escape",
		"resume", "continue", "quit",
	}
	previousEnd := 0
	for sequence, action := range orderedActions {
		raw, err := os.ReadFile(result.ActionFrames[action])
		if err != nil {
			t.Fatalf("read %s frame: %v", action, err)
		}
		var frame freshFrameRecord
		if err := json.Unmarshal(raw, &frame); err != nil {
			t.Fatalf("decode %s frame: %v", action, err)
		}
		if frame.Sequence != sequence+1 || frame.Action != action || frame.Start != previousEnd || frame.End <= frame.Start || frame.End > len(terminal) {
			t.Fatalf("%s frame = %#v, prior=%d terminal=%d", action, frame, previousEnd, len(terminal))
		}
		if frame.PrefixSHA256 != digestBytes(terminal[:frame.End]) || frame.InputSHA256 == "" || frame.RenderSHA256 == "" {
			t.Fatalf("%s frame hashes = %#v", action, frame)
		}
		previousEnd = frame.End
	}
	statsScreenPath := strings.TrimSuffix(result.ActionFrames["stats"], "-frame.json") + "-screen.txt"
	statsScreen, err := os.ReadFile(statsScreenPath)
	if err != nil {
		t.Fatalf("read sealed stats screen: %v", err)
	}
	for _, want := range []string{"Activity (last 7 days)", "[r to toggle period]", "Total runs: 1", "Total cost: $0.00"} {
		if !strings.Contains(string(statsScreen), want) {
			t.Fatalf("sealed stats screen missing %q: %s", want, statsScreen)
		}
	}
	statsEscapeScreenPath := strings.TrimSuffix(result.ActionFrames["stats_escape"], "-frame.json") + "-screen.txt"
	statsEscapeScreen, err := os.ReadFile(statsEscapeScreenPath)
	if err != nil || strings.Contains(string(statsEscapeScreen), "Activity (last 7 days)") {
		t.Fatalf("stats Escape frame = %q, err=%v, want dismissed before config", statsEscapeScreen, err)
	}
	for _, want := range []string{"FIRST_REPLY", "RESUME_REPLY", "CONTINUE_REPLY"} {
		if !strings.Contains(result.APIStoreProbe, want) {
			t.Fatalf("API/store probe lacks %q: %s", want, result.APIStoreProbe)
		}
	}
	for _, runID := range []string{result.ResumeRunID, result.ContinueRunID} {
		if got := result.ChildEventCounts[runID]; got.AssistantMessage != 1 || got.RunCompleted != 1 {
			t.Fatalf("child %s event counts = %#v, want exactly one assistant message and completion", runID, got)
		}
	}
}

func TestRealPTYStatefulCommandBatchSealsFirstReplyBeforeCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("direct PTY acceptance is Unix-only")
	}
	repo := repoRoot(t)
	bin := t.TempDir()
	daemon, cli := filepath.Join(bin, "harnessd"), filepath.Join(bin, "harnesscli")
	for _, target := range []struct{ out, pkg string }{{daemon, "./cmd/harnessd"}, {cli, "./cmd/harnesscli"}} {
		cmd := exec.Command("go", "build", "-o", target.out, target.pkg)
		cmd.Dir = repo
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\\n%s", target.pkg, err, output)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	result, err := RunStatefulCommandBatch(ctx, Config{Daemon: daemon, CLI: cli, SourceRoot: repo, ArtifactRoot: testArtifactRoot(t, "stateful-command-batch"), Timeout: 20 * time.Second})
	if err != nil {
		t.Fatalf("stateful PTY batch: %v", err)
	}
	if result.RunID == "" || result.ConversationID == "" || !strings.Contains(result.APIStoreProbe, "FIRST_REPLY") {
		t.Fatalf("missing API/store correlation: %#v", result)
	}
	for _, action := range []string{"first_prompt", "title", "dashboard", "workflow", "tasks", "undo", "plugins", "quit"} {
		if result.ActionFrames[action] == "" {
			t.Fatalf("missing sealed frame for %s: %#v", action, result.ActionFrames)
		}
	}
	first, err := os.ReadFile(strings.TrimSuffix(result.ActionFrames["first_prompt"], "-frame.json") + "-screen.txt")
	if err != nil || !strings.Contains(string(first), "FIRST_REPLY") {
		t.Fatalf("first reply frame = %q, err=%v", first, err)
	}
	var firstFrame, titleFrame freshFrameRecord
	for action, target := range map[string]*freshFrameRecord{"first_prompt": &firstFrame, "title": &titleFrame} {
		raw, err := os.ReadFile(result.ActionFrames[action])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			t.Fatal(err)
		}
	}
	if firstFrame.Action != "first_prompt" || titleFrame.Action != "title" || titleFrame.ActionStartOffset < firstFrame.End || titleFrame.Sequence <= firstFrame.Sequence {
		t.Fatalf("title was not causally typed after first reply seal: first=%#v title=%#v", firstFrame, titleFrame)
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
