package tools

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// LineHash returns the 12-character hex content hash of a line (trimmed of
// trailing whitespace) used for hash-addressed reads and edits: `read` with
// `hash_lines: true` prefixes each returned line with this hash, and `edit`
// accepts it back via `start_line_hash`/`end_line_hash` to anchor a
// replacement to a specific line rather than the first (or every) textual
// match.
func LineHash(line string) string {
	h := sha256.Sum256([]byte(strings.TrimRight(line, " \t\r")))
	return fmt.Sprintf("%x", h[:6])
}
