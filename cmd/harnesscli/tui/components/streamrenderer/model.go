package streamrenderer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// State represents the renderer's current phase.
type State int

const (
	StateIdle      State = iota
	StateThinking        // extended thinking phase
	StateStreaming        // streaming assistant text
	StateComplete        // finished streaming
)

// Model accumulates streaming text and manages display state.
type Model struct {
	state          State
	content        []string // assistant text chunks
	thinkingChunks []string // thinking deltas
	tokenCount     int
	durationSecs   float64
	maxLines       int // 0 = unlimited
}

var (
	dimStyle = lipgloss.NewStyle().Faint(true)
)

const defaultMaxLines = 500

// New creates a new stream renderer.
func New() Model {
	return Model{state: StateIdle, maxLines: defaultMaxLines}
}

// State returns the current rendering state.
func (m Model) State() State { return m.state }

// Content returns the accumulated assistant text.
func (m Model) Content() string { return strings.Join(m.content, "") }

// Summary returns the post-completion "Worked for Xs . Ntok" line.
func (m Model) Summary() string {
	if m.state != StateComplete {
		return ""
	}
	return dimStyle.Render(fmt.Sprintf("Worked for %.1fs · %d tokens", m.durationSecs, m.tokenCount))
}

// StartStreaming transitions to streaming state.
func (m *Model) StartStreaming() {
	m.state = StateStreaming
}

// StartThinking transitions to thinking state.
func (m *Model) StartThinking() {
	m.state = StateThinking
}

// AppendDelta adds a text chunk.
func (m *Model) AppendDelta(delta string) {
	if m.state == StateIdle {
		m.state = StateStreaming
	}
	m.content = append(m.content, delta)
}

// AppendThinkingDelta adds a thinking chunk.
func (m *Model) AppendThinkingDelta(delta string) {
	m.thinkingChunks = append(m.thinkingChunks, delta)
}

// Complete transitions to complete state with token + timing metadata.
func (m *Model) Complete(tokens int, durationSecs float64) {
	m.state = StateComplete
	m.tokenCount = tokens
	m.durationSecs = durationSecs
}

// Reset clears all state back to idle.
func (m *Model) Reset() {
	m.state = StateIdle
	m.content = nil
	m.thinkingChunks = nil
	m.tokenCount = 0
	m.durationSecs = 0
}

// View renders the current content for the given width.
func (m Model) View(width int) string {
	if width <= 0 {
		width = 80
	}

	var lines []string

	// Thinking section
	if len(m.thinkingChunks) > 0 && m.state == StateThinking {
		thinking := strings.Join(m.thinkingChunks, "")
		lines = append(lines, dimStyle.Render("Thinking: "+truncate(thinking, width-10)))
	}

	// Main content
	content := strings.Join(m.content, "")
	if content != "" {
		contentLines := strings.Split(content, "\n")
		lines = append(lines, contentLines...)
	}

	// Summary line
	if m.state == StateComplete && (m.tokenCount > 0 || m.durationSecs > 0) {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("Worked for %.1fs · %d tokens", m.durationSecs, m.tokenCount)))
	}

	// Truncate if too many lines
	if m.maxLines > 0 && len(lines) > m.maxLines {
		lines = lines[len(lines)-m.maxLines:]
		lines = append([]string{dimStyle.Render("... (truncated) ...")}, lines...)
	}

	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	if max <= 3 {
		return "..."
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
