package transcriptexport

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ExportStatus represents the current state of the export operation.
type ExportStatus int

const (
	ExportStatusIdle    ExportStatus = iota
	ExportStatusSuccess              // export completed successfully
	ExportStatusError                // export failed
)

// color definitions matching the TUI theme.
var (
	successColor = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	errorColor   = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF5F87"}
)

// StatusModel is a BubbleTea-compatible model that displays the export status.
// It uses immutable value semantics — all mutation methods return a new StatusModel.
type StatusModel struct {
	Status   ExportStatus
	FilePath string // set on success
	ErrMsg   string // set on error
}

// NewStatusModel creates a StatusModel in the idle state.
func NewStatusModel() StatusModel {
	return StatusModel{Status: ExportStatusIdle}
}

// SetSuccess returns a new StatusModel in the success state with the given file path.
func (m StatusModel) SetSuccess(path string) StatusModel {
	return StatusModel{
		Status:   ExportStatusSuccess,
		FilePath: path,
	}
}

// SetError returns a new StatusModel in the error state with the given error message.
func (m StatusModel) SetError(errMsg string) StatusModel {
	return StatusModel{
		Status: ExportStatusError,
		ErrMsg: errMsg,
	}
}

// Reset returns a new StatusModel in the idle state.
func (m StatusModel) Reset() StatusModel {
	return StatusModel{Status: ExportStatusIdle}
}

// View renders the export status into a string for display.
// width is used to constrain the output.
//   - Idle:    "" (empty string)
//   - Success: "✓ Transcript saved to {filepath}" in green
//   - Error:   "✗ Export failed: {errmsg}" in red
func (m StatusModel) View(width int) string {
	switch m.Status {
	case ExportStatusSuccess:
		text := fmt.Sprintf("✓ Transcript saved to %s", m.FilePath)
		style := lipgloss.NewStyle().
			Foreground(successColor).
			MaxWidth(width)
		return style.Render(text)
	case ExportStatusError:
		text := fmt.Sprintf("✗ Export failed: %s", m.ErrMsg)
		style := lipgloss.NewStyle().
			Foreground(errorColor).
			MaxWidth(width)
		return style.Render(text)
	default:
		return ""
	}
}
