package linear

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLinearAdapter_IssueCreate(t *testing.T) {
	body := issueCreateJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/linear", strings.NewReader(body))
	req.Header.Set("X-Linear-Signature", "abcdef1234567890abcdef1234567890")

	adapter := NewLinearAdapter()
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Source != "linear" {
		t.Errorf("Source = %q; want %q", env.Source, "linear")
	}
	if env.SourceID != "ENG-123" {
		t.Errorf("SourceID = %q; want %q", env.SourceID, "ENG-123")
	}
	if env.ThreadID != "ENG-123" {
		t.Errorf("ThreadID = %q; want %q", env.ThreadID, "ENG-123")
	}
	if env.Action != "start" {
		t.Errorf("Action = %q; want %q", env.Action, "start")
	}
	if env.Signature != "abcdef1234567890abcdef1234567890" {
		t.Errorf("Signature = %q; want %q", env.Signature, "abcdef1234567890abcdef1234567890")
	}
	if env.RawBody == nil {
		t.Error("RawBody should not be nil")
	}
	// Message should contain the issue identifier and title.
	if !strings.Contains(env.Message, "ENG-123") {
		t.Errorf("Message missing identifier, got: %q", env.Message)
	}
	if !strings.Contains(env.Message, "Fix the login bug") {
		t.Errorf("Message missing title, got: %q", env.Message)
	}
}

func TestLinearAdapter_CommentCreate(t *testing.T) {
	body := commentCreateJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/linear", strings.NewReader(body))
	req.Header.Set("X-Linear-Signature", "deadbeef1234567890deadbeef123456")

	adapter := NewLinearAdapter()
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Action != "steer" {
		t.Errorf("Action = %q; want %q", env.Action, "steer")
	}
	if env.ThreadID != "ENG-123" {
		t.Errorf("ThreadID = %q; want %q", env.ThreadID, "ENG-123")
	}
}

func TestLinearAdapter_NoSignatureHeader(t *testing.T) {
	body := issueCreateJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/linear", strings.NewReader(body))
	// X-Linear-Signature intentionally omitted — adapter sets empty sig, validator rejects later.

	adapter := NewLinearAdapter()
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("adapter should not error on missing sig header (validation is done by validator): %v", err)
	}
	if env.Signature != "" {
		t.Errorf("Signature should be empty when header is missing, got %q", env.Signature)
	}
}

func TestLinearAdapter_UnsupportedEventType(t *testing.T) {
	body := unsupportedLinearTypeJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/linear", strings.NewReader(body))
	req.Header.Set("X-Linear-Signature", "abc123")

	adapter := NewLinearAdapter()
	_, err := adapter.ParseWebhookRequest(req)
	if err == nil {
		t.Fatal("expected error for unsupported event type, got nil")
	}
}

func TestLinearAdapter_RawBodyPreserved(t *testing.T) {
	body := issueCreateJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/linear", strings.NewReader(body))
	req.Header.Set("X-Linear-Signature", "abc123")

	adapter := NewLinearAdapter()
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(env.RawBody) != body {
		t.Errorf("RawBody not preserved, got %q", string(env.RawBody))
	}
}
