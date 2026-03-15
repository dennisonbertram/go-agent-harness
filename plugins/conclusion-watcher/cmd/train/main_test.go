package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	cw "go-agent-harness/plugins/conclusion-watcher"
)

// ============================================================
// Input event parsing tests
// ============================================================

func TestParseEvents_MessageCreated(t *testing.T) {
	line := `{"type":"message.created","run_id":"abc","step":3,"payload":{"role":"assistant","content":"The file probably contains...","tool_calls":[{"name":"write","arguments":"{}"}]}}`
	ev, err := parseEvent([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != "message.created" {
		t.Errorf("expected type message.created, got %s", ev.Type)
	}
	if ev.RunID != "abc" {
		t.Errorf("expected run_id abc, got %s", ev.RunID)
	}
	if ev.Step != 3 {
		t.Errorf("expected step 3, got %d", ev.Step)
	}
	if ev.Payload.Role != "assistant" {
		t.Errorf("expected role assistant, got %s", ev.Payload.Role)
	}
	if ev.Payload.Content != "The file probably contains..." {
		t.Errorf("unexpected content: %s", ev.Payload.Content)
	}
	if len(ev.Payload.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(ev.Payload.ToolCalls))
	}
}

func TestParseEvents_ToolCallCompleted(t *testing.T) {
	line := `{"type":"tool_call.completed","run_id":"abc","step":2,"payload":{"name":"read","result":"file content"}}`
	ev, err := parseEvent([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != "tool_call.completed" {
		t.Errorf("expected type tool_call.completed, got %s", ev.Type)
	}
	if ev.Payload.Name != "read" {
		t.Errorf("expected name read, got %s", ev.Payload.Name)
	}
}

func TestParseEvents_InvalidJSON(t *testing.T) {
	_, err := parseEvent([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGroupEvents_ByRunID(t *testing.T) {
	events := []inputEvent{
		{Type: "message.created", RunID: "run1", Step: 1},
		{Type: "tool_call.completed", RunID: "run2", Step: 1},
		{Type: "message.created", RunID: "run1", Step: 2},
	}
	grouped := groupByRunID(events)
	if len(grouped["run1"]) != 2 {
		t.Errorf("expected 2 events for run1, got %d", len(grouped["run1"]))
	}
	if len(grouped["run2"]) != 1 {
		t.Errorf("expected 1 event for run2, got %d", len(grouped["run2"]))
	}
}

func TestBuildToolHistory_LastTen(t *testing.T) {
	// Build 15 tool_call.completed events; history should only show last 10.
	var events []inputEvent
	for i := 1; i <= 15; i++ {
		events = append(events, inputEvent{
			Type:  "tool_call.completed",
			RunID: "run1",
			Step:  i,
			Payload: eventPayload{
				Name:   "read_file",
				Result: "content",
			},
		})
	}
	history := buildToolHistory(events, 16)
	if len(history) != 10 {
		t.Errorf("expected 10 history entries, got %d: %v", len(history), history)
	}
}

func TestBuildToolHistory_Format(t *testing.T) {
	events := []inputEvent{
		{
			Type:  "tool_call.completed",
			RunID: "run1",
			Step:  2,
			Payload: eventPayload{
				Name:      "read_file",
				Arguments: `{"path":"foo.go"}`,
			},
		},
	}
	history := buildToolHistory(events, 3)
	if len(history) == 0 {
		t.Fatal("expected at least one history entry")
	}
	// Format: "step N: <tool_name>(<args_truncated_to_50_chars>)"
	if !strings.HasPrefix(history[0], "step 2:") {
		t.Errorf("expected history entry to start with 'step 2:', got: %s", history[0])
	}
	if !strings.Contains(history[0], "read_file") {
		t.Errorf("expected history entry to contain tool name, got: %s", history[0])
	}
}

func TestBuildToolHistory_ArgsTruncated(t *testing.T) {
	longArgs := strings.Repeat("x", 100)
	events := []inputEvent{
		{
			Type:  "tool_call.completed",
			RunID: "run1",
			Step:  1,
			Payload: eventPayload{
				Name:      "bash",
				Arguments: longArgs,
			},
		},
	}
	history := buildToolHistory(events, 2)
	if len(history) == 0 {
		t.Fatal("expected history entry")
	}
	entry := history[0]
	// Extract args part from between ( and )
	start := strings.Index(entry, "(")
	end := strings.LastIndex(entry, ")")
	if start == -1 || end == -1 {
		t.Fatalf("expected (args) in entry, got: %s", entry)
	}
	argsInEntry := entry[start+1 : end]
	if len(argsInEntry) > 50 {
		t.Errorf("expected args truncated to 50 chars, got %d: %s", len(argsInEntry), argsInEntry)
	}
}

func TestBuildToolHistory_OnlyBeforeCurrentStep(t *testing.T) {
	events := []inputEvent{
		{Type: "tool_call.completed", RunID: "run1", Step: 1, Payload: eventPayload{Name: "read_file"}},
		{Type: "tool_call.completed", RunID: "run1", Step: 3, Payload: eventPayload{Name: "write_file"}},
		{Type: "tool_call.completed", RunID: "run1", Step: 2, Payload: eventPayload{Name: "grep"}},
	}
	// Current step is 2; step 3 should be excluded.
	history := buildToolHistory(events, 2)
	for _, h := range history {
		if strings.Contains(h, "write_file") {
			t.Errorf("expected step 3 (write_file) to be excluded from history at step 2, got: %v", history)
		}
	}
}

// ============================================================
// Output format tests
// ============================================================

func TestWriteOutput_Format(t *testing.T) {
	rec := outputRecord{
		RunID:                    "run-abc",
		Step:                     3,
		HasUnjustifiedConclusion: true,
		Patterns:                 []cw.PatternType{cw.PatternArchitectureAssumption},
		Evidence:                 "The file probably contains",
		Explanation:              "No read tool was called before this assertion",
		OriginalText:             "The file probably contains the logic",
	}
	var buf bytes.Buffer
	if err := writeOutputRecord(&buf, rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	line := buf.String()
	if !strings.HasSuffix(strings.TrimSpace(line), "}") {
		t.Errorf("expected valid JSON line, got: %s", line)
	}

	var decoded outputRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &decoded); err != nil {
		t.Fatalf("failed to decode output line: %v", err)
	}
	if decoded.RunID != "run-abc" {
		t.Errorf("expected run_id run-abc, got %s", decoded.RunID)
	}
	if decoded.Step != 3 {
		t.Errorf("expected step 3, got %d", decoded.Step)
	}
	if !decoded.HasUnjustifiedConclusion {
		t.Error("expected has_unjustified_conclusion true")
	}
	if len(decoded.Patterns) != 1 || decoded.Patterns[0] != cw.PatternArchitectureAssumption {
		t.Errorf("expected architecture_assumption pattern, got: %v", decoded.Patterns)
	}
	if decoded.Evidence != "The file probably contains" {
		t.Errorf("unexpected evidence: %s", decoded.Evidence)
	}
	if decoded.OriginalText != "The file probably contains the logic" {
		t.Errorf("unexpected original_text: %s", decoded.OriginalText)
	}
}

// ============================================================
// Summary report tests
// ============================================================

func TestBuildSummary_Counts(t *testing.T) {
	records := []outputRecord{
		{RunID: "r1", HasUnjustifiedConclusion: true, Patterns: []cw.PatternType{cw.PatternHedgeAssertion}},
		{RunID: "r1", HasUnjustifiedConclusion: false},
		{RunID: "r2", HasUnjustifiedConclusion: true, Patterns: []cw.PatternType{cw.PatternArchitectureAssumption}},
		{RunID: "r2", HasUnjustifiedConclusion: true, Patterns: []cw.PatternType{cw.PatternHedgeAssertion}},
	}
	summary := buildSummary(records)
	if summary.TotalSteps != 4 {
		t.Errorf("expected 4 total steps, got %d", summary.TotalSteps)
	}
	if summary.JumpsDetected != 3 {
		t.Errorf("expected 3 jumps, got %d", summary.JumpsDetected)
	}
	if summary.TotalRuns != 2 {
		t.Errorf("expected 2 runs, got %d", summary.TotalRuns)
	}
	if summary.ByPattern[cw.PatternHedgeAssertion] != 2 {
		t.Errorf("expected 2 hedge_assertion, got %d", summary.ByPattern[cw.PatternHedgeAssertion])
	}
	if summary.ByPattern[cw.PatternArchitectureAssumption] != 1 {
		t.Errorf("expected 1 architecture_assumption, got %d", summary.ByPattern[cw.PatternArchitectureAssumption])
	}
}

func TestPrintSummary_Output(t *testing.T) {
	summary := summaryReport{
		TotalSteps:    10,
		JumpsDetected: 3,
		TotalRuns:     2,
		ByPattern: map[cw.PatternType]int{
			cw.PatternHedgeAssertion:    2,
			cw.PatternSkippedDiagnostic: 1,
		},
	}
	var buf bytes.Buffer
	printSummary(&buf, summary)
	out := buf.String()

	if !strings.Contains(out, "10") {
		t.Errorf("expected total steps in summary output, got: %s", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("expected jumps detected in summary output, got: %s", out)
	}
	if !strings.Contains(out, "hedge_assertion") {
		t.Errorf("expected pattern name in summary output, got: %s", out)
	}
}

// ============================================================
// Integration: processTranscript
// ============================================================

type recordingEvaluator struct {
	results map[string]*cw.EvaluatorResult
	calls   int
	mu      sync.Mutex
}

func (r *recordingEvaluator) Evaluate(_ context.Context, llmText string, _ []string, _ []string) (*cw.EvaluatorResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if result, ok := r.results[llmText]; ok {
		return result, nil
	}
	return &cw.EvaluatorResult{HasUnjustifiedConclusion: false}, nil
}

func TestProcessTranscript_EndToEnd(t *testing.T) {
	inputLines := []string{
		`{"type":"tool_call.completed","run_id":"run1","step":1,"payload":{"name":"read_file","arguments":"{\"path\":\"foo.go\"}","result":"file contents"}}`,
		`{"type":"message.created","run_id":"run1","step":2,"payload":{"role":"assistant","content":"The design is clearly wrong","tool_calls":[{"name":"write_file","arguments":"{}"}]}}`,
		`{"type":"message.created","run_id":"run1","step":3,"payload":{"role":"assistant","content":"I confirmed the issue","tool_calls":[]}}`,
	}
	input := strings.Join(inputLines, "\n")

	eval := &recordingEvaluator{
		results: map[string]*cw.EvaluatorResult{
			"The design is clearly wrong": {
				HasUnjustifiedConclusion: true,
				Patterns:                 []cw.PatternType{cw.PatternArchitectureAssumption},
				Evidence:                 "clearly",
				Explanation:              "no exploration",
			},
			"I confirmed the issue": {
				HasUnjustifiedConclusion: false,
			},
		},
	}

	var outBuf bytes.Buffer
	records, err := processTranscript(context.Background(), bufio.NewScanner(strings.NewReader(input)), eval, 4, &outBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 output records, got %d", len(records))
	}
	if !records[0].HasUnjustifiedConclusion {
		t.Error("expected first record to have has_unjustified_conclusion=true")
	}
	if records[0].RunID != "run1" {
		t.Errorf("expected run_id run1, got %s", records[0].RunID)
	}
	if records[0].Step != 2 {
		t.Errorf("expected step 2, got %d", records[0].Step)
	}
	if records[1].HasUnjustifiedConclusion {
		t.Error("expected second record to have has_unjustified_conclusion=false")
	}

	// Output should be valid JSONL.
	outLines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(outLines) != 2 {
		t.Errorf("expected 2 output lines, got %d: %v", len(outLines), outLines)
	}
	for i, line := range outLines {
		var rec outputRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("output line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestProcessTranscript_SkipsNonAssistantMessages(t *testing.T) {
	inputLines := []string{
		`{"type":"message.created","run_id":"run1","step":1,"payload":{"role":"user","content":"do this"}}`,
		`{"type":"message.created","run_id":"run1","step":2,"payload":{"role":"system","content":"you are helpful"}}`,
	}
	input := strings.Join(inputLines, "\n")
	eval := &recordingEvaluator{results: map[string]*cw.EvaluatorResult{}}

	var outBuf bytes.Buffer
	records, err := processTranscript(context.Background(), bufio.NewScanner(strings.NewReader(input)), eval, 1, &outBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for non-assistant messages, got %d", len(records))
	}
}

func TestProcessTranscript_EmptyInput(t *testing.T) {
	eval := &recordingEvaluator{results: map[string]*cw.EvaluatorResult{}}
	var outBuf bytes.Buffer
	records, err := processTranscript(context.Background(), bufio.NewScanner(strings.NewReader("")), eval, 1, &outBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for empty input, got %d", len(records))
	}
}
