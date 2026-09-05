package harness

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// TestRestoreRewindPoint_OldestPointAfterEditChainNotRefused is a regression
// test for issue #1371: it chains three agent edits to the same file and
// restores all the way back to the oldest point. If FinalizeRewindPoint
// regresses to updating only the point tied to the current tool call (the
// bug's root cause), the oldest point's expected_hash goes stale on the very
// first later edit and this restore is refused.
func TestRestoreRewindPoint_OldestPointAfterEditChainNotRefused(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	convID := "edit-chain-conv"

	versions := []string{"v1", "v2", "v3"}
	pointIDs := []string{"p0", "p1", "p2"}
	if err := os.WriteFile(path, []byte(versions[0]), 0o600); err != nil {
		t.Fatal(err)
	}
	for i, id := range pointIDs {
		point := RewindPoint{ID: id, ConversationID: convID, Step: i, Tool: "edit"}
		if err := CaptureRewindPreImage(ctx, store, point, root, []byte(`{"path":"a.txt"}`)); err != nil {
			t.Fatalf("capture %s: %v", id, err)
		}
		if i+1 < len(versions) {
			if err := os.WriteFile(path, []byte(versions[i+1]), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := FinalizeRewindPoint(ctx, store, id, root); err != nil {
			t.Fatalf("finalize %s: %v", id, err)
		}
	}

	// The file is now at v3, written by three chained agent edits with
	// nothing external in between. Restoring to the oldest point (p0, whose
	// pre-image is v1) must succeed without force.
	result, err := store.RestoreRewindPoint(ctx, convID, "p0", root, false)
	if err != nil {
		t.Fatalf("RestoreRewindPoint refused the oldest point after an agent-only edit chain: %v", err)
	}
	if result.FilesRestored != 1 {
		t.Fatalf("result=%+v", result)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "v1" {
		t.Fatalf("file=%q err=%v, want v1", got, err)
	}
}

// TestFinalizeRewindPoint_DoesNotCrossContaminateOtherPaths guards the fix's
// path scoping: refreshing expected_hash across a conversation's earlier
// points must stay confined to the path the current tool call touched. If a
// future change widened the UPDATE to every path in the conversation, this
// test would catch it: b.txt's expected_hash would then reflect a.txt's
// unrelated edit and either falsely refuse or falsely accept a real external
// edit to b.txt.
func TestFinalizeRewindPoint_DoesNotCrossContaminateOtherPaths(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	convID := "multi-path-conv"
	aPath := filepath.Join(root, "a.txt")
	bPath := filepath.Join(root, "b.txt")

	if err := os.WriteFile(aPath, []byte("a1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("b1"), 0o600); err != nil {
		t.Fatal(err)
	}
	firstPoint := RewindPoint{ID: "first", ConversationID: convID, Step: 0, Tool: "write"}
	if err := CaptureRewindPreImage(ctx, store, firstPoint, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := CaptureRewindPreImage(ctx, store, RewindPoint{ID: "first-b", ConversationID: convID, Step: 0, Tool: "write"}, root, []byte(`{"path":"b.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "first", root); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "first-b", root); err != nil {
		t.Fatal(err)
	}

	// A later agent edit touches only a.txt.
	editPoint := RewindPoint{ID: "second", ConversationID: convID, Step: 1, Tool: "edit"}
	if err := CaptureRewindPreImage(ctx, store, editPoint, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aPath, []byte("a2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "second", root); err != nil {
		t.Fatal(err)
	}

	// Something external now edits b.txt, which no agent tool call touched.
	if err := os.WriteFile(bPath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Restoring the a.txt point must still succeed (agent-only edit chain).
	if _, err := store.RestoreRewindPoint(ctx, convID, "first", root, false); err != nil {
		t.Fatalf("RestoreRewindPoint refused the a.txt point: %v", err)
	}
	// Restoring the b.txt point must still be refused: b.txt's expected_hash
	// must not have been overwritten by the unrelated a.txt finalize.
	if _, err := store.RestoreRewindPoint(ctx, convID, "first-b", root, false); err == nil {
		t.Fatal("RestoreRewindPoint accepted an externally modified b.txt after an unrelated a.txt finalize")
	}
}

// TestRestoreRewindPoint_SecondConsecutiveRestoreToOlderPointNotRefused is a
// regression for a follow-up to issue #1371: after RestoreRewindPoint writes
// a point's pre-image back to disk, surviving snapshot rows for that path
// (the target point and any older ones) still carried the pre-rewind
// expected_hash, so a second, older restore in the same path chain was
// refused as "modified outside the agent" even though the only change was
// the first restore itself.
func TestRestoreRewindPoint_SecondConsecutiveRestoreToOlderPointNotRefused(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	convID := "double-restore-conv"

	if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	pointA := RewindPoint{ID: "pA", ConversationID: convID, Step: 0, Tool: "edit"}
	if err := CaptureRewindPreImage(ctx, store, pointA, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "pA", root); err != nil {
		t.Fatal(err)
	}

	pointB := RewindPoint{ID: "pB", ConversationID: convID, Step: 1, Tool: "edit"}
	if err := CaptureRewindPreImage(ctx, store, pointB, root, []byte(`{"path":"a.txt"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("v3"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := FinalizeRewindPoint(ctx, store, "pB", root); err != nil {
		t.Fatal(err)
	}

	// Restore to the newest point first: pB's pre-image is v2.
	if _, err := store.RestoreRewindPoint(ctx, convID, "pB", root, false); err != nil {
		t.Fatalf("first restore (newest point) refused: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "v2" {
		t.Fatalf("after first restore file=%q err=%v, want v2", got, err)
	}

	// Restore to the older point next, without force. This must succeed:
	// the only change since the first restore was that restore itself.
	if _, err := store.RestoreRewindPoint(ctx, convID, "pA", root, false); err != nil {
		t.Fatalf("second restore to an older point refused: %v", err)
	}
	got, err = os.ReadFile(path)
	if err != nil || string(got) != "v1" {
		t.Fatalf("after second restore file=%q err=%v, want v1", got, err)
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

// TestRestoreRewindPoint_MultiRunTruncatesOnlyAfterPoint reproduces issue
// #1370: rewinding to a point captured during a conversation's second run
// must keep every message from the first run plus the second run's user
// prompt, deleting the tool-call message this point precedes and everything
// after it. The point's Step field is a run-local tool-call counter (run2's
// first mutating call is step 0 within run2), which is not comparable to
// conversation_messages.step (a conversation-wide index) -- comparing them
// directly deletes run 1's messages too.
//
// MessageBoundary is the index of the assistant message carrying the
// rewound tool call, not the index just after it: restoring must delete
// that assistant message too, not just its tool result, otherwise the
// persisted history ends with an assistant message whose tool_calls have no
// tool-result messages, which real providers (e.g. OpenAI) reject on the
// next turn even though the fake/stub providers used in tests do not.
func TestRestoreRewindPoint_MultiRunTruncatesOnlyAfterPoint(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	convID := "multi-run-conv"

	// Final persisted state after both runs completed: run 1 (4 messages)
	// followed by run 2 (4 messages). Steps are the conversation-wide index
	// 0..7, matching what SaveConversationWithCost writes at each run's
	// completion.
	all := []Message{
		{Role: "user", Content: "run1: write a.txt"},
		{Role: "assistant", Content: "", ToolCalls: []ToolCall{{ID: "c1", Name: "write"}}},
		{Role: "tool", Name: "write", ToolCallID: "c1", Content: "written"},
		{Role: "assistant", Content: "run1 done"},
		{Role: "user", Content: "run2: edit a.txt"},
		{Role: "assistant", Content: "", ToolCalls: []ToolCall{{ID: "c2", Name: "edit"}}},
		{Role: "tool", Name: "edit", ToolCallID: "c2", Content: "edited"},
		{Role: "assistant", Content: "run2 done"},
	}

	// The rewind point is captured mid-run-2, immediately before the "edit"
	// tool executes. The conversation holds 4 run-1 messages, run2's user
	// prompt at index 4, and run2's assistant tool-call message at index 5:
	// a message boundary of 5 (the tool-call message's own index).
	point := RewindPoint{ID: "run2-edit", ConversationID: convID, Step: 0, Tool: "edit", MessageBoundary: 5}
	if err := store.SaveRewindPoint(ctx, point); err != nil {
		t.Fatalf("SaveRewindPoint: %v", err)
	}

	// Run 2 completes and overwrites the full conversation-wide history, as
	// Runner.completeRun does via SaveConversationWithCost.
	if err := store.SaveConversation(ctx, convID, all); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	result, err := store.RestoreRewindPoint(ctx, convID, "run2-edit", t.TempDir(), true)
	if err != nil {
		t.Fatalf("RestoreRewindPoint: %v", err)
	}
	if result.MessagesTruncated != 3 {
		t.Errorf("MessagesTruncated = %d, want 3 (run2's tool-call message, tool result, and final answer)", result.MessagesTruncated)
	}
	got, err := store.LoadMessages(ctx, convID)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("LoadMessages returned %d messages, want 5 (run1's 4 plus run2's user prompt): %#v", len(got), got)
	}
	for i, want := range all[:5] {
		if got[i].Content != want.Content || got[i].Role != want.Role {
			t.Errorf("message[%d] = %+v, want %+v", i, got[i], want)
		}
	}
	if got[3].Content != "run1 done" {
		t.Fatalf("run 1's final answer was truncated; got[3]=%+v", got[3])
	}
	last := got[len(got)-1]
	if last.Role == "assistant" && len(last.ToolCalls) > 0 {
		t.Fatalf("restore left a dangling assistant message with tool_calls as the last persisted message: %+v", last)
	}
}

// TestRestoreRewindPoint_FallsBackWhenBoundaryUnset proves that points saved
// before MessageBoundary existed (or by any caller that never sets it) do not
// silently over-delete: restore falls back to the legacy step comparison
// (documented, if imperfect) and logs a warning rather than deleting
// everything at step>=0.
func TestRestoreRewindPoint_FallsBackWhenBoundaryUnset(t *testing.T) {
	ctx := context.Background()
	store := newTestConversationStore(t)
	convID := "legacy-point-conv"
	if err := store.SaveConversation(ctx, convID, []Message{
		{Role: "user", Content: "keep"},
		{Role: "assistant", Content: "drop"},
	}); err != nil {
		t.Fatal(err)
	}
	// No MessageBoundary set: zero value, matching every rewind point
	// captured before this field existed.
	if err := store.SaveRewindPoint(ctx, RewindPoint{ID: "legacy", ConversationID: convID, Step: 0, Tool: "write"}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	result, err := store.RestoreRewindPoint(ctx, convID, "legacy", t.TempDir(), true)
	if err != nil {
		t.Fatalf("RestoreRewindPoint: %v", err)
	}
	if result.MessagesTruncated != 1 {
		t.Fatalf("MessagesTruncated = %d, want 1 (legacy step-based fallback keeps step<=0)", result.MessagesTruncated)
	}
	got, err := store.LoadMessages(ctx, convID)
	if err != nil || len(got) != 1 || got[0].Content != "keep" {
		t.Fatalf("LoadMessages = %#v, err=%v, want [keep]", got, err)
	}
	if !strings.Contains(buf.String(), "legacy") {
		t.Fatalf("expected a logged warning naming the point falling back to step-based truncation, got: %q", buf.String())
	}
}

func TestExtractRewindPathsUsesWriteEditAndPatchArguments(t *testing.T) {
	paths := ExtractRewindPaths("apply_patch", []byte(`{"patch":"--- a/a.txt\n+++ b/a.txt\n--- a/b.txt\n+++ b/b.txt"}`))
	if len(paths) != 2 || paths[0] != "a.txt" || paths[1] != "b.txt" {
		t.Fatalf("paths = %#v", paths)
	}
}
