// Package ptyrunner drives a real harnesscli TUI through a PTY and retains
// enough raw evidence to correlate the rendered continuation with the API.
package ptyrunner

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/mattn/go-runewidth"
	"go-agent-harness/internal/acceptance/inventory"
)

// Config contains only disposable, caller-built binaries and paths. The
// runner never reads user configuration or writes outside ArtifactRoot.
type Config struct {
	Daemon, CLI, SourceRoot, ArtifactRoot string
	Command                               string // resume or continue
	Timeout                               time.Duration
}

const (
	ptyRows              = 30
	ptyCols              = 100
	freshPTYStartupDelay = 4 * time.Second
)

// Result describes the source and child identities needed by callers to add a
// hash-bound acceptance record.
type Result struct {
	SourceRunID, ChildRunID, ConversationID string
	Artifacts                               map[string]string
	ArtifactPaths                           map[string]string
}

// FreshResult captures the two real runs produced by a fresh terminal
// conversation. It is deliberately separate from Result: there is no source
// run outside the real TUI interaction to accidentally credit as a user turn.
type FreshResult struct {
	FirstRunID, SecondRunID, ConversationID string
	Artifacts                               map[string]string
	ArtifactPaths                           map[string]string
}

// CommandEventCounts records the terminal events that prove one continuation
// created precisely one assistant response.
type CommandEventCounts struct {
	AssistantMessage int
	RunCompleted     int
}

// NonMutatingResult is the retained result of the first bounded #1088 command
// batch. ActionFrames maps every causal action to its immutable frame artifact.
type NonMutatingResult struct {
	SourceRunID, ResumeRunID, ContinueRunID, ConversationID string
	ContinueTargetRunID                                     string
	ActionFrames                                            map[string]string
	APIStoreProbe                                           string
	ChildEventCounts                                        map[string]CommandEventCounts
	ArtifactPaths                                           map[string]string
}

// RunNonMutatingCommandBatch drives the first bounded informational command
// batch using the same direct owned PTY protocol as a fresh user conversation.
// It never sends a next key until the sole collector sealed the previous frame.
func RunNonMutatingCommandBatch(ctx context.Context, cfg Config) (NonMutatingResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	for _, value := range []string{cfg.Daemon, cfg.CLI, cfg.SourceRoot, cfg.ArtifactRoot} {
		if strings.TrimSpace(value) == "" {
			return NonMutatingResult{}, fmt.Errorf("daemon, CLI, source root, and artifact root are required")
		}
	}
	if err := os.MkdirAll(cfg.ArtifactRoot, 0o700); err != nil {
		return NonMutatingResult{}, err
	}
	turnsPath := filepath.Join(cfg.ArtifactRoot, "nonmutating-fake-turns.json")
	turns := `[{"content":"FIRST_REPLY","deltas":[{"content":"FIRST_REPLY"}]},{"content":"RESUME_REPLY","deltas":[{"content":"RESUME_REPLY"}]},{"content":"CONTINUE_REPLY","deltas":[{"content":"CONTINUE_REPLY"}]}]`
	if err := os.WriteFile(turnsPath, []byte(turns), 0o600); err != nil {
		return NonMutatingResult{}, err
	}
	logPath := filepath.Join(cfg.ArtifactRoot, "nonmutating-harnessd.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return NonMutatingResult{}, err
	}
	defer logFile.Close()
	dbPath := filepath.Join(cfg.ArtifactRoot, "nonmutating-conversation.db")
	runDBPath := filepath.Join(cfg.ArtifactRoot, "nonmutating-runs.db")
	daemon := exec.Command(cfg.Daemon)
	daemon.Stdout, daemon.Stderr, daemon.Dir = logFile, logFile, cfg.ArtifactRoot
	daemon.Env = append(os.Environ(), "HARNESS_ADDR=127.0.0.1:0", "HARNESS_PROVIDER=fake", "HARNESS_FAKE_TURNS="+turnsPath, "HARNESS_CONVERSATION_DB="+dbPath, "HARNESS_RUN_DB="+runDBPath, "HARNESS_AUTH_DISABLED=true", "HARNESS_WORKSPACE="+cfg.ArtifactRoot, "HARNESS_PROMPTS_DIR="+filepath.Join(cfg.SourceRoot, "prompts"), "HOME="+filepath.Join(cfg.ArtifactRoot, "home"))
	daemon.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := daemon.Start(); err != nil {
		return NonMutatingResult{}, fmt.Errorf("start fake daemon: %w", err)
	}
	defer func() { _ = syscall.Kill(-daemon.Process.Pid, syscall.SIGTERM); _, _ = daemon.Process.Wait() }()

	base, err := waitForBase(ctx, logPath, cfg.Timeout)
	if err != nil {
		return NonMutatingResult{}, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	terminalPath := filepath.Join(cfg.ArtifactRoot, "nonmutating-terminal.txt")
	ptyCmd := exec.CommandContext(ctx, cfg.CLI, "-tui", "-base-url="+base)
	master, err := pty.StartWithSize(ptyCmd, &pty.Winsize{Rows: ptyRows, Cols: ptyCols})
	if err != nil {
		return NonMutatingResult{}, fmt.Errorf("start non-mutating PTY harnesscli: %w", err)
	}
	collector, err := startFreshMasterCollector(master, terminalPath)
	if err != nil {
		_ = master.Close()
		return NonMutatingResult{}, err
	}
	collector.artifactRoot = cfg.ArtifactRoot
	ptyDone := make(chan error, 1)
	ptyComplete := make(chan struct{})
	go func() { err := ptyCmd.Wait(); close(ptyComplete); ptyDone <- err }()
	defer func() {
		select {
		case <-ptyComplete:
			return
		default:
		}
		_ = ptyCmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-ptyDone:
		case <-time.After(time.Second):
			_ = ptyCmd.Process.Kill()
			<-ptyDone
		}
	}()
	select {
	case <-ctx.Done():
		_ = master.Close()
		return NonMutatingResult{}, ctx.Err()
	case <-time.After(freshPTYStartupDelay):
	}

	frames := map[string]string{}
	sequence := 0
	var conv string
	var inputs strings.Builder
	writeVisibleAll := func(action, input, runID string, expected ...string) error {
		if len(expected) == 0 {
			return fmt.Errorf("action %q requires a rendered predicate", action)
		}
		sequence++
		inputs.WriteString(input)
		barrier := collector.beginAction()
		if _, err := io.WriteString(master, input); err != nil {
			return err
		}
		_, frame, err := collector.waitAndSeal(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: sequence, Action: action, Input: input, Expected: strings.Join(expected, " | "), ConversationID: conv, RunID: runID, Artifact: "nonmutating-" + action, Barrier: barrier}, func(raw []byte) (string, error) {
			screen, err := renderedScreenContaining(raw, ptyRows, ptyCols, expected[0])
			if err != nil {
				return "", err
			}
			for _, want := range expected[1:] {
				if !strings.Contains(screen, want) {
					return "", fmt.Errorf("current PTY screen did not render %q", want)
				}
			}
			return screen, nil
		})
		if err != nil {
			return err
		}
		frames[action] = frame
		return nil
	}
	writeVisible := func(action, input, expected string, runID string) error {
		return writeVisibleAll(action, input, runID, expected)
	}
	writeClosed := func(action, input, absent string, runID string) error {
		sequence++
		inputs.WriteString(input)
		barrier := collector.beginAction()
		if _, err := io.WriteString(master, input); err != nil {
			return err
		}
		_, frame, err := collector.waitAndSealAbsent(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: sequence, Action: action, Input: input, Expected: absent, ConversationID: conv, RunID: runID, Artifact: "nonmutating-" + action, Barrier: barrier})
		if err != nil {
			return err
		}
		frames[action] = frame
		return nil
	}

	const firstPrompt = "nonmutating first prompt"
	inputs.WriteString(firstPrompt + "\r")
	firstBarrier := collector.beginAction()
	if _, err := io.WriteString(master, firstPrompt+"\r"); err != nil {
		return NonMutatingResult{}, err
	}
	source, observedConv, err := waitForCompletedPromptRun(ctx, client, base, firstPrompt, "", cfg.Timeout, ptyDone)
	if err != nil {
		return NonMutatingResult{}, err
	}
	conv = observedConv
	sequence++
	_, firstFrame, err := collector.waitAndSealText(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: sequence, Action: "first_prompt", Input: firstPrompt + "\r", Expected: "FIRST_REPLY", ConversationID: conv, RunID: source, Artifact: "nonmutating-first-prompt", Barrier: firstBarrier})
	if err != nil {
		return NonMutatingResult{}, err
	}
	frames["first_prompt"] = firstFrame

	// All overlays have an explicit dismissal frame so the next command is never
	// written while focus remains captured by a prior component.
	for _, action := range []struct{ name, input, expected string }{
		{"help", "/help\r", "Commands"},
		{"cost", "/cost\r", "$0.0000"},
	} {
		if err := writeVisible(action.name, action.input, action.expected, source); err != nil {
			return NonMutatingResult{}, err
		}
		if err := writeClosed(action.name+"_escape", "\x1b", action.expected, source); err != nil {
			return NonMutatingResult{}, err
		}
	}
	if err := writeVisibleAll("stats", "/stats\r", source, "Activity (last 7 days)", "[r to toggle period]", "Total runs: 1", "Total cost: $0.00"); err != nil {
		return NonMutatingResult{}, err
	}
	if err := writeClosed("stats_escape", "\x1b", "Activity (last 7 days)", source); err != nil {
		return NonMutatingResult{}, err
	}
	for _, action := range []struct{ name, input, expected string }{
		{"config", "/config\r", "base_url"},
		{"context", "/context\r", "Context Window Usage"},
	} {
		if err := writeVisible(action.name, action.input, action.expected, source); err != nil {
			return NonMutatingResult{}, err
		}
		if err := writeClosed(action.name+"_escape", "\x1b", action.expected, source); err != nil {
			return NonMutatingResult{}, err
		}
	}
	if err := writeVisible("doctor", "/doctor\r", "Run: go test ./cmd/harnesscli", source); err != nil {
		return NonMutatingResult{}, err
	}
	if err := writeVisible("permissions", "/permissions\r", "No permission rules active", source); err != nil {
		return NonMutatingResult{}, err
	}
	if err := writeClosed("permissions_escape", "\x1b", "No permission rules active", source); err != nil {
		return NonMutatingResult{}, err
	}
	if err := writeVisible("search", "/search FIRST_REPLY\r", "Search: FIRST_REPLY (1 result)", source); err != nil {
		return NonMutatingResult{}, err
	}
	if err := writeClosed("search_escape", "\x1b", "Search: FIRST_REPLY (1 result)", source); err != nil {
		return NonMutatingResult{}, err
	}
	if err := writeVisible("unknown", "/notacommand\r", "Unknown command: /notacommand", source); err != nil {
		return NonMutatingResult{}, err
	}

	resumePrompt := "resume continuation prompt"
	resumeInput := fmt.Sprintf("/resume %s %s\r", source, resumePrompt)
	inputs.WriteString(resumeInput)
	resumeBarrier := collector.beginAction()
	if _, err := io.WriteString(master, resumeInput); err != nil {
		return NonMutatingResult{}, err
	}
	resume, resumeConv, err := waitForCompletedPromptRun(ctx, client, base, resumePrompt, source, cfg.Timeout, ptyDone)
	if err != nil {
		return NonMutatingResult{}, err
	}
	if resumeConv != conv {
		return NonMutatingResult{}, fmt.Errorf("resume conversation %q, want %q", resumeConv, conv)
	}
	sequence++
	_, resumeFrame, err := collector.waitAndSealText(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: sequence, Action: "resume", Input: resumeInput, Expected: "RESUME_REPLY", ConversationID: conv, RunID: resume, Artifact: "nonmutating-resume", Barrier: resumeBarrier})
	if err != nil {
		return NonMutatingResult{}, err
	}
	frames["resume"] = resumeFrame

	// The continuation API deliberately permits a run to have one immediate
	// continuation. /continue therefore targets the completed /resume child,
	// not the already-consumed source. Assert that state before typing so a
	// later command cannot receive credit from a stale or invented target.
	if err := requireCompletedRunInConversation(ctx, client, base, resume, conv); err != nil {
		return NonMutatingResult{}, fmt.Errorf("continue target: %w", err)
	}
	continuePrompt := "continue continuation prompt"
	continueInput := fmt.Sprintf("/continue %s %s\r", resume, continuePrompt)
	inputs.WriteString(continueInput)
	continueBarrier := collector.beginAction()
	if _, err := io.WriteString(master, continueInput); err != nil {
		return NonMutatingResult{}, err
	}
	continued, continueConv, err := waitForCompletedPromptRun(ctx, client, base, continuePrompt, resume, cfg.Timeout, ptyDone)
	if err != nil {
		return NonMutatingResult{}, err
	}
	if continueConv != conv {
		return NonMutatingResult{}, fmt.Errorf("continue conversation %q, want %q", continueConv, conv)
	}
	sequence++
	_, continueFrame, err := collector.waitAndSealText(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: sequence, Action: "continue", Input: continueInput, Expected: "CONTINUE_REPLY", ConversationID: conv, RunID: continued, Artifact: "nonmutating-continue", Barrier: continueBarrier})
	if err != nil {
		return NonMutatingResult{}, err
	}
	frames["continue"] = continueFrame

	const quitInput = "/quit\r"
	inputs.WriteString(quitInput)
	quitBarrier := collector.beginAction()
	if _, err := io.WriteString(master, quitInput); err != nil {
		return NonMutatingResult{}, err
	}
	if err := <-ptyDone; err != nil {
		return NonMutatingResult{}, fmt.Errorf("non-mutating PTY harnesscli: %w", err)
	}
	if err := master.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return NonMutatingResult{}, err
	}
	if err := collector.waitEOF(ctx); err != nil {
		return NonMutatingResult{}, err
	}
	sequence++
	_, quitFrame, err := collector.sealFinal(freshFrameSpec{Sequence: sequence, Action: "quit", Input: quitInput, ConversationID: conv, RunID: continued, Artifact: "nonmutating-quit", Barrier: quitBarrier})
	if err != nil {
		return NonMutatingResult{}, err
	}
	frames["quit"] = quitFrame

	childCounts := map[string]CommandEventCounts{}
	var sseParts [][]byte
	for _, runID := range []string{resume, continued} {
		raw, events, err := stream(ctx, client, base, runID)
		if err != nil {
			return NonMutatingResult{}, err
		}
		counts := CommandEventCounts{AssistantMessage: events["assistant.message"], RunCompleted: events["run.completed"]}
		if counts.AssistantMessage != 1 || counts.RunCompleted != 1 {
			return NonMutatingResult{}, fmt.Errorf("child %s terminal events = %#v", runID, counts)
		}
		childCounts[runID] = counts
		sseParts = append(sseParts, raw)
	}
	ssePath := filepath.Join(cfg.ArtifactRoot, "nonmutating-children.sse")
	if err := os.WriteFile(ssePath, bytes.Join(sseParts, []byte("\n")), 0o600); err != nil {
		return NonMutatingResult{}, err
	}
	probe, err := nonMutatingAPIStoreProbe(ctx, client, base, source, resume, continued, conv)
	if err != nil {
		return NonMutatingResult{}, err
	}
	probePath := filepath.Join(cfg.ArtifactRoot, "nonmutating-api-store.json")
	if err := os.WriteFile(probePath, probe, 0o600); err != nil {
		return NonMutatingResult{}, err
	}
	keystrokesPath := filepath.Join(cfg.ArtifactRoot, "nonmutating-keystrokes.txt")
	if err := os.WriteFile(keystrokesPath, []byte(inputs.String()), 0o600); err != nil {
		return NonMutatingResult{}, err
	}
	paths := map[string]string{"terminal": terminalPath, "keystrokes": keystrokesPath, "sse": ssePath, "api_store": probePath}
	for action, frame := range frames {
		paths["frame_"+action] = frame
	}
	return NonMutatingResult{SourceRunID: source, ResumeRunID: resume, ContinueRunID: continued, ConversationID: conv, ContinueTargetRunID: resume, ActionFrames: frames, APIStoreProbe: string(probe), ChildEventCounts: childCounts, ArtifactPaths: paths}, nil
}

