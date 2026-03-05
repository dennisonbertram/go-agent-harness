package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func runCommand(ctx context.Context, timeout time.Duration, command string, args ...string) (string, int, bool, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, command, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	timedOut := errors.Is(ctxTimeout.Err(), context.DeadlineExceeded)

	output := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimSpace(stderr.String())
	}
	if err != nil && exitCode == -1 {
		return output, exitCode, timedOut, fmt.Errorf("run command: %w", err)
	}
	return output, exitCode, timedOut, nil
}
