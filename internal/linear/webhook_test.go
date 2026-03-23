package linear

import (
	"strings"
	"testing"
)

// issueCreateJSON is a realistic Linear Issue.create webhook payload.
const issueCreateJSON = `{
	"type": "Issue",
	"action": "create",
	"organizationId": "org-abc123",
	"webhookId": "hook-xyz789",
	"data": {
		"id": "issue-uuid-001",
		"identifier": "ENG-123",
		"title": "Fix the login bug",
		"description": "Users cannot log in when 2FA is enabled.",
		"teamId": "team-uuid-001"
	}
}`

// issueUpdateJSON is a Linear Issue.update webhook payload.
const issueUpdateJSON = `{
	"type": "Issue",
	"action": "update",
	"organizationId": "org-abc123",
	"data": {
		"id": "issue-uuid-001",
		"identifier": "ENG-123",
		"title": "Fix the login bug",
		"description": "Updated description.",
		"teamId": "team-uuid-001"
	}
}`

// commentCreateJSON is a realistic Linear Comment.create webhook payload.
const commentCreateJSON = `{
	"type": "Comment",
	"action": "create",
	"organizationId": "org-abc123",
	"data": {
		"id": "comment-uuid-001",
		"body": "Please fix this ASAP, it is blocking users.",
		"issueId": "issue-uuid-001",
		"issue": {
			"identifier": "ENG-123",
			"title": "Fix the login bug"
		}
	}
}`

// unsupportedLinearTypeJSON has a type not supported by the adapter.
const unsupportedLinearTypeJSON = `{
	"type": "Project",
	"action": "create",
	"data": {"id": "proj-001"}
}`

func TestParseWebhookPayload_IssueCreate(t *testing.T) {
	payload, err := ParseWebhookPayload([]byte(issueCreateJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "Issue" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "Issue")
	}
	if payload.Action != "create" {
		t.Errorf("Action = %q; want %q", payload.Action, "create")
	}
	if payload.IssueID != "issue-uuid-001" {
		t.Errorf("IssueID = %q; want %q", payload.IssueID, "issue-uuid-001")
	}
	if payload.Identifier != "ENG-123" {
		t.Errorf("Identifier = %q; want %q", payload.Identifier, "ENG-123")
	}
	if payload.Title != "Fix the login bug" {
		t.Errorf("Title = %q; want %q", payload.Title, "Fix the login bug")
	}
	if payload.Description != "Users cannot log in when 2FA is enabled." {
		t.Errorf("Description = %q; want %q", payload.Description, "Users cannot log in when 2FA is enabled.")
	}
	if payload.TeamID != "team-uuid-001" {
		t.Errorf("TeamID = %q; want %q", payload.TeamID, "team-uuid-001")
	}
	if payload.OrgID != "org-abc123" {
		t.Errorf("OrgID = %q; want %q", payload.OrgID, "org-abc123")
	}
	if payload.CommentBody != "" {
		t.Errorf("CommentBody should be empty for Issue event, got %q", payload.CommentBody)
	}
	if payload.RawBody == nil {
		t.Error("RawBody should not be nil")
	}
}

func TestParseWebhookPayload_CommentCreate(t *testing.T) {
	payload, err := ParseWebhookPayload([]byte(commentCreateJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "Comment" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "Comment")
	}
	if payload.Action != "create" {
		t.Errorf("Action = %q; want %q", payload.Action, "create")
	}
	if payload.CommentBody != "Please fix this ASAP, it is blocking users." {
		t.Errorf("CommentBody = %q; want %q", payload.CommentBody, "Please fix this ASAP, it is blocking users.")
	}
	if payload.Identifier != "ENG-123" {
		t.Errorf("Identifier = %q; want %q", payload.Identifier, "ENG-123")
	}
	if payload.Title != "Fix the login bug" {
		t.Errorf("Title = %q; want %q", payload.Title, "Fix the login bug")
	}
	if payload.IssueID != "issue-uuid-001" {
		t.Errorf("IssueID = %q; want %q", payload.IssueID, "issue-uuid-001")
	}
}

func TestParseWebhookPayload_UnsupportedType(t *testing.T) {
	_, err := ParseWebhookPayload([]byte(unsupportedLinearTypeJSON))
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
	body := []byte(issueCreateJSON)
	payload, err := ParseWebhookPayload(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(payload.RawBody) != string(body) {
		t.Error("RawBody not preserved in parsed payload")
	}
}

func TestDeriveAction(t *testing.T) {
	tests := []struct {
		eventType string
		action    string
		want      string
	}{
		{"Issue", "create", "start"},
		{"Issue", "update", "steer"},
		{"Issue", "delete", ""},
		{"Comment", "create", "steer"},
		{"Comment", "update", ""},
		{"Comment", "delete", ""},
		{"Project", "create", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		got := DeriveAction(tt.eventType, tt.action)
		if got != tt.want {
			t.Errorf("DeriveAction(%q, %q) = %q; want %q", tt.eventType, tt.action, got, tt.want)
		}
	}
}

func TestComposeMessage_IssueCreate(t *testing.T) {
	payload := &LinearWebhookPayload{
		EventType:   "Issue",
		Identifier:  "ENG-123",
		Title:       "Fix the login bug",
		Description: "Users cannot log in when 2FA is enabled.",
	}
	got := ComposeMessage(payload)
	want := "ENG-123: Fix the login bug\n\nUsers cannot log in when 2FA is enabled."
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}

func TestComposeMessage_IssueNoDescription(t *testing.T) {
	payload := &LinearWebhookPayload{
		EventType:  "Issue",
		Identifier: "ENG-456",
		Title:      "Add export feature",
	}
	got := ComposeMessage(payload)
	want := "ENG-456: Add export feature"
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}

func TestComposeMessage_Comment(t *testing.T) {
	payload := &LinearWebhookPayload{
		EventType:   "Comment",
		Identifier:  "ENG-123",
		Title:       "Fix the login bug",
		CommentBody: "Please fix this ASAP.",
	}
	got := ComposeMessage(payload)
	if !strings.Contains(got, "ENG-123") {
		t.Errorf("ComposeMessage() missing identifier, got: %q", got)
	}
	if !strings.Contains(got, "Please fix this ASAP.") {
		t.Errorf("ComposeMessage() missing comment body, got: %q", got)
	}
}
