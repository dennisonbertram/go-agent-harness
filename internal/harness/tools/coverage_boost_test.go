package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type errPolicy struct{}

func (e errPolicy) Allow(_ context.Context, _ PolicyInput) (PolicyDecision, error) {
	return PolicyDecision{}, errors.New("policy boom")
}

func TestJobManagerCleanupAndResolveDirBranches(t *testing.T) {
	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)
	mgr.maxJobs = 0
	if _, err := mgr.runBackground(context.Background(), "echo hi", 1, "."); err == nil {
		t.Fatalf("expected max job limit error")
	}

	mgr2 := NewJobManager(workspace, func() time.Time { return time.Unix(1000, 0) })
	mgr2.ttl = 0
	mgr2.maxJobs = 2
	_, err := mgr2.runBackground(context.Background(), "echo hi", 1, ".")
	if err != nil {
		t.Fatalf("runBackground: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	mgr2.cleanupExpired()
	if _, err := resolveWorkingDir(workspace, "nested"); err != nil {
		// nested may not exist, but path resolution should still be inside workspace.
		if !strings.Contains(err.Error(), "escapes") && !strings.Contains(err.Error(), "absolute") {
			t.Fatalf("unexpected resolveWorkingDir error: %v", err)
		}
	}
}