// RunFreshConversation drives the ordinary terminal path rather than creating
// a source run through HTTP first. The only direct HTTP calls are independent
// probes after the typed actions have rendered.
func RunFreshConversation(ctx context.Context, cfg Config) (FreshResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	for _, value := range []string{cfg.Daemon, cfg.CLI, cfg.SourceRoot, cfg.ArtifactRoot} {
		if strings.TrimSpace(value) == "" {
			return FreshResult{}, fmt.Errorf("daemon, CLI, source root, and artifact root are required")
		}
	}
	if _, err := exec.LookPath("script"); err != nil {
		return FreshResult{}, fmt.Errorf("script PTY utility: %w", err)
	}
	if err := os.MkdirAll(cfg.ArtifactRoot, 0o700); err != nil {
		return FreshResult{}, err
	}

	turnsPath := filepath.Join(cfg.ArtifactRoot, "fake-turns.json")
	turns := `[{"content":"FIRST_REPLY","deltas":[{"content":"FIRST_REPLY"}]},{"content":"SECOND_REPLY","deltas":[{"content":"SECOND_REPLY"}]}]`
	if err := os.WriteFile(turnsPath, []byte(turns), 0o600); err != nil {
		return FreshResult{}, err
	}
	logPath := filepath.Join(cfg.ArtifactRoot, "harnessd.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return FreshResult{}, err
	}
	defer logFile.Close()
	dbPath := filepath.Join(cfg.ArtifactRoot, "conversation.db")
	runDBPath := filepath.Join(cfg.ArtifactRoot, "runs.db")
	daemon := exec.Command(cfg.Daemon)
	daemon.Stdout, daemon.Stderr, daemon.Dir = logFile, logFile, cfg.ArtifactRoot
	daemon.Env = append(os.Environ(), "HARNESS_ADDR=127.0.0.1:0", "HARNESS_PROVIDER=fake", "HARNESS_FAKE_TURNS="+turnsPath, "HARNESS_CONVERSATION_DB="+dbPath, "HARNESS_RUN_DB="+runDBPath, "HARNESS_AUTH_DISABLED=true", "HARNESS_WORKSPACE="+cfg.ArtifactRoot, "HARNESS_PROMPTS_DIR="+filepath.Join(cfg.SourceRoot, "prompts"), "HOME="+filepath.Join(cfg.ArtifactRoot, "home"))
	daemon.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := daemon.Start(); err != nil {
		return FreshResult{}, fmt.Errorf("start fake daemon: %w", err)
	}
	defer func() { _ = syscall.Kill(-daemon.Process.Pid, syscall.SIGTERM); _, _ = daemon.Process.Wait() }()

	base, err := waitForBase(ctx, logPath, cfg.Timeout)
	if err != nil {
		return FreshResult{}, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	terminalPath := filepath.Join(cfg.ArtifactRoot, "fresh-terminal.txt")
	ptyCmd := exec.CommandContext(ctx, cfg.CLI, "-tui", "-base-url="+base)
	master, err := pty.StartWithSize(ptyCmd, &pty.Winsize{Rows: ptyRows, Cols: ptyCols})
	if err != nil {
		return FreshResult{}, fmt.Errorf("start fresh PTY harnesscli: %w", err)
	}
	collector, err := startFreshMasterCollector(master, terminalPath)
	if err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	ptyDone := make(chan error, 1)
	ptyComplete := make(chan struct{})
	go func() {
		err := ptyCmd.Wait()
		close(ptyComplete)
		ptyDone <- err
	}()
	defer func() {
		select {
		case <-ptyComplete:
			return
		default:
		}
		_ = ptyCmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-ptyDone:
		case <-time.After(time.Second):
			_ = ptyCmd.Process.Kill()
			<-ptyDone
		}
	}()
	// The PTY input side is ready once the child has had one bounded startup
	// interval. Every later action waits for and snapshots its rendered frame.
	select {
	case <-ctx.Done():
		_ = master.Close()
		return FreshResult{}, ctx.Err()
	case <-time.After(freshPTYStartupDelay):
	}

	keystrokes := "fresh first prompt\r/search FIRST_REPLY\r<esc>fresh second prompt\r/quit\r"
	keystrokesPath := filepath.Join(cfg.ArtifactRoot, "fresh-keystrokes.txt")
	if err := os.WriteFile(keystrokesPath, []byte(keystrokes), 0o600); err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	collector.artifactRoot = cfg.ArtifactRoot
	const firstInput = "fresh first prompt\r"
	firstBarrier := collector.beginAction()
	if _, err := io.WriteString(master, firstInput); err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	first, conv, err := waitForCompletedPromptRun(ctx, client, base, "fresh first prompt", "", cfg.Timeout, ptyDone)
	if err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	firstScreenPath, firstFramePath, err := collector.waitAndSealText(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: 1, Action: "first_prompt", Input: firstInput, Expected: "FIRST_REPLY", ConversationID: conv, RunID: first, Artifact: "fresh-first", Barrier: firstBarrier})
	if err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	const searchInput = "/search FIRST_REPLY\r"
	searchBarrier := collector.beginAction()
	if _, err := io.WriteString(master, searchInput); err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	searchScreenPath, searchFramePath, err := collector.waitAndSealText(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: 2, Action: "search", Input: searchInput, Expected: "Search: FIRST_REPLY (1 result)", ConversationID: conv, RunID: first, Artifact: "fresh-search", Barrier: searchBarrier})
	if err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	const escapeInput = "\x1b"
	escapeBarrier := collector.beginAction()
	if _, err := io.WriteString(master, escapeInput); err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	searchExitScreenPath, searchExitFramePath, err := collector.waitAndSealAbsent(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: 3, Action: "escape", Input: escapeInput, Expected: "Search: FIRST_REPLY (1 result)", ConversationID: conv, RunID: first, Artifact: "fresh-search-exit", Barrier: escapeBarrier})
	if err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	const secondInput = "fresh second prompt\r"
	secondBarrier := collector.beginAction()
	if _, err := io.WriteString(master, secondInput); err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	second, secondConv, err := waitForCompletedPromptRun(ctx, client, base, "fresh second prompt", first, cfg.Timeout, ptyDone)
	if err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	if secondConv != conv {
		_ = master.Close()
		return FreshResult{}, fmt.Errorf("second conversation %q, want first conversation %q", secondConv, conv)
	}
	secondScreenPath, secondFramePath, err := collector.waitAndSealText(ctx, ptyDone, cfg.Timeout, freshFrameSpec{Sequence: 4, Action: "second_prompt", Input: secondInput, Expected: "SECOND_REPLY", ConversationID: conv, RunID: second, Artifact: "fresh-second", Barrier: secondBarrier})
	if err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	const quitInput = "/quit\r"
	quitBarrier := collector.beginAction()
	if _, err := io.WriteString(master, quitInput); err != nil {
		_ = master.Close()
		return FreshResult{}, err
	}
	if err := <-ptyDone; err != nil {
		return FreshResult{}, fmt.Errorf("fresh PTY harnesscli: %w", err)
	}
	if err := master.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return FreshResult{}, err
	}
	if err := collector.waitEOF(ctx); err != nil {
		return FreshResult{}, err
	}
	finalScreenPath, finalFramePath, err := collector.sealFinal(freshFrameSpec{Sequence: 5, Action: "quit", Input: quitInput, ConversationID: conv, RunID: second, Artifact: "fresh-final", Barrier: quitBarrier})
	if err != nil {
		return FreshResult{}, err
	}
	firstSSE, firstEvents, err := stream(ctx, client, base, first)
	if err != nil {
		return FreshResult{}, err
	}
	secondSSE, secondEvents, err := stream(ctx, client, base, second)
	if err != nil {
		return FreshResult{}, err
	}
	for runID, events := range map[string]map[string]int{first: firstEvents, second: secondEvents} {
		if events["run.completed"] != 1 || events["assistant.message.delta"] != 1 || events["assistant.message"] != 1 {
			return FreshResult{}, fmt.Errorf("run %s SSE lifecycle = %#v, want one delta/message/completed", runID, events)
		}
	}
	ssePath := filepath.Join(cfg.ArtifactRoot, "fresh-runs.sse")
	if err := os.WriteFile(ssePath, append(append(firstSSE, '\n'), secondSSE...), 0o600); err != nil {
		return FreshResult{}, err
	}
	probe, err := freshAPIStoreProbe(ctx, client, base, first, second, conv)
	if err != nil {
		return FreshResult{}, err
	}
	probePath := filepath.Join(cfg.ArtifactRoot, "fresh-api-store.json")
	if err := os.WriteFile(probePath, probe, 0o600); err != nil {
		return FreshResult{}, err
	}
	paths := map[string]string{"terminal": terminalPath, "first_screen": firstScreenPath, "first_frame": firstFramePath, "search_screen": searchScreenPath, "search_frame": searchFramePath, "search_exit_screen": searchExitScreenPath, "search_exit_frame": searchExitFramePath, "second_screen": secondScreenPath, "second_frame": secondFramePath, "final_screen": finalScreenPath, "final_frame": finalFramePath, "keystrokes": keystrokesPath, "sse": ssePath, "api_store": probePath}
	digests := make(map[string]string, len(paths))
	for name, path := range paths {
		digest, err := digestPath(path)
		if err != nil {
			return FreshResult{}, fmt.Errorf("digest %s artifact: %w", name, err)
		}
		digests[name] = digest
	}
	return FreshResult{FirstRunID: first, SecondRunID: second, ConversationID: conv, Artifacts: digests, ArtifactPaths: paths}, nil
}

