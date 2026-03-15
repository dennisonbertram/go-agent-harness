package planoverlay_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go-agent-harness/cmd/harnesscli/tui/components/planoverlay"
)

// ─── State constants ──────────────────────────────────────────────────────────

// TestTUI055_PlanStateConstants verifies the four PlanState constants are distinct.
func TestTUI055_PlanStateConstants(t *testing.T) {
	states := []planoverlay.PlanState{
		planoverlay.PlanStateHidden,
		planoverlay.PlanStatePending,
		planoverlay.PlanStateApproved,
		planoverlay.PlanStateRejected,
	}
	seen := make(map[planoverlay.PlanState]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("duplicate PlanState value: %v", s)
		}
		seen[s] = true
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 distinct PlanState values, got %d", len(seen))
	}
}

// ─── New ─────────────────────────────────────────────────────────────────────

// TestTUI055_NewStartsHidden verifies New() returns a hidden, invisible model.
func TestTUI055_NewStartsHidden(t *testing.T) {
	m := planoverlay.New()
	if m.CurrentState() != planoverlay.PlanStateHidden {
		t.Errorf("expected PlanStateHidden, got %v", m.CurrentState())
	}
	if m.IsVisible() {
		t.Error("new model should not be visible")
	}
}

// ─── Show ─────────────────────────────────────────────────────────────────────

// TestTUI055_ShowSetsPending verifies Show transitions to PlanStatePending.
func TestTUI055_ShowSetsPending(t *testing.T) {
	m := planoverlay.New()
	m2 := m.Show("step 1\nstep 2")
	if m2.CurrentState() != planoverlay.PlanStatePending {
		t.Errorf("expected PlanStatePending after Show, got %v", m2.CurrentState())
	}
	if m2.PlanText != "step 1\nstep 2" {
		t.Errorf("expected plan text stored, got %q", m2.PlanText)
	}
	if !m2.IsVisible() {
		t.Error("model should be visible after Show")
	}
}

// TestTUI055_ShowResetsScrollOffset verifies scroll offset is zeroed on Show.
func TestTUI055_ShowResetsScrollOffset(t *testing.T) {
	m := planoverlay.New()
	m.Height = 2
	m = m.Show("line1\nline2\nline3\nline4\nline5")
	m = m.ScrollDown(5)
	m = m.ScrollDown(5)
	if m.ScrollOffset == 0 {
		t.Skip("scroll did not advance (maxLines <= Height), test not applicable")
	}
	m2 := m.Show("fresh plan")
	if m2.ScrollOffset != 0 {
		t.Errorf("expected ScrollOffset=0 after Show, got %d", m2.ScrollOffset)
	}
}

// TestTUI055_ShowImmutability verifies Show returns a new copy (original unchanged).
func TestTUI055_ShowImmutability(t *testing.T) {
	orig := planoverlay.New()
	_ = orig.Show("some plan")
	if orig.CurrentState() != planoverlay.PlanStateHidden {
		t.Error("Show mutated the original model")
	}
	if orig.PlanText != "" {
		t.Error("Show wrote PlanText into original model")
	}
}

// ─── Approve ─────────────────────────────────────────────────────────────────

// TestTUI055_ApprovePendingToApproved verifies Approve works from PlanStatePending.
func TestTUI055_ApprovePendingToApproved(t *testing.T) {
	m := planoverlay.New().Show("my plan")
	m2 := m.Approve()
	if m2.CurrentState() != planoverlay.PlanStateApproved {
		t.Errorf("expected PlanStateApproved after Approve, got %v", m2.CurrentState())
	}
}

// TestTUI055_ApproveNoopWhenNotPending verifies Approve is a no-op from other states.
func TestTUI055_ApproveNoopWhenNotPending(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setup     func() planoverlay.Model
		wantState planoverlay.PlanState
	}{
		{"hidden", func() planoverlay.Model { return planoverlay.New() }, planoverlay.PlanStateHidden},
		{"approved", func() planoverlay.Model { return planoverlay.New().Show("x").Approve() }, planoverlay.PlanStateApproved},
		{"rejected", func() planoverlay.Model { return planoverlay.New().Show("x").Reject() }, planoverlay.PlanStateRejected},
	} {
		m := tc.setup()
		m2 := m.Approve()
		if m2.CurrentState() != tc.wantState {
			t.Errorf("[%s] Approve no-op failed: want %v, got %v", tc.name, tc.wantState, m2.CurrentState())
		}
	}
}

// TestTUI055_ApproveImmutability verifies Approve returns a new copy.
func TestTUI055_ApproveImmutability(t *testing.T) {
	pending := planoverlay.New().Show("plan")
	_ = pending.Approve()
	if pending.CurrentState() != planoverlay.PlanStatePending {
		t.Error("Approve mutated the original model")
	}
}

