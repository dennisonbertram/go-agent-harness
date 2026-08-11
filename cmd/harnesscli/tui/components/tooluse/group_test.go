package tooluse

import (
	"fmt"
	"strings"
	"testing"
)

var callSeq int

// successCall builds a distinct completed call. Real calls always carry distinct
// CallIDs; Add is upsert-by-CallID, so reusing one would mean an update.
func successCall(name string) Model {
	callSeq++
	return Model{CallID: fmt.Sprintf("%s-%d", name, callSeq), ToolName: name, Status: "completed", Args: `{"path":"."}`, Width: 100}
}

func failedCall(name, errText string) Model {
	callSeq++
	return Model{CallID: fmt.Sprintf("%s-%d", name, callSeq), ToolName: name, Status: "failed", ErrorText: errText, Width: 100}
}

// TestSuccessfulCallsCollapseByDefault is the core of issue #1308: a run that
// emits many successful tool calls must not spend a transcript line on each one.
func TestSuccessfulCallsCollapseByDefault(t *testing.T) {
	g := NewGroup(100)
	for _, name := range []string{"ls", "read", "git_status", "read", "ls", "read"} {
		g = g.Add(successCall(name))
	}

	out := g.View()
	if lines := strings.Count(strings.TrimRight(out, "\n"), "\n") + 1; lines != 1 {
		t.Errorf("collapsed group rendered %d lines, want 1:\n%s", lines, out)
	}
	// False-positive control: an empty line would also be one line.
	if !strings.Contains(out, "6") {
		t.Errorf("summary must name the call count, got: %q", out)
	}
}

// TestFailedCallStaysVisibleWhenCollapsed guards the rule that matters most: an
// error the user has to go looking for is worse than a noisy transcript.
func TestFailedCallStaysVisibleWhenCollapsed(t *testing.T) {
	g := NewGroup(100)
	g = g.Add(successCall("ls"))
	g = g.Add(failedCall("bash", "sandbox violation: absolute path \"/app\" escapes workspace"))
	g = g.Add(successCall("read"))

	if g.IsExpanded() {
		t.Fatal("precondition: group must start collapsed")
	}
	out := g.View()
	if !strings.Contains(out, "sandbox violation") {
		t.Errorf("failure text must remain visible while collapsed, got:\n%s", out)
	}
}

// TestSingleCallIsNotSummarized — a one-item summary is worse than the item.
func TestSingleCallIsNotSummarized(t *testing.T) {
	g := NewGroup(100).Add(successCall("ls"))

	if out := g.View(); !strings.Contains(out, "ls") {
		t.Errorf("a lone call must render as itself, got: %q", out)
	}
}

// TestExpandRevealsEveryCall checks the expanded rendering carries each call's
// own line, so expansion cannot pass by rendering a longer summary.
func TestExpandRevealsEveryCall(t *testing.T) {
	g := NewGroup(100)
	for _, name := range []string{"ls", "read", "git_status"} {
		g = g.Add(successCall(name))
	}

	g = g.Toggle()
	if !g.IsExpanded() {
		t.Fatal("Toggle did not expand the group")
	}

	out := g.View()
	for _, name := range []string{"ls", "read", "git_status"} {
		if !strings.Contains(out, name) {
			t.Errorf("expanded group missing call %q:\n%s", name, out)
		}
	}
	if lines := strings.Count(strings.TrimRight(out, "\n"), "\n") + 1; lines < 3 {
		t.Errorf("expanded group rendered %d lines, want at least 3", lines)
	}
}

// TestToggleReturnsToCollapsed checks the control is a toggle, not a one-way door.
func TestToggleReturnsToCollapsed(t *testing.T) {
	g := NewGroup(100)
	g = g.Add(successCall("ls")).Add(successCall("read"))

	g = g.Toggle().Toggle()

	if g.IsExpanded() {
		t.Error("second Toggle must collapse the group again")
	}
	if lines := strings.Count(strings.TrimRight(g.View(), "\n"), "\n") + 1; lines != 1 {
		t.Errorf("re-collapsed group rendered %d lines, want 1", lines)
	}
}

// TestGroupStaysExpandedAsCallsArrive — a group must not snap shut mid-run.
func TestGroupStaysExpandedAsCallsArrive(t *testing.T) {
	g := NewGroup(100).Add(successCall("ls")).Add(successCall("read"))
	g = g.Toggle()

	g = g.Add(successCall("git_status"))

	if !g.IsExpanded() {
		t.Error("group collapsed itself when a new call arrived")
	}
}

// TestEmptyGroupRendersNothing — no calls, no summary line.
func TestEmptyGroupRendersNothing(t *testing.T) {
	if out := NewGroup(100).View(); strings.TrimSpace(out) != "" {
		t.Errorf("empty group rendered %q, want nothing", out)
	}
}

// TestGroupCountsOnlyItsMembers is a false-positive control on the count.
func TestGroupCountsOnlyItsMembers(t *testing.T) {
	g := NewGroup(100)
	for i := 0; i < 12; i++ {
		g = g.Add(successCall("ls"))
	}
	if got := g.Len(); got != 12 {
		t.Errorf("Len() = %d, want 12", got)
	}
	if out := g.View(); !strings.Contains(out, "12") {
		t.Errorf("summary must report 12, got: %q", out)
	}
}

// TestAddUpsertsByCallID pins the semantic the TUI relies on: a lifecycle update
// for a call already in the group replaces it rather than duplicating it.
func TestAddUpsertsByCallID(t *testing.T) {
	running := Model{CallID: "c1", ToolName: "bash", Status: "running", Width: 100}
	g := NewGroup(100).Add(running).Add(successCall("ls"))
	if got := g.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}

	done := running
	done.Status = "completed"
	g = g.Add(done)

	if got := g.Len(); got != 2 {
		t.Errorf("Len() = %d after updating an existing call, want 2", got)
	}
	if g.HasFailure() {
		t.Error("no member failed")
	}
}

// TestUpdatedCallCanBecomeAFailure ensures a call that fails after being added
// still surfaces its error while collapsed.
func TestUpdatedCallCanBecomeAFailure(t *testing.T) {
	running := Model{CallID: "c9", ToolName: "bash", Status: "running", Width: 100}
	g := NewGroup(100).Add(running).Add(successCall("ls")).Add(successCall("read"))

	failed := running
	failed.Status = "failed"
	failed.ErrorText = "sandbox violation"
	g = g.Add(failed)

	if !g.HasFailure() {
		t.Fatal("group must report the failure")
	}
	if out := g.View(); !strings.Contains(out, "sandbox violation") {
		t.Errorf("collapsed view must show the failure, got:\n%s", out)
	}
}