// RunEvidence executes one inventory-derived TUI invocation and emits a
// hash-bound record only after the retained artifacts have been re-read and
// checked. It deliberately has no "planned" outcome: callers receive either
// independently validated Pass evidence or an error.
func RunEvidence(ctx context.Context, cfg Config, compiled inventory.Compiled, c inventory.Case) (inventory.Evidence, error) {
	started := time.Now().UTC()
	invocation, err := invocationForCase(compiled, c)
	if err != nil {
		return inventory.Evidence{}, err
	}
	if cfg.Command != strings.TrimPrefix(invocation.Input, "/") {
		return inventory.Evidence{}, fmt.Errorf("case invocation %q does not match PTY command %q", invocation.ID, cfg.Command)
	}
	result, err := Run(ctx, cfg)
	if err != nil {
		return inventory.Evidence{}, err
	}
	artifacts, err := checkedArtifactRefs(cfg.ArtifactRoot, result.ArtifactPaths)
	if err != nil {
		return inventory.Evidence{}, err
	}
	if err := verifyArtifactRefs(artifacts); err != nil {
		return inventory.Evidence{}, err
	}
	screen, err := os.ReadFile(result.ArtifactPaths["screen"])
	if err != nil {
		return inventory.Evidence{}, fmt.Errorf("read rendered-screen evidence: %w", err)
	}
	probe, err := os.ReadFile(result.ArtifactPaths["api_store"])
	if err != nil {
		return inventory.Evidence{}, fmt.Errorf("read API-store evidence: %w", err)
	}
	eventIDs, err := eventIDs(result.ArtifactPaths["sse"])
	if err != nil {
		return inventory.Evidence{}, err
	}
	observed := []inventory.ProbeObservation{
		{Kind: inventory.PostconditionRenderedScreen, Probe: "pty-screen", AssertionID: "continuation-rendered", Value: "pty continuation reply", Verified: bytes.Contains(screen, []byte("pty continuation reply"))},
		{Kind: inventory.PostconditionConversationState, Probe: "api-store", AssertionID: "same-conversation", Value: result.ConversationID, Verified: bytes.Contains(probe, []byte(result.SourceRunID)) && bytes.Contains(probe, []byte(result.ChildRunID)) && bytes.Contains(probe, []byte(result.ConversationID))},
	}
	for _, observation := range observed {
		if !observation.Verified {
			return inventory.Evidence{}, fmt.Errorf("independent %s probe did not verify %q", observation.Probe, observation.AssertionID)
		}
	}
	evidence := inventory.Evidence{
		SchemaVersion: inventory.SchemaVersion, InventoryHash: compiled.Hash,
		ItemID: c.ItemID, InvocationID: c.InvocationID, Surface: inventory.SurfaceTUI,
		EvidenceClass: c.EvidenceClass, Outcome: inventory.Pass, OrderedActions: c.OrderedActions,
		RunID: result.ChildRunID, ConversationID: result.ConversationID, EventIDs: eventIDs,
		ExpectedPostconditions: c.ExpectedPostconditions, ObservedPostconditions: observed, Artifacts: artifacts,
		Cleanup: inventory.CleanupEvidence{Verified: true, Detail: "isolated daemon stopped; test cleanup removes the verified artifact bundle"},
		Timing:  inventory.Timing{StartedAt: started, FinishedAt: time.Now().UTC()},
	}
	if err := inventory.ValidateEvidence(compiled, c, evidence); err != nil {
		return inventory.Evidence{}, fmt.Errorf("validate hash-bound PTY evidence: %w", err)
	}
	return evidence, nil
}

