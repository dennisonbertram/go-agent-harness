package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseWebhookRequest_IssuesOpened(t *testing.T) {
	body := issuesOpenedJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", "abc-delivery-123")

	adapter := NewGitHubAdapter("")
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Source != "github" {
		t.Errorf("Source = %q; want %q", env.Source, "github")
	}
	if env.SourceID != "abc-delivery-123" {
		t.Errorf("SourceID = %q; want %q", env.SourceID, "abc-delivery-123")
	}
	if env.ThreadID != "42" {
		t.Errorf("ThreadID = %q; want %q", env.ThreadID, "42")
	}
	if env.Action != "start" {
		t.Errorf("Action = %q; want %q", env.Action, "start")
	}
	if env.RepoOwner != "acme-corp" {
		t.Errorf("RepoOwner = %q; want %q", env.RepoOwner, "acme-corp")
	}
	if env.RepoName != "go-agent-harness" {
		t.Errorf("RepoName = %q; want %q", env.RepoName, "go-agent-harness")
	}
	if env.RawBody == nil {
		t.Error("RawBody should not be nil")
	}
	// Message should contain the issue title
	if !strings.Contains(env.Message, "Bug: unexpected nil panic") {
		t.Errorf("Message missing issue title, got: %q", env.Message)
	}
}

func TestParseWebhookRequest_IssueComment(t *testing.T) {
	body := issueCommentCreatedJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-456")

	adapter := NewGitHubAdapter("")
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Action != "steer" {
		t.Errorf("Action = %q; want %q", env.Action, "steer")
	}
	if env.ThreadID != "42" {
		t.Errorf("ThreadID = %q; want %q", env.ThreadID, "42")
	}
	// Message should contain the latest comment
	if !strings.Contains(env.Message, "I can reproduce this on v1.2.3") {
		t.Errorf("Message missing comment, got: %q", env.Message)
	}
}

func TestParseWebhookRequest_PullRequest(t *testing.T) {
	body := pullRequestOpenedJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-789")

	adapter := NewGitHubAdapter("")
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Action != "start" {
		t.Errorf("Action = %q; want %q", env.Action, "start")
	}
	if env.ThreadID != "99" {
		t.Errorf("ThreadID = %q; want %q", env.ThreadID, "99")
	}
}

func TestParseWebhookRequest_WithSignature(t *testing.T) {
	secret := "my-webhook-secret"
	body := issuesOpenedJSON

	// Compute the expected HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-GitHub-Delivery", "delivery-hmac")
	req.Header.Set("X-Hub-Signature-256", sig)

	adapter := NewGitHubAdapter(secret)
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Signature != sig {
		t.Errorf("Signature = %q; want %q", env.Signature, sig)
	}
}

func TestParseWebhookRequest_MissingEventHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader("{}"))
	req.Header.Set("X-GitHub-Delivery", "delivery-123")
	// X-GitHub-Event intentionally omitted

	adapter := NewGitHubAdapter("")
	_, err := adapter.ParseWebhookRequest(req)
	if err == nil {
		t.Fatal("expected error for missing X-GitHub-Event, got nil")
	}
}

func TestParseWebhookRequest_MissingDeliveryHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(issuesOpenedJSON))
	req.Header.Set("X-GitHub-Event", "issues")
	// X-GitHub-Delivery intentionally omitted

	adapter := NewGitHubAdapter("")
	_, err := adapter.ParseWebhookRequest(req)
	if err == nil {
		t.Fatal("expected error for missing X-GitHub-Delivery, got nil")
	}
}

func TestParseWebhookRequest_UnsupportedEventType(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(`{"action":"created"}`))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-push")

	adapter := NewGitHubAdapter("")
	_, err := adapter.ParseWebhookRequest(req)
	if err == nil {
		t.Fatal("expected error for unsupported event type 'push', got nil")
	}
}

func TestParseWebhookRequest_RawBodyPreserved(t *testing.T) {
	body := issueCommentCreatedJSON
	req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-raw")

	adapter := NewGitHubAdapter("")
	env, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(env.RawBody) != body {
		t.Errorf("RawBody not preserved, got %q", string(env.RawBody))
	}
}

func TestParseWebhookRequest_ThreadIDIsIssueNumber(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		body      string
		wantID    string
	}{
		{"issues", "issues", issuesOpenedJSON, "42"},
		{"issue_comment", "issue_comment", issueCommentCreatedJSON, "42"},
		{"pull_request", "pull_request", pullRequestOpenedJSON, "99"},
		{"pull_request_review", "pull_request_review", pullRequestReviewSubmittedJSON, "99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/webhooks/github", strings.NewReader(tt.body))
			req.Header.Set("X-GitHub-Event", tt.eventType)
			req.Header.Set("X-GitHub-Delivery", fmt.Sprintf("delivery-%s", tt.name))

			adapter := NewGitHubAdapter("")
			env, err := adapter.ParseWebhookRequest(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env.ThreadID != tt.wantID {
				t.Errorf("ThreadID = %q; want %q", env.ThreadID, tt.wantID)
			}
		})
	}
}
