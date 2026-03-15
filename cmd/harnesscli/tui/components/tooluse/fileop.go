package tooluse

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FileOpKind classifies the file operation type.
type FileOpKind int

const (
	// FileOpRead represents a read file operation.
	FileOpRead FileOpKind = iota
	// FileOpWrite represents a write file operation.
	FileOpWrite
	// FileOpEdit represents an edit file operation.
	FileOpEdit
	// FileOpUnknown represents an unrecognized tool name.
	FileOpUnknown
)

// maxFileNameDisplay is the maximum number of runes to show from a filename
// before truncating with "…".
const maxFileNameDisplay = 40

// FileOpSummary computes a one-line summary for a file operation tool result.
//
// Examples:
//   - "Read 42 lines"
//   - "Wrote 7 lines to utils.go"
//   - "Added 1 line to main.go"
//   - "Edited server.go"
type FileOpSummary struct {
	Kind      FileOpKind
	FileName  string // basename only (filepath.Base)
	LineCount int
}

// Line returns the formatted summary line with ⎿ tree-connector prefix.
// Returns "" if Kind==FileOpUnknown or LineCount==0 (except for FileOpEdit
// with no + lines, which uses "Edited {file}").
func (s FileOpSummary) Line() string {
	switch s.Kind {
	case FileOpRead:
		if s.LineCount == 0 {
			return ""
		}
		return treeStyle.Render(treeSymbol) + "  " + fmt.Sprintf("Read %d lines", s.LineCount)

	case FileOpWrite:
		if s.LineCount == 0 {
			return ""
		}
		name := truncateFileName(s.FileName)
		return treeStyle.Render(treeSymbol) + "  " + fmt.Sprintf("Wrote %d lines to %s", s.LineCount, name)

	case FileOpEdit:
		name := truncateFileName(s.FileName)
		if s.LineCount > 0 {
			return treeStyle.Render(treeSymbol) + "  " + fmt.Sprintf("Added %d lines to %s", s.LineCount, name)
		}
		if name == "" {
			return ""
		}
		return treeStyle.Render(treeSymbol) + "  " + fmt.Sprintf("Edited %s", name)

	default:
		return ""
	}
}

// ParseFileOp extracts FileOpSummary from a tool result string and tool name.
//
// toolName: "read_file", "write_file", "edit_file", etc.
// fileName: path to the file (basename will be extracted via filepath.Base).
// result: tool result text (count lines to get LineCount).
func ParseFileOp(toolName, fileName, result string) FileOpSummary {
	kind := classifyToolName(toolName)
	if kind == FileOpUnknown {
		return FileOpSummary{Kind: FileOpUnknown}
	}

	baseName := filepath.Base(fileName)
	// filepath.Base returns "." for empty string; normalize that to ""
	if baseName == "." {
		baseName = ""
	}

	var lineCount int
	switch kind {
	case FileOpRead, FileOpWrite:
		lineCount = countLines(result)
	case FileOpEdit:
		// Count lines starting with "+" (diff additions)
		lineCount = countPlusLines(result)
	}

	return FileOpSummary{
		Kind:      kind,
		FileName:  baseName,
		LineCount: lineCount,
	}
}

// classifyToolName maps a tool name string to its FileOpKind.
func classifyToolName(name string) FileOpKind {
	switch name {
	case "read_file", "Read":
		return FileOpRead
	case "write_file", "Write", "Create":
		return FileOpWrite
	case "edit_file", "Edit", "str_replace_editor":
		return FileOpEdit
	default:
		return FileOpUnknown
	}
}

// countLines returns the number of lines in s (splitting on "\n").
// An empty string returns 0.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	// strings.Count counts occurrences of "\n"; add 1 for the final line
	// unless the string ends with "\n" (then the last "line" is empty).
	lines := strings.Split(s, "\n")
	// Trim trailing empty line caused by a trailing newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return len(lines)
}

// countPlusLines returns the number of lines in s that start with "+".
func countPlusLines(s string) int {
	if s == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "+") {
			count++
		}
	}
	return count
}

// truncateFileName truncates a filename to maxFileNameDisplay runes, appending
// "…" if truncation occurred.
func truncateFileName(name string) string {
	if name == "" {
		return ""
	}
	runes := utf8.RuneCountInString(name)
	if runes <= maxFileNameDisplay {
		return name
	}
	return truncateRunes(name, maxFileNameDisplay-1) + ellipsis
}
