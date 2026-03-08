package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const maxOutputBytes = 4096

// Executor runs a job and returns the result.
type Executor interface {
	Execute(ctx context.Context, job Job) (output string, err error)
}

// ShellExecutor runs shell commands.
type ShellExecutor struct{}

// shellConfig is the JSON structure for shell execution config.
type shellConfig struct {
	Command string `json:"command"`
}

// Execute runs the shell command specified in the job's ExecConfig.
func (e *ShellExecutor) Execute(ctx context.Context, job Job) (string, error) {
	var cfg shellConfig
	if err := json.Unmarshal([]byte(job.ExecConfig), &cfg); err != nil {
		return "", fmt.Errorf("parse execution config: %w", err)
	}
	if cfg.Command == "" {
		return "", fmt.Errorf("execution config missing 'command' field")
	}

	timeout := time.Duration(job.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cfg.Command)
	out, err := cmd.CombinedOutput()

	// Truncate output to maxOutputBytes.
	if len(out) > maxOutputBytes {
		out = out[:maxOutputBytes]
	}
	output := string(out)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("command timed out after %d seconds", job.TimeoutSec)
		}
		return output, fmt.Errorf("command failed: %w", err)
	}
	return output, nil
}
