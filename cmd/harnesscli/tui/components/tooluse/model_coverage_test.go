package tooluse

import (
	"strings"
	"testing"
	"time"
)

func TestNewInitializesIdentityFields(t *testing.T) {
	t.Parallel()

	m := New("call-new", "bash")
	if m.CallID != "call-new" {
		t.Fatalf("CallID = %q, want %q", m.CallID, "call-new")
	}
	if m.ToolName != "bash" {
		t.Fatalf("ToolName = %q, want %q", m.ToolName, "bash")
	}
}

func TestModelViewRendersLifecycleStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		model    Model
		wantText string
	}{
		{
			name:     "pending maps to running",
			model:    Model{CallID: "call-1", ToolName: "bash", Status: "pending", Width: 80},
			wantText: "bash(call-1)",
		},
		{
			name:     "completed",
			model:    Model{CallID: "call-2", ToolName: "read_file", Status: "completed", Width: 80, Timer: NewTimer().start(time.Unix(0, 0)).stop(time.Unix(2, 0))},
			wantText: "read_file(call-2)",
		},
		{
			name:     "error",
			model:    Model{CallID: "call-3", ToolName: "write_file", Status: "error", ErrorText: "permission denied", Hint: "check file permissions", Width: 80},
			wantText: "permission denied",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.model.View()
			if got == "" {
				t.Fatal("View() must return non-empty output")
			}
			if !strings.Contains(StripANSI(got), tc.wantText) {
				t.Fatalf("View() = %q, want %q", got, tc.wantText)
			}
		})
	}
}

func TestModelViewExpandedUsesComponentEntryPoint(t *testing.T) {
	m := Model{
		CallID:   "call-expanded",
		ToolName: "write_file",
		Status:   "completed",
		Expanded: true,
		Width:    80,
		Args:     "main.go",
		Params:   []Param{{Key: "path", Value: "main.go"}},
		Result:   "line one\nline two",
	}

	got := StripANSI(m.View())
	if !strings.Contains(got, "path: main.go") {
		t.Fatalf("expanded view must render params through the top-level entry point, got %q", got)
	}
	if !strings.Contains(got, "line one") {
		t.Fatalf("expanded view must render result through the top-level entry point, got %q", got)
	}
}

func TestModelViewExpandedBashOutputUsesTopLevelEntryPoint(t *testing.T) {
	m := Model{
		CallID:   "call-bash",
		ToolName: "bash",
		Status:   "completed",
		Expanded: true,
		Width:    80,
		Args:     "call-bash",
		Command:  "echo hello",
		Result:   strings.Repeat("line\n", 12),
	}

	got := StripANSI(m.View())
	if !strings.Contains(got, "$ echo hello") {
		t.Fatalf("expanded bash output must render the command label, got %q", got)
	}
	if !strings.Contains(got, "ctrl+o to expand") {
		t.Fatalf("expanded bash output must keep truncation semantics through the top-level entry point, got %q", got)
	}
}

func TestModelViewExpandedUnifiedDiffUsesDiffView(t *testing.T) {
	m := Model{
		CallID:   "call-diff",
		ToolName: "git_diff",
		Status:   "completed",
		Expanded: true,
		Width:    80,
		Args:     "HEAD",
		Result: `--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
-old line
+new line
 context line`,
	}

	got := StripANSI(m.View())
	if !strings.Contains(got, "main.go") {
		t.Fatalf("expanded unified diff must surface the diff viewer header, got %q", got)
	}
	if !strings.Contains(got, "╌") {
		t.Fatalf("expanded unified diff must render the diff viewer border, got %q", got)
	}
	if strings.Contains(got, "⎿  --- a/main.go") {
		t.Fatalf("expanded unified diff should not fall back to generic tree-line rendering, got %q", got)
	}
}