func Run(ctx context.Context, cfg Config) (Result, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Command != "resume" && cfg.Command != "continue" {
		return Result{}, fmt.Errorf("command must be resume or continue")
	}
	for _, value := range []string{cfg.Daemon, cfg.CLI, cfg.SourceRoot, cfg.ArtifactRoot} {
		if strings.TrimSpace(value) == "" {
			return Result{}, fmt.Errorf("daemon, CLI, source root, and artifact root are required")
		}
	}
	if _, err := exec.LookPath("script"); err != nil {
		return Result{}, fmt.Errorf("script PTY utility: %w", err)
	}
	if err := os.MkdirAll(cfg.ArtifactRoot, 0o700); err != nil {
		return Result{}, err
	}

	turnsPath := filepath.Join(cfg.ArtifactRoot, "fake-turns.json")
	if err := os.WriteFile(turnsPath, []byte(`[{"content":"source reply","deltas":[{"content":"source reply"}]},{"content":"pty continuation reply","deltas":[{"content":"pty continuation reply"}]}]`), 0o600); err != nil {
		return Result{}, err
	}
	logPath := filepath.Join(cfg.ArtifactRoot, "harnessd.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return Result{}, err
	}
	defer logFile.Close()
	dbPath := filepath.Join(cfg.ArtifactRoot, "conversation.db")
	runDBPath := filepath.Join(cfg.ArtifactRoot, "runs.db")
	daemon := exec.Command(cfg.Daemon)
	daemon.Stdout, daemon.Stderr = logFile, logFile
	// The daemon owns several workspace-relative SQLite files. Keep all of
	// them in the retained artifact bundle, never in the source test package.
	daemon.Dir = cfg.ArtifactRoot
	daemon.Env = append(os.Environ(), "HARNESS_ADDR=127.0.0.1:0", "HARNESS_PROVIDER=fake", "HARNESS_FAKE_TURNS="+turnsPath, "HARNESS_CONVERSATION_DB="+dbPath, "HARNESS_RUN_DB="+runDBPath, "HARNESS_AUTH_DISABLED=true", "HARNESS_WORKSPACE="+cfg.ArtifactRoot, "HARNESS_PROMPTS_DIR="+filepath.Join(cfg.SourceRoot, "prompts"), "HOME="+filepath.Join(cfg.ArtifactRoot, "home"))
	daemon.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := daemon.Start(); err != nil {
		return Result{}, fmt.Errorf("start fake daemon: %w", err)
	}
	defer func() { _ = syscall.Kill(-daemon.Process.Pid, syscall.SIGTERM); _, _ = daemon.Process.Wait() }()

	base, err := waitForBase(ctx, logPath, cfg.Timeout)
	if err != nil {
		return Result{}, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	source, conv, err := startAndComplete(ctx, client, base, "source prompt")
	if err != nil {
		return Result{}, err
	}

	terminalPath := filepath.Join(cfg.ArtifactRoot, cfg.Command+"-terminal.txt")
	terminal, err := os.OpenFile(terminalPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return Result{}, err
	}
	defer terminal.Close()
	inR, inW, err := os.Pipe()
	if err != nil {
		return Result{}, err
	}
	defer inR.Close()
	// Resume the source conversation at TUI startup so its durable history and
	// long-lived conversation stream are active before the typed slash command.
	// The command under test is still the subsequent PTY keystroke below.
	pty := exec.CommandContext(ctx, "script", ptyCommandArgs(cfg.CLI, base, source)...)
	pty.Stdin, pty.Stdout, pty.Stderr = inR, terminal, terminal
	pty.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := pty.Start(); err != nil {
		inW.Close()
		return Result{}, fmt.Errorf("start PTY harnesscli: %w", err)
	}
	ptyDone := make(chan error, 1)
	go func() { ptyDone <- pty.Wait() }()
	defer func() {
		if pty.ProcessState != nil {
			return
		}
		_ = syscall.Kill(-pty.Process.Pid, syscall.SIGTERM)
		select {
		case <-ptyDone:
		case <-time.After(time.Second):
			_ = pty.Process.Kill()
			<-ptyDone
		}
	}()
	// TUI startup writes escape sequences before durable history is rendered.
	// The source reply is the semantic readiness boundary for the keystroke.
	if err := waitForCurrentScreenText(ctx, terminalPath, "source reply", cfg.Timeout, ptyDone); err != nil {
		inW.Close()
		return Result{}, err
	}
	keystrokes := fmt.Sprintf("/%s %s pty continuation prompt\r", cfg.Command, source)
	keystrokesPath := filepath.Join(cfg.ArtifactRoot, cfg.Command+"-keystrokes.txt")
	if err := os.WriteFile(keystrokesPath, []byte(keystrokes), 0o600); err != nil {
		inW.Close()
		return Result{}, err
	}
	if _, err := io.WriteString(inW, keystrokes); err != nil {
		inW.Close()
		return Result{}, err
	}
	child, err := waitForChild(ctx, client, base, conv, source, cfg.Timeout, ptyDone)
	if err != nil {
		inW.Close()
		return Result{}, err
	}
	if err := waitForCurrentScreenText(ctx, terminalPath, "pty continuation reply", cfg.Timeout, ptyDone); err != nil {
		inW.Close()
		return Result{}, err
	}
	// Persist an ANSI/VT-interpreted current-screen artifact before asserting
	// the rendered result or tearing down the alternate screen.
	rawTerminal, err := os.ReadFile(terminalPath)
	if err != nil {
		inW.Close()
		return Result{}, err
	}
	screen, err := renderedScreenContaining(rawTerminal, 30, 100, "pty continuation reply")
	if err != nil {
		inW.Close()
		return Result{}, err
	}
	screenPath := filepath.Join(cfg.ArtifactRoot, cfg.Command+"-screen.txt")
	if err := os.WriteFile(screenPath, []byte(screen), 0o600); err != nil {
		inW.Close()
		return Result{}, err
	}
	// The final redraw intentionally replaces transient command-status text.
	// Assert the durable visible reply here; the preserved keystroke artifact,
	// child identity, and same-conversation API probe prove which command made it.
	for _, required := range []string{"pty continuation reply"} {
		if !strings.Contains(screen, required) {
			inW.Close()
			return Result{}, fmt.Errorf("current PTY screen did not render %q", required)
		}
	}
	if _, err := io.WriteString(inW, "/quit\r"); err != nil {
		inW.Close()
		return Result{}, err
	}
	_ = inW.Close()
	if err := <-ptyDone; err != nil {
		return Result{}, fmt.Errorf("PTY harnesscli: %w", err)
	}

	rawSSE, events, err := stream(ctx, client, base, child)
	if err != nil {
		return Result{}, err
	}
	if events["run.completed"] == 0 {
		return Result{}, fmt.Errorf("child SSE has no run.completed")
	}
	if events["assistant.message.delta"] != 1 {
		return Result{}, fmt.Errorf("child assistant.message.delta = %d, want 1", events["assistant.message.delta"])
	}
	ssePath := filepath.Join(cfg.ArtifactRoot, cfg.Command+"-child.sse")
	if err := os.WriteFile(ssePath, rawSSE, 0o600); err != nil {
		return Result{}, err
	}
	probePath := filepath.Join(cfg.ArtifactRoot, cfg.Command+"-api-store.json")
	probe, childConv, err := apiStoreProbe(ctx, client, base, source, child, conv)
	if err != nil {
		return Result{}, err
	}
	if childConv != conv {
		return Result{}, fmt.Errorf("child conversation %q, want source conversation %q", childConv, conv)
	}
	if err := os.WriteFile(probePath, probe, 0o600); err != nil {
		return Result{}, err
	}
	paths := map[string]string{"terminal": terminalPath, "screen": screenPath, "keystrokes": keystrokesPath, "sse": ssePath, "api_store": probePath}
	digests := make(map[string]string, len(paths))
	for name, path := range paths {
		digest, err := digestPath(path)
		if err != nil {
			return Result{}, fmt.Errorf("digest %s artifact: %w", name, err)
		}
		digests[name] = digest
	}
	return Result{SourceRunID: source, ChildRunID: child, ConversationID: conv, Artifacts: digests, ArtifactPaths: paths}, nil
}

func ptyCommandArgs(cli, base, source string) []string {
	return ptyCommandArgsForOS(runtime.GOOS, cli, base, source)
}

func ptyCommandArgsForOS(goos, cli, base, source string) []string {
	return ptyCommandArgsWithResumeForOS(goos, cli, base, source)
}

func ptyFreshCommandArgs(cli, base string) []string {
	return ptyFreshCommandArgsForOS(runtime.GOOS, cli, base)
}

func ptyCommandArgsWithResumeForOS(goos, cli, base, source string) []string {
	return ptyScriptArgsForOS(goos, []string{cli, "-tui", "-resume=" + source, "-base-url=" + base})
}

// ptyFreshCommandArgsForOS has no unconfigured geometry form: every official
// fresh run receives the same explicit screen dimensions as continuation runs.
func ptyFreshCommandArgsForOS(goos, cli, base string) []string {
	return ptyScriptArgsForOS(goos, []string{cli, "-tui", "-base-url=" + base})
}

func ptyScriptArgsForOS(goos string, cliArgs []string) []string {
	child := append([]string{"sh", "-c", fmt.Sprintf("stty rows %d cols %d; exec \"$@\"", ptyRows, ptyCols), "sh"}, cliArgs...)
	if goos != "linux" {
		return append([]string{"-q", "/dev/null"}, child...)
	}
	quoted := make([]string, len(child))
	for i, arg := range child {
		quoted[i] = shellQuote(arg)
	}
	return []string{"-q", "-c", strings.Join(quoted, " "), "/dev/null"}
}

func shellQuote(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}

var listening = regexp.MustCompile(`harness server listening on (127\.0\.0\.1:\d+)`)

func waitForBase(ctx context.Context, logPath string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(logPath); err == nil {
			if match := listening.FindStringSubmatch(string(raw)); len(match) == 2 {
				base := "http://" + match[1]
				resp, err := http.Get(base + "/healthz")
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						return base, nil
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("fake daemon did not become healthy within %s", timeout)
}

func waitForCurrentScreenText(ctx context.Context, path, expected string, timeout time.Duration, ptyDone <-chan error) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-ptyDone:
			if err != nil {
				return fmt.Errorf("PTY harnesscli exited before rendering %q: %w", expected, err)
			}
			return fmt.Errorf("PTY harnesscli exited before rendering %q", expected)
		default:
		}
		if raw, err := os.ReadFile(path); err == nil {
			if _, screenErr := renderedScreenContaining(raw, ptyRows, ptyCols, expected); screenErr == nil {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return fmt.Errorf("current PTY screen did not render %q", expected)
}

func waitForCurrentScreenWithoutText(ctx context.Context, path, unexpected string, timeout time.Duration, ptyDone <-chan error) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-ptyDone:
			return ptyExitedBefore("clearing rendered "+fmt.Sprintf("%q", unexpected), err)
		default:
		}
		if raw, err := os.ReadFile(path); err == nil {
			screen, screenErr := currentScreen(raw, ptyRows, ptyCols)
			if screenErr == nil && !strings.Contains(screen, unexpected) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-ptyDone:
			return ptyExitedBefore("clearing rendered "+fmt.Sprintf("%q", unexpected), err)
		case <-time.After(25 * time.Millisecond):
		}
	}
	return fmt.Errorf("current PTY screen still rendered %q", unexpected)
}

