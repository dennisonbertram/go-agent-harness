package telegram_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-agent-harness/apps/socialagent/telegram"
)

// --- ParseUpdate tests ---

func TestParseUpdate_ValidMessage(t *testing.T) {
	body := `{
		"update_id": 42,
		"message": {
			"message_id": 7,
			"from": {
				"id": 123,
				"first_name": "Alice",
				"last_name": "Smith",
				"username": "alice"
			},
			"chat": {"id": 456},
			"text": "Hello bot",
			"date": 1700000000
		}
	}`
	r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	bot := telegram.NewBot("test-token")
	update, err := bot.ParseUpdate(r)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if update.UpdateID != 42 {
		t.Errorf("expected UpdateID=42, got %d", update.UpdateID)
	}
	if update.Message == nil {
		t.Fatal("expected Message to be non-nil")
	}
	if update.Message.MessageID != 7 {
		t.Errorf("expected MessageID=7, got %d", update.Message.MessageID)
	}
	if update.Message.Text != "Hello bot" {
		t.Errorf("expected Text='Hello bot', got %q", update.Message.Text)
	}
	if update.Message.Chat.ID != 456 {
		t.Errorf("expected ChatID=456, got %d", update.Message.Chat.ID)
	}
	if update.Message.From == nil {
		t.Fatal("expected From to be non-nil")
	}
	if update.Message.From.FirstName != "Alice" {
		t.Errorf("expected FirstName='Alice', got %q", update.Message.From.FirstName)
	}
}

func TestParseUpdate_MissingMessage(t *testing.T) {
	body := `{"update_id": 99}`
	r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	bot := telegram.NewBot("test-token")
	_, err := bot.ParseUpdate(r)
	if err == nil {
		t.Fatal("expected error for missing message, got nil")
	}
}

func TestParseUpdate_MessageWithoutText(t *testing.T) {
	body := `{
		"update_id": 10,
		"message": {
			"message_id": 3,
			"chat": {"id": 100},
			"text": "",
			"date": 1700000001
		}
	}`
	r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	bot := telegram.NewBot("test-token")
	_, err := bot.ParseUpdate(r)
	if err == nil {
		t.Fatal("expected error for message without text, got nil")
	}
}

func TestParseUpdate_InvalidJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("{bad json"))
	r.Header.Set("Content-Type", "application/json")

	bot := telegram.NewBot("test-token")
	_, err := bot.ParseUpdate(r)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseUpdate_OversizedBody(t *testing.T) {
	// Build a body larger than 1MB (the limit). The LimitReader truncates it,
	// so the JSON is incomplete and json.Unmarshal must return an error.
	const limit = 1 << 20 // 1MB
	// Create a valid JSON prefix, then pad to exceed the limit.
	prefix := []byte(`{"update_id":1,"message":{"message_id":1,"chat":{"id":1},"text":"x","date":0}`)
	padding := bytes.Repeat([]byte("x"), limit+512) // > 1MB total filler
	oversized := append(prefix, padding...)

	r := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(oversized))
	r.Header.Set("Content-Type", "application/json")

	bot := telegram.NewBot("test-token")
	_, err := bot.ParseUpdate(r)
	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
}

// --- SendMessage tests ---

func TestSendMessage_Success(t *testing.T) {
	var capturedPath string
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	bot := telegram.NewBotWithBaseURL("test-token", srv.URL)
	err := bot.SendMessage(context.Background(), 456, "Hello!")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedPath := "/bottest-token/sendMessage"
	if capturedPath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, capturedPath)
	}
	if chatID, ok := capturedBody["chat_id"].(float64); !ok || int64(chatID) != 456 {
		t.Errorf("expected chat_id=456 in body, got %v", capturedBody["chat_id"])
	}
	if text, ok := capturedBody["text"].(string); !ok || text != "Hello!" {
		t.Errorf("expected text='Hello!' in body, got %v", capturedBody["text"])
	}
	if _, exists := capturedBody["parse_mode"]; exists {
		t.Errorf("expected parse_mode to be absent from request body, but got %v", capturedBody["parse_mode"])
	}
}

func TestSendMessage_TelegramAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer srv.Close()

	bot := telegram.NewBotWithBaseURL("test-token", srv.URL)
	err := bot.SendMessage(context.Background(), 999, "Test")
	if err == nil {
		t.Fatal("expected error from Telegram API error response, got nil")
	}
	if !strings.Contains(err.Error(), "Bad Request") {
		t.Errorf("expected error to contain 'Bad Request', got: %v", err)
	}
}

func TestSendMessage_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	bot := telegram.NewBotWithBaseURL("test-token", srv.URL)
	err := bot.SendMessage(context.Background(), 456, "Test")
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

// --- SetWebhook tests ---

func TestSetWebhook_Success(t *testing.T) {
	var capturedPath string
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer srv.Close()

	bot := telegram.NewBotWithBaseURL("test-token", srv.URL)
	err := bot.SetWebhook(context.Background(), "https://example.com/webhook")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedPath := "/bottest-token/setWebhook"
	if capturedPath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, capturedPath)
	}
	if url, ok := capturedBody["url"].(string); !ok || url != "https://example.com/webhook" {
		t.Errorf("expected url='https://example.com/webhook' in body, got %v", capturedBody["url"])
	}
}

func TestSetWebhook_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer srv.Close()

	bot := telegram.NewBotWithBaseURL("test-token", srv.URL)
	err := bot.SetWebhook(context.Background(), "https://example.com/webhook")
	if err == nil {
		t.Fatal("expected error from Telegram API error response, got nil")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected error to contain 'Unauthorized', got: %v", err)
	}
}

// --- DisplayName tests ---

func TestDisplayName_Nil(t *testing.T) {
	bot := telegram.NewBot("test-token")
	name := bot.DisplayName(nil)
	if name != "Unknown" {
		t.Errorf("expected 'Unknown' for nil user, got %q", name)
	}
}

func TestDisplayName_FirstNameOnly(t *testing.T) {
	bot := telegram.NewBot("test-token")
	u := &telegram.User{ID: 1, FirstName: "Alice"}
	name := bot.DisplayName(u)
	if name != "Alice" {
		t.Errorf("expected 'Alice', got %q", name)
	}
}

func TestDisplayName_FirstAndLastName(t *testing.T) {
	bot := telegram.NewBot("test-token")
	u := &telegram.User{ID: 1, FirstName: "Alice", LastName: "Smith"}
	name := bot.DisplayName(u)
	if name != "Alice Smith" {
		t.Errorf("expected 'Alice Smith', got %q", name)
	}
}

func TestDisplayName_EmptyFirstName(t *testing.T) {
	bot := telegram.NewBot("test-token")
	u := &telegram.User{ID: 1, FirstName: "", LastName: ""}
	name := bot.DisplayName(u)
	// An empty user with no name — trimmed empty string, return Unknown
	if name != "Unknown" {
		t.Errorf("expected 'Unknown' for empty name user, got %q", name)
	}
}
