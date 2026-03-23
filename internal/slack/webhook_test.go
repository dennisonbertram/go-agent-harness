package slack

import (
	"testing"
)

// appMentionEventJSON is a realistic Slack app_mention event_callback payload.
const appMentionEventJSON = `{
	"type": "event_callback",
	"event_id": "Ev123ABC",
	"team_id": "T012AB3C4",
	"event": {
		"type": "app_mention",
		"user": "U012AB3C4",
		"text": "<@UBOT123> do something useful",
		"ts": "1234567890.123456",
		"thread_ts": "1234567890.000000",
		"channel": "C01234567"
	}
}`

// topLevelMessageJSON is a Slack event without thread_ts (top-level message).
const topLevelMessageJSON = `{
	"type": "event_callback",
	"event_id": "Ev456DEF",
	"team_id": "T012AB3C4",
	"event": {
		"type": "message",
		"user": "U567DEF89",
		"text": "Hello there",
		"ts": "9876543210.654321",
		"channel": "C09876543"
	}
}`

// unsupportedTypeJSON has a type other than "event_callback".
const unsupportedTypeJSON = `{
	"type": "url_verification",
	"challenge": "abc123"
}`

func TestParseWebhookPayload_AppMention(t *testing.T) {
	payload, err := ParseWebhookPayload([]byte(appMentionEventJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "event_callback" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "event_callback")
	}
	if payload.EventID != "Ev123ABC" {
		t.Errorf("EventID = %q; want %q", payload.EventID, "Ev123ABC")
	}
	if payload.TeamID != "T012AB3C4" {
		t.Errorf("TeamID = %q; want %q", payload.TeamID, "T012AB3C4")
	}
	if payload.ChannelID != "C01234567" {
		t.Errorf("ChannelID = %q; want %q", payload.ChannelID, "C01234567")
	}
	if payload.ThreadTS != "1234567890.000000" {
		t.Errorf("ThreadTS = %q; want %q", payload.ThreadTS, "1234567890.000000")
	}
	if payload.MessageTS != "1234567890.123456" {
		t.Errorf("MessageTS = %q; want %q", payload.MessageTS, "1234567890.123456")
	}
	if payload.InnerType != "app_mention" {
		t.Errorf("InnerType = %q; want %q", payload.InnerType, "app_mention")
	}
	if payload.Text != "<@UBOT123> do something useful" {
		t.Errorf("Text = %q; want %q", payload.Text, "<@UBOT123> do something useful")
	}
	if payload.UserID != "U012AB3C4" {
		t.Errorf("UserID = %q; want %q", payload.UserID, "U012AB3C4")
	}
	if payload.RawBody == nil {
		t.Error("RawBody should not be nil")
	}
}

func TestParseWebhookPayload_TopLevelMessage(t *testing.T) {
	payload, err := ParseWebhookPayload([]byte(topLevelMessageJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "event_callback" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "event_callback")
	}
	if payload.ThreadTS != "" {
		t.Errorf("ThreadTS should be empty for top-level message, got %q", payload.ThreadTS)
	}
	if payload.MessageTS != "9876543210.654321" {
		t.Errorf("MessageTS = %q; want %q", payload.MessageTS, "9876543210.654321")
	}
}

func TestParseWebhookPayload_UnsupportedType(t *testing.T) {
	_, err := ParseWebhookPayload([]byte(unsupportedTypeJSON))
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestParseWebhookPayload_InvalidJSON(t *testing.T) {
	_, err := ParseWebhookPayload([]byte(`not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseWebhookPayload_RawBodyPreserved(t *testing.T) {
	body := []byte(appMentionEventJSON)
	payload, err := ParseWebhookPayload(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(payload.RawBody) != string(body) {
		t.Error("RawBody not preserved in parsed payload")
	}
}

func TestComposeMessage_WithChannel(t *testing.T) {
	payload := &SlackWebhookPayload{
		ChannelID: "C01234567",
		Text:      "<@UBOT> fix the bug",
	}
	got := ComposeMessage(payload)
	want := "[C01234567] <@UBOT> fix the bug"
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}

func TestComposeMessage_NoChannel(t *testing.T) {
	payload := &SlackWebhookPayload{
		Text: "hello world",
	}
	got := ComposeMessage(payload)
	want := "hello world"
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}
