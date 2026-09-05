package server

import (
	"bytes"
	"context"
	"encoding/json"
	"go-agent-harness/internal/fakeprovider"
	"go-agent-harness/internal/harness"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewindPointsEndpointListsPoints(t *testing.T) {
	store := newTestSQLiteStore(t)
	if err := store.SaveConversation(context.Background(), "rewind-http", []harness.Message{{Role: "user", Content: "x"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRewindPoint(context.Background(), harness.RewindPoint{ID: "p", ConversationID: "rewind-http", Tool: "write"}); err != nil {
		t.Fatal(err)
	}
	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{ConversationStore: store})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/conversations/rewind-http/rewind-points", nil)
	New(runner).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

// TestRestoreRewindEndpointRestoresFileAndTruncatesMessages verifies
// POST /v1/conversations/{id}/rewind writes the pre-image back to disk and
// truncates messages after the rewind point, through the real HTTP handler.
func TestRestoreRewindEndpointRestoresFileAndTruncatesMessages(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "notes.txt")
	if err := os.WriteFile(path, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}

	convID := "rewind-restore-http"
	if err := store.SaveConversation(ctx, convID, []harness.Message{
		{Role: "user", Content: "keep"},
		{Role: "assistant", Content: "drop"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateConversationMeta(ctx, convID, workspace, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRewindPoint(ctx, harness.RewindPoint{
		ID:             "restore-point",
		ConversationID: convID,
		Step:           0,
		Tool:           "write",
		Files: []harness.RewindFileSnapshot{{
			Path:         "notes.txt",
			Content:      []byte("before"),
			Exists:       true,
			ExpectedHash: harness.RewindContentHash([]byte("after")),
		}},
	}); err != nil {
		t.Fatal(err)
	}

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{ConversationStore: store})

	body, _ := json.Marshal(map[string]any{"point_id": "restore-point"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/conversations/"+convID+"/rewind", bytes.NewReader(body))
	New(runner).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}

	var result harness.RewindRestoreResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	if result.FilesRestored != 1 {
		t.Errorf("FilesRestored = %d, want 1", result.FilesRestored)
	}
	if result.MessagesTruncated != 1 {
		t.Errorf("MessagesTruncated = %d, want 1", result.MessagesTruncated)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "before" {
		t.Fatalf("file content = %q, err=%v, want %q", got, err, "before")
	}
}

// TestRestoreRewindEndpointRequiresPointID verifies a missing point_id is
// rejected as a client error rather than reaching the store.
func TestRestoreRewindEndpointRequiresPointID(t *testing.T) {
	store := newTestSQLiteStore(t)
	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{ConversationStore: store})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/conversations/rewind-missing-point/rewind", bytes.NewReader([]byte(`{}`)))
	New(runner).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", rr.Code, rr.Body.String())
	}
}

// TestIssue1256_ForkRewindUsesInheritedTrustedWorkspace proves that the fork's
// persisted owner root, not the server process CWD or a client value, controls
// its destructive restore.
func TestIssue1256_ForkRewindUsesInheritedTrustedWorkspace(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "safe.txt")
	if err := os.WriteFile(path, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConversation(ctx, "source-1256", []harness.Message{{Role: "user", Content: "keep"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateConversationMeta(ctx, "source-1256", workspace, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ForkConversation(ctx, "source-1256", "fork-1256"); err != nil {
		t.Fatalf("ForkConversation: %v", err)
	}
	owner, err := store.GetConversationOwner(ctx, "fork-1256")
	if err != nil || owner == nil || owner.Workspace != workspace {
		t.Fatalf("fork owner = %+v, err=%v; want trusted workspace %q", owner, err, workspace)
	}
	if err := store.SaveRewindPoint(ctx, harness.RewindPoint{ID: "safe-point", ConversationID: "fork-1256", Tool: "write", Files: []harness.RewindFileSnapshot{{Path: "safe.txt", Content: []byte("before"), Exists: true, ExpectedHash: harness.RewindContentHash([]byte("after"))}}}); err != nil {
		t.Fatal(err)
	}

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{ConversationStore: store})
	body, _ := json.Marshal(map[string]any{"point_id": "safe-point"})
	rr := httptest.NewRecorder()
	New(runner).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/conversations/fork-1256/rewind", bytes.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "before" {
		t.Fatalf("fork workspace content = %q, err=%v, want before", got, err)
	}
}

// TestIssue1256_EmptyWorkspaceFailsClosedWithoutMutatingFiles ensures a
// rewind point cannot obtain a root from request, CWD, or another workspace.
func TestIssue1256_EmptyWorkspaceFailsClosedWithoutMutatingFiles(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "safe.txt")
	if err := os.WriteFile(path, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConversation(ctx, "empty-1256", []harness.Message{{Role: "user", Content: "keep"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRewindPoint(ctx, harness.RewindPoint{ID: "empty-point", ConversationID: "empty-1256", Tool: "write", Files: []harness.RewindFileSnapshot{{Path: "safe.txt", Content: []byte("before"), Exists: true, ExpectedHash: harness.RewindContentHash([]byte("after"))}}}); err != nil {
		t.Fatal(err)
	}
	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{ConversationStore: store})
	body, _ := json.Marshal(map[string]any{"point_id": "empty-point"})
	rr := httptest.NewRecorder()
	New(runner).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/conversations/empty-1256/rewind", bytes.NewReader(body)))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", rr.Code, rr.Body.String())
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "after" {
		t.Fatalf("untrusted workspace file = %q, err=%v, want untouched after", got, err)
	}
}

// TestRestoreRewindEndpointRefusesExternalModificationWithoutForce verifies
// the HTTP handler surfaces the store's refusal as a 409 without a force flag,
// and that force:true proceeds anyway.
func TestRestoreRewindEndpointRefusesExternalModificationWithoutForce(t *testing.T) {
	store := newTestSQLiteStore(t)
	ctx := context.Background()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "notes.txt")
	if err := os.WriteFile(path, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	convID := "rewind-refuse-http"
	if err := store.SaveConversation(ctx, convID, []harness.Message{{Role: "user", Content: "keep"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateConversationMeta(ctx, convID, workspace, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRewindPoint(ctx, harness.RewindPoint{
		ID:             "refuse-point",
		ConversationID: convID,
		Files: []harness.RewindFileSnapshot{{
			Path:         "notes.txt",
			Content:      []byte("before"),
			Exists:       true,
			ExpectedHash: harness.RewindContentHash([]byte("agent")),
		}},
	}); err != nil {
		t.Fatal(err)
	}

	runner := harness.NewRunner(&staticProvider{result: harness.CompletionResult{Content: "ok"}}, harness.NewRegistry(), harness.RunnerConfig{ConversationStore: store})

	body, _ := json.Marshal(map[string]any{"point_id": "refuse-point"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/conversations/"+convID+"/rewind", bytes.NewReader(body))
	New(runner).ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", rr.Code, rr.Body.String())
	}

	forceBody, _ := json.Marshal(map[string]any{"point_id": "refuse-point", "force": true})
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/conversations/"+convID+"/rewind", bytes.NewReader(forceBody))
	New(runner).ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("force restore status=%d body=%s, want 200", rr2.Code, rr2.Body.String())
	}
}

// TestRestoreRewindEndpoint_MultiRunKeepsPriorMessagesAndInvalidatesLiveMirror
// reproduces issue #1370's exact repro: two completed runs on one
// conversation (run 1 writes a.txt, run 2 edits it), then a rewind to run
// 2's first tool call. It proves both halves of the fix through the real
// HTTP handler: (1) the DB truncation keeps run 1's messages plus run 2's
// user prompt, deleting run 2's tool-call message and everything after it
// (not just its tool result -- leaving the tool-call message dangling with
// no tool-result response is a shape real providers reject), and (2) GET
// /messages reflects that truncation immediately afterward instead of
// continuing to serve the runner's stale in-memory mirror (which, before
// the fix, resurrects the deleted messages until a daemon restart).
func TestRestoreRewindEndpoint_MultiRunKeepsPriorMessagesAndInvalidatesLiveMirror(t *testing.T) {
	store := newTestSQLiteStore(t)
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("v0"), 0o600); err != nil {
		t.Fatal(err)
	}

	registry := harness.NewRegistry()
	writeFile := func(_ context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(workspace, args.Path), []byte(args.Content), 0o600); err != nil {
			return "", err
		}
		return "ok", nil
	}
	for _, name := range []string{"write", "edit"} {
		if err := registry.Register(harness.ToolDefinition{
			Name:       name,
			Mutating:   true,
			Parameters: map[string]any{"type": "object"},
		}, writeFile); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	prov := fakeprovider.New([]fakeprovider.Turn{
		{ToolCalls: []harness.ToolCall{{ID: "c1", Name: "write", Arguments: `{"path":"a.txt","content":"v1"}`}}},
		{Content: "run1 done"},
		{ToolCalls: []harness.ToolCall{{ID: "c2", Name: "edit", Arguments: `{"path":"a.txt","content":"v2"}`}}},
		{Content: "run2 done"},
	})

	runner := harness.NewRunner(prov, registry, harness.RunnerConfig{
		DefaultModel:         "test",
		MaxSteps:             20,
		ConversationStore:    store,
		WorkspaceBaseOptions: harness.WorkspaceProvisionOptions{RepoPath: workspace},
	})
	handler := New(runner)
	convID := "rewind-mirror-conv"

	run1, err := runner.StartRun(harness.RunRequest{Prompt: "write a.txt", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run1: %v", err)
	}
	pollUntilRunTerminal(t, runner, run1.ID)

	run2, err := runner.StartRun(harness.RunRequest{Prompt: "edit a.txt", ConversationID: convID})
	if err != nil {
		t.Fatalf("StartRun run2: %v", err)
	}
	pollUntilRunTerminal(t, runner, run2.ID)

	getMessages := func() []harness.Message {
		t.Helper()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/conversations/"+convID+"/messages", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("GET /messages status=%d body=%s", rr.Code, rr.Body.String())
		}
		var decoded struct {
			Messages []harness.Message `json:"messages"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode /messages: %v body=%s", err, rr.Body.String())
		}
		return decoded.Messages
	}

	before := getMessages()
	if len(before) != 8 {
		t.Fatalf("before rewind: got %d messages, want 8 (both runs' full history): %#v", len(before), before)
	}

	rrPoints := httptest.NewRecorder()
	handler.ServeHTTP(rrPoints, httptest.NewRequest(http.MethodGet, "/v1/conversations/"+convID+"/rewind-points", nil))
	if rrPoints.Code != http.StatusOK {
		t.Fatalf("GET rewind-points status=%d body=%s", rrPoints.Code, rrPoints.Body.String())
	}
	var listed struct {
		Points []harness.RewindPoint `json:"points"`
	}
	if err := json.Unmarshal(rrPoints.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode rewind-points: %v", err)
	}
	var editPointID string
	for _, p := range listed.Points {
		if p.Tool == "edit" {
			editPointID = p.ID
		}
	}
	if editPointID == "" {
		t.Fatalf("no rewind point captured for the edit tool call: %#v", listed.Points)
	}

	body, _ := json.Marshal(map[string]any{"point_id": editPointID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/conversations/"+convID+"/rewind", bytes.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("POST rewind status=%d body=%s", rr.Code, rr.Body.String())
	}
	var result harness.RewindRestoreResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode rewind result: %v", err)
	}
	if result.MessagesTruncated != 3 {
		t.Errorf("MessagesTruncated = %d, want 3 (run2's tool-call message, tool result, and final answer)", result.MessagesTruncated)
	}

	after := getMessages()
	if len(after) != 5 {
		t.Fatalf("after rewind: got %d messages, want 5 (run1's 4 plus run2's user prompt): %#v", len(after), after)
	}
	for _, m := range after {
		if strings.Contains(m.Content, "run2 done") {
			t.Fatalf("GET /messages still serves run2's truncated final answer after rewind (stale in-memory mirror): %#v", after)
		}
	}
	if after[3].Content != "run1 done" {
		t.Fatalf("run1's final answer was truncated; after[3]=%+v", after[3])
	}
	last := after[len(after)-1]
	if last.Role == "assistant" && len(last.ToolCalls) > 0 {
		t.Fatalf("restore left a dangling assistant message with tool_calls as the last persisted message: %+v", last)
	}
}