// ─── Reject ──────────────────────────────────────────────────────────────────

// TestTUI055_RejectPendingToRejected verifies Reject works from PlanStatePending.
func TestTUI055_RejectPendingToRejected(t *testing.T) {
	m := planoverlay.New().Show("my plan").Reject()
	if m.CurrentState() != planoverlay.PlanStateRejected {
		t.Errorf("expected PlanStateRejected, got %v", m.CurrentState())
	}
}

// TestTUI055_RejectNoopWhenNotPending verifies Reject is a no-op from non-pending states.
func TestTUI055_RejectNoopWhenNotPending(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setup     func() planoverlay.Model
		wantState planoverlay.PlanState
	}{
		{"hidden", func() planoverlay.Model { return planoverlay.New() }, planoverlay.PlanStateHidden},
		{"approved", func() planoverlay.Model { return planoverlay.New().Show("x").Approve() }, planoverlay.PlanStateApproved},
		{"rejected", func() planoverlay.Model { return planoverlay.New().Show("x").Reject() }, planoverlay.PlanStateRejected},
	} {
		m := tc.setup()
		m2 := m.Reject()
		if m2.CurrentState() != tc.wantState {
			t.Errorf("[%s] Reject no-op failed: want %v, got %v", tc.name, tc.wantState, m2.CurrentState())
		}
	}
}

// TestTUI055_RejectImmutability verifies Reject returns a new copy.
func TestTUI055_RejectImmutability(t *testing.T) {
	pending := planoverlay.New().Show("plan")
	_ = pending.Reject()
	if pending.CurrentState() != planoverlay.PlanStatePending {
		t.Error("Reject mutated the original model")
	}
}

// ─── Hide ────────────────────────────────────────────────────────────────────

// TestTUI055_HideFromAllStates verifies Hide works from any state.
func TestTUI055_HideFromAllStates(t *testing.T) {
	cases := []planoverlay.Model{
		planoverlay.New(),                     // hidden
		planoverlay.New().Show("x"),           // pending
		planoverlay.New().Show("x").Approve(), // approved
		planoverlay.New().Show("x").Reject(),  // rejected
	}
	for i, m := range cases {
		m2 := m.Hide()
		if m2.CurrentState() != planoverlay.PlanStateHidden {
			t.Errorf("[case %d] expected PlanStateHidden after Hide, got %v", i, m2.CurrentState())
		}
		if m2.IsVisible() {
			t.Errorf("[case %d] IsVisible() should be false after Hide", i)
		}
	}
}

// TestTUI055_HideImmutability verifies Hide returns a new copy.
func TestTUI055_HideImmutability(t *testing.T) {
	pending := planoverlay.New().Show("plan")
	_ = pending.Hide()
	if pending.CurrentState() != planoverlay.PlanStatePending {
		t.Error("Hide mutated the original model")
	}
}

// ─── Scroll ───────────────────────────────────────────────────────────────────

// TestTUI055_ScrollUpClampsAtZero verifies ScrollUp never goes below zero.
func TestTUI055_ScrollUpClampsAtZero(t *testing.T) {
	m := planoverlay.New().Show("a\nb\nc")
	m = m.ScrollUp() // offset is already 0
	if m.ScrollOffset != 0 {
		t.Errorf("expected ScrollOffset=0 after ScrollUp at floor, got %d", m.ScrollOffset)
	}
	m = m.ScrollUp()
	m = m.ScrollUp()
	if m.ScrollOffset != 0 {
		t.Errorf("expected ScrollOffset=0 after multiple ScrollUp at floor, got %d", m.ScrollOffset)
	}
}

// TestTUI055_ScrollDownClampsAtMax verifies ScrollDown is bounded by maxLines-Height.
func TestTUI055_ScrollDownClampsAtMax(t *testing.T) {
	m := planoverlay.New()
	m.Height = 5
	m = m.Show("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10")

	maxLines := 10
	// Scroll far past the end.
	for i := 0; i < 50; i++ {
		m = m.ScrollDown(maxLines)
	}
	limit := maxLines - m.Height
	if limit < 0 {
		limit = 0
	}
	if m.ScrollOffset > limit {
		t.Errorf("ScrollOffset %d exceeded limit %d", m.ScrollOffset, limit)
	}
}

// TestTUI055_ScrollImmutability verifies scroll methods return new copies.
func TestTUI055_ScrollImmutability(t *testing.T) {
	m := planoverlay.New().Show("a\nb\nc\nd\ne\nf\ng\nh")
	m.Height = 3
	orig := m.ScrollOffset
	_ = m.ScrollDown(8)
	if m.ScrollOffset != orig {
		t.Error("ScrollDown mutated the original model")
	}
	_ = m.ScrollUp()
	if m.ScrollOffset != orig {
		t.Error("ScrollUp mutated the original model")
	}
}

