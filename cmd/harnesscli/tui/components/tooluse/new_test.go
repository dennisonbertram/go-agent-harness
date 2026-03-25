package tooluse

import "testing"

func TestNewInitializesIdentityFields(t *testing.T) {
	t.Parallel()

	model := New("call-123", "bash")
	if model.CallID != "call-123" {
		t.Fatalf("CallID = %q, want %q", model.CallID, "call-123")
	}
	if model.ToolName != "bash" {
		t.Fatalf("ToolName = %q, want %q", model.ToolName, "bash")
	}
	if model.Status != "" {
		t.Fatalf("Status = %q, want empty", model.Status)
	}
}
