package slack

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlackAdapter_ValidRequest(t *testing.T) {
	body := appMentionEventJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/slack", strings.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=abc123def456")

	adapter := NewSlackAdapter()
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Source != "slack" {
		t.Errorf("Source = %q; want %q", env.Source, "slack")
	}
	if env.SourceID != "Ev123ABC" {
		t.Errorf("SourceID = %q; want %q", env.SourceID, "Ev123ABC")
	}
	if env.Action != "steer" {
		t.Errorf("Action = %q; want %q", env.Action, "steer")
	}
	// ThreadID should be channelID:threadTS for threaded messages.
	wantThreadID := "C01234567:1234567890.000000"
	if env.ThreadID != wantThreadID {
		t.Errorf("ThreadID = %q; want %q", env.ThreadID, wantThreadID)
	}
	// Signature should be packed as "timestamp:sig".
	wantSig := "1234567890:v0=abc123def456"
	if env.Signature != wantSig {
		t.Errorf("Signature = %q; want %q", env.Signature, wantSig)
	}
	if env.RawBody == nil {
		t.Error("RawBody should not be nil")
	}
	// Message should contain the text.
	if !strings.Contains(env.Message, "<@UBOT123> do something useful") {
		t.Errorf("Message missing text, got: %q", env.Message)
	}
}

func TestSlackAdapter_TopLevelMessage_ThreadIDUsesMessageTS(t *testing.T) {
	body := topLevelMessageJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/slack", strings.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", "9876543210")
	req.Header.Set("X-Slack-Signature", "v0=def456abc123")

	adapter := NewSlackAdapter()
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// For top-level messages, ThreadID should fall back to channelID:messageTS.
	wantThreadID := "C09876543:9876543210.654321"
	if env.ThreadID != wantThreadID {
		t.Errorf("ThreadID = %q; want %q", env.ThreadID, wantThreadID)
	}
}

func TestSlackAdapter_MissingTimestampHeader(t *testing.T) {
	body := appMentionEventJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/slack", strings.NewReader(body))
	// X-Slack-Request-Timestamp intentionally omitted.
	req.Header.Set("X-Slack-Signature", "v0=abc123def456")

	adapter := NewSlackAdapter()
	_, err := adapter.ParseWebhookRequest(req)
	if err == nil {
		t.Fatal("expected error for missing X-Slack-Request-Timestamp, got nil")
	}
}

func TestSlackAdapter_MissingSignatureHeader(t *testing.T) {
	body := appMentionEventJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/slack", strings.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	// X-Slack-Signature intentionally omitted.

	adapter := NewSlackAdapter()
	_, err := adapter.ParseWebhookRequest(req)
	if err == nil {
		t.Fatal("expected error for missing X-Slack-Signature, got nil")
	}
}

func TestSlackAdapter_UnsupportedEventType(t *testing.T) {
	body := unsupportedTypeJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/slack", strings.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=abc123def456")

	adapter := NewSlackAdapter()
	_, err := adapter.ParseWebhookRequest(req)
	if err == nil {
		t.Fatal("expected error for unsupported event type, got nil")
	}
}

func TestSlackAdapter_RawBodyPreserved(t *testing.T) {
	body := appMentionEventJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/slack", strings.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=abc123def456")

	adapter := NewSlackAdapter()
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(env.RawBody) != body {
		t.Errorf("RawBody not preserved, got %q", string(env.RawBody))
	}
}