// TestTUI055_ScrollUpDecrementsOffset verifies ScrollUp decrements by one.
func TestTUI055_ScrollUpDecrementsOffset(t *testing.T) {
	m := planoverlay.New()
	m.Height = 2
	m = m.Show("a\nb\nc\nd\ne")
	// Advance first.
	m = m.ScrollDown(5)
	m = m.ScrollDown(5)
	before := m.ScrollOffset
	if before == 0 {
		t.Skip("scroll did not advance, test not applicable")
	}
	m2 := m.ScrollUp()
	if m2.ScrollOffset != before-1 {
		t.Errorf("expected offset %d after ScrollUp, got %d", before-1, m2.ScrollOffset)
	}
}

// ─── IsVisible ───────────────────────────────────────────────────────────────

// TestTUI055_IsVisibleOnlyWhenNotHidden verifies IsVisible returns true only for non-hidden states.
func TestTUI055_IsVisibleOnlyWhenNotHidden(t *testing.T) {
	cases := []struct {
		state   planoverlay.PlanState
		wantVis bool
	}{
		{planoverlay.PlanStateHidden, false},
		{planoverlay.PlanStatePending, true},
		{planoverlay.PlanStateApproved, true},
		{planoverlay.PlanStateRejected, true},
	}
	for _, tc := range cases {
		m := planoverlay.New()
		switch tc.state {
		case planoverlay.PlanStatePending:
			m = m.Show("x")
		case planoverlay.PlanStateApproved:
			m = m.Show("x").Approve()
		case planoverlay.PlanStateRejected:
			m = m.Show("x").Reject()
		}
		if m.IsVisible() != tc.wantVis {
			t.Errorf("state %v: IsVisible()=%v, want %v", tc.state, m.IsVisible(), tc.wantVis)
		}
	}
}

// ─── View ─────────────────────────────────────────────────────────────────────

// TestTUI055_ViewEmptyWhenHidden verifies View returns "" when hidden.
func TestTUI055_ViewEmptyWhenHidden(t *testing.T) {
	m := planoverlay.New()
	if out := m.View(); out != "" {
		t.Errorf("expected empty View when hidden, got %q", out)
	}
}

// TestTUI055_ViewContainsPlanText verifies plan text appears in View.
func TestTUI055_ViewContainsPlanText(t *testing.T) {
	m := planoverlay.New()
	m.Width = 80
	m.Height = 20
	m = m.Show("Step 1: gather requirements\nStep 2: implement\nStep 3: test")
	out := m.View()
	for _, line := range []string{"Step 1: gather requirements", "Step 2: implement"} {
		if !strings.Contains(out, line) {
			t.Errorf("plan text line %q not found in View output", line)
		}
	}
}

// TestTUI055_ViewContainsHeader verifies "Plan Mode" appears in View.
func TestTUI055_ViewContainsHeader(t *testing.T) {
	m := planoverlay.New()
	m.Width = 80
	m.Height = 20
	m = m.Show("a plan")
	out := m.View()
	if !strings.Contains(out, "Plan Mode") {
		t.Error("View should contain 'Plan Mode' header")
	}
}

// TestTUI055_ViewBadgePending verifies the pending badge is present when state is pending.
func TestTUI055_ViewBadgePending(t *testing.T) {
	m := planoverlay.New()
	m.Width = 80
	m.Height = 20
	m = m.Show("a plan")
	out := m.View()
	// The badge text may wrap across lines in narrow terminals; check for key words.
	if !strings.Contains(out, "Awaiting") {
		t.Errorf("expected 'Awaiting' badge text in pending view, got:\n%s", out)
	}
	if !strings.Contains(out, "Approval") {
		t.Errorf("expected 'Approval' badge text in pending view, got:\n%s", out)
	}
}

// TestTUI055_ViewBadgeApproved verifies the approved badge is present when state is approved.
func TestTUI055_ViewBadgeApproved(t *testing.T) {
	m := planoverlay.New()
	m.Width = 80
	m.Height = 20
	m = m.Show("a plan").Approve()
	out := m.View()
	if !strings.Contains(out, "Approved") {
		t.Errorf("expected 'Approved' badge in approved view, got:\n%s", out)
	}
}

// TestTUI055_ViewBadgeRejected verifies the rejected badge is present when state is rejected.
func TestTUI055_ViewBadgeRejected(t *testing.T) {
	m := planoverlay.New()
	m.Width = 80
	m.Height = 20
	m = m.Show("a plan").Reject()
	out := m.View()
	if !strings.Contains(out, "Rejected") {
		t.Errorf("expected 'Rejected' badge in rejected view, got:\n%s", out)
	}
}

