package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ExecFunc is the function signature for running a CLI command.
// The function receives the working directory, command name, and arguments,
// and returns the combined stdout+stderr output and any error.
// This type is injected into adapters so that tests can substitute fake executors
// without requiring the real CLI to be installed.
type ExecFunc func(ctx context.Context, dir, command string, args ...string) (string, error)

// DefaultExec runs the given command in the specified directory and returns
// the combined stdout+stderr output.
func DefaultExec(ctx context.Context, dir, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(buf.String())
		if out != "" {
			return out, fmt.Errorf("%s: %w\n%s", command, err, out)
		}
		return "", fmt.Errorf("%s: %w", command, err)
	}
	return strings.TrimSpace(buf.String()), nil
}
