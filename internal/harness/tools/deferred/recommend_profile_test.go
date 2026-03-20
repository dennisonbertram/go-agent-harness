package deferred

import (
	"context"
	"encoding/json"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
)

// TestRecommendProfileTool_Definition verifies that the recommend_profile tool is TierDeferred.
func TestRecommendProfileTool_Definition(t *testing.T) {
	t.Parallel()

	tool := RecommendProfileTool()
	assertToolDef(t, tool, "recommend_profile", tools.TierDeferred)
	assertHasTags(t, tool, "profile", "recommend")
}

// TestRecommendProfileTool_ReturnsRecommendation verifies that a valid task gets a recommendation response.
func TestRecommendProfileTool_ReturnsRecommendation(t *testing.T) {
	t.Parallel()

	tool := RecommendProfileTool()
	args, _ := json.Marshal(map[string]string{"task": "review the code for security issues"})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Result should be valid JSON containing profile_name, reason, and confidence.
	var rec map[string]any
	if err := json.Unmarshal([]byte(result), &rec); err != nil {
		t.Fatalf("result is not valid JSON: %v\nresult: %s", err, result)
	}
	profileName, ok := rec["profile_name"].(string)
	if !ok || profileName == "" {
		t.Errorf("expected non-empty 'profile_name' in result, got %v", rec)
	}
	reason, ok := rec["reason"].(string)
	if !ok || reason == "" {
		t.Errorf("expected non-empty 'reason' in result, got %v", rec)
	}
	confidence, ok := rec["confidence"].(string)
	if !ok || confidence == "" {
		t.Errorf("expected non-empty 'confidence' in result, got %v", rec)
	}
}

// TestRecommendProfileTool_MissingTask verifies that the tool returns an error when task is empty.
func TestRecommendProfileTool_MissingTask(t *testing.T) {
	t.Parallel()

	tool := RecommendProfileTool()
	_, err := tool.Handler(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing task")
	}
}

// TestRecommendProfileTool_FallbackToFull verifies that a generic task falls back to the full profile.
func TestRecommendProfileTool_FallbackToFull(t *testing.T) {
	t.Parallel()

	tool := RecommendProfileTool()
	args, _ := json.Marshal(map[string]string{"task": "do some generic work"})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(result), &rec); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	profileName, _ := rec["profile_name"].(string)
	if profileName != "full" {
		t.Errorf("expected fallback 'full', got %q", profileName)
	}
}

// TestRecommendProfileTool_ResearcherProfile verifies that research tasks are routed to researcher.
func TestRecommendProfileTool_ResearcherProfile(t *testing.T) {
	t.Parallel()

	tool := RecommendProfileTool()
	args, _ := json.Marshal(map[string]string{"task": "research the documentation for the API"})
	result, err := tool.Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(result), &rec); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	profileName, _ := rec["profile_name"].(string)
	if profileName != "researcher" {
		t.Errorf("expected 'researcher', got %q", profileName)
	}
}

// TestRecommendProfileTool_InvalidJSON verifies the tool returns an error for invalid JSON input.
func TestRecommendProfileTool_InvalidJSON(t *testing.T) {
	t.Parallel()

	tool := RecommendProfileTool()
	_, err := tool.Handler(context.Background(), json.RawMessage(`not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestRecommendProfileTool_HasRequiredParam verifies the tool schema requires the task parameter.
func TestRecommendProfileTool_HasRequiredParam(t *testing.T) {
	t.Parallel()

	tool := RecommendProfileTool()
	required, _ := tool.Definition.Parameters["required"].([]string)
	hasTask := false
	for _, r := range required {
		if r == "task" {
			hasTask = true
			break
		}
	}
	if !hasTask {
		t.Error("expected 'task' to be in required parameters")
	}
}
