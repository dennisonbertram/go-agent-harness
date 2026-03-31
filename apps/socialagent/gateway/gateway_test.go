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

const testWebhookSecret = "test-secret"

func makeWebhookRequest(t *testing.T, update telegram.Update) *http.Request {
	t.Helper()
	return makeWebhookRequestWithSecret(t, update, testWebhookSecret)
}

func makeWebhookRequestWithSecret(t *testing.T, update telegram.Update, secret string) *http.Request {
	t.Helper()
	body, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("marshal update: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	}
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

func makeUpdateWithID(updateID int, userID, chatID int64, text string) telegram.Update {
	return telegram.Update{
		UpdateID: updateID,
		Message: &telegram.Message{
			MessageID: updateID,
			From:      &telegram.User{ID: userID, FirstName: "Alice"},
			Chat:      telegram.Chat{ID: chatID},
			Text:      text,
		},
	}
}

// --- tests ---

// TestHappyPath verifies: valid webhook → handler returns 200 immediately →
// background goroutine creates user → calls harness with correct fields →
// sends response back to correct chat_id.
func TestHappyPath(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{result: &harness.RunResult{Output: "hello from agent", RunID: "run-42"}}
	bot := &fakeBot{}
	systemPrompt := "You are a helpful assistant."

	gw := gateway.NewGateway(bot, store, h, systemPrompt, testWebhookSecret)

	update := makeUpdateWithID(100, 123, 456, "What is 2+2?")
	req := makeWebhookRequest(t, update)
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	// Handler must return 200 immediately, before background work completes.
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Wait for background goroutine to finish.
	gw.Wait()

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

// TestHarnessError verifies: when harness returns an error, handler returns 200
// immediately, and the background goroutine sends an error message to the user.
func TestHarnessError(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{err: errors.New("harness unavailable")}
	bot := &fakeBot{}

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	update := makeUpdateWithID(200, 111, 222, "help me")
	req := makeWebhookRequest(t, update)
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	// Handler returns 200 immediately.
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Wait for background goroutine to finish.
	gw.Wait()

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
// the handler returns 200 and does NOT dispatch a background goroutine or call harness.
func TestInvalidWebhook(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{}
	bot := &fakeBot{parseErr: errors.New("no text")}

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	req := httptest.NewRequest(http.MethodPost, "/webhook/telegram", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	// No background goroutine was dispatched, so Wait returns immediately.
	gw.Wait()

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

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	// Use distinct update IDs to avoid deduplication.
	for i := 0; i < 2; i++ {
		update := makeUpdateWithID(300+i, 999, 999, "concurrent message")
		req := makeWebhookRequest(t, update)
		rec := httptest.NewRecorder()
		gw.HandleWebhook(rec, req)
	}

	// Wait for all background goroutines to complete.
	gw.Wait()

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

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	// Use distinct update IDs for each user.
	update1 := makeUpdateWithID(400, 1001, 1001, "message from user 1")
	update2 := makeUpdateWithID(401, 1002, 1002, "message from user 2")

	req1 := makeWebhookRequest(t, update1)
	req2 := makeWebhookRequest(t, update2)
	rec1 := httptest.NewRecorder()
	rec2 := httptest.NewRecorder()

	// Both handlers return immediately; background goroutines run concurrently.
	gw.HandleWebhook(rec1, req1)
	gw.HandleWebhook(rec2, req2)

	// Wait for both background goroutines to finish.
	gw.Wait()

	if maxConcurrent < 2 {
		t.Errorf("expected both users to run concurrently, maxConcurrent=%d", maxConcurrent)
	}
}

// TestDuplicateUpdateID verifies: sending the same update_id twice results in
// harness being called only once (Telegram retry deduplication).
func TestDuplicateUpdateID(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{result: &harness.RunResult{Output: "hello", RunID: "run-1"}}
	bot := &fakeBot{}

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	// Send the same update twice with the same update_id.
	for i := 0; i < 2; i++ {
		update := makeUpdateWithID(500, 777, 777, "duplicate message")
		req := makeWebhookRequest(t, update)
		rec := httptest.NewRecorder()
		gw.HandleWebhook(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// Wait for any background goroutines to complete.
	gw.Wait()

	// Harness should have been called exactly once.
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 1 {
		t.Errorf("expected harness called once for duplicate update_id, got %d calls", len(h.requests))
	}
}

// TestWebhookAuth_ValidSecret verifies: a request with the correct secret
// proceeds normally — harness is called and a response is sent.
func TestWebhookAuth_ValidSecret(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{result: &harness.RunResult{Output: "auth ok", RunID: "run-auth-1"}}
	bot := &fakeBot{}

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	update := makeUpdateWithID(600, 111, 111, "authenticated message")
	req := makeWebhookRequestWithSecret(t, update, testWebhookSecret)
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	gw.Wait()

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 1 {
		t.Errorf("expected harness called once for valid secret, got %d calls", len(h.requests))
	}
}

// TestWebhookAuth_InvalidSecret verifies: a request with a wrong secret
// returns 200 but does NOT call the harness (spoofed request is silently dropped).
func TestWebhookAuth_InvalidSecret(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{}
	bot := &fakeBot{}

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	update := makeUpdateWithID(700, 222, 222, "spoofed message")
	req := makeWebhookRequestWithSecret(t, update, "wrong-secret")
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	gw.Wait()

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 0 {
		t.Errorf("expected no harness calls for invalid secret, got %d", len(h.requests))
	}
}

// TestWebhookAuth_MissingSecret verifies: a request with no
// X-Telegram-Bot-Api-Secret-Token header returns 200 but does NOT call the harness.
func TestWebhookAuth_MissingSecret(t *testing.T) {
	store := newFakeStore()
	h := &fakeHarness{}
	bot := &fakeBot{}

	gw := gateway.NewGateway(bot, store, h, "prompt", testWebhookSecret)

	update := makeUpdateWithID(800, 333, 333, "no secret message")
	// Use makeWebhookRequestWithSecret with empty string so no header is set.
	req := makeWebhookRequestWithSecret(t, update, "")
	rec := httptest.NewRecorder()

	gw.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	gw.Wait()

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.requests) != 0 {
		t.Errorf("expected no harness calls for missing secret, got %d", len(h.requests))
	}
}
