package skills

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// maxCommandOutput is the maximum number of bytes kept from a single command's stdout.
const maxCommandOutput = 32 * 1024 // 32 KiB

// commandTimeout is the per-command execution timeout.
const commandTimeout = 10 * time.Second

// commandPattern matches !`command` patterns outside of fenced code blocks.
// The backtick-delimited command is captured in group 1.
var commandPattern = regexp.MustCompile("!`([^`]+)`")

// preprocessCommands finds !`command` patterns in content and replaces them
// with the command's stdout. Commands inside fenced code blocks are skipped.
// Variable interpolation should happen BEFORE preprocessing.
func preprocessCommands(ctx context.Context, content string, workDir string) string {
	if !strings.Contains(content, "!`") {
		return content
	}

	// Split content by fenced code block markers (```).
	// Even-indexed segments are outside code blocks; odd-indexed are inside.
	segments := strings.Split(content, "```")
	for i := range segments {
		if i%2 == 0 {
			// Outside a code block -- process commands.
			segments[i] = commandPattern.ReplaceAllStringFunc(segments[i], func(match string) string {
				sub := commandPattern.FindStringSubmatch(match)
				if len(sub) < 2 {
					return match
				}
				cmd := sub[1]
				return executeCommand(ctx, cmd, workDir)
			})
		}
		// Odd-indexed segments (inside code blocks) are left unchanged.
	}
	return strings.Join(segments, "```")
}

// executeCommand runs a single shell command and returns its stdout.
// On failure or timeout the error is returned as an inline marker.
func executeCommand(ctx context.Context, cmd string, workDir string) string {
	cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	c := exec.CommandContext(cmdCtx, "/bin/bash", "-c", cmd)
	if workDir != "" {
		c.Dir = workDir
	}

	var stdout bytes.Buffer
	c.Stdout = &stdout

	err := c.Run()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("[shell-preprocess error: command %q timed out after %s]", cmd, commandTimeout)
		}
		if ctx.Err() != nil {
			return fmt.Sprintf("[shell-preprocess error: command %q cancelled: %v]", cmd, ctx.Err())
		}
		return fmt.Sprintf("[shell-preprocess error: command %q failed: %v]", cmd, err)
	}

	out := stdout.Bytes()
	if len(out) > maxCommandOutput {
		out = out[:maxCommandOutput]
	}

	return strings.TrimRight(string(out), "\n")
}
