package tooluse

import "testing"

func TestModelNewAndViewStub(t *testing.T) {
	t.Parallel()

	m := New("call-1", "bash")
	if m.CallID != "call-1" || m.ToolName != "bash" {
		t.Fatalf("unexpected model: %+v", m)
	}
	if got := m.View(); got != "" {
		t.Fatalf("expected stub view to return empty string, got %q", got)
	}
}
