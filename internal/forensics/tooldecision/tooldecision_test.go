package tooldecision_test

import (
	"encoding/json"
	"testing"

	"go-agent-harness/internal/forensics/tooldecision"
)

// TestToolDecisionSnapshotCallSequenceID verifies the human-readable call ID.
func TestToolDecisionSnapshotCallSequenceID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		seq  int
		want string
	}{
		{1, "call_1"},
		{2, "call_2"},
		{10, "call_10"},
	}
	for _, c := range cases {
		s := tooldecision.ToolDecisionSnapshot{CallSequence: c.seq}
		if got := s.CallSequenceID(); got != c.want {
			t.Errorf("CallSequenceID(%d) = %q, want %q", c.seq, got, c.want)
		}
	}
}

// TestToolDecisionSnapshotJSONRoundTrip verifies that ToolDecisionSnapshot
// serializes and deserializes correctly.
func TestToolDecisionSnapshotJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := tooldecision.ToolDecisionSnapshot{
		Step:           2,
		CallSequence:   5,
		AvailableTools: []string{"read_file", "write_file", "bash"},
		SelectedTools:  []string{"read_file"},
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got tooldecision.ToolDecisionSnapshot
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Step != orig.Step {
		t.Errorf("Step: got %d, want %d", got.Step, orig.Step)
	}
	if got.CallSequence != orig.CallSequence {
		t.Errorf("CallSequence: got %d, want %d", got.CallSequence, orig.CallSequence)
	}
	if len(got.AvailableTools) != len(orig.AvailableTools) {
		t.Errorf("AvailableTools len: got %d, want %d", len(got.AvailableTools), len(orig.AvailableTools))
	}
	if len(got.SelectedTools) != len(orig.SelectedTools) {
		t.Errorf("SelectedTools len: got %d, want %d", len(got.SelectedTools), len(orig.SelectedTools))
	}
}

// TestAntiPatternAlertJSONRoundTrip verifies serialization of AntiPatternAlert.
func TestAntiPatternAlertJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := tooldecision.AntiPatternAlert{
		Type:      tooldecision.AntiPatternRetryLoop,
		ToolName:  "bash",
		CallCount: 3,
		Step:      4,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got tooldecision.AntiPatternAlert
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != orig.Type {
		t.Errorf("Type: got %q, want %q", got.Type, orig.Type)
	}
	if got.ToolName != orig.ToolName {
		t.Errorf("ToolName: got %q, want %q", got.ToolName, orig.ToolName)
	}
	if got.CallCount != orig.CallCount {
		t.Errorf("CallCount: got %d, want %d", got.CallCount, orig.CallCount)
	}
	if got.Step != orig.Step {
		t.Errorf("Step: got %d, want %d", got.Step, orig.Step)
	}
}

// TestHookMutationJSONRoundTrip verifies serialization of HookMutation.
func TestHookMutationJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := tooldecision.HookMutation{
		ToolCallID: "call_abc",
		HookName:   "sanitize_hook",
		Action:     tooldecision.HookActionModify,
		ArgsBefore: `{"path":"/etc/passwd"}`,
		ArgsAfter:  `{"path":"/safe/path"}`,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got tooldecision.HookMutation
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ToolCallID != orig.ToolCallID {
		t.Errorf("ToolCallID: got %q, want %q", got.ToolCallID, orig.ToolCallID)
	}
	if got.Action != orig.Action {
		t.Errorf("Action: got %q, want %q", got.Action, orig.Action)
	}
	if got.ArgsBefore != orig.ArgsBefore {
		t.Errorf("ArgsBefore: got %q, want %q", got.ArgsBefore, orig.ArgsBefore)
	}
	if got.ArgsAfter != orig.ArgsAfter {
		t.Errorf("ArgsAfter: got %q, want %q", got.ArgsAfter, orig.ArgsAfter)
	}
}

// TestClassifyHookAction verifies the action classification logic.
func TestClassifyHookAction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		blocked    bool
		argsBefore string
		argsAfter  string
		want       tooldecision.HookMutationAction
	}{
		{
			name:       "block overrides all",
			blocked:    true,
			argsBefore: `{"x":1}`,
			argsAfter:  `{"x":2}`,
			want:       tooldecision.HookActionBlock,
		},
		{
			name:       "same args = allow",
			blocked:    false,
			argsBefore: `{"x":1}`,
			argsAfter:  `{"x":1}`,
			want:       tooldecision.HookActionAllow,
		},
		{
			name:       "empty before = inject",
			blocked:    false,
			argsBefore: "",
			argsAfter:  `{"x":1}`,
			want:       tooldecision.HookActionInject,
		},
		{
			name:       "null before = inject",
			blocked:    false,
			argsBefore: "null",
			argsAfter:  `{"x":1}`,
			want:       tooldecision.HookActionInject,
		},
		{
			name:       "different args = modify",
			blocked:    false,
			argsBefore: `{"path":"/etc/passwd"}`,
			argsAfter:  `{"path":"/safe"}`,
			want:       tooldecision.HookActionModify,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tooldecision.ClassifyHookAction(c.blocked, c.argsBefore, c.argsAfter)
			if got != c.want {
				t.Errorf("ClassifyHookAction(%v, %q, %q) = %q, want %q",
					c.blocked, c.argsBefore, c.argsAfter, got, c.want)
			}
		})
	}
}

// TestAntiPatternRetryLoopConstant verifies the retry loop constant value.
func TestAntiPatternRetryLoopConstant(t *testing.T) {
	t.Parallel()

	if tooldecision.AntiPatternRetryLoop != "retry_loop" {
		t.Errorf("AntiPatternRetryLoop = %q, want %q", tooldecision.AntiPatternRetryLoop, "retry_loop")
	}
}

// TestHookMutationActionConstants verifies all action constant values.
func TestHookMutationActionConstants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		got  tooldecision.HookMutationAction
		want string
	}{
		{tooldecision.HookActionAllow, "Allow"},
		{tooldecision.HookActionBlock, "Block"},
		{tooldecision.HookActionModify, "Modify"},
		{tooldecision.HookActionInject, "Inject"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("action constant = %q, want %q", c.got, c.want)
		}
	}
}