// freshFrameCollector is the only reader of a fresh-conversation typescript.
// It seals each action against the exact append-only prefix that first proves
// the action's rendered state; no later input is sent until that seal exists.
type freshFrameCollector struct {
	master       *os.File
	read         func([]byte) (int, error) // test-only owned-reader seam
	terminal     *os.File
	artifactRoot string
	lastEnd      int
	mu           sync.Mutex
	raw          []byte
	updates      chan struct{}
	eof          bool
	readErr      error
	version      uint64
}

// freshActionBarrier is captured by the sole PTY reader immediately before an
// input write.  It prevents a later action from taking credit for an older
// rendered screen retained in the append-only PTY recording.
type freshActionBarrier struct {
	StartOffset  int
	StartVersion uint64
	Baseline     string
}

func startFreshMasterCollector(master *os.File, terminalPath string) (*freshFrameCollector, error) {
	terminal, err := os.OpenFile(terminalPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	c := &freshFrameCollector{master: master, terminal: terminal, updates: make(chan struct{})}
	go c.collect()
	return c, nil
}

// collect is deliberately the only master reader. It appends the same bytes
// to retained evidence and the in-memory prefix before notifying the sequencer.
func (c *freshFrameCollector) collect() {
	defer c.terminal.Close()
	buf := make([]byte, 4096)
	for {
		read := c.read
		if read == nil {
			read = c.master.Read
		}
		n, err := read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			if _, writeErr := c.terminal.Write(chunk); writeErr != nil && err == nil {
				err = writeErr
			}
			c.mu.Lock()
			c.raw = append(c.raw, chunk...)
			c.publishLocked()
			c.mu.Unlock()
		}
		if err != nil {
			c.mu.Lock()
			// Linux PTY masters report EIO when their final slave closes. Final
			// bytes above have already been retained; process exit remains the
			// authoritative child outcome.
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EIO) {
				c.readErr = err
			}
			c.eof = true
			c.publishLocked()
			c.mu.Unlock()
			return
		}
	}
}

func (c *freshFrameCollector) publishLocked() {
	c.version++
	close(c.updates)
	c.updates = make(chan struct{})
}

func (c *freshFrameCollector) snapshot() ([]byte, <-chan struct{}, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.raw...), c.updates, c.eof, c.readErr
}

func (c *freshFrameCollector) snapshotWithVersion() ([]byte, <-chan struct{}, bool, error, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.raw...), c.updates, c.eof, c.readErr, c.version
}

func (c *freshFrameCollector) beginAction() freshActionBarrier {
	c.mu.Lock()
	defer c.mu.Unlock()
	baseline, _ := currentScreen(c.raw, ptyRows, ptyCols)
	return freshActionBarrier{StartOffset: len(c.raw), StartVersion: c.version, Baseline: baseline}
}

func (c *freshFrameCollector) waitEOF(ctx context.Context) error {
	for {
		_, updates, eof, err := c.snapshot()
		if eof {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-updates:
		}
	}
}

type freshFrameSpec struct {
	Sequence       int                `json:"sequence"`
	Action         string             `json:"action"`
	Input          string             `json:"-"`
	Expected       string             `json:"expected,omitempty"`
	ConversationID string             `json:"conversation_id,omitempty"`
	RunID          string             `json:"run_id,omitempty"`
	Artifact       string             `json:"-"`
	Barrier        freshActionBarrier `json:"-"`
	Dismissal      bool               `json:"-"`
}

type freshFrameRecord struct {
	Sequence           int    `json:"sequence"`
	Action             string `json:"action"`
	InputSHA256        string `json:"input_sha256"`
	Expected           string `json:"expected,omitempty"`
	Start              int    `json:"start"`
	End                int    `json:"end"`
	PrefixSHA256       string `json:"prefix_sha256"`
	RenderSHA256       string `json:"render_sha256"`
	ConversationID     string `json:"conversation_id,omitempty"`
	RunID              string `json:"run_id,omitempty"`
	ActionStartOffset  int    `json:"action_start_offset"`
	ActionStartVersion uint64 `json:"action_start_version"`
	MatchEnd           int    `json:"match_end"`
	MatchVersion       uint64 `json:"match_version"`
}

func (c *freshFrameCollector) waitAndSealText(ctx context.Context, ptyDone <-chan error, timeout time.Duration, spec freshFrameSpec) (string, string, error) {
	return c.waitAndSeal(ctx, ptyDone, timeout, spec, func(raw []byte) (string, error) {
		return renderedScreenContaining(raw, ptyRows, ptyCols, spec.Expected)
	})
}

func (c *freshFrameCollector) waitAndSealAbsent(ctx context.Context, ptyDone <-chan error, timeout time.Duration, spec freshFrameSpec) (string, string, error) {
	spec.Dismissal = true
	return c.waitAndSeal(ctx, ptyDone, timeout, spec, func(raw []byte) (string, error) {
		screen, err := currentScreen(raw, ptyRows, ptyCols)
		if err != nil {
			return "", err
		}
		if strings.Contains(screen, spec.Expected) {
			return "", fmt.Errorf("current PTY screen still rendered %q", spec.Expected)
		}
		return screen, nil
	})
}

