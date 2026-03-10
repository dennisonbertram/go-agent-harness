package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"go-agent-harness/internal/harness"
)

// newTestSQLiteStore creates a fresh SQLite-backed store for HTTP tests.
func newTestSQLiteStore(t *testing.T) *harness.SQLiteConversationStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "http-test.db")
	store, err := harness.NewSQLiteConversationStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteConversationStore: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestCompactConversationEndpoint_Basic(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	ctx := context.Background()

	// Pre-populate a conversation with 6 messages.
	msgs := []harness.Message{
		{Role: "user", Content: "msg-0"},
		{Role: "assistant", Content: "msg-1"},
		{Role: "user", Content: "msg-2"},
		{Role: "assistant", Content: "msg-3"},
		{Role: "user", Content: "msg-4"},
		{Role: "assistant", Content: "msg-5"},
	}
	if err := store.SaveConversation(ctx, "conv-http-compact", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := bytes.NewBufferString(`{"keep_from_step":4,"summary":"Summary of first 4 messages"}`)
	res, err := http.Post(ts.URL+"/v1/conversations/conv-http-compact/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, b)
	}

	var resp map[string]any
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["compacted"] != true {
		t.Errorf("expected compacted=true, got %v", resp["compacted"])
	}
	msgCount, ok := resp["message_count"].(float64)
	if !ok {
		t.Fatalf("expected message_count in response, got %T %v", resp["message_count"], resp["message_count"])
	}
	// 1 summary + 2 kept (msgs 4 and 5) = 3
	if int(msgCount) != 3 {
		t.Errorf("expected message_count=3, got %d", int(msgCount))
	}

	// Verify the messages via GET
	loaded, err := store.LoadMessages(ctx, "conv-http-compact")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 messages after compact, got %d", len(loaded))
	}
	if !loaded[0].IsCompactSummary {
		t.Error("first message should be compact summary")
	}
	if loaded[0].Content != "Summary of first 4 messages" {
		t.Errorf("unexpected summary content: %q", loaded[0].Content)
	}
}

func TestCompactConversationEndpoint_NoStore(t *testing.T) {
	t.Parallel()

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{}, // no store
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := bytes.NewBufferString(`{"keep_from_step":2,"summary":"summary"}`)
	res, err := http.Post(ts.URL+"/v1/conversations/any-conv/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", res.StatusCode)
	}
}

func TestCompactConversationEndpoint_InvalidJSON(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := bytes.NewBufferString(`not json`)
	res, err := http.Post(ts.URL+"/v1/conversations/any-conv/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", res.StatusCode)
	}
}

func TestCompactConversationEndpoint_EmptySummary(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := bytes.NewBufferString(`{"keep_from_step":2,"summary":""}`)
	res, err := http.Post(ts.URL+"/v1/conversations/any-conv/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty summary, got %d", res.StatusCode)
	}
}

func TestCompactConversationEndpoint_NegativeKeepFrom(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := bytes.NewBufferString(`{"keep_from_step":-1,"summary":"summary"}`)
	res, err := http.Post(ts.URL+"/v1/conversations/any-conv/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for negative keep_from_step, got %d", res.StatusCode)
	}
}

func TestCompactConversationEndpoint_NonExistentConversation(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := bytes.NewBufferString(`{"keep_from_step":0,"summary":"summary"}`)
	res, err := http.Post(ts.URL+"/v1/conversations/does-not-exist/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(res.Body)
		t.Errorf("expected 404 for non-existent conversation, got %d: %s", res.StatusCode, b)
	}
}

func TestCompactConversationEndpoint_WrongMethod(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	// GET should not be allowed on the compact endpoint
	res, err := http.Get(ts.URL + "/v1/conversations/any-conv/compact")
	if err != nil {
		t.Fatalf("GET compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}

func TestCompactConversationEndpoint_SummaryRoleDefault(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	ctx := context.Background()

	msgs := []harness.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}
	if err := store.SaveConversation(ctx, "conv-role-default", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	// No "role" field in request — should default to "system"
	body := bytes.NewBufferString(`{"keep_from_step":2,"summary":"compact context"}`)
	res, err := http.Post(ts.URL+"/v1/conversations/conv-role-default/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, b)
	}

	loaded, err := store.LoadMessages(ctx, "conv-role-default")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) == 0 {
		t.Fatal("expected messages after compact")
	}
	// Summary message should have role "system" by default
	if loaded[0].Role != "system" {
		t.Errorf("expected summary role=system, got %q", loaded[0].Role)
	}
	if !loaded[0].IsCompactSummary {
		t.Error("expected IsCompactSummary=true on first message")
	}
}

func TestCompactConversationEndpoint_CustomRole(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	ctx := context.Background()

	msgs := []harness.Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	if err := store.SaveConversation(ctx, "conv-custom-role", msgs); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	body := bytes.NewBufferString(`{"keep_from_step":2,"summary":"compact ctx","role":"user"}`)
	res, err := http.Post(ts.URL+"/v1/conversations/conv-custom-role/compact", "application/json", body)
	if err != nil {
		t.Fatalf("POST compact: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, b)
	}

	loaded, err := store.LoadMessages(ctx, "conv-custom-role")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) == 0 || loaded[0].Role != "user" {
		t.Errorf("expected role=user, got %q", func() string {
			if len(loaded) > 0 {
				return loaded[0].Role
			}
			return "<empty>"
		}())
	}
}

func TestCompactConversationEndpoint_Concurrent(t *testing.T) {
	t.Parallel()

	store := newTestSQLiteStore(t)
	ctx := context.Background()
	const n = 5

	for i := 0; i < n; i++ {
		msgs := []harness.Message{
			{Role: "user", Content: fmt.Sprintf("q%d", i)},
			{Role: "assistant", Content: fmt.Sprintf("a%d", i)},
			{Role: "user", Content: fmt.Sprintf("q%d-2", i)},
			{Role: "assistant", Content: fmt.Sprintf("a%d-2", i)},
		}
		if err := store.SaveConversation(ctx, fmt.Sprintf("cc-%d", i), msgs); err != nil {
			t.Fatalf("SaveConversation %d: %v", i, err)
		}
	}

	runner := harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "ok"}},
		harness.NewRegistry(),
		harness.RunnerConfig{ConversationStore: store},
	)
	ts := httptest.NewServer(New(runner))
	defer ts.Close()

	errs := make(chan error, n)
	done := make(chan struct{}, n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			body := bytes.NewBufferString(fmt.Sprintf(`{"keep_from_step":2,"summary":"summary-%d"}`, idx))
			res, err := http.Post(ts.URL+fmt.Sprintf("/v1/conversations/cc-%d/compact", idx), "application/json", body)
			if err != nil {
				errs <- fmt.Errorf("POST %d: %w", idx, err)
				return
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(res.Body)
				errs <- fmt.Errorf("cc-%d: expected 200, got %d: %s", idx, res.StatusCode, b)
				return
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < n; i++ {
		select {
		case err := <-errs:
			t.Error(err)
		case <-done:
		}
	}
}
