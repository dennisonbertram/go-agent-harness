package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteConversationStoreRewindPointRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	if err := store.SaveConversation(ctx, "rewind-conv", []Message{{Role: "user", Content: "edit it"}}); err != nil {
		t.Fatal(err)
	}
	point := RewindPoint{ID: "point-1", ConversationID: "rewind-conv", Step: 1, Tool: "write", Files: []RewindFileSnapshot{{Path: "notes.txt", Content: []byte("before"), Exists: true}}}
	if err := store.SaveRewindPoint(ctx, point); err != nil {
		t.Fatalf("SaveRewindPoint: %v", err)
	}
	points, err := store.ListRewindPoints(ctx, "rewind-conv")
	if err != nil {
		t.Fatalf("ListRewindPoints: %v", err)
	}
	if len(points) != 1 || points[0].ID != point.ID || points[0].Files[0].Path != "notes.txt" || string(points[0].Files[0].Content) != "before" {
		t.Fatalf("points = %#v", points)
	}
}

func TestSQLiteConversationStoreRestoreRewindRefusesExternalModification(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConversation(ctx, "restore-conv", []Message{{Role: "user", Content: "keep"}, {Role: "assistant", Content: "drop"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRewindPoint(ctx, RewindPoint{ID: "restore-point", ConversationID: "restore-conv", Step: 0, Tool: "write", Files: []RewindFileSnapshot{{Path: "notes.txt", Content: []byte("before"), Exists: true, ExpectedHash: RewindContentHash([]byte("agent"))}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RestoreRewindPoint(ctx, "restore-conv", "restore-point", root, false); err == nil {
		t.Fatal("RestoreRewindPoint accepted externally modified file")
	}
	result, err := store.RestoreRewindPoint(ctx, "restore-conv", "restore-point", root, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilesRestored != 1 {
		t.Fatalf("result=%+v", result)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "before" {
		t.Fatalf("file=%q", got)
	}
	msgs, err := store.LoadMessages(ctx, "restore-conv")
	if err != nil || len(msgs) != 1 || msgs[0].Content != "keep" {
		t.Fatalf("msgs=%#v err=%v", msgs, err)
	}
}

func TestCaptureRewindPreImageSkipsOversizedFiles(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big.txt"), make([]byte, rewindMaxFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CaptureRewindPreImage(ctx, store, RewindPoint{ID: "cap", ConversationID: "capconv", Tool: "write"}, root, []byte(`{"path":"big.txt"}`)); err != nil {
		t.Fatal(err)
	}
	points, err := store.ListRewindPoints(ctx, "capconv")
	if err != nil || len(points) != 1 || !points[0].Files[0].Skipped {
		t.Fatalf("points=%#v err=%v", points, err)
	}
}

func TestCapturedAndFinalizedRewindRejectsExternalEdit(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	point := RewindPoint{ID: "real", ConversationID: "real-conv", Step: 0, Tool: "write"}
	if err := CaptureRewindPreImage(ctx, store, point, root, []byte(`{"path":"notes.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("agent"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "real", root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RestoreRewindPoint(ctx, "real-conv", "real", root, false); err != nil {
		t.Fatalf("unchanged restore: %v", err)
	}
	if err := os.WriteFile(path, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RestoreRewindPoint(ctx, "real-conv", "real", root, false); err == nil {
		t.Fatal("external edit accepted")
	}
}

func TestRestoreRewindPoint_OlderPointAfterAgentEditNotRefused(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")

	// Run 1: agent writes a.txt v1.
	if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	writePoint := RewindPoint{ID: "write-point", ConversationID: "multi-edit-conv", Step: 0, Tool: "write"}
	if err := CaptureRewindPreImage(ctx, store, writePoint, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "write-point", root); err != nil {
		t.Fatal(err)
	}

	// Run 2: agent edits a.txt v1 -> v2. The pre-image capture for this point
	// snapshots the current (v1) content, then the tool writes v2.
	editPoint := RewindPoint{ID: "edit-point", ConversationID: "multi-edit-conv", Step: 1, Tool: "edit"}
	if err := CaptureRewindPreImage(ctx, store, editPoint, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "edit-point", root); err != nil {
		t.Fatal(err)
	}

	// Rewinding to the older write-point must succeed without force: nothing
	// outside the agent touched a.txt, the file was only later edited by the
	// agent itself in run 2.
	result, err := store.RestoreRewindPoint(ctx, "multi-edit-conv", "write-point", root, false)
	if err != nil {
		t.Fatalf("RestoreRewindPoint refused an agent-only later edit: %v", err)
	}
	if result.FilesRestored != 1 {
		t.Fatalf("result=%+v", result)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "v1" {
		t.Fatalf("file=%q err=%v, want v1", got, err)
	}
}

func TestRestoreRewindPoint_StillRefusesExternalEdit(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")

	if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	writePoint := RewindPoint{ID: "write-point", ConversationID: "external-edit-conv", Step: 0, Tool: "write"}
	if err := CaptureRewindPreImage(ctx, store, writePoint, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "write-point", root); err != nil {
		t.Fatal(err)
	}

	editPoint := RewindPoint{ID: "edit-point", ConversationID: "external-edit-conv", Step: 1, Tool: "edit"}
	if err := CaptureRewindPreImage(ctx, store, editPoint, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "edit-point", root); err != nil {
		t.Fatal(err)
	}

	// Something outside the agent now edits a.txt (v2 -> v3) with no further
	// tool call, so no finalize runs to refresh expected_hash.
	if err := os.WriteFile(path, []byte("v3"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := store.RestoreRewindPoint(ctx, "external-edit-conv", "write-point", root, false); err == nil {
		t.Fatal("RestoreRewindPoint accepted an externally modified file for the older point")
	}
	if _, err := store.RestoreRewindPoint(ctx, "external-edit-conv", "edit-point", root, false); err == nil {
		t.Fatal("RestoreRewindPoint accepted an externally modified file for the latest point")
	}
}

func TestConversationSnapshotCapSkipsAdditionalContent(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	first := make([]byte, rewindMaxConversationBytes)
	if err := store.SaveRewindPoint(ctx, RewindPoint{ID: "one", ConversationID: "cap-total", Files: []RewindFileSnapshot{{Path: "one", Content: first, Exists: true}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRewindPoint(ctx, RewindPoint{ID: "two", ConversationID: "cap-total", Files: []RewindFileSnapshot{{Path: "two", Content: []byte("x"), Exists: true}}}); err != nil {
		t.Fatal(err)
	}
	points, err := store.ListRewindPoints(ctx, "cap-total")
	if err != nil {
		t.Fatal(err)
	}
	if !points[0].Files[0].Skipped {
		t.Fatalf("expected cap skip: %#v", points[0])
	}
}

func TestExtractRewindPathsUsesWriteEditAndPatchArguments(t *testing.T) {
	paths := ExtractRewindPaths("apply_patch", []byte(`{"patch":"--- a/a.txt\n+++ b/a.txt\n--- a/b.txt\n+++ b/b.txt"}`))
	if len(paths) != 2 || paths[0] != "a.txt" || paths[1] != "b.txt" {
		t.Fatalf("paths = %#v", paths)
	}
}