// TestTUI055_ViewFooterHintOnlyWhenPending verifies the key-hint footer only appears in pending state.
func TestTUI055_ViewFooterHintOnlyWhenPending(t *testing.T) {
	m := planoverlay.New()
	m.Width = 80
	m.Height = 20
	m = m.Show("plan text")

	pendingOut := m.View()
	if !strings.Contains(pendingOut, "approve") {
		t.Error("pending view should contain approve hint")
	}
	if !strings.Contains(pendingOut, "reject") {
		t.Error("pending view should contain reject hint")
	}

	approvedOut := m.Approve().View()
	if strings.Contains(approvedOut, "y approve") {
		t.Error("approved view should not contain key hint")
	}

	rejectedOut := m.Reject().View()
	if strings.Contains(rejectedOut, "y approve") {
		t.Error("rejected view should not contain key hint")
	}
}

// TestTUI055_ViewMoreLinesFooter verifies "more lines" appears when content is scrollable.
func TestTUI055_ViewMoreLinesFooter(t *testing.T) {
	m := planoverlay.New()
	m.Width = 80
	m.Height = 10 // very small height to force truncation
	// Build plan text with many lines.
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("Plan line %02d: do something important", i+1)
	}
	m = m.Show(strings.Join(lines, "\n"))
	out := m.View()
	if !strings.Contains(out, "more line") {
		t.Errorf("expected 'more line(s)' footer when content is truncated, got:\n%s", out)
	}
}

// TestTUI055_ViewBoundaryDimensions verifies View(0,0) and large sizes don't panic.
func TestTUI055_ViewBoundaryDimensions(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on boundary dimensions: %v", r)
		}
	}()
	for _, dims := range [][2]int{{0, 0}, {5, 5}, {200, 50}} {
		m := planoverlay.New()
		m.Width = dims[0]
		m.Height = dims[1]
		m = m.Show("test plan")
		_ = m.View()
	}
}

// TestTUI055_ViewEmptyPlanText verifies View doesn't panic with empty plan text.
func TestTUI055_ViewEmptyPlanText(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic with empty plan text: %v", r)
		}
	}()
	m := planoverlay.New()
	m.Width = 80
	m.Height = 20
	m = m.Show("")
	out := m.View()
	if !strings.Contains(out, "no plan text") {
		t.Errorf("expected placeholder for empty plan text, got:\n%s", out)
	}
}

// ─── Concurrency ──────────────────────────────────────────────────────────────

// TestTUI055_ConcurrentModels verifies 10 goroutines can each hold their own
// Model and call all methods without data races.
func TestTUI055_ConcurrentModels(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := planoverlay.New()
			m.Width = 80
			m.Height = 20
			m = m.Show("parallel plan\nline 2\nline 3")
			_ = m.View()
			m = m.ScrollDown(3)
			m = m.ScrollUp()
			_ = m.IsVisible()
			_ = m.CurrentState()
			m = m.Approve()
			_ = m.View()
			m = m.Hide()
			_ = m.View()
		}()
	}
	wg.Wait()
}

// ─── Snapshots ────────────────────────────────────────────────────────────────

// snapshotTest renders the plan overlay and compares/creates a snapshot file.
func snapshotTest(t *testing.T, width, height int, snapshotName string) {
	t.Helper()
	m := planoverlay.New()
	m.Width = width
	m.Height = height
	m = m.Show("# Plan\n\n1. Research the problem space\n2. Design the solution\n3. Implement the feature\n4. Write tests\n5. Review and iterate\n6. Deploy to staging\n7. Monitor and observe\n8. Promote to production")

	out := m.View()

	snapshotDir := filepath.Join("testdata", "snapshots")
	snapshotPath := filepath.Join(snapshotDir, snapshotName)

	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		// First run: create snapshot.
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			t.Fatalf("failed to create snapshot dir: %v", err)
		}
		if err := os.WriteFile(snapshotPath, []byte(out), 0644); err != nil {
			t.Fatalf("failed to write snapshot: %v", err)
		}
		t.Logf("created snapshot: %s", snapshotPath)
		return
	}

	existing, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("failed to read snapshot: %v", err)
	}
	if string(existing) != out {
		t.Errorf("snapshot mismatch for %s\ngot:\n%s\nwant:\n%s", snapshotName, out, string(existing))
	}
}

// TestTUI055_VisualSnapshot_80x24 captures the 80x24 snapshot.
func TestTUI055_VisualSnapshot_80x24(t *testing.T) {
	snapshotTest(t, 80, 24, "TUI-055-plan-80x24.txt")
}

// TestTUI055_VisualSnapshot_120x40 captures the 120x40 snapshot.
func TestTUI055_VisualSnapshot_120x40(t *testing.T) {
	snapshotTest(t, 120, 40, "TUI-055-plan-120x40.txt")
}
