package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-agent-harness/apps/socialagent/db"
	"go-agent-harness/apps/socialagent/gateway"
	"go-agent-harness/apps/socialagent/harness"
	"go-agent-harness/apps/socialagent/telegram"
)

// --- fake implementations ---

type fakeStore struct {
	mu    sync.Mutex
	users map[int64]*db.User
	calls []int64 // telegramIDs passed to GetOrCreateUser
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: make(map[int64]*db.User)}
}

func (f *fakeStore) GetOrCreateUser(ctx context.Context, telegramID int64, displayName string) (*db.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, telegramID)
	u, ok := f.users[telegramID]
	if !ok {
		u = &db.User{
			ID:             fmt.Sprintf("uuid-%d", telegramID),
			TelegramID:     telegramID,
			ConversationID: fmt.Sprintf("conv-%d", telegramID),
			DisplayName:    displayName,
		}
		f.users[telegramID] = u
	}
	return u, nil
}

// fakeHarness records calls and optionally delays or returns errors.
type fakeHarness struct {
	mu       sync.Mutex
	requests []harness.RunRequest
	result   *harness.RunResult
	err      error
	delay    time.Duration
}

func (f *fakeHarness) SendAndWait(ctx context.Context, req harness.RunRequest) (*harness.RunResult, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &harness.RunResult{Output: "default output", RunID: "run-1"}, nil
}

// fakeBot captures sent messages and can be configured to fail ParseUpdate.
type fakeBot struct {
	mu       sync.Mutex
	messages []sentMessage
	parseErr error
}

type sentMessage struct {
	chatID int64
	text   string
}

func (f *fakeBot) ParseUpdate(r *http.Request) (*telegram.Update, error) {
	if f.parseErr != nil {
		return nil, f.parseErr
	}
	var update telegram.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		return nil, err
	}
	if update.Message == nil || update.Message.Text == "" {
		return nil, errors.New("no text message")
	}
	return &update, nil
}

func (f *fakeBot) SendMessage(ctx context.Context, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, sentMessage{chatID: chatID, text: text})
	return nil
}

func (f *fakeBot) DisplayName(u *telegram.User) string {
	if u == nil {
		return "Unknown"
	}
	return u.FirstName
}

// recordingHarness tracks concurrent active calls to verify serialization.
type recordingHarness struct {
	delay       time.Duration
	activeCalls *int32
	maxActive   *int32
}

func (r *recordingHarness) SendAndWait(ctx context.Context, req harness.RunRequest) (*harness.RunResult, error) {
	current := atomic.AddInt32(r.activeCalls, 1)
	defer atomic.AddInt32(r.activeCalls, -1)

	// Update max if needed (CAS loop).
	for {
		max := atomic.LoadInt32(r.maxActive)
		if current <= max {
			break
		}
		if atomic.CompareAndSwapInt32(r.maxActive, max, current) {
			break
		}
	}

	if r.delay > 0 {
		time.Sleep(r.delay)
	}

	return &harness.RunResult{Output: "ok", RunID: "run-x"}, nil
}

// --- helpers ---

func makeWebhookRequest(t *testing.T, update telegram.Update) *http.Request {
	t.Helper()
	body, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal update: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func makeUpdate(userID, chatID int64, text string) telegram.Update {
	return telegram.Update{
		UpdateID: 1,
		Message: &telegram.Message{
			MessageID: 1,
			From:      &telegram.User{ID: userID, FirstName: "Alice"},
			Chat:      telegram.Chat{ID: chatID},
			Text:      text,
		},
	}
}

// --- tests ---

// TestHappyPath verifies: valid webhook → user created → harness called with
// correct conversation_id and system_prompt → response sent back to correct chat_id.
func TestHappyPath(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{result: &harness.RunResult{Output: "hello from agent", RunID: "run-42"}}
	bot := &fakeBot{}
	systemPrompt := "You are a helpful assistant."

	gw := gateway.NewGateway(bot, store, h, systemPrompt)

	update := makeUpdate(123, 456, "What is 2+2?")
	req := makeWebhookRequest(t, update)
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Verify store was called with correct telegramID.
	store.mu.Lock()
	if len(store.calls) != 1 || store.calls[0] != 123 {
		t.Errorf("expected store call with telegramID=123, got %v", store.calls)
	}
	store.mu.Unlock()

	// Verify harness was called with correct fields.
	h.mu.Lock()
	if len(h.requests) != 1 {
		t.Fatalf("expected 1 harness request, got %d", len(h.requests))
	}
	r := h.requests[0]
	h.mu.Unlock()

	if r.Prompt != "What is 2+2?" {
		t.Errorf("expected prompt 'What is 2+2?', got %q", r.Prompt)
	}
	if r.ConversationID != "conv-123" {
		t.Errorf("expected conversation_id 'conv-123', got %q", r.ConversationID)
	}
	if r.SystemPrompt != systemPrompt {
		t.Errorf("expected system_prompt %q, got %q", systemPrompt, r.SystemPrompt)
	}
	if r.TenantID != "uuid-123" {
		t.Errorf("expected tenant_id 'uuid-123', got %q", r.TenantID)
	}

	// Verify the response was sent to the correct chat.
	bot.mu.Lock()
	if len(bot.messages) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(bot.messages))
	}
	msg := bot.messages[0]
	bot.mu.Unlock()

	if msg.chatID != 456 {
		t.Errorf("expected chatID=456, got %d", msg.chatID)
	}
	if msg.text != "hello from agent" {
		t.Errorf("expected text 'hello from agent', got %q", msg.text)
	}
}

