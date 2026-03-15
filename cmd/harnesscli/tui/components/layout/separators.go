package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Separator renders a full-width horizontal rule.
type Separator struct {
	width     int
	asciiMode bool
}

// NewSeparator creates a separator for the given width.
// If asciiMode is true, uses '-' instead of '─'.
func NewSeparator(width int, asciiMode bool) Separator {
	return Separator{width: width, asciiMode: asciiMode}
}

var sepStyle = lipgloss.NewStyle().Faint(true)

// Render returns the separator line string.
func (s Separator) Render() string {
	if s.width <= 0 {
		return ""
	}
	char := "─"
	if s.asciiMode {
		char = "-"
	}
	return sepStyle.Render(strings.Repeat(char, s.width))
}

// DialogBox renders a bordered box with title and content.
type DialogBox struct {
	width     int
	height    int
	asciiMode bool
}

// NewDialogBox creates a dialog box.
func NewDialogBox(width, height int, asciiMode bool) DialogBox {
	return DialogBox{width: width, height: height, asciiMode: asciiMode}
}

var dialogStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.AdaptiveColor{Light: "#D9D9D9", Dark: "#383838"}).
	Padding(0, 1)

var dialogStyleASCII = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.AdaptiveColor{Light: "#D9D9D9", Dark: "#383838"}).
	Padding(0, 1)

// Render renders the dialog with title and content.
func (d DialogBox) Render(title, content string) string {
	style := dialogStyle
	if d.asciiMode {
		style = dialogStyleASCII
	}
	if d.width > 4 {
		style = style.Width(d.width - 4)
	}

	titleLine := lipgloss.NewStyle().Bold(true).Render(title)
	body := titleLine + "\n\n" + content
	return style.Render(body)
}
