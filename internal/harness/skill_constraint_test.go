package harness

import (
	"sync"
	"testing"
)

func TestSkillConstraintTracker_NoConstraint(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	// With no constraint active, all tools are allowed
	if !tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash to be allowed with no constraint")
	}
	if !tracker.IsToolAllowed("run-1", "read_file") {
		t.Error("expected read_file to be allowed with no constraint")
	}
	if !tracker.IsToolAllowed("run-1", "AskUserQuestion") {
		t.Error("expected AskUserQuestion to be allowed with no constraint")
	}

	// Active should return false
	_, ok := tracker.Active("run-1")
	if ok {
		t.Error("expected no active constraint")
	}
}

func TestSkillConstraintTracker_WithConstraint(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "code-review",
		AllowedTools: []string{"read_file", "grep"},
	})

	// Listed tools should be allowed
	if !tracker.IsToolAllowed("run-1", "read_file") {
		t.Error("expected read_file to be allowed")
	}
	if !tracker.IsToolAllowed("run-1", "grep") {
		t.Error("expected grep to be allowed")
	}

	// Unlisted tools should be blocked
	if tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash to be blocked")
	}
	if tracker.IsToolAllowed("run-1", "write_file") {
		t.Error("expected write_file to be blocked")
	}
	if tracker.IsToolAllowed("run-1", "edit_file") {
		t.Error("expected edit_file to be blocked")
	}

	// Active should return the constraint
	c, ok := tracker.Active("run-1")
	if !ok {
		t.Fatal("expected active constraint")
	}
	if c.SkillName != "code-review" {
		t.Errorf("expected skill name 'code-review', got %q", c.SkillName)
	}
	if len(c.AllowedTools) != 2 {
		t.Errorf("expected 2 allowed tools, got %d", len(c.AllowedTools))
	}
}

func TestSkillConstraintTracker_Deactivate(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "code-review",
		AllowedTools: []string{"read_file"},
	})

	// Verify constraint is active
	if tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash to be blocked before deactivation")
	}

	// Deactivate
	tracker.Deactivate("run-1")

	// All tools should be allowed again
	if !tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash to be allowed after deactivation")
	}
	if !tracker.IsToolAllowed("run-1", "read_file") {
		t.Error("expected read_file to be allowed after deactivation")
	}

	_, ok := tracker.Active("run-1")
	if ok {
		t.Error("expected no active constraint after deactivation")
	}
}

func TestSkillConstraintTracker_Reactivate(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	// Activate first skill
	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "code-review",
		AllowedTools: []string{"read_file", "grep"},
	})

	if !tracker.IsToolAllowed("run-1", "read_file") {
		t.Error("expected read_file allowed under code-review")
	}
	if tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash blocked under code-review")
	}

	// Activate second skill (replaces first)
	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "deploy",
		AllowedTools: []string{"bash", "write_file"},
	})

	// New constraint applies
	if !tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash allowed under deploy")
	}
	if !tracker.IsToolAllowed("run-1", "write_file") {
		t.Error("expected write_file allowed under deploy")
	}
	// Old skill's tools no longer applies
	if tracker.IsToolAllowed("run-1", "read_file") {
		t.Error("expected read_file blocked under deploy")
	}
	if tracker.IsToolAllowed("run-1", "grep") {
		t.Error("expected grep blocked under deploy")
	}

	c, ok := tracker.Active("run-1")
	if !ok {
		t.Fatal("expected active constraint")
	}
	if c.SkillName != "deploy" {
		t.Errorf("expected skill name 'deploy', got %q", c.SkillName)
	}
}

func TestSkillConstraintTracker_EmptyAllowedTools(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	// nil AllowedTools = no restriction
	tracker.Activate("run-nil", SkillConstraint{
		SkillName:    "unrestricted",
		AllowedTools: nil,
	})
	if !tracker.IsToolAllowed("run-nil", "bash") {
		t.Error("nil AllowedTools should allow all tools")
	}
	if !tracker.IsToolAllowed("run-nil", "anything") {
		t.Error("nil AllowedTools should allow all tools")
	}

	// Empty slice AllowedTools = only always-available tools
	tracker.Activate("run-empty", SkillConstraint{
		SkillName:    "locked-down",
		AllowedTools: []string{},
	})
	if tracker.IsToolAllowed("run-empty", "bash") {
		t.Error("empty AllowedTools should block bash")
	}
	if tracker.IsToolAllowed("run-empty", "read_file") {
		t.Error("empty AllowedTools should block read_file")
	}
	// Always-available should still work
	if !tracker.IsToolAllowed("run-empty", "AskUserQuestion") {
		t.Error("AskUserQuestion should always be allowed")
	}
	if !tracker.IsToolAllowed("run-empty", "find_tool") {
		t.Error("find_tool should always be allowed")
	}
	if !tracker.IsToolAllowed("run-empty", "skill") {
		t.Error("skill should always be allowed")
	}
}

