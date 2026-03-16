package tools

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// lineHash returns a 12-character hex hash of the line content (trimmed of trailing whitespace).
func lineHash(line string) string {
	h := sha256.Sum256([]byte(strings.TrimRight(line, " \t\r")))
	return fmt.Sprintf("%x", h[:6])
}
