package tui

import (
	"encoding/base64"
	"fmt"
	"os"
)

// CopyToClipboard writes text to the system clipboard using the OSC52 escape
// sequence. It falls back silently in headless/CI mode (when TERM is unset or
// "dumb"). Returns true if the copy was likely successful, false for fallback.
func CopyToClipboard(text string) bool {
	if IsHeadless() {
		return false
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	// OSC52: \033]52;c;{base64}\a
	_, err := fmt.Fprintf(os.Stdout, "\033]52;c;%s\a", encoded)
	return err == nil
}

// IsHeadless returns true when running in CI/non-interactive mode, detected
// by checking whether TERM is unset or set to "dumb".
func IsHeadless() bool {
	term := os.Getenv("TERM")
	return term == "" || term == "dumb"
}