func TestSkillConstraintTracker_AlwaysAvailable(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	// Activate a restrictive constraint
	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "strict-skill",
		AllowedTools: []string{"read_file"},
	})

	// Always-available tools should pass regardless
	for name := range AlwaysAvailableTools {
		if !tracker.IsToolAllowed("run-1", name) {
			t.Errorf("always-available tool %q should be allowed", name)
		}
	}

	// Non-always-available, non-listed tool should be blocked
	if tracker.IsToolAllowed("run-1", "bash") {
		t.Error("bash should be blocked")
	}
}

func TestSkillConstraintTracker_PerRunIsolation(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	tracker.Activate("run-a", SkillConstraint{
		SkillName:    "skill-a",
		AllowedTools: []string{"read_file"},
	})
	tracker.Activate("run-b", SkillConstraint{
		SkillName:    "skill-b",
		AllowedTools: []string{"bash"},
	})

	// run-a allows read_file but not bash
	if !tracker.IsToolAllowed("run-a", "read_file") {
		t.Error("run-a should allow read_file")
	}
	if tracker.IsToolAllowed("run-a", "bash") {
		t.Error("run-a should block bash")
	}

	// run-b allows bash but not read_file
	if !tracker.IsToolAllowed("run-b", "bash") {
		t.Error("run-b should allow bash")
	}
	if tracker.IsToolAllowed("run-b", "read_file") {
		t.Error("run-b should block read_file")
	}

	// run-c has no constraint, allows everything
	if !tracker.IsToolAllowed("run-c", "bash") {
		t.Error("run-c should allow bash (no constraint)")
	}
	if !tracker.IsToolAllowed("run-c", "read_file") {
		t.Error("run-c should allow read_file (no constraint)")
	}
}

func TestSkillConstraintTracker_Cleanup(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "test-skill",
		AllowedTools: []string{"read_file"},
	})

	// Verify constraint is active
	if tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash blocked before cleanup")
	}

	// Cleanup
	tracker.Cleanup("run-1")

	// All tools allowed again
	if !tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash allowed after cleanup")
	}

	_, ok := tracker.Active("run-1")
	if ok {
		t.Error("expected no active constraint after cleanup")
	}
}

func TestSkillConstraintTracker_CleanupIdempotent(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	// Cleanup on non-existent run should not panic
	tracker.Cleanup("nonexistent")
	tracker.Cleanup("nonexistent")

	// Activate, cleanup twice
	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "test",
		AllowedTools: []string{"bash"},
	})
	tracker.Cleanup("run-1")
	tracker.Cleanup("run-1") // second cleanup should not panic
}

func TestSkillConstraintTracker_Concurrent(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			runID := "run-concurrent"
			for i := 0; i < iterations; i++ {
				tracker.Activate(runID, SkillConstraint{
					SkillName:    "skill",
					AllowedTools: []string{"read_file", "bash"},
				})
				tracker.IsToolAllowed(runID, "bash")
				tracker.IsToolAllowed(runID, "write_file")
				tracker.Active(runID)
				tracker.Deactivate(runID)
				tracker.IsToolAllowed(runID, "bash")
				tracker.Cleanup(runID)
			}
		}(g)
	}

	wg.Wait()
}

func TestSkillConstraintTracker_SingleToolConstraint(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "single",
		AllowedTools: []string{"bash"},
	})

	if !tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash to be allowed")
	}
	if tracker.IsToolAllowed("run-1", "read_file") {
		t.Error("expected read_file to be blocked")
	}
}

func TestSkillConstraintTracker_ManyToolsConstraint(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	tools := make([]string, 50)
	for i := range tools {
		tools[i] = "tool_" + string(rune('a'+i%26))
	}
	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "many-tools",
		AllowedTools: tools,
	})

	if !tracker.IsToolAllowed("run-1", "tool_a") {
		t.Error("expected tool_a to be allowed")
	}
	if tracker.IsToolAllowed("run-1", "not_in_list") {
		t.Error("expected not_in_list to be blocked")
	}
}

func TestSkillConstraintTracker_ToolNotRegistered(t *testing.T) {
	t.Parallel()
	tracker := NewSkillConstraintTracker()

	// Constraint lists a tool that might not exist in the registry.
	// IsToolAllowed doesn't check registration, just the constraint list.
	tracker.Activate("run-1", SkillConstraint{
		SkillName:    "test",
		AllowedTools: []string{"nonexistent_tool"},
	})

	if !tracker.IsToolAllowed("run-1", "nonexistent_tool") {
		t.Error("expected tool in allowed list to pass, even if not registered")
	}
	if tracker.IsToolAllowed("run-1", "bash") {
		t.Error("expected bash to be blocked")
	}
}
