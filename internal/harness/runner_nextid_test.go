package harness

import (
	"regexp"
	"strings"
	"testing"
)

// uuidRe matches the UUID v4 format embedded in a nextID result.
var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// TestNextID_HasPrefix verifies the returned string starts with the given prefix and underscore.
func TestNextID_HasPrefix(t *testing.T) {
	t.Parallel()
	r := &Runner{}
	id := r.nextID("run")
	if !strings.HasPrefix(id, "run_") {
		t.Errorf("expected id to start with 'run_', got %q", id)
	}
}

// TestNextID_ContainsUUID verifies the suffix is a valid UUID v4.
func TestNextID_ContainsUUID(t *testing.T) {
	t.Parallel()
	r := &Runner{}
	id := r.nextID("run")
	parts := strings.SplitN(id, "_", 2)
	if len(parts) != 2 {
		t.Fatalf("expected exactly one underscore separator, got %q", id)
	}
	if !uuidRe.MatchString(parts[1]) {
		t.Errorf("suffix %q is not a valid UUID v4", parts[1])
	}
}

// TestNextID_Unique verifies that consecutive calls produce distinct IDs.
func TestNextID_Unique(t *testing.T) {
	t.Parallel()
	r := &Runner{}
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := r.nextID("run")
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}

// TestNextID_DifferentPrefixes verifies the prefix is respected.
func TestNextID_DifferentPrefixes(t *testing.T) {
	t.Parallel()
	r := &Runner{}
	for _, prefix := range []string{"run", "step", "call"} {
		id := r.nextID(prefix)
		if !strings.HasPrefix(id, prefix+"_") {
			t.Errorf("expected prefix %q in id %q", prefix+"_", id)
		}
	}
}
