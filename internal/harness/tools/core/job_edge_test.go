package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestJobOutputTool_UnknownShellID verifies job_output surfaces the
// "unknown shell_id" error from the manager for an id that was never
// started, rather than returning an empty/zero-value result.
func TestJobOutputTool_UnknownShellID(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobOutputTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"shell_id":"job_does_not_exist"}`))
	if err == nil {
		t.Fatal("expected error for unknown shell_id")
	}
	if !strings.Contains(err.Error(), "unknown shell_id") {
		t.Errorf("expected 'unknown shell_id' error, got %q", err.Error())
	}
}

// TestJobOutputTool_CompletedJob verifies job_output, called with wait:true
// against a background job that has already finished, reports running:false,
// the correct exit_code, and the command's actual stdout content.
func TestJobOutputTool_CompletedJob(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	start, err := jm.RunBackground("echo hello-from-job", 5, "")
	if err != nil {
		t.Fatalf("start background job: %v", err)
	}
	shellID, _ := start["shell_id"].(string)
	if shellID == "" {
		t.Fatal("expected non-empty shell_id from RunBackground")
	}

	tool := JobOutputTool(jm)
	args, _ := json.Marshal(map[string]any{"shell_id": shellID, "wait": true})
	resultStr, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if running, _ := result["running"].(bool); running {
		t.Error("expected running:false once wait:true has observed job completion")
	}
	if code, ok := result["exit_code"].(float64); !ok || code != 0 {
		t.Errorf("expected exit_code 0, got %v", result["exit_code"])
	}
	output, _ := result["output"].(string)
	if !strings.Contains(output, "hello-from-job") {
		t.Errorf("expected output to contain the command's stdout, got %q", output)
	}
}

// TestJobKillTool_UnknownShellID verifies job_kill surfaces the "unknown
// shell_id" error for an id that was never started.
func TestJobKillTool_UnknownShellID(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobKillTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"shell_id":"job_does_not_exist"}`))
	if err == nil {
		t.Fatal("expected error for unknown shell_id")
	}
	if !strings.Contains(err.Error(), "unknown shell_id") {
		t.Errorf("expected 'unknown shell_id' error, got %q", err.Error())
	}
}

// TestJobKillTool_RunningJob verifies job_kill against a genuinely
// long-running job reports killed:true and that a subsequent job_output call
// observes the job as no longer running, proving the kill actually cancels
// the job rather than just returning a canned success response.
func TestJobKillTool_RunningJob(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	start, err := jm.RunBackground("sleep 30", 60, "")
	if err != nil {
		t.Fatalf("start background job: %v", err)
	}
	shellID, _ := start["shell_id"].(string)
	if shellID == "" {
		t.Fatal("expected non-empty shell_id from RunBackground")
	}

	killTool := JobKillTool(jm)
	args, _ := json.Marshal(map[string]any{"shell_id": shellID})
	resultStr, err := killTool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error killing job: %v", err)
	}
	var killResult map[string]any
	if err := json.Unmarshal([]byte(resultStr), &killResult); err != nil {
		t.Fatalf("parse kill result: %v", err)
	}
	if killed, _ := killResult["killed"].(bool); !killed {
		t.Error("expected killed:true")
	}

	outputTool := JobOutputTool(jm)
	outArgs, _ := json.Marshal(map[string]any{"shell_id": shellID})
	outResultStr, err := outputTool.Handler(context.Background(), outArgs)
	if err != nil {
		t.Fatalf("unexpected error reading killed job output: %v", err)
	}
	var outResult map[string]any
	if err := json.Unmarshal([]byte(outResultStr), &outResult); err != nil {
		t.Fatalf("parse output result: %v", err)
	}
	if running, _ := outResult["running"].(bool); running {
		t.Error("expected running:false after job_kill")
	}
}

// TestJobOutputTool_BadJSON verifies malformed JSON args produce a parse error.
func TestJobOutputTool_BadJSON(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobOutputTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"shell_id": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
	if !strings.Contains(err.Error(), "parse job_output args") {
		t.Errorf("expected parse error, got %q", err.Error())
	}
}

// TestJobKillTool_BadJSON verifies malformed JSON args produce a parse error.
func TestJobKillTool_BadJSON(t *testing.T) {
	jm := tools.NewJobManager(t.TempDir(), nil)
	tool := JobKillTool(jm)
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"shell_id": 5}`))
	if err == nil {
		t.Fatal("expected error for malformed JSON input")
	}
	if !strings.Contains(err.Error(), "parse job_kill args") {
		t.Errorf("expected parse error, got %q", err.Error())
	}
}
