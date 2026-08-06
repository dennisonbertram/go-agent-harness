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
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

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

// Result describes the source and child identities needed by callers to add a
// hash-bound acceptance record.
type Result struct {
	SourceRunID, ChildRunID, ConversationID string
	Artifacts                               map[string]string
	ArtifactPaths                           map[string]string
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
	child := []string{"sh", "-c", "stty rows 30 cols 100; exec \"$@\"", "sh", cli, "-tui", "-resume=" + source, "-base-url=" + base}
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
			if _, screenErr := renderedScreenContaining(raw, 30, 100, expected); screenErr == nil {
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
// home sequence begins a fresh frame; inspect the prior current screen before
// applying that redraw.
func renderedScreenContaining(raw []byte, rows, cols int, expected string) (string, error) {
	var latest string
	for offset := 0; offset < len(raw); {
		next := bytes.Index(raw[offset:], []byte("\x1b[H"))
		if next < 0 {
			break
		}
		end := offset + next
		screen, err := currentScreen(raw[:end], rows, cols)
		if err != nil {
			return "", err
		}
		if strings.Contains(screen, expected) {
			latest = screen
		}
		offset = end + len("\x1b[H")
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
				if params == "?1049" || params == "?1047" || params == "?47" {
					active = alternate
					active.clear()
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
		case '\n':
			active.lineFeed()
		case '\b':
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
	b.x, b.y = 0, 0
}

func (b *vtBuffer) lineFeed() {
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
		b.x = 0
		b.lineFeed()
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
		b.y = max(0, b.y-value(0, 1))
	case 'B':
		b.y = min(b.rows-1, b.y+value(0, 1))
	case 'C':
		b.x = min(b.cols-1, b.x+value(0, 1))
	case 'D':
		b.x = max(0, b.x-value(0, 1))
	case 'G':
		b.x = min(b.cols-1, value(0, 1)-1)
	case 'H', 'f':
		b.y = min(b.rows-1, value(0, 1)-1)
		b.x = min(b.cols-1, value(1, 1)-1)
	case 'J':
		b.eraseDisplay(value(0, 0))
	case 'K':
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
	case 2, 3:
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