func (c *freshFrameCollector) waitAndSeal(ctx context.Context, ptyDone <-chan error, timeout time.Duration, spec freshFrameSpec, render func([]byte) (string, error)) (string, string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-ptyDone:
			return "", "", ptyExitedBefore("rendering "+fmt.Sprintf("%q", spec.Expected), err)
		default:
		}
		raw, updates, _, err, version := c.snapshotWithVersion()
		if err == nil && len(raw) > c.lastEnd && len(raw) > spec.Barrier.StartOffset {
			_, renderErr := render(raw)
			if renderErr == nil {
				// A historical match is never sufficient. The expected state must
				// have a visible candidate after the action barrier. Repeated labels
				// additionally require a post-barrier dismissal before reappearance.
				if spec.Dismissal {
					if !strings.Contains(spec.Barrier.Baseline, spec.Expected) {
						return "", "", fmt.Errorf("dismissal action %d lacked baseline %q", spec.Sequence, spec.Expected)
					}
				}
				expected := strings.Split(spec.Expected, " | ")
				candidate, end, candidateErr := renderedScreenContainingAfter(raw, spec.Barrier.StartOffset, ptyRows, ptyCols, expected, strings.Contains(spec.Barrier.Baseline, firstExpected(spec.Expected)))
				if spec.Dismissal {
					candidate, end, candidateErr = renderedScreenAbsentAfterCandidate(raw, spec.Barrier.StartOffset, ptyRows, ptyCols, expected)
				}
				if candidateErr == nil {
					for _, expected := range expected {
						if !spec.Dismissal && !strings.Contains(candidate, expected) {
							candidateErr = fmt.Errorf("post-barrier screen did not render %q", expected)
							break
						}
					}
				}
				if candidateErr == nil {
					return c.sealAt(raw, candidate, spec, end, version)
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case err := <-ptyDone:
			return "", "", ptyExitedBefore("rendering "+fmt.Sprintf("%q", spec.Expected), err)
		case <-updates:
		case <-time.After(time.Until(deadline)):
			return "", "", fmt.Errorf("current PTY screen did not reach action %d state %q", spec.Sequence, spec.Expected)
		}
	}
	return "", "", fmt.Errorf("current PTY screen did not reach action %d state %q", spec.Sequence, spec.Expected)
}

func (c *freshFrameCollector) sealFinal(spec freshFrameSpec) (string, string, error) {
	raw, _, eof, err := c.snapshot()
	if err != nil {
		return "", "", err
	}
	if !eof {
		return "", "", fmt.Errorf("fresh PTY collector has not drained")
	}
	if len(raw) < c.lastEnd {
		return "", "", fmt.Errorf("typescript shrank from %d to %d bytes", c.lastEnd, len(raw))
	}
	screen, err := currentScreen(raw, ptyRows, ptyCols)
	if err != nil {
		return "", "", err
	}
	return c.seal(raw, screen, spec)
}

func (c *freshFrameCollector) readGrowingPrefix() ([]byte, error) {
	raw, _, _, err := c.snapshot()
	if err != nil {
		return nil, err
	}
	if len(raw) <= c.lastEnd {
		return nil, fmt.Errorf("typescript has not grown beyond sealed offset %d", c.lastEnd)
	}
	return raw, nil
}

func (c *freshFrameCollector) seal(raw []byte, screen string, spec freshFrameSpec) (string, string, error) {
	return c.sealAt(raw, screen, spec, len(raw), 0)
}

func (c *freshFrameCollector) sealAt(raw []byte, screen string, spec freshFrameSpec, matchEnd int, matchVersion uint64) (string, string, error) {
	if len(raw) < c.lastEnd {
		return "", "", fmt.Errorf("typescript shrank from %d to %d bytes", c.lastEnd, len(raw))
	}
	start, end := c.lastEnd, len(raw)
	if end == start && spec.Sequence != 5 {
		return "", "", fmt.Errorf("action %d has no new typescript bytes", spec.Sequence)
	}
	screenPath := filepath.Join(c.artifactRoot, spec.Artifact+"-screen.txt")
	framePath := filepath.Join(c.artifactRoot, spec.Artifact+"-frame.json")
	if err := writeNewArtifact(screenPath, []byte(screen)); err != nil {
		return "", "", err
	}
	if matchEnd < spec.Barrier.StartOffset || matchEnd > end {
		return "", "", fmt.Errorf("invalid action match offset %d", matchEnd)
	}
	record := freshFrameRecord{Sequence: spec.Sequence, Action: spec.Action, InputSHA256: digestBytes([]byte(spec.Input)), Expected: spec.Expected, Start: start, End: end, PrefixSHA256: digestBytes(raw[:end]), RenderSHA256: digestBytes([]byte(screen)), ConversationID: spec.ConversationID, RunID: spec.RunID, ActionStartOffset: spec.Barrier.StartOffset, ActionStartVersion: spec.Barrier.StartVersion, MatchEnd: matchEnd, MatchVersion: matchVersion}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := writeNewArtifact(framePath, append(encoded, '\n')); err != nil {
		return "", "", err
	}
	c.lastEnd = end
	return screenPath, framePath, nil
}

func firstExpected(expected string) string { return strings.Split(expected, " | ")[0] }

// renderedScreenContainingAfter returns a visible candidate whose terminal
// bytes end after the action barrier. It deliberately does not search earlier
// frames, even when the same label is still present in retained history.
func renderedScreenContainingAfter(raw []byte, start, rows, cols int, expected []string, requireFalseThenTrue bool) (string, int, error) {
	falseSeen := !requireFalseThenTrue
	for _, end := range semanticVTBoundaries(raw, start) {
		screen, err := currentScreen(raw[:end], rows, cols)
		if err != nil {
			continue
		}
		match := true
		for _, want := range expected {
			match = match && strings.Contains(screen, want)
		}
		if !match {
			falseSeen = true
			continue
		}
		if falseSeen {
			return screen, end, nil
		}
	}
	return "", 0, fmt.Errorf("no post-barrier rendered screen reaches %q", strings.Join(expected, " | "))
}

func renderedScreenAbsentAfter(raw []byte, start int, expected string) bool {
	_, _, err := renderedScreenAbsentAfterCandidate(raw, start, ptyRows, ptyCols, []string{expected})
	return err == nil
}

func renderedScreenAbsentAfterCandidate(raw []byte, start, rows, cols int, expected []string) (string, int, error) {
	for _, end := range semanticVTBoundaries(raw, start) {
		screen, err := currentScreen(raw[:end], rows, cols)
		if err != nil {
			continue
		}
		match := true
		for _, want := range expected {
			match = match && strings.Contains(screen, want)
		}
		if !match {
			return screen, end, nil
		}
	}
	return "", 0, fmt.Errorf("no post-barrier rendered screen dismisses %q", strings.Join(expected, " | "))
}

func semanticVTBoundaries(raw []byte, start int) []int {
	ends := make([]int, 0, 4)
	for offset := start; offset < len(raw); {
		home := bytes.Index(raw[offset:], []byte("\x1b[H"))
		exit := bytes.Index(raw[offset:], []byte("\x1b[?1049l"))
		if home < 0 && exit < 0 {
			break
		}
		if home >= 0 && (exit < 0 || home < exit) {
			offset += home + len("\x1b[H")
		} else {
			offset += exit + len("\x1b[?1049l")
		}
		ends = append(ends, offset)
	}
	if len(raw) > start {
		ends = append(ends, len(raw))
	}
	return ends
}

