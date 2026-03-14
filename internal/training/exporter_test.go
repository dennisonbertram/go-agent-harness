package training

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportFromJSONL_BasicRun(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_abc.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"fix the bug","run_id":"run_abc"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"llm.completion.started","data":{}}
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"tool.call","data":{"name":"read_file","args":{"path":"/foo.go"},"call_id":"tc_1","step":1}}
{"ts":"2026-03-14T12:00:03Z","seq":4,"type":"tool.result","data":{"call_id":"tc_1","name":"read_file","output":"package main","success":true,"step":1}}
{"ts":"2026-03-14T12:00:04Z","seq":5,"type":"llm.completion.finished","data":{"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150},"cost_usd":0.01}}
{"ts":"2026-03-14T12:00:05Z","seq":6,"type":"run.completed","data":{"output":"done","steps":1}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}

	if bundle.RunID != "run_abc" {
		t.Errorf("RunID = %q, want run_abc", bundle.RunID)
	}
	if bundle.Outcome != "pass" {
		t.Errorf("Outcome = %q, want pass", bundle.Outcome)
	}
	if bundle.Steps != 1 {
		t.Errorf("Steps = %d, want 1", bundle.Steps)
	}
	if len(bundle.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(bundle.ToolCalls))
	}
	tc := bundle.ToolCalls[0]
	if tc.Name != "read_file" {
		t.Errorf("ToolCall.Name = %q, want read_file", tc.Name)
	}
	if !tc.Success {
		t.Error("ToolCall.Success = false, want true")
	}
	if bundle.CostUSD != 0.01 {
		t.Errorf("CostUSD = %f, want 0.01", bundle.CostUSD)
	}
	if bundle.TokenCount != 150 {
		t.Errorf("TokenCount = %d, want 150", bundle.TokenCount)
	}
}

func TestExportFromJSONL_FailedRun(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_fail.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"do stuff","run_id":"run_fail"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"run.failed","data":{"error":"timeout","steps":3}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	if bundle.Outcome != "fail" {
		t.Errorf("Outcome = %q, want fail", bundle.Outcome)
	}
	if bundle.Steps != 3 {
		t.Errorf("Steps = %d, want 3", bundle.Steps)
	}
}

func TestExportFromJSONL_EfficiencyScore(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_eff.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"go","run_id":"run_eff"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"llm.completion.finished","data":{"usage":{"total_tokens":500},"cost_usd":0.05}}
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"run.completed","data":{"output":"ok","steps":5}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	// EfficiencyScore = 1.0 / (steps * cost) normalized
	// = 1.0 / (5 * 0.05) = 1.0 / 0.25 = 4.0, capped at 1.0
	if bundle.EfficiencyScore < 0 || bundle.EfficiencyScore > 1.0 {
		t.Errorf("EfficiencyScore = %f, want in [0,1]", bundle.EfficiencyScore)
	}
}

func TestExportFromJSONL_FirstTryRate(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_ftr.jsonl")
	// 3 tool calls: first two unique, third is retry of first (same name+args)
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"go","run_id":"run_ftr"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"tool.call","data":{"name":"read_file","args":{"path":"/a.go"},"call_id":"tc_1","step":1}}
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"tool.result","data":{"call_id":"tc_1","name":"read_file","output":"ok","success":false,"step":1}}
{"ts":"2026-03-14T12:00:03Z","seq":4,"type":"tool.call","data":{"name":"write_file","args":{"path":"/b.go"},"call_id":"tc_2","step":2}}
{"ts":"2026-03-14T12:00:04Z","seq":5,"type":"tool.result","data":{"call_id":"tc_2","name":"write_file","output":"ok","success":true,"step":2}}
{"ts":"2026-03-14T12:00:05Z","seq":6,"type":"tool.call","data":{"name":"read_file","args":{"path":"/a.go"},"call_id":"tc_3","step":3}}
{"ts":"2026-03-14T12:00:06Z","seq":7,"type":"tool.result","data":{"call_id":"tc_3","name":"read_file","output":"ok","success":true,"step":3}}
{"ts":"2026-03-14T12:00:07Z","seq":8,"type":"run.completed","data":{"output":"done","steps":3}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	// 3 tool calls total; tc_3 is a retry (same name+args as tc_1)
	// FirstTryRate = non-retried / total = 2/3
	if len(bundle.ToolCalls) != 3 {
		t.Fatalf("ToolCalls len = %d, want 3", len(bundle.ToolCalls))
	}
	expected := 2.0 / 3.0
	if bundle.FirstTryRate < expected-0.01 || bundle.FirstTryRate > expected+0.01 {
		t.Errorf("FirstTryRate = %f, want ~%f", bundle.FirstTryRate, expected)
	}
}

