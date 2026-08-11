package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"go-agent-harness/cmd/harnesscli/tui/components/messagebubble"
	"go-agent-harness/cmd/harnesscli/tui/components/transcriptexport"
)

// initAgentsPrompt is the fixed prompt /init sends through the normal run
// path. It instructs the model to inspect the workspace and return only the
// AGENTS.md markdown; the TUI writes that markdown to <workspace>/AGENTS.md
// when the run completes. Keep it deterministic: the completion write path
// (extractAgentsMarkdown) relies on the reply being markdown, optionally
// wrapped in a single code fence.
const initAgentsPrompt = `Analyze this repository and write an AGENTS.md file for it.

AGENTS.md gives coding agents the context they need to work effectively in a repo. Base everything on what you actually find in the workspace — never invent commands, structure, or conventions.

Cover, in this order:
1. Overview: one paragraph on what the project is and does.
2. Layout: the top-level directories and what each contains.
3. Commands: exact build, test, and lint commands verified against the repo's own tooling (go.mod, package.json, Makefile, CI config, etc.).
4. Conventions: code style, testing patterns, and commit/PR rules an agent must follow.

Keep the result under 150 lines. Output ONLY the final markdown content for AGENTS.md — no preamble, no explanation, no trailing commentary.`

// executeInitCommand implements /init: it runs initAgentsPrompt against the
// current workspace via the normal run path and, when the run completes,
// writes the assistant's markdown to <workspace>/AGENTS.md (see
// completeInitAgentsMd, driven by pendingInitAgentsMd). An existing AGENTS.md
// is only overwritten when the user passes the explicit "confirm" token —
// the same approval pattern as /rewind <id> confirm.
func executeInitCommand(m *Model, cmd Command) ([]tea.Cmd, bool) {
	if len(cmd.Args) > 1 || (len(cmd.Args) == 1 && cmd.Args[0] != "confirm") {
		return []tea.Cmd{m.setStatusMsg("Usage: /init [confirm] — generate AGENTS.md for this workspace")}, false
	}
	confirmed := len(cmd.Args) == 1

	ws := m.initWorkspace()
	target := filepath.Join(ws, "AGENTS.md")
	_, statErr := os.Stat(target)
	if statErr == nil && !confirmed {
		return []tea.Cmd{m.setStatusMsg("AGENTS.md already exists — run /init confirm to overwrite")}, false
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return []tea.Cmd{m.setStatusMsg("Could not inspect AGENTS.md: " + statErr.Error())}, false
	}
	if m.runActive {
		return []tea.Cmd{m.setStatusMsg("A run is already active — wait for it to finish before /init")}, false
	}

	// Record the generation prompt as the user turn and start the run. The
	// transcript keeps the literal prompt (the truthful record of what was
	// sent); the bubble shows a short label to keep the viewport readable.
	m.pendingInitAgentsMd = true
	m.pendingInitRunID = ""
	m.pendingInitTarget = target
	m.pendingInitTargetExisted = statErr == nil
	m.lastAssistantText = ""
	m.responseStarted = false
	m.activeAssistantLineCount = 0
	m.clearThinkingBar()
	m.pendingLastMsg = "/init — generate AGENTS.md"
	m.transcript = append(m.transcript, transcriptexport.TranscriptEntry{
		Role:      "user",
		Content:   initAgentsPrompt,
		Timestamp: time.Now(),
	})
	m.appendMessageBubble(messagebubble.RoleUser, "Generate AGENTS.md for this workspace (/init)")
	effModel, effProvider := m.effectiveModelAndProvider()
	return []tea.Cmd{
		m.setStatusMsg("Generating AGENTS.md..."),
		startRunCmd(m.config.BaseURL, initAgentsPrompt, m.conversationID, effModel, effProvider, m.selectedReasoningEffort, m.selectedProfile, ws, m.config.APIKey, nil, m.extraDirs),
	}, false
}

// completeInitAgentsMd writes only the generated markdown from the exact
// accepted successful /init run. The target is re-statted immediately before
// commit so a tool or another process cannot silently create a file that this
// run then replaces.
func (m *Model) completeInitAgentsMd(runID string) tea.Cmd {
	if !m.clearPendingInitAgentsMd(runID) {
		return nil
	}
	content := extractAgentsMarkdown(m.lastAssistantText)
	if content == "" {
		return m.setStatusMsg("AGENTS.md not written — the run produced no markdown")
	}
	path := m.pendingInitTarget
	if path == "" {
		path = filepath.Join(m.initWorkspace(), "AGENTS.md")
	}
	if err := writeAgentsMarkdownAtomically(path, []byte(content+"\n"), m.pendingInitTargetExisted); err != nil {
		return m.setStatusMsg("Could not write AGENTS.md: " + err.Error())
	}
	return m.setStatusMsg("Wrote " + path)
}

// clearPendingInitAgentsMd consumes only the pending state owned by runID. An
// empty or foreign identity never grants permission to write a workspace file.
func (m *Model) clearPendingInitAgentsMd(runID string) bool {
	if !m.pendingInitAgentsMd || runID == "" || runID != m.pendingInitRunID {
		return false
	}
	m.pendingInitAgentsMd = false
	m.pendingInitRunID = ""
	return true
}

// clearUnboundPendingInitAgentsMd consumes /init only when the harness failed
// before accepting a run. startRunCmd reports those failures with an empty
// RunID; accepting any later RunStartedMsg would otherwise let that unrelated
// run inherit this pending workspace write. A bound or foreign identity remains
// governed by clearPendingInitAgentsMd.
func (m *Model) clearUnboundPendingInitAgentsMd() bool {
	if !m.pendingInitAgentsMd || m.pendingInitRunID != "" {
		return false
	}
	m.pendingInitAgentsMd = false
	return true
}

func writeAgentsMarkdownAtomically(path string, content []byte, existedAtStart bool) (err error) {
	info, statErr := os.Stat(path)
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect target: %w", statErr)
	}
	if statErr == nil && !existedAtStart {
		return fmt.Errorf("AGENTS.md appeared while generating; it was not overwritten (run /init confirm again)")
	}

	mode := os.FileMode(0o644)
	if statErr == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".AGENTS.md-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if err = tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	if _, err = tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace AGENTS.md: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open workspace directory for sync: %w", err)
	}
	defer directory.Close()
	if err = directory.Sync(); err != nil {
		return fmt.Errorf("sync workspace directory: %w", err)
	}
	return nil
}

// initWorkspace returns the workspace root for /init, defaulting to the
// process working directory when the TUI was started without one (mirrors
// resolveWorkspacePath in cmd/harnesscli).
func (m Model) initWorkspace() string {
	if ws := strings.TrimSpace(m.config.Workspace); ws != "" {
		return ws
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// extractAgentsMarkdown converts a /init run's assistant reply into file
// content: surrounding whitespace is trimmed and a single wrapping code fence
// (```markdown or bare ```) is removed when present. Fences inside the
// document are preserved.
func extractAgentsMarkdown(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	lines = lines[1:] // drop the opening fence line
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "```" {
		lines = lines[:n-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
