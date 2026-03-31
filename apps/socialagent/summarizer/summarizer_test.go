package summarizer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/apps/socialagent/harness"
)

// --- Mock implementations ---

type mockHarnessClient struct {
	output string
	err    error
	// capture the last request for assertions
	lastReq harness.RunRequest
}

func (m *mockHarnessClient) SendAndWait(_ context.Context, req harness.RunRequest) (*harness.RunResult, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &harness.RunResult{Output: m.output}, nil
}

type mockProfileStore struct {
	called     bool
	userID     string
	summary    string
	interests  []string
	lookingFor string
	err        error
}

func (m *mockProfileStore) UpsertProfile(_ context.Context, userID, summary string, interests []string, lookingFor string) error {
	m.called = true
	m.userID = userID
	m.summary = summary
	m.interests = interests
	m.lookingFor = lookingFor
	return m.err
}

// --- Helpers ---

// newTestServer returns an httptest.Server that serves a fixed list of messages.
func newTestServer(t *testing.T, messages []Message) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/v1/conversations/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		result := struct {
			Messages []Message `json:"messages"`
		}{Messages: messages}
		_ = json.NewEncoder(w).Encode(result)
	}))
}

// --- Tests ---

func TestUpdateProfile_Success(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hi, I love hiking and rock climbing."},
		{Role: "assistant", Content: "That sounds fun! Are you looking to meet other hikers?"},
		{Role: "user", Content: "Yes, exactly. I want to find hiking partners."},
	}
	srv := newTestServer(t, messages)
	defer srv.Close()

	profileJSON := `{"summary":"A hiking enthusiast who wants to meet like-minded people.","interests":["hiking","rock climbing"],"looking_for":"hiking partners"}`

	h := &mockHarnessClient{output: profileJSON}
	store := &mockProfileStore{}

	s := New(h, store, srv.URL)
	err := s.UpdateProfile(context.Background(), "user-123", "conv-456", "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !store.called {
		t.Fatal("expected UpsertProfile to be called")
	}
	if store.userID != "user-123" {
		t.Errorf("got userID %q, want %q", store.userID, "user-123")
	}
	if store.summary != "A hiking enthusiast who wants to meet like-minded people." {
		t.Errorf("unexpected summary: %q", store.summary)
	}
	if len(store.interests) != 2 || store.interests[0] != "hiking" || store.interests[1] != "rock climbing" {
		t.Errorf("unexpected interests: %v", store.interests)
	}
	if store.lookingFor != "hiking partners" {
		t.Errorf("unexpected looking_for: %q", store.lookingFor)
	}
	// Verify dedicated conversation ID used.
	if h.lastReq.ConversationID != "summary-user-123" {
		t.Errorf("expected conversation ID %q, got %q", "summary-user-123", h.lastReq.ConversationID)
	}
}

func TestUpdateProfile_EmptyConversation(t *testing.T) {
	srv := newTestServer(t, []Message{})
	defer srv.Close()

	h := &mockHarnessClient{}
	store := &mockProfileStore{}

	s := New(h, store, srv.URL)
	err := s.UpdateProfile(context.Background(), "user-123", "conv-empty", "Bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.called {
		t.Fatal("expected UpsertProfile NOT to be called for empty conversation")
	}
}

func TestUpdateProfile_HarnessError(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	srv := newTestServer(t, messages)
	defer srv.Close()

	h := &mockHarnessClient{err: fmt.Errorf("LLM unavailable")}
	store := &mockProfileStore{}

	s := New(h, store, srv.URL)
	err := s.UpdateProfile(context.Background(), "user-123", "conv-456", "Carol")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "harness summarize") {
		t.Errorf("expected error to mention 'harness summarize', got: %v", err)
	}
	if store.called {
		t.Fatal("UpsertProfile should not be called when harness fails")
	}
}

func TestUpdateProfile_InvalidJSON(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	srv := newTestServer(t, messages)
	defer srv.Close()

	rawOutput := "I cannot produce a JSON summary right now."
	h := &mockHarnessClient{output: rawOutput}
	store := &mockProfileStore{}

	s := New(h, store, srv.URL)
	err := s.UpdateProfile(context.Background(), "user-123", "conv-456", "Dave")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !store.called {
		t.Fatal("expected UpsertProfile to be called")
	}
	if store.summary != rawOutput {
		t.Errorf("expected raw output as summary, got: %q", store.summary)
	}
	// Interests and looking_for should be zero values.
	if len(store.interests) != 0 {
		t.Errorf("expected empty interests, got: %v", store.interests)
	}
	if store.lookingFor != "" {
		t.Errorf("expected empty looking_for, got: %q", store.lookingFor)
	}
}

func TestBuildSummarizationPrompt(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "I enjoy photography."},
		{Role: "assistant", Content: "Great hobby!"},
	}
	prompt := buildSummarizationPrompt("Eve", messages)

	if !strings.Contains(prompt, "Eve") {
		t.Error("expected prompt to contain display name 'Eve'")
	}
	if !strings.Contains(prompt, "I enjoy photography.") {
		t.Error("expected prompt to contain user message content")
	}
	if !strings.Contains(prompt, "Great hobby!") {
		t.Error("expected prompt to contain assistant message content")
	}
	if !strings.Contains(prompt, "User:") {
		t.Error("expected prompt to label user messages with 'User:'")
	}
	if !strings.Contains(prompt, "Assistant:") {
		t.Error("expected prompt to label assistant messages with 'Assistant:'")
	}
}

func TestBuildSummarizationPrompt_TruncatesLong(t *testing.T) {
	// Create 60 messages — only the last 50 should appear.
	messages := make([]Message, 60)
	for i := range messages {
		messages[i] = Message{
			Role:    "user",
			Content: fmt.Sprintf("message number %d", i),
		}
	}

	prompt := buildSummarizationPrompt("Frank", messages)

	// The first 10 messages (indices 0-9) should NOT appear (they are dropped).
	// Use a word-boundary-style check: append a space or end-of-token to avoid
	// "message number 1" matching "message number 10", etc.
	// All dropped indices are 0-9; these never appear as suffixes of 10+.
	// To be safe, match the full line "User: message number N\n".
	for i := 0; i < 10; i++ {
		needle := fmt.Sprintf("User: message number %d\n", i)
		if strings.Contains(prompt, needle) {
			t.Errorf("prompt should not contain dropped message %d", i)
		}
	}
	// The last 50 messages (indices 10–59) should appear.
	for i := 10; i < 60; i++ {
		needle := fmt.Sprintf("message number %d", i)
		if !strings.Contains(prompt, needle) {
			t.Errorf("prompt should contain message %d", i)
		}
	}
}

func TestFetchMessages_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := New(nil, nil, srv.URL)
	_, err := s.fetchMessages(context.Background(), "conv-123")
	if err == nil {
		t.Fatal("expected error from non-200 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestFetchMessages_Success(t *testing.T) {
	want := []Message{
		{Role: "user", Content: "Hello there"},
		{Role: "assistant", Content: "Hi!"},
	}
	srv := newTestServer(t, want)
	defer srv.Close()

	s := New(nil, nil, srv.URL)
	got, err := s.fetchMessages(context.Background(), "conv-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d messages, want %d", len(got), len(want))
	}
	for i, m := range want {
		if got[i].Role != m.Role || got[i].Content != m.Content {
			t.Errorf("message %d mismatch: got %+v, want %+v", i, got[i], m)
		}
	}
}
