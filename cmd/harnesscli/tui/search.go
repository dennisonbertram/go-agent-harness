package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"go-agent-harness/cmd/harnesscli/tui/components/transcriptexport"
)

// SearchResult represents a single match within the conversation transcript.
type SearchResult struct {
	// EntryIndex is the position of the matched entry in the transcript slice.
	EntryIndex int
	// Role is the speaker: "user", "assistant", or "tool".
	Role string
	// Timestamp is the time the entry was recorded.
	Timestamp time.Time
	// Snippet is a short excerpt (up to 80 runes) around the match.
	Snippet string
	// MatchStart is the rune offset of the match start within Snippet.
	MatchStart int
	// MatchEnd is the rune offset of the match end within Snippet (exclusive).
	MatchEnd int
}

// snippetWindow is the number of runes to show on each side of a match.
const snippetWindow = 40

// runeIndexFold finds the first occurrence of lowerSub (already lowercased) inside
// s using Unicode case-folding. It returns the byte offset and rune offset of the
// start of the match in s, or (-1, -1) if not found.
// The comparison is done entirely in rune space to avoid byte-length mismatches
// that arise when ToLower changes the byte length of a character (e.g. Turkish İ,
// German ẞ).
func runeIndexFold(s, query string) (byteOffset, runeOffset int) {
	sRunes := []rune(s)
	qRunes := []rune(strings.ToLower(query))
	if len(qRunes) == 0 || len(sRunes) < len(qRunes) {
		return -1, -1
	}
	sLower := []rune(strings.ToLower(s))
	for i := 0; i <= len(sLower)-len(qRunes); i++ {
		match := true
		for j, qr := range qRunes {
			if sLower[i+j] != qr {
				match = false
				break
			}
		}
		if match {
			// Compute byte offset from rune offset.
			byteOff := len(string(sRunes[:i]))
			return byteOff, i
		}
	}
	return -1, -1
}

// SearchTranscript searches the transcript for entries containing query (case-insensitive).
// Returns all matching entries in chronological (index) order.
// Returns nil if query is empty.
// All offsets (MatchStart, MatchEnd) are rune offsets into Snippet.
func SearchTranscript(transcript []transcriptexport.TranscriptEntry, query string) []SearchResult {
	if query == "" {
		return nil
	}
	qRunes := []rune(query)
	var results []SearchResult
	for i, entry := range transcript {
		_, runeIdx := runeIndexFold(entry.Content, query)
		if runeIdx < 0 {
			continue
		}
		snippet, matchStart, matchEnd := extractSnippet(entry.Content, runeIdx, len(qRunes))
		results = append(results, SearchResult{
			EntryIndex: i,
			Role:       entry.Role,
			Timestamp:  entry.Timestamp,
			Snippet:    snippet,
			MatchStart: matchStart,
			MatchEnd:   matchEnd,
		})
	}
	return results
}

// extractSnippet returns a windowed snippet (in runes) around the match at rune
// position matchRunePos with length matchRuneLen, plus the start/end rune offsets
// of the match within the snippet.
// Ellipsis (…) is prepended/appended when the snippet does not reach the start/end
// of the content.
func extractSnippet(content string, matchRunePos, matchRuneLen int) (snippet string, matchStart, matchEnd int) {
	runes := []rune(content)
	total := len(runes)

	startRune := matchRunePos - snippetWindow
	truncatedLeft := startRune > 0
	if startRune < 0 {
		startRune = 0
	}
	endRune := matchRunePos + matchRuneLen + snippetWindow
	truncatedRight := endRune < total
	if endRune > total {
		endRune = total
	}

	part := runes[startRune:endRune]

	// Build the snippet string with ellipsis markers.
	var sb strings.Builder
	if truncatedLeft {
		sb.WriteRune('…')
	}
	sb.WriteString(string(part))
	if truncatedRight {
		sb.WriteRune('…')
	}

	snippet = sb.String()

	// Compute match offsets as rune offsets into snippet.
	// The snippet rune slice starts with optional '…' (1 rune) then the content window.
	ellipsisOffset := 0
	if truncatedLeft {
		ellipsisOffset = 1
	}
	matchStart = (matchRunePos - startRune) + ellipsisOffset
	matchEnd = matchStart + matchRuneLen
	return snippet, matchStart, matchEnd
}

// highlightStyle is the lipgloss style applied to matching text in search results.
var highlightStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{
	Light: "#d97706",
	Dark:  "#fbbf24",
})

// HighlightMatch wraps the first occurrence of query in text with bold/yellow styling.
// The comparison is case-insensitive but the original casing is preserved.
// Returns text unchanged if query is empty or not found.
// All slicing is performed in rune space to avoid misalignment with multi-byte characters.
func HighlightMatch(text, query string) string {
	if query == "" {
		return text
	}
	_, runeIdx := runeIndexFold(text, query)
	if runeIdx < 0 {
		return text
	}
	textRunes := []rune(text)
	qLen := utf8.RuneCountInString(query)
	before := string(textRunes[:runeIdx])
	matched := string(textRunes[runeIdx : runeIdx+qLen])
	after := string(textRunes[runeIdx+qLen:])
	return before + highlightStyle.Render(matched) + after
}

// searchSessions searches stored session metadata for sessions whose LastMsg
// contains query (case-insensitive). Returns results with a synthesised snippet.
// If store is nil, returns nil.
func searchSessions(store *SessionStore, query string) []SearchResult {
	if store == nil || query == "" {
		return nil
	}
	sessions := store.List()
	qRunes := []rune(query)
	var results []SearchResult
	for i, s := range sessions {
		_, runeIdx := runeIndexFold(s.LastMsg, query)
		if runeIdx < 0 {
			continue
		}
		snippet, matchStart, matchEnd := extractSnippet(s.LastMsg, runeIdx, len(qRunes))
		results = append(results, SearchResult{
			EntryIndex: i,
			Role:       "session",
			Timestamp:  s.StartedAt,
			Snippet:    snippet,
			MatchStart: matchStart,
			MatchEnd:   matchEnd,
		})
	}
	return results
}

// searchResultCountLabel returns "1 result" or "N results".
func searchResultCountLabel(n int) string {
	if n == 1 {
		return "1 result"
	}
	return fmt.Sprintf("%d results", n)
}
