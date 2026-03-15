package statusbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Model is the status bar component displayed at the bottom of the TUI.
type Model struct {
	width    int
	model    string
	workdir  string
	branch   string
	permMode string // "", "plan", "accept-edits", "auto-approve"
	mcpFails int
	running  bool
	costUSD  float64
}

// New creates a new status bar model for the given terminal width.
func New(width int) Model {
	return Model{width: width}
}

func (m *Model) SetModel(name string)    { m.model = name }
func (m *Model) SetWorkdir(path string)  { m.workdir = path }
func (m *Model) SetBranch(branch string) { m.branch = branch }
func (m *Model) SetPermMode(mode string) { m.permMode = mode }
func (m *Model) SetMCPFailures(n int)    { m.mcpFails = n }
func (m *Model) SetRunning(r bool)       { m.running = r }
func (m *Model) SetCost(usd float64)     { m.costUSD = usd }
func (m *Model) SetWidth(w int)          { m.width = w }

// Styles for status bar segments.
var (
	dimStyle  = lipgloss.NewStyle().Faint(true)
	boldStyle = lipgloss.NewStyle().Bold(true)
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAF00"))
)

// View renders the status bar as a single line.
func (m Model) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	var parts []string

	// Model name (most prominent)
	if m.model != "" {
		parts = append(parts, boldStyle.Render(truncate(m.model, 24)))
	}

	// Working directory
	if m.workdir != "" {
		dir := shortenPath(m.workdir, 20)
		parts = append(parts, dimStyle.Render(dir))
	}

	// Git branch
	if m.branch != "" {
		parts = append(parts, dimStyle.Render("("+m.branch+")"))
	}

	// Permission mode
	if m.permMode != "" && m.permMode != "default" {
		parts = append(parts, warnStyle.Render("["+m.permMode+"]"))
	}

	// MCP failures
	if m.mcpFails > 0 {
		parts = append(parts, warnStyle.Render(fmt.Sprintf("%d MCP fail", m.mcpFails)))
	}

	// Running indicator
	if m.running {
		parts = append(parts, dimStyle.Render("..."))
	}

	// Cost (right-align if space permits)
	if m.costUSD > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("$%.4f", m.costUSD)))
	}

	sep := dimStyle.Render("  ~  ")
	line := strings.Join(parts, sep)

	return lipgloss.NewStyle().MaxWidth(w).Render(line)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "..."
}

func shortenPath(path string, max int) string {
	if len(path) <= max {
		return path
	}
	parts := strings.Split(path, "/")
	if len(parts) > 2 {
		return ".../" + parts[len(parts)-1]
	}
	return truncate(path, max)
}
