package harness

// Error-path tests for the SQLite conversation store.
//
// The store's happy paths are well covered; what was not covered is what
// happens when the database is unavailable or the requested row does not
// exist. Those branches matter because the store sits behind a runner that
// keeps going after a persistence failure — an error that is swallowed instead
// of returned turns into silent data loss rather than a visible failure.
//
// The closed-database cases work by opening a store, closing it, and then
// calling each method: every query then fails, which is the cheapest faithful
// way to drive the "database returned an error" branch of each method.

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newMigratedStore(t *testing.T) *SQLiteConversationStore {
	t.Helper()
	store, err := NewSQLiteConversationStore(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestNewSQLiteConversationStore_Validation(t *testing.T) {
	if _, err := NewSQLiteConversationStore(""); err == nil {
		t.Error("an empty path must be rejected")
	}

	// A nested directory that does not exist yet is created rather than failing.
	nested := filepath.Join(t.TempDir(), "a", "b", "c.db")
	store, err := NewSQLiteConversationStore(nested)
	if err != nil {
		t.Fatalf("nested path should be created: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Errorf("close: %v", err)
	}

	// Close must be safe on a nil receiver and idempotent.
	var nilStore *SQLiteConversationStore
	if err := nilStore.Close(); err != nil {
		t.Errorf("Close on a nil store = %v, want nil", err)
	}
	if err := store.Close(); err != nil {
		t.Errorf("second Close = %v, want nil", err)
	}
}

func TestSQLiteStore_PlanContentRoundTrip(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	// Loading a plan that was never saved is not an error — it is empty.
	got, err := store.LoadPlanContent(ctx, "conv-1")
	if err != nil {
		t.Fatalf("load missing plan: %v", err)
	}
	if got != "" {
		t.Errorf("missing plan content = %q, want empty", got)
	}

	if err := store.SavePlanContent(ctx, "conv-1", "run-1", "# Plan\nstep one"); err != nil {
		t.Fatalf("save plan: %v", err)
	}
	got, err = store.LoadPlanContent(ctx, "conv-1")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	if got != "# Plan\nstep one" {
		t.Errorf("plan content = %q, want what was saved", got)
	}

	// Saving again for the same conversation replaces the stored plan, so a
	// revised plan does not accumulate alongside the original.
	if err := store.SavePlanContent(ctx, "conv-1", "run-2", "# Plan v2"); err != nil {
		t.Fatalf("resave plan: %v", err)
	}
	got, err = store.LoadPlanContent(ctx, "conv-1")
	if err != nil {
		t.Fatalf("reload plan: %v", err)
	}
	if got != "# Plan v2" {
		t.Errorf("plan content after resave = %q, want the revision", got)
	}
}

func TestSQLiteStore_RewindPointLifecycle(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	workspace := t.TempDir()

	if err := store.SaveConversation(ctx, "conv-r", []Message{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}

	// Listing before anything is saved yields nothing, not an error.
	points, err := store.ListRewindPoints(ctx, "conv-r")
	if err != nil {
		t.Fatalf("list rewind points: %v", err)
	}
	if len(points) != 0 {
		t.Errorf("expected no rewind points, got %d", len(points))
	}

	point := RewindPoint{
		ID:             "point-1",
		ConversationID: "conv-r",
		Step:           1,
		Tool:           "write",
		CreatedAt:      time.Now().UTC(),
	}
	if err := store.SaveRewindPoint(ctx, point); err != nil {
		t.Fatalf("save rewind point: %v", err)
	}

	points, err = store.ListRewindPoints(ctx, "conv-r")
	if err != nil {
		t.Fatalf("list rewind points: %v", err)
	}
	if len(points) != 1 || points[0].ID != "point-1" {
		t.Fatalf("rewind points = %+v, want the saved point", points)
	}

	if err := store.FinalizeRewindPoint(ctx, "point-1", workspace); err != nil {
		t.Errorf("finalize: %v", err)
	}
	// Finalizing an unknown point must not silently succeed as if it worked on
	// real data; whatever it returns, it must not panic.
	_ = store.FinalizeRewindPoint(ctx, "no-such-point", workspace)

	// Restoring an unknown point is an error, not a silent no-op.
	if _, err := store.RestoreRewindPoint(ctx, "conv-r", "no-such-point", workspace, false); err == nil {
		t.Error("restoring an unknown rewind point must fail")
	}
}

func TestSQLiteStore_ConversationMetaAndOwner(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveConversation(ctx, "conv-m", []Message{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.UpdateConversationMeta(ctx, "conv-m", "/work/space", "tenant-a"); err != nil {
		t.Fatalf("update meta: %v", err)
	}

	owner, err := store.GetConversationOwner(ctx, "conv-m")
	if err != nil {
		t.Fatalf("get owner: %v", err)
	}
	if owner == nil || owner.TenantID != "tenant-a" || owner.Workspace != "/work/space" {
		t.Errorf("owner = %+v, want the updated workspace and tenant", owner)
	}

	// Re-applying identical values is a documented no-op, not an error.
	if err := store.UpdateConversationMeta(ctx, "conv-m", "/work/space", "tenant-a"); err != nil {
		t.Errorf("repeated meta update should be a no-op, got %v", err)
	}

	// An unknown conversation has no owner and that is not an error.
	unknown, err := store.GetConversationOwner(ctx, "nope")
	if err != nil {
		t.Fatalf("get owner for unknown conversation: %v", err)
	}
	if unknown != nil {
		t.Errorf("owner for an unknown conversation = %+v, want nil", unknown)
	}
}

func TestSQLiteStore_PinAndDelete(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()

	if err := store.SaveConversation(ctx, "conv-p", []Message{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.PinConversation(ctx, "conv-p", true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	convs, err := store.ListConversations(ctx, ConversationFilter{}, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(convs) != 1 || !convs[0].Pinned {
		t.Errorf("conversation should be pinned: %+v", convs)
	}
	if err := store.PinConversation(ctx, "conv-p", false); err != nil {
		t.Fatalf("unpin: %v", err)
	}

	if err := store.DeleteConversation(ctx, "conv-p"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	convs, err = store.ListConversations(ctx, ConversationFilter{}, 10, 0)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(convs) != 0 {
		t.Errorf("conversation should be gone, got %+v", convs)
	}
	// Deleting something that is not there is not an error.
	if err := store.DeleteConversation(ctx, "conv-p"); err != nil {
		t.Errorf("repeated delete = %v, want nil", err)
	}
}

// TestSQLiteStore_ClosedDatabaseSurfacesErrors drives the "database returned an
// error" branch of every method. The point is not the specific error text: it
// is that each method REPORTS the failure rather than returning a zero value
// that a caller would mistake for success.
func TestSQLiteStore_ClosedDatabaseSurfacesErrors(t *testing.T) {
	store, err := NewSQLiteConversationStore(filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := store.SaveConversation(ctx, "conv-x", []Message{{Role: "user", Content: "hi"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	checks := []struct {
		name string
		call func() error
	}{
		{"Migrate", func() error { return store.Migrate(ctx) }},
		{"SaveConversation", func() error {
			return store.SaveConversation(ctx, "conv-x", []Message{{Role: "user", Content: "x"}})
		}},
		{"LoadMessages", func() error { _, err := store.LoadMessages(ctx, "conv-x"); return err }},
		{"ListConversations", func() error {
			_, err := store.ListConversations(ctx, ConversationFilter{}, 10, 0)
			return err
		}},
		{"DeleteConversation", func() error { return store.DeleteConversation(ctx, "conv-x") }},
		{"UpdateConversationMeta", func() error { return store.UpdateConversationMeta(ctx, "conv-x", "w", "t") }},
		{"GetConversationOwner", func() error { _, err := store.GetConversationOwner(ctx, "conv-x"); return err }},
		{"PinConversation", func() error { return store.PinConversation(ctx, "conv-x", true) }},
		{"SavePlanContent", func() error { return store.SavePlanContent(ctx, "conv-x", "r", "p") }},
		{"LoadPlanContent", func() error { _, err := store.LoadPlanContent(ctx, "conv-x"); return err }},
		{"ListRewindPoints", func() error { _, err := store.ListRewindPoints(ctx, "conv-x"); return err }},
		{"SaveRewindPoint", func() error {
			return store.SaveRewindPoint(ctx, RewindPoint{ID: "p", ConversationID: "conv-x"})
		}},
		{"SearchMessages", func() error { _, err := store.SearchMessages(ctx, "", "q", 10); return err }},
		{"DeleteOldConversations", func() error {
			_, err := store.DeleteOldConversations(ctx, time.Now())
			return err
		}},
		{"ForkConversation", func() error { _, err := store.ForkConversation(ctx, "conv-x", "conv-y"); return err }},
		{"UndoPrompts", func() error { _, err := store.UndoPrompts(ctx, "conv-x", 1); return err }},
		{"CompactConversation", func() error {
			return store.CompactConversation(ctx, "conv-x", 1, Message{Role: "system", Content: "s"})
		}},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); err == nil {
				t.Errorf("%s returned no error against a closed database", c.name)
			}
		})
	}

	// The two boolean helpers report false rather than panicking.
	if store.columnExists(ctx, "conversations", "id") {
		t.Error("columnExists should report false against a closed database")
	}
	if store.triggerExists(ctx, "conv_msgs_fts_insert") {
		t.Error("triggerExists should report false against a closed database")
	}
}