func TestExportFromJSONL_Truncation(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_trunc.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"go","run_id":"run_trunc"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"llm.completion.finished","data":{"usage":{"total_tokens":200000},"cost_usd":1.0}}
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"run.completed","data":{"output":"done","steps":1}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	if bundle.TokenCount != 200000 {
		t.Errorf("TokenCount = %d, want 200000", bundle.TokenCount)
	}
	if !bundle.Truncated {
		t.Error("Truncated = false, want true (tokens > 180000)")
	}
	if bundle.TruncationStrategy != "middle_drop" {
		t.Errorf("TruncationStrategy = %q, want middle_drop", bundle.TruncationStrategy)
	}
}

func TestExportFromJSONL_ContextSnapshots(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_ctx.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"go","run_id":"run_ctx"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"context.window.snapshot","data":{"step":1,"estimated_total_tokens":50000,"max_context_tokens":128000,"usage_ratio":0.39}}
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"context.window.snapshot","data":{"step":2,"estimated_total_tokens":80000,"max_context_tokens":128000,"usage_ratio":0.625}}
{"ts":"2026-03-14T12:00:03Z","seq":4,"type":"run.completed","data":{"output":"done","steps":2}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	if len(bundle.ContextSnapshots) != 2 {
		t.Fatalf("ContextSnapshots len = %d, want 2", len(bundle.ContextSnapshots))
	}
	if bundle.MaxContextRatio < 0.62 || bundle.MaxContextRatio > 0.63 {
		t.Errorf("MaxContextRatio = %f, want ~0.625", bundle.MaxContextRatio)
	}
}

func TestExportFromJSONL_AntiPatterns(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_ap.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"go","run_id":"run_ap"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"anti_pattern.detected","data":{"type":"retry_loop","tool_name":"bash","call_count":3,"step":2}}
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"run.completed","data":{"output":"done","steps":3}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	if len(bundle.AntiPatterns) != 1 {
		t.Fatalf("AntiPatterns len = %d, want 1", len(bundle.AntiPatterns))
	}
	if bundle.AntiPatterns[0].Type != "retry_loop" {
		t.Errorf("AntiPattern.Type = %q, want retry_loop", bundle.AntiPatterns[0].Type)
	}
}

func TestExportFromJSONL_Messages(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "run_msg.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"fix bug","run_id":"run_msg","system_prompt":"You are a helper"}}
{"ts":"2026-03-14T12:00:01Z","seq":2,"type":"llm.completion.finished","data":{"content":"I will fix it","role":"assistant"}}
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"run.completed","data":{"output":"done","steps":1}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	// Should have user message (from prompt) and assistant message
	if len(bundle.Messages) < 2 {
		t.Fatalf("Messages len = %d, want >= 2", len(bundle.Messages))
	}
	if bundle.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want user", bundle.Messages[0].Role)
	}
	if bundle.SystemPrompt != "You are a helper" {
		t.Errorf("SystemPrompt = %q, want 'You are a helper'", bundle.SystemPrompt)
	}
}

func TestExportFromJSONL_FileNotFound(t *testing.T) {
	_, err := ExportFromJSONL("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExportFromJSONL_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(fp, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	if bundle.Outcome != "unknown" {
		t.Errorf("Outcome = %q, want unknown", bundle.Outcome)
	}
}

func TestExportFromJSONL_MalformedLine(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "bad.jsonl")
	data := `{"ts":"2026-03-14T12:00:00Z","seq":1,"type":"run.started","data":{"prompt":"go","run_id":"run_bad"}}
NOT VALID JSON
{"ts":"2026-03-14T12:00:02Z","seq":3,"type":"run.completed","data":{"output":"done","steps":1}}
`
	if err := os.WriteFile(fp, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	// Should skip malformed lines, not fail entirely
	bundle, err := ExportFromJSONL(fp)
	if err != nil {
		t.Fatalf("ExportFromJSONL: %v", err)
	}
	if bundle.RunID != "run_bad" {
		t.Errorf("RunID = %q, want run_bad", bundle.RunID)
	}
}
