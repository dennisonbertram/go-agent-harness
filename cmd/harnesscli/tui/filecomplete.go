package tui

import (
	"os"
	"path/filepath"
	"strings"
)

const maxFileCompletions = 20

// pathLikePrefixes lists the prefixes that make an @token a path reference.
// This mirrors the regex in fileexpand.go: only unquoted tokens starting with
// /, ~/, ./, or ../ are treated as file paths.
var pathLikePrefixes = []string{"/", "~/", "./", "../"}

// FilePathCompleter returns file path completions when the input contains an @
// token that looks like a file path.
//
// When the input contains an @-sign followed by a partial path that starts with
// /, ~/, ./, or ../, this function:
//   - Expands a leading ~/ to the user's home directory.
//   - Lists directory entries matching the partial path prefix.
//   - Appends a trailing "/" to directory entries.
//   - Limits results to 20 suggestions.
//   - Returns nil when the input contains no @ token or the partial path is not
//     path-like (e.g. email addresses and @mentions are ignored).
//
// The returned completions are full replacement strings (including the @ prefix
// and any text before the last @ token).
func FilePathCompleter(input string) []string {
	// Find the last @ in the input.
	lastAt := strings.LastIndex(input, "@")
	if lastAt < 0 {
		return nil
	}

	// The text before and including @ (prefix we keep in completions).
	beforeAt := input[:lastAt+1]

	// The partial path after @.
	partial := input[lastAt+1:]

	// If the partial is empty or starts with a space, no file completion.
	if partial == "" || strings.HasPrefix(partial, " ") {
		return nil
	}

	// Only trigger completions when partial starts with a path-like prefix.
	isPathLike := false
	for _, pfx := range pathLikePrefixes {
		if strings.HasPrefix(partial, pfx) {
			isPathLike = true
			break
		}
	}
	if !isPathLike {
		return nil
	}

	// Expand tilde for ~/ paths.
	expandedPartial, err := expandTilde(partial)
	if err != nil {
		return nil
	}

	// Determine the directory and file prefix to search.
	dir := filepath.Dir(expandedPartial)
	base := filepath.Base(expandedPartial)

	// When partial ends with '/', Dir returns the partial stripped of trailing slash
	// but we want to list that directory.
	if strings.HasSuffix(expandedPartial, "/") || strings.HasSuffix(expandedPartial, string(filepath.Separator)) {
		dir = expandedPartial
		base = ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// Nonexistent or unreadable directory — no completions.
		return nil
	}

	var results []string
	for _, entry := range entries {
		name := entry.Name()
		// Filter by prefix.
		if base != "" && !strings.HasPrefix(name, base) {
			continue
		}

		// Build the completed path.
		completedPath := filepath.Join(dir, name)
		if entry.IsDir() {
			completedPath += "/"
		}

		// Full completion = text before @ + @ + completed path.
		results = append(results, beforeAt+completedPath)

		if len(results) >= maxFileCompletions {
			break
		}
	}

	return results
}