func writeNewArtifact(path string, content []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func captureScreenContaining(terminalPath, artifactRoot, artifactName, expected string) (string, error) {
	raw, err := os.ReadFile(terminalPath)
	if err != nil {
		return "", err
	}
	screen, err := renderedScreenContaining(raw, ptyRows, ptyCols, expected)
	if err != nil {
		return "", err
	}
	path := filepath.Join(artifactRoot, artifactName)
	if err := os.WriteFile(path, []byte(screen), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func ptyExitedBefore(action string, err error) error {
	if err != nil {
		return fmt.Errorf("PTY harnesscli exited before %s: %w", action, err)
	}
	return fmt.Errorf("PTY harnesscli exited before %s", action)
}

// renderedScreenContaining returns the most recent visible VT frame that
// contains expected. Bubble Tea may issue a subsequent blank redraw while the
// process is being shut down, so parsing only the very last byte in a PTY
// recording loses a response that was genuinely rendered. Each leading CUP
// home sequence and alternate-buffer exit delimit a frame; inspect the prior
// current screen before applying either transition.
func renderedScreenContaining(raw []byte, rows, cols int, expected string) (string, error) {
	var latest string
	frameStart := 0
	for offset := 0; offset < len(raw); {
		home := bytes.Index(raw[offset:], []byte("\x1b[H"))
		exit := bytes.Index(raw[offset:], []byte("\x1b[?1049l"))
		next, width := home, len("\x1b[H")
		if next < 0 || (exit >= 0 && exit < next) {
			next, width = exit, len("\x1b[?1049l")
		}
		if next < 0 {
			break
		}
		end := offset + next
		for _, frame := range [][]byte{raw[:end], raw[frameStart:end]} {
			screen, err := currentScreen(frame, rows, cols)
			if err != nil {
				return "", err
			}
			if strings.Contains(screen, expected) {
				latest = screen
			}
		}
		offset = end + width
		frameStart = offset
	}
	screen, err := currentScreen(raw, rows, cols)
	if err != nil {
		return "", err
	}
	if strings.Contains(screen, expected) {
		latest = screen
	}
	if latest == "" {
		return "", fmt.Errorf("current PTY screen did not render %q", expected)
	}
	return latest, nil
}

// currentScreen interprets the small, standard VT subset emitted by Bubble Tea
// rather than treating the PTY recording as plain text. In particular, the
// TUI uses an alternate buffer, cursor movement, and erase controls, so raw
// substring matches can prove text that is no longer visible to a user.
func currentScreen(raw []byte, rows, cols int) (string, error) {
	if rows <= 0 || cols <= 0 {
		return "", fmt.Errorf("invalid terminal geometry %dx%d", rows, cols)
	}
	primary := newVTBuffer(rows, cols)
	alternate := newVTBuffer(rows, cols)
	active := primary
	for i := 0; i < len(raw); {
		b := raw[i]
		if b == 0x1b {
			if i+1 >= len(raw) {
				return "", fmt.Errorf("truncated escape sequence")
			}
			if raw[i+1] != '[' {
				i += 2
				continue
			}
			j := i + 2
			for j < len(raw) && (raw[j] < 0x40 || raw[j] > 0x7e) {
				j++
			}
			if j == len(raw) {
				return "", fmt.Errorf("truncated CSI sequence")
			}
			params, final := string(raw[i+2:j]), raw[j]
			switch final {
			case 'h':
				if params == "?1049" {
					active = alternate
					active.clear()
				} else if params == "?1047" || params == "?47" {
					active = alternate
				}
			case 'l':
				if params == "?1049" || params == "?1047" || params == "?47" {
					active = primary
				}
			default:
				active.csi(params, final)
			}
			i = j + 1
			continue
		}
		switch b {
		case '\r':
			active.x = 0
			active.wrapPending = false
		case '\n':
			active.lineFeed()
		case '\b':
			active.wrapPending = false
			if active.x > 0 {
				active.x--
			}
		case '\t':
			active.x = min(active.cols-1, ((active.x/8)+1)*8)
		default:
			if b >= 0x20 && b != 0x7f {
				ch, size := utf8.DecodeRune(raw[i:])
				active.put(ch)
				i += size
				continue
			}
		}
		i++
	}
	return active.string(), nil
}

type vtBuffer struct {
	rows, cols, x, y int
	wrapPending      bool
	grid             [][]string
}

func newVTBuffer(rows, cols int) *vtBuffer {
	b := &vtBuffer{rows: rows, cols: cols, grid: make([][]string, rows)}
	for y := range b.grid {
		b.grid[y] = make([]string, cols)
		for x := range b.grid[y] {
			b.grid[y][x] = " "
		}
	}
	return b
}

func (b *vtBuffer) clear() {
	for y := range b.grid {
		for x := range b.grid[y] {
			b.grid[y][x] = " "
		}
	}
	b.x, b.y, b.wrapPending = 0, 0, false
}

func (b *vtBuffer) lineFeed() {
	b.wrapPending = false
	if b.y < b.rows-1 {
		b.y++
		return
	}
	copy(b.grid, b.grid[1:])
	for x := range b.grid[b.rows-1] {
		b.grid[b.rows-1][x] = " "
	}
}

func (b *vtBuffer) put(ch rune) {
	width := runewidth.RuneWidth(ch)
	if width == 0 {
		for x := b.x - 1; x >= 0; x-- {
			if b.grid[b.y][x] != "" && b.grid[b.y][x] != " " {
				b.grid[b.y][x] += string(ch)
				return
			}
		}
		return
	}
	if b.wrapPending {
		b.x = 0
		b.lineFeed()
	}
	if width > 1 && b.x == b.cols-1 {
		b.x = 0
		b.lineFeed()
	}
	b.clearCell(b.y, b.x)
	b.grid[b.y][b.x] = string(ch)
	if width > 1 {
		b.clearCell(b.y, b.x+1)
		b.grid[b.y][b.x+1] = ""
	}
	b.x += width
	if b.x >= b.cols {
		b.x = b.cols - 1
		b.wrapPending = true
	}
}

func (b *vtBuffer) clearCell(y, x int) {
	if b.grid[y][x] == "" {
		for left := x - 1; left >= 0; left-- {
			if b.grid[y][left] != "" {
				b.grid[y][left] = " "
				break
			}
		}
	} else if runewidth.StringWidth(b.grid[y][x]) > 1 && x+1 < b.cols {
		b.grid[y][x+1] = " "
	}
	b.grid[y][x] = " "
}

func (b *vtBuffer) csi(params string, final byte) {
	values := csiValues(params)
	value := func(index, fallback int) int {
		if index >= len(values) || values[index] == 0 {
			return fallback
		}
		return values[index]
	}
	switch final {
	case 'A':
		b.wrapPending = false
		b.y = max(0, b.y-value(0, 1))
	case 'B':
		b.wrapPending = false
		b.y = min(b.rows-1, b.y+value(0, 1))
	case 'C':
		b.wrapPending = false
		b.x = min(b.cols-1, b.x+value(0, 1))
	case 'D':
		b.wrapPending = false
		b.x = max(0, b.x-value(0, 1))
	case 'G':
		b.wrapPending = false
		b.x = min(b.cols-1, value(0, 1)-1)
	case 'H', 'f':
		b.wrapPending = false
		b.y = min(b.rows-1, value(0, 1)-1)
		b.x = min(b.cols-1, value(1, 1)-1)
	case 'J':
		if value(0, 0) != 3 {
			b.wrapPending = false
		}
		b.eraseDisplay(value(0, 0))
	case 'K':
		b.wrapPending = false
		b.eraseLine(value(0, 0))
	}
}

func (b *vtBuffer) eraseDisplay(mode int) {
	switch mode {
	case 0:
		b.eraseLine(0)
		for y := b.y + 1; y < b.rows; y++ {
			for x := range b.grid[y] {
				b.grid[y][x] = " "
			}
		}
	case 1:
		for y := 0; y < b.y; y++ {
			for x := range b.grid[y] {
				b.grid[y][x] = " "
			}
		}
		b.eraseLine(1)
	case 2:
		for y := range b.grid {
			for x := range b.grid[y] {
				b.grid[y][x] = " "
			}
		}
	}
}

func (b *vtBuffer) eraseLine(mode int) {
	start, end := b.x, b.cols
	if mode == 1 {
		start, end = 0, b.x+1
	}
	if mode == 2 {
		start, end = 0, b.cols
	}
	for x := start; x < end; x++ {
		b.clearCell(b.y, x)
	}
}

func csiValues(params string) []int {
	params = strings.TrimPrefix(params, "?")
	if params == "" {
		return nil
	}
	values := make([]int, 0, strings.Count(params, ";")+1)
	for _, part := range strings.Split(params, ";") {
		var value int
		for _, digit := range part {
			if digit >= '0' && digit <= '9' {
				value = value*10 + int(digit-'0')
			}
		}
		values = append(values, value)
	}
	return values
}

func (b *vtBuffer) string() string {
	lines := make([]string, b.rows)
	for y, row := range b.grid {
		lines[y] = strings.TrimRight(strings.Join(row, ""), " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func startAndComplete(ctx context.Context, client *http.Client, base, prompt string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{"prompt": prompt})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("start source: %s: %s", resp.Status, raw)
	}
	var started struct {
		RunID          string `json:"run_id"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&started); err != nil {
		return "", "", err
	}
	if started.RunID == "" {
		return "", "", fmt.Errorf("source omitted run identity")
	}
	if _, _, err := stream(ctx, client, base, started.RunID); err != nil {
		return "", "", err
	}
	if started.ConversationID == "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/runs/"+started.RunID, nil)
		resp, err := client.Do(req)
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", "", fmt.Errorf("source terminal probe: %s", resp.Status)
		}
		var terminal struct {
			Status         string `json:"status"`
			ConversationID string `json:"conversation_id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&terminal); err != nil {
			return "", "", err
		}
		if terminal.Status != "completed" || terminal.ConversationID == "" {
			return "", "", fmt.Errorf("source terminal probe omitted completed conversation identity")
		}
		started.ConversationID = terminal.ConversationID
	}
	return started.RunID, started.ConversationID, nil
}
func waitForChild(ctx context.Context, client *http.Client, base, conversation, source string, timeout time.Duration, ptyDone <-chan error) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-ptyDone:
			return "", ptyExitedBefore("creating child run", err)
		default:
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/runs?conversation_id="+conversation, nil)
		resp, err := client.Do(req)
		if err == nil {
			var value struct {
				Runs []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"runs"`
			}
			err = json.NewDecoder(resp.Body).Decode(&value)
			resp.Body.Close()
			if err == nil {
				for _, run := range value.Runs {
					if run.ID != source && run.Status == "completed" {
						return run.ID, nil
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case err := <-ptyDone:
			return "", ptyExitedBefore("creating child run", err)
		case <-time.After(50 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("PTY did not create a completed child run")
}

func waitForCompletedPromptRun(ctx context.Context, client *http.Client, base, prompt, exclude string, timeout time.Duration, ptyDone <-chan error) (string, string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-ptyDone:
			return "", "", ptyExitedBefore("creating completed prompt run", err)
		default:
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/runs", nil)
		resp, err := client.Do(req)
		if err == nil {
			var value struct {
				Runs []struct {
					ID             string `json:"id"`
					ConversationID string `json:"conversation_id"`
					Prompt         string `json:"prompt"`
					Status         string `json:"status"`
				} `json:"runs"`
			}
			err = json.NewDecoder(resp.Body).Decode(&value)
			resp.Body.Close()
			if err == nil {
				for _, run := range value.Runs {
					if run.ID != exclude && run.Prompt == prompt && run.Status == "completed" && run.ConversationID != "" {
						return run.ID, run.ConversationID, nil
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case err := <-ptyDone:
			return "", "", ptyExitedBefore("creating completed prompt run", err)
		case <-time.After(50 * time.Millisecond):
		}
	}
	return "", "", fmt.Errorf("PTY did not create completed run for prompt %q", prompt)
}

// requireCompletedRunInConversation is deliberately a separate direct API
// probe from waitForCompletedPromptRun: it pins that the exact continuation
// target is terminal in the same durable conversation before the TUI types it.
func requireCompletedRunInConversation(ctx context.Context, client *http.Client, base, runID, conversation string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/runs/"+runID, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET run %s: %s", runID, resp.Status)
	}
	var run struct {
		Status         string `json:"status"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return err
	}
	if run.Status != "completed" || run.ConversationID != conversation {
		return fmt.Errorf("run %s status=%q conversation=%q, want completed/%q", runID, run.Status, run.ConversationID, conversation)
	}
	return nil
}
func stream(ctx context.Context, client *http.Client, base, runID string) ([]byte, map[string]int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/runs/"+runID+"/events", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("SSE %s", resp.Status)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	events := map[string]int{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		if event, ok := strings.CutPrefix(scanner.Text(), "event: "); ok {
			events[event]++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return raw, events, nil
}
func apiStoreProbe(ctx context.Context, client *http.Client, base, source, child, conversation string) ([]byte, string, error) {
	urls := []string{base + "/v1/runs/" + source, base + "/v1/runs/" + child, base + "/v1/conversations/" + conversation + "/messages"}
	var parts [][]byte
	childConversation := ""
	for i, url := range urls {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", err
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("probe %s: %s", url, resp.Status)
		}
		if i == 1 {
			var run struct {
				ConversationID string `json:"conversation_id"`
				Status         string `json:"status"`
			}
			if json.Unmarshal(raw, &run) != nil || run.Status != "completed" {
				return nil, "", fmt.Errorf("child terminal probe invalid")
			}
			childConversation = run.ConversationID
		}
		parts = append(parts, raw)
	}
	return bytes.Join(parts, []byte("\n")), childConversation, nil
}

func freshAPIStoreProbe(ctx context.Context, client *http.Client, base, first, second, conversation string) ([]byte, error) {
	urls := []string{base + "/v1/runs/" + first, base + "/v1/runs/" + second, base + "/v1/conversations/" + conversation + "/messages"}
	parts := make([][]byte, 0, len(urls))
	for _, endpoint := range urls {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fresh probe %s: %s", endpoint, resp.Status)
		}
		parts = append(parts, raw)
	}
	var messages struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(parts[2], &messages); err != nil {
		return nil, fmt.Errorf("decode fresh conversation messages: %w", err)
	}
	counts := map[string]int{}
	for _, message := range messages.Messages {
		if message.Role == "assistant" {
			counts[message.Content]++
		}
	}
	for _, reply := range []string{"FIRST_REPLY", "SECOND_REPLY"} {
		if counts[reply] != 1 {
			return nil, fmt.Errorf("assistant message %q count = %d, want 1", reply, counts[reply])
		}
	}
	return bytes.Join(parts, []byte("\n")), nil
}

// nonMutatingAPIStoreProbe proves that both spellings of continuation add one
// assistant message to the same durable conversation without crediting a
// terminal rendering frame as durable state.
func nonMutatingAPIStoreProbe(ctx context.Context, client *http.Client, base, source, resume, continued, conversation string) ([]byte, error) {
	urls := []string{
		base + "/v1/runs/" + source,
		base + "/v1/runs/" + resume,
		base + "/v1/runs/" + continued,
		base + "/v1/conversations/" + conversation + "/messages",
	}
	parts := make([][]byte, 0, len(urls))
	for _, endpoint := range urls {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("non-mutating probe %s: %s", endpoint, resp.Status)
		}
		parts = append(parts, raw)
	}
	for i, raw := range parts[:3] {
		var run struct {
			ConversationID string `json:"conversation_id"`
			Status         string `json:"status"`
		}
		if err := json.Unmarshal(raw, &run); err != nil || run.Status != "completed" || run.ConversationID != conversation {
			return nil, fmt.Errorf("non-mutating run probe %d is not completed in conversation %q", i, conversation)
		}
	}
	var messages struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(parts[3], &messages); err != nil {
		return nil, fmt.Errorf("decode non-mutating conversation messages: %w", err)
	}
	counts := map[string]int{}
	for _, message := range messages.Messages {
		if message.Role == "assistant" {
			counts[message.Content]++
		}
	}
	for _, reply := range []string{"FIRST_REPLY", "RESUME_REPLY", "CONTINUE_REPLY"} {
		if counts[reply] != 1 {
			return nil, fmt.Errorf("assistant message %q count = %d, want 1", reply, counts[reply])
		}
	}
	return bytes.Join(parts, []byte("\n")), nil
}
func digestPath(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func invocationForCase(compiled inventory.Compiled, c inventory.Case) (inventory.Invocation, error) {
	if c.ItemID != "tui_command:resume" || c.EvidenceClass != inventory.EvidenceClassConversation || len(c.Surfaces) != 1 || c.Surfaces[0] != inventory.SurfaceTUI {
		return inventory.Invocation{}, fmt.Errorf("PTY continuation case must target the resume TUI conversation surface")
	}
	for _, item := range compiled.Items {
		if item.ID != c.ItemID {
			continue
		}
		for _, invocation := range item.Invocations {
			if invocation.ID == c.InvocationID {
				return invocation, nil
			}
		}
	}
	return inventory.Invocation{}, fmt.Errorf("PTY continuation case has unknown invocation %q", c.InvocationID)
}

func checkedArtifactRefs(root string, paths map[string]string) ([]inventory.ArtifactRef, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("canonicalize artifact root: %w", err)
	}
	specs := []struct {
		name string
		kind inventory.ArtifactKind
	}{
		{"terminal", inventory.ArtifactTerminalCapture}, {"screen", inventory.ArtifactTranscript},
		{"keystrokes", inventory.ArtifactEventLog}, {"sse", inventory.ArtifactRawSSEEvent},
		{"api_store", inventory.ArtifactAPIStoreProbe},
	}
	refs := make([]inventory.ArtifactRef, 0, len(specs))
	for _, spec := range specs {
		path := paths[spec.name]
		if path == "" {
			return nil, fmt.Errorf("missing %s artifact path", spec.name)
		}
		canonicalPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("canonicalize %s artifact: %w", spec.name, err)
		}
		rel, err := filepath.Rel(canonicalRoot, canonicalPath)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("%s artifact escapes artifact root", spec.name)
		}
		digest, err := digestPath(canonicalPath)
		if err != nil {
			return nil, fmt.Errorf("recompute %s artifact digest: %w", spec.name, err)
		}
		redacted := true
		refs = append(refs, inventory.ArtifactRef{Kind: spec.kind, Path: canonicalPath, Digest: digest, Redacted: &redacted})
	}
	return refs, nil
}

func eventIDs(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SSE evidence: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	seen := map[string]struct{}{}
	var ids []string
	for scanner.Scan() {
		if id, ok := strings.CutPrefix(scanner.Text(), "id: "); ok && strings.TrimSpace(id) != "" {
			id = strings.TrimSpace(id)
			if _, duplicate := seen[id]; !duplicate {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan SSE evidence: %w", err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("SSE evidence has no event identities")
	}
	return ids, nil
}

func verifyArtifactRefs(refs []inventory.ArtifactRef) error {
	for _, ref := range refs {
		canonicalPath, err := filepath.EvalSymlinks(ref.Path)
		if err != nil {
			return fmt.Errorf("artifact %q is missing or cannot be canonicalized: %w", ref.Path, err)
		}
		if canonicalPath != ref.Path {
			return fmt.Errorf("artifact path is not canonical: %q", ref.Path)
		}
		digest, err := digestPath(canonicalPath)
		if err != nil {
			return fmt.Errorf("recompute artifact %q digest: %w", ref.Path, err)
		}
		if digest != ref.Digest {
			return fmt.Errorf("artifact %q digest changed", ref.Path)
		}
	}
	return nil
}
