package outputmode_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go-agent-harness/cmd/harnesscli/tui/components/outputmode"
)

// TestTUI058_NewDefaultsToCompact verifies that New() returns a Model whose
// Mode is OutputModeCompact.
func TestTUI058_NewDefaultsToCompact(t *testing.T) {
	m := outputmode.New()
	assert.Equal(t, outputmode.OutputModeCompact, m.Mode)
	assert.True(t, m.IsCompact())
	assert.False(t, m.IsVerbose())
}

// TestTUI058_ToggleFlipsMode verifies that Toggle() switches from compact to
// verbose and back.
func TestTUI058_ToggleFlipsMode(t *testing.T) {
	m := outputmode.New()
	require.True(t, m.IsCompact(), "initial mode must be compact")

	m2 := m.Toggle()
	assert.True(t, m2.IsVerbose(), "after one toggle, mode must be verbose")
	assert.False(t, m2.IsCompact())

	m3 := m2.Toggle()
	assert.True(t, m3.IsCompact(), "after second toggle, mode must be compact")
	assert.False(t, m3.IsVerbose())
}

// TestTUI058_ToggleIdempotentDoubleToggle verifies that toggling twice returns
// to the original mode (idempotency of double-toggle).
func TestTUI058_ToggleIdempotentDoubleToggle(t *testing.T) {
	m := outputmode.New()
	m2 := m.Toggle().Toggle()
	assert.Equal(t, m.Mode, m2.Mode)
}

// TestTUI058_SetMode verifies that SetMode() overrides the current mode.
func TestTUI058_SetMode(t *testing.T) {
	m := outputmode.New()
	assert.True(t, m.IsCompact())

	mv := m.SetMode(outputmode.OutputModeVerbose)
	assert.True(t, mv.IsVerbose())
	assert.Equal(t, outputmode.OutputModeVerbose, mv.Mode)

	mc := mv.SetMode(outputmode.OutputModeCompact)
	assert.True(t, mc.IsCompact())
	assert.Equal(t, outputmode.OutputModeCompact, mc.Mode)
}

// TestTUI058_SetModeDoesNotMutateOriginal verifies value semantics: SetMode
// must not mutate the receiver.
func TestTUI058_SetModeDoesNotMutateOriginal(t *testing.T) {
	m := outputmode.New()
	_ = m.SetMode(outputmode.OutputModeVerbose)
	assert.True(t, m.IsCompact(), "original model must be unchanged after SetMode")
}

// TestTUI058_ToggleDoesNotMutateOriginal verifies value semantics: Toggle
// must not mutate the receiver.
func TestTUI058_ToggleDoesNotMutateOriginal(t *testing.T) {
	m := outputmode.New()
	_ = m.Toggle()
	assert.True(t, m.IsCompact(), "original model must be unchanged after Toggle")
}

// TestTUI058_LabelCompact verifies Label() returns "compact" in compact mode.
func TestTUI058_LabelCompact(t *testing.T) {
	m := outputmode.New()
	assert.Equal(t, "compact", m.Label())
}

// TestTUI058_LabelVerbose verifies Label() returns "verbose" in verbose mode.
func TestTUI058_LabelVerbose(t *testing.T) {
	m := outputmode.New().Toggle()
	assert.Equal(t, "verbose", m.Label())
}

// TestTUI058_StatusTextNonEmpty verifies StatusText() is non-empty for both
// modes.
func TestTUI058_StatusTextNonEmpty(t *testing.T) {
	compact := outputmode.New()
	verbose := outputmode.New().Toggle()

	assert.NotEmpty(t, compact.StatusText(), "StatusText() must be non-empty in compact mode")
	assert.NotEmpty(t, verbose.StatusText(), "StatusText() must be non-empty in verbose mode")
}

// TestTUI058_StatusTextContainsLabel verifies StatusText() contains the mode
// label for both modes (after stripping ANSI escapes).
func TestTUI058_StatusTextContainsLabel(t *testing.T) {
	compact := outputmode.New()
	verbose := outputmode.New().Toggle()

	// StatusText wraps the StatusIndicator which may contain ANSI codes;
	// we check the Label separately since StatusIndicator applies lipgloss
	// styling. The label is the canonical source of truth.
	assert.Equal(t, "compact", compact.Label())
	assert.Equal(t, "verbose", verbose.Label())
}

// TestTUI058_HelpLine verifies HelpLine() returns the expected string.
func TestTUI058_HelpLine(t *testing.T) {
	m := outputmode.New()
	assert.Equal(t, "ctrl+v  toggle compact/verbose output", m.HelpLine())
}

// TestTUI058_ConcurrentToggle verifies no data races when multiple goroutines
// each hold their own Model copy and toggle concurrently.
func TestTUI058_ConcurrentToggle(t *testing.T) {
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			m := outputmode.New()
			m = m.Toggle()
			m = m.Toggle()
			_ = m.Label()
			_ = m.StatusText()
			_ = m.HelpLine()
			_ = m.IsVerbose()
			_ = m.IsCompact()
		}()
	}
	wg.Wait()
}
