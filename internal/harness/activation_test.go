package harness

import (
	"sort"
	"sync"
	"testing"
)

func TestActivationTracker_BasicActivateAndCheck(t *testing.T) {
	tr := NewActivationTracker()
	tr.Activate("run-1", "bash")

	if !tr.IsActive("run-1", "bash") {
		t.Fatal("expected bash to be active for run-1")
	}
	if tr.IsActive("run-1", "grep") {
		t.Fatal("expected grep to NOT be active for run-1")
	}
	if tr.IsActive("run-2", "bash") {
		t.Fatal("expected bash to NOT be active for run-2")
	}
}

func TestActivationTracker_PerRunIsolation(t *testing.T) {
	tr := NewActivationTracker()
	tr.Activate("run-a", "bash", "grep")
	tr.Activate("run-b", "write_file")

	// run-a should have bash and grep but not write_file
	if !tr.IsActive("run-a", "bash") {
		t.Fatal("run-a should have bash")
	}
	if !tr.IsActive("run-a", "grep") {
		t.Fatal("run-a should have grep")
	}
	if tr.IsActive("run-a", "write_file") {
		t.Fatal("run-a should NOT have write_file")
	}

	// run-b should have write_file but not bash or grep
	if !tr.IsActive("run-b", "write_file") {
		t.Fatal("run-b should have write_file")
	}
	if tr.IsActive("run-b", "bash") {
		t.Fatal("run-b should NOT have bash")
	}
	if tr.IsActive("run-b", "grep") {
		t.Fatal("run-b should NOT have grep")
	}
}

func TestActivationTracker_Concurrent(t *testing.T) {
	tr := NewActivationTracker()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Concurrent activations
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			runID := "run-concurrent"
			toolName := "tool-" + string(rune('a'+idx%26))
			tr.Activate(runID, toolName)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			runID := "run-concurrent"
			toolName := "tool-" + string(rune('a'+idx%26))
			_ = tr.IsActive(runID, toolName)
		}(i)
	}

	// Concurrent ActiveTools calls
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = tr.ActiveTools("run-concurrent")
		}()
	}

	wg.Wait()

	// After all goroutines finish, verify cleanup also works concurrently
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tr.Cleanup("run-concurrent")
		}()
	}
	wg.Wait()
}

func TestActivationTracker_Cleanup(t *testing.T) {
	tr := NewActivationTracker()
	tr.Activate("run-x", "bash", "grep", "write_file")

	// Precondition: tools are active
	if !tr.IsActive("run-x", "bash") {
		t.Fatal("precondition: bash should be active")
	}

	tr.Cleanup("run-x")

	if tr.IsActive("run-x", "bash") {
		t.Fatal("bash should NOT be active after cleanup")
	}
	if tr.IsActive("run-x", "grep") {
		t.Fatal("grep should NOT be active after cleanup")
	}
	if tr.IsActive("run-x", "write_file") {
		t.Fatal("write_file should NOT be active after cleanup")
	}

	tools := tr.ActiveTools("run-x")
	if tools != nil {
		t.Fatalf("expected nil after cleanup, got %v", tools)
	}
}

func TestActivationTracker_MultiTool(t *testing.T) {
	tr := NewActivationTracker()
	tr.Activate("run-m", "bash", "grep", "write_file", "read_file")

	for _, name := range []string{"bash", "grep", "write_file", "read_file"} {
		if !tr.IsActive("run-m", name) {
			t.Fatalf("expected %s to be active", name)
		}
	}
}

func TestActivationTracker_IdempotentActivate(t *testing.T) {
	tr := NewActivationTracker()
	tr.Activate("run-idem", "bash")
	tr.Activate("run-idem", "bash")
	tr.Activate("run-idem", "bash")

	if !tr.IsActive("run-idem", "bash") {
		t.Fatal("bash should be active after idempotent activations")
	}

	tools := tr.ActiveTools("run-idem")
	if len(tools) != 1 {
		t.Fatalf("expected exactly 1 tool, got %d: %v", len(tools), tools)
	}
	if tools[0] != "bash" {
		t.Fatalf("expected bash, got %s", tools[0])
	}
}

func TestActivationTracker_ActiveTools(t *testing.T) {
	tr := NewActivationTracker()

	// Empty run returns nil
	tools := tr.ActiveTools("run-empty")
	if tools != nil {
		t.Fatalf("expected nil for empty run, got %v", tools)
	}

	// Add tools and verify complete set
	tr.Activate("run-at", "grep", "bash", "write_file")

	tools = tr.ActiveTools("run-at")
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(tools), tools)
	}

	sort.Strings(tools)
	expected := []string{"bash", "grep", "write_file"}
	for i, name := range expected {
		if tools[i] != name {
			t.Fatalf("expected tools[%d]=%s, got %s", i, name, tools[i])
		}
	}
}

func TestActivationTracker_CleanupIdempotent(t *testing.T) {
	tr := NewActivationTracker()

	// Cleanup on a non-existent run should not panic
	tr.Cleanup("never-existed")

	// Cleanup twice should not panic
	tr.Activate("run-ci", "bash")
	tr.Cleanup("run-ci")
	tr.Cleanup("run-ci")

	if tr.IsActive("run-ci", "bash") {
		t.Fatal("bash should not be active after double cleanup")
	}
}
