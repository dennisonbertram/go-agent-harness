package tui

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	maxFileSize = 1024 * 1024 // 1 MB
	maxAtFiles  = 10
	binarySniff = 512 // bytes checked for null
)

// atTokenRegexp matches @"quoted path" or @path-like-token.
//
// Unquoted tokens MUST start with a path-like prefix to avoid matching email
// addresses, @mentions, and SSH URLs:
//   - @/...    absolute path
//   - @~/...   tilde-relative path
//   - @./...   explicit relative path
//   - @../...  parent-relative path
//   - any unquoted token that contains at least one slash after the initial segment
//
// Quoted form @"..." is unrestricted (explicit opt-in by the user).
var atTokenRegexp = regexp.MustCompile(`@"([^"]+)"|@((?:\./|/|~/|\.\./)\S*)`)

// trailingPunct lists characters that are trimmed from the end of unquoted paths.
// These commonly follow a path reference in prose (e.g. "see @./file.txt, then").
const trailingPunct = `,.;:!?)`

// ExpandAtPaths finds all @path tokens in prompt, reads each file, and replaces
// the token with XML-wrapped file contents:
//
//	<file path="path/to/file">
//	<![CDATA[
//	[contents]
//	]]>
//	</file>
//
// Rules:
//   - Unquoted @ tokens must start with /, ~/, ./, or ../ to be treated as paths.
//   - Trailing punctuation (,.;:!?)) is stripped from unquoted paths.
//   - Relative paths are resolved against os.Getwd().
//   - ~/... paths are expanded to the home directory.
//   - Quoted paths (@"path with spaces") are supported.
//   - Maximum file size: 1 MB.
//   - Binary files (null bytes in first 512 bytes) are rejected.
//   - Symlinks are rejected for security.
//   - Non-regular files (directories, devices) are rejected.
//   - Maximum 10 @path tokens per prompt.
//   - Prompts with no @path tokens are returned unchanged.
//   - Path attribute and contents are XML-escaped to prevent injection.
func ExpandAtPaths(prompt string) (string, error) {
	if prompt == "" {
		return "", nil
	}

	matches := atTokenRegexp.FindAllStringSubmatchIndex(prompt, -1)
	if len(matches) == 0 {
		return prompt, nil
	}

	// Check limit BEFORE reading any files.
	if len(matches) > maxAtFiles {
		return "", fmt.Errorf("too many @path tokens (%d); limit is %d files per prompt",
			len(matches), maxAtFiles)
	}

	var sb strings.Builder
	lastEnd := 0

	for _, match := range matches {
		// match[0], match[1] = full match start/end
		// match[2], match[3] = capture group 1 (quoted) start/end (-1 if not matched)
		// match[4], match[5] = capture group 2 (unquoted) start/end (-1 if not matched)

		fullStart := match[0]
		fullEnd := match[1]

		var rawPath string
		isUnquoted := match[2] < 0
		if !isUnquoted {
			// Quoted path: @"..."
			rawPath = prompt[match[2]:match[3]]
		} else {
			// Unquoted path — trim trailing punctuation.
			rawPath = prompt[match[4]:match[5]]
			rawPath = strings.TrimRight(rawPath, trailingPunct)
			// After trimming, adjust fullEnd to exclude the trimmed characters.
			fullEnd = fullStart + 1 + len(rawPath) // @-sign + trimmed path
		}

		// Expand ~ if present.
		resolvedPath, err := expandTilde(rawPath)
		if err != nil {
			return "", fmt.Errorf("path expansion failed for %q: %w", rawPath, err)
		}

		// Read and validate the file.
		content, err := readAtFile(resolvedPath)
		if err != nil {
			return "", err
		}

		// Append text before this match unchanged.
		sb.WriteString(prompt[lastEnd:fullStart])

		// Append XML-safe inline block.
		// Escape the path attribute value.
		escapedPath := xmlAttrEscape(resolvedPath)
		sb.WriteString("<file path=\"")
		sb.WriteString(escapedPath)
		sb.WriteString("\">\n")
		// Wrap contents in CDATA, splitting any "]]>" that appears in the content.
		sb.WriteString("<![CDATA[")
		sb.WriteString(cdataSafe(content))
		sb.WriteString("]]>")
		if len(content) > 0 && content[len(content)-1] != '\n' {
			sb.WriteByte('\n')
		}
		sb.WriteString("</file>")

		lastEnd = fullEnd
	}

	// Append trailing text.
	sb.WriteString(prompt[lastEnd:])
	return sb.String(), nil
}

// xmlAttrEscape escapes a string for safe use in an XML attribute value.
// It escapes &, <, >, and ".
func xmlAttrEscape(s string) string {
	var buf strings.Builder
	xml.EscapeText(&buf, []byte(s)) //nolint:errcheck // strings.Builder never errors
	// xml.EscapeText does not escape double-quotes; handle them explicitly.
	return strings.ReplaceAll(buf.String(), `"`, "&quot;")
}

// cdataSafe returns the string safe for inclusion inside a CDATA section.
// The CDATA terminator "]]>" cannot appear inside a CDATA section, so we
// split it: "]]>" becomes "]]>" + "]]>" — i.e., close CDATA, output "]]>",
// reopen CDATA.
func cdataSafe(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]>]]><![CDATA[")
}

// expandTilde replaces a leading ~/ with the user's home directory.
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return home + path[1:], nil
}

// readAtFile reads the file at path and returns its contents as a string.
// Returns an error if the file is a symlink, non-regular, too large, binary, or not found.
func readAtFile(path string) (string, error) {
	// Use Lstat to detect symlinks (Stat follows them).
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s (Use Tab after @ to browse files)", path)
		}
		return "", fmt.Errorf("cannot stat file %s: %w", path, err)
	}

	// Reject symlinks for security.
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("file %s is a symlink — symlinks are not supported for security reasons", path)
	}

	// Reject non-regular files (directories, device files, named pipes, etc.).
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("path %s is not a regular file (it may be a directory or special file)", path)
	}

	// Check size limit with humanized output.
	if info.Size() > maxFileSize {
		humanSize := humanizeBytes(info.Size())
		return "", fmt.Errorf("file %s is too large (%s). Limit: 1MB per file", path, humanSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %s: %w", path, err)
	}

	// Binary detection: check first 512 bytes for null bytes.
	sniff := data
	if len(sniff) > binarySniff {
		sniff = sniff[:binarySniff]
	}
	for _, b := range sniff {
		if b == 0 {
			return "", fmt.Errorf("file %s is a binary file — only text files can be attached", path)
		}
	}

	return string(data), nil
}

// humanizeBytes converts a byte count to a human-readable string like "2.3MB".
func humanizeBytes(n int64) string {
	const mb = 1024 * 1024
	if n >= mb {
		return fmt.Sprintf("%.1fMB", float64(n)/float64(mb))
	}
	const kb = 1024
	if n >= kb {
		return fmt.Sprintf("%.1fKB", float64(n)/float64(kb))
	}
	return fmt.Sprintf("%dB", n)
}
