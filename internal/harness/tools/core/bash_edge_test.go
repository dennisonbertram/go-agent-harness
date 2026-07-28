package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestBashTool_DangerousCommandRejected verifies a command matching the
// dangerous-command safety patterns (e.g. "rm -rf /") is rejected before
// execution rather than run.
func TestBashTool_DangerousCommandRejected(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"rm -rf /"}`))
	if err == nil {
		t.Fatal("expected error for dangerous command")
	}
	if !strings.Contains(err.Error(), "safety policy") {
		t.Errorf("expected safety-policy rejection, got %q", err.Error())
	}
}

// TestBashTool_RunInBackground verifies run_in_background:true routes
// through JobManager.RunBackgroundWithContext and returns a shell_id rather
// than blocking for command completion, and that the description field is
// attached to the background-start result.
func TestBashTool_RunInBackground(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	args := json.RawMessage(`{"command":"echo bg","run_in_background":true,"description":"my job"}`)
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	shellID, _ := result["shell_id"].(string)
	if shellID == "" {
		t.Fatal("expected non-empty shell_id for a backgrounded command")
	}
	if started, _ := result["started"].(bool); !started {
		t.Error("expected started:true")
	}
	if desc, _ := result["description"].(string); desc != "my job" {
		t.Errorf("expected description 'my job' to be attached, got %q", desc)
	}

	// Confirm the job actually completes and its output is retrievable via
	// job_output, proving RunBackgroundWithContext really launched it.
	outTool := JobOutputTool(jm)
	outArgs, _ := json.Marshal(map[string]any{"shell_id": shellID, "wait": true})
	outStr, err := outTool.Handler(context.Background(), outArgs)
	if err != nil {
		t.Fatalf("job_output: %v", err)
	}
	if !strings.Contains(outStr, "bg") {
		t.Errorf("expected background job output to contain 'bg', got %q", outStr)
	}
}

// TestBashTool_ForegroundDescriptionAttached verifies the description field
// is attached to the foreground-run result too.
func TestBashTool_ForegroundDescriptionAttached(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	args := json.RawMessage(`{"command":"echo fg","description":"fg job"}`)
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if desc, _ := result["description"].(string); desc != "fg job" {
		t.Errorf("expected description 'fg job' to be attached, got %q", desc)
	}
}

// TestBashTool_BadJSON verifies malformed JSON args produce a parse error.
func TestBashTool_BadJSON(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"command": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
	if !strings.Contains(err.Error(), "parse bash args") {
		t.Errorf("expected parse error, got %q", err.Error())
	}
}

// TestBashTool_SudoStripped verifies a sudo-prefixed command is stripped of
// "sudo " and executed as the underlying command, rather than failing
// because sudo is unavailable/interactive in the sandbox.
func TestBashTool_SudoStripped(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := BashTool(jm)
	resultStr, err := tool.Handler(context.Background(), json.RawMessage(`{"command":"sudo echo sudo-stripped"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resultStr, "sudo-stripped") {
		t.Errorf("expected sudo prefix to be stripped and command to run, got %q", resultStr)
	}
}
