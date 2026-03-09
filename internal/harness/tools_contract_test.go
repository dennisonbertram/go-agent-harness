package harness

import (
	"testing"
)

func TestDefaultRegistryToolContract(t *testing.T) {
	t.Parallel()

	registry := NewDefaultRegistry(t.TempDir())
	defs := registry.Definitions()

	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
		if def.Parameters == nil {
			t.Fatalf("tool %q missing parameters schema", def.Name)
		}
	}

	expected := []string{
		"AskUserQuestion",
		"apply_patch",
		"bash",
		"edit",
		"job_kill",
		"job_output",
		"observational_memory",
		"read",
		"todos",
		"write",
	}
	if len(names) != len(expected) {
		t.Fatalf("expected %d tools, got %d (%v)", len(expected), len(names), names)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("unexpected tools order/value. got=%v want=%v", names, expected)
		}
	}
}
