package slack

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SlackWebhookPayload holds normalized Slack event data.
type SlackWebhookPayload struct {
	EventType string // outer type: "event_callback"
	EventID   string // event_id
	TeamID    string // team_id
	ChannelID string // event.channel
	ThreadTS  string // event.thread_ts (empty for top-level messages)
	MessageTS string // event.ts
	InnerType string // event.type: "app_mention", "message"
	Text      string // event.text (message content)
	UserID    string // event.user (sender)
	RawBody   []byte
}

// slackEventCallback is the JSON shape for Slack event_callback payloads.
type slackEventCallback struct {
	Type    string `json:"type"`
	EventID string `json:"event_id"`
	TeamID  string `json:"team_id"`
	Event   struct {
		Type     string `json:"type"`
		User     string `json:"user"`
		Text     string `json:"text"`
		TS       string `json:"ts"`
		ThreadTS string `json:"thread_ts"`
		Channel  string `json:"channel"`
	} `json:"event"`
}

// ParseWebhookPayload decodes a Slack event_callback JSON body.
// Returns error for unsupported event types (only "event_callback" supported).
func ParseWebhookPayload(body []byte) (*SlackWebhookPayload, error) {
	var ev slackEventCallback
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, fmt.Errorf("failed to parse slack event payload: %w", err)
	}
	if ev.Type != "event_callback" {
		return nil, fmt.Errorf("unsupported Slack event type: %q (only \"event_callback\" is supported)", ev.Type)
	}
	return &SlackWebhookPayload{
		EventType: ev.Type,
		EventID:   ev.EventID,
		TeamID:    ev.TeamID,
		ChannelID: ev.Event.Channel,
		ThreadTS:  ev.Event.ThreadTS,
		MessageTS: ev.Event.TS,
		InnerType: ev.Event.Type,
		Text:      ev.Event.Text,
		UserID:    ev.Event.User,
		RawBody:   body,
	}, nil
}

// ComposeMessage builds the trigger message from a Slack event payload.
// Format: "[channel] <text>" or just the text when channel is empty.
func ComposeMessage(payload *SlackWebhookPayload) string {
	var sb strings.Builder
	if payload.ChannelID != "" {
		sb.WriteString("[")
		sb.WriteString(payload.ChannelID)
		sb.WriteString("] ")
	}
	sb.WriteString(payload.Text)
	return sb.String()
}
