package cron

import (
	"context"
	"strings"
	"testing"
)

func TestShellExecutor_Success(t *testing.T) {
	executor := &ShellExecutor{}
	job := Job{
		ExecConfig: `{"command":"echo hello world"}`,
		TimeoutSec: 10,
	}

	output, err := executor.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.TrimSpace(output) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", output)
	}
}

func TestShellExecutor_NonZeroExit(t *testing.T) {
	executor := &ShellExecutor{}
	job := Job{
		ExecConfig: `{"command":"exit 1"}`,
		TimeoutSec: 10,
	}

	_, err := executor.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "command failed") {
		t.Fatalf("expected 'command failed' error, got: %v", err)
	}
}

func TestShellExecutor_InvalidJSON(t *testing.T) {
	executor := &ShellExecutor{}
	job := Job{
		ExecConfig: `not json`,
		TimeoutSec: 10,
	}

	_, err := executor.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse execution config") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestShellExecutor_EmptyCommand(t *testing.T) {
	executor := &ShellExecutor{}
	job := Job{
		ExecConfig: `{"command":""}`,
		TimeoutSec: 10,
	}

	_, err := executor.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if !strings.Contains(err.Error(), "missing 'command' field") {
		t.Fatalf("expected missing command error, got: %v", err)
	}
}

func TestShellExecutor_Timeout(t *testing.T) {
	executor := &ShellExecutor{}
	job := Job{
		ExecConfig: `{"command":"sleep 10"}`,
		TimeoutSec: 1,
	}

	_, err := executor.Execute(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestShellExecutor_CapturesStderr(t *testing.T) {
	executor := &ShellExecutor{}
	job := Job{
		ExecConfig: `{"command":"echo stderr_output >&2"}`,
		TimeoutSec: 10,
	}

	output, err := executor.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(output, "stderr_output") {
		t.Fatalf("expected stderr output, got %q", output)
	}
}

func TestShellExecutor_TruncatesOutput(t *testing.T) {
	executor := &ShellExecutor{}
	// Generate output larger than 4096 bytes.
	job := Job{
		ExecConfig: `{"command":"dd if=/dev/zero bs=8192 count=1 2>/dev/null | tr '\\0' 'A'"}`,
		TimeoutSec: 10,
	}

	output, err := executor.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(output) > maxOutputBytes {
		t.Fatalf("expected output truncated to %d bytes, got %d", maxOutputBytes, len(output))
	}
}

func TestShellExecutor_DefaultTimeout(t *testing.T) {
	executor := &ShellExecutor{}
	job := Job{
		ExecConfig: `{"command":"echo fast"}`,
		TimeoutSec: 0, // should default to 30s
	}

	output, err := executor.Execute(context.Background(), job)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.TrimSpace(output) != "fast" {
		t.Fatalf("expected 'fast', got %q", output)
	}
}