// TestHarnessError verifies: when harness returns an error, an error message is
// sent to the user and the handler still returns 200.
func TestHarnessError(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{err: errors.New("harness unavailable")}
	bot := &fakeBot{}

	gw := gateway.NewGateway(bot, store, h, "prompt")

	update := makeUpdate(111, 222, "help me")
	req := makeWebhookRequest(t, update)
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	bot.mu.Lock()
	defer bot.mu.Unlock()
	if len(bot.messages) != 1 {
		t.Fatalf("expected 1 error message sent, got %d", len(bot.messages))
	}
	if bot.messages[0].chatID != 222 {
		t.Errorf("expected chatID=222, got %d", bot.messages[0].chatID)
	}
	if bot.messages[0].text != "Sorry, something went wrong. Please try again." {
		t.Errorf("unexpected error text: %q", bot.messages[0].text)
	}
}

// TestInvalidWebhook verifies: when ParseUpdate returns an error (no text),
// the handler returns 200 and does NOT call harness.
func TestInvalidWebhook(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{}
	bot := &fakeBot{parseErr: errors.New("no text")}

	gw := gateway.NewGateway(bot, store, h, "prompt")

	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 0 {
		t.Errorf("expected no harness calls, got %d", len(h.requests))
	}
}

// TestPerUserMutex verifies: two concurrent requests for the same user are
// serialized — max concurrent harness calls for that user is 1.
func TestPerUserMutex(t *testing.T) {
	store := newFakeStore()
	bot := &fakeBot{}

	var activeCalls int32
	var maxConcurrent int32

	h := &recordingHarness{
		delay:       50 * time.Millisecond,
		activeCalls: &activeCalls,
		maxActive:   &maxConcurrent,
	}

	gw := gateway.NewGateway(bot, store, h, "prompt")

	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			update := makeUpdate(999, 999, "concurrent message")
			req := makeWebhookRequest(t, update)
			rec := httptest.NewRecorder()
			gw.HandleWebhook(rec, req)
		}()
	}

	wg.Wait()

	if maxConcurrent > 1 {
		t.Errorf("per-user mutex violated: max concurrent calls for same user = %d (expected ≤1)", maxConcurrent)
	}
}

// TestDifferentUsersConcurrent verifies: two concurrent requests for different
// users both proceed simultaneously — they are NOT blocked by each other.
func TestDifferentUsersConcurrent(t *testing.T) {
	store := newFakeStore()
	bot := &fakeBot{}

	var activeCalls int32
	var maxConcurrent int32

	h := &recordingHarness{
		delay:       50 * time.Millisecond,
		activeCalls: &activeCalls,
		maxActive:   &maxConcurrent,
	}

	gw := gateway.NewGateway(bot, store, h, "prompt")

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		update := makeUpdate(1001, 1001, "message from user 1")
		req := makeWebhookRequest(t, update)
		rec := httptest.NewRecorder()
		gw.HandleWebhook(rec, req)
	}()

	go func() {
		defer wg.Done()
		update := makeUpdate(1002, 1002, "message from user 2")
		req := makeWebhookRequest(t, update)
		rec := httptest.NewRecorder()
		gw.HandleWebhook(rec, req)
	}()

	wg.Wait()
	elapsed := time.Since(start)

	// If different users were serialized, it would take ~100ms.
	// If concurrent (correct), it should take ~50ms. Allow 90ms slack.
	if elapsed > 90*time.Millisecond {
		t.Errorf("different users appear to be serialized: elapsed=%v (expected ~50ms)", elapsed)
	}

	if maxConcurrent < 2 {
		t.Errorf("expected both users to run concurrently, maxConcurrent=%d", maxConcurrent)
	}
}
