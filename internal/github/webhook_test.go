package github

import (
	"testing"
)

// issuesOpenedJSON is a realistic GitHub "issues" opened event payload.
const issuesOpenedJSON = `{
	"action": "opened",
	"issue": {
		"number": 42,
		"title": "Bug: unexpected nil panic",
		"body": "Steps to reproduce:\n1. Call foo()\n2. Observe panic"
	},
	"repository": {
		"name": "go-agent-harness",
		"owner": {
			"login": "acme-corp"
		}
	}
}`

// issuesLabeledJSON is a GitHub "issues" labeled event.
const issuesLabeledJSON = `{
	"action": "labeled",
	"issue": {
		"number": 7,
		"title": "Feature request: add export",
		"body": "Please add CSV export"
	},
	"repository": {
		"name": "my-repo",
		"owner": {
			"login": "octocat"
		}
	}
}`

// issueCommentCreatedJSON is a GitHub "issue_comment" created event.
const issueCommentCreatedJSON = `{
	"action": "created",
	"issue": {
		"number": 42,
		"title": "Bug: unexpected nil panic",
		"body": "Steps to reproduce"
	},
	"comment": {
		"body": "I can reproduce this on v1.2.3"
	},
	"repository": {
		"name": "go-agent-harness",
		"owner": {
			"login": "acme-corp"
		}
	}
}`

// pullRequestOpenedJSON is a GitHub "pull_request" opened event.
const pullRequestOpenedJSON = `{
	"action": "opened",
	"pull_request": {
		"number": 99,
		"title": "feat: new widget",
		"body": "This PR adds the new widget component."
	},
	"repository": {
		"name": "go-agent-harness",
		"owner": {
			"login": "acme-corp"
		}
	}
}`

// pullRequestSynchronizeJSON is a GitHub "pull_request" synchronize event.
const pullRequestSynchronizeJSON = `{
	"action": "synchronize",
	"pull_request": {
		"number": 99,
		"title": "feat: new widget",
		"body": "This PR adds the new widget component."
	},
	"repository": {
		"name": "go-agent-harness",
		"owner": {
			"login": "acme-corp"
		}
	}
}`

// pullRequestReviewSubmittedJSON is a GitHub "pull_request_review" submitted event.
const pullRequestReviewSubmittedJSON = `{
	"action": "submitted",
	"pull_request": {
		"number": 99,
		"title": "feat: new widget",
		"body": "This PR adds the new widget component."
	},
	"review": {
		"body": "Looks good to me, just one nit."
	},
	"repository": {
		"name": "go-agent-harness",
		"owner": {
			"login": "acme-corp"
		}
	}
}`

func TestParseWebhookPayload_Issues(t *testing.T) {
	payload, err := ParseWebhookPayload("issues", []byte(issuesOpenedJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "issues" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "issues")
	}
	if payload.IssueNumber != 42 {
		t.Errorf("IssueNumber = %d; want 42", payload.IssueNumber)
	}
	if payload.IssueTitle != "Bug: unexpected nil panic" {
		t.Errorf("IssueTitle = %q", payload.IssueTitle)
	}
	if payload.IssueBody != "Steps to reproduce:\n1. Call foo()\n2. Observe panic" {
		t.Errorf("IssueBody = %q", payload.IssueBody)
	}
	if payload.Action != "opened" {
		t.Errorf("Action = %q; want %q", payload.Action, "opened")
	}
	if payload.RepoOwner != "acme-corp" {
		t.Errorf("RepoOwner = %q; want %q", payload.RepoOwner, "acme-corp")
	}
	if payload.RepoName != "go-agent-harness" {
		t.Errorf("RepoName = %q; want %q", payload.RepoName, "go-agent-harness")
	}
	if payload.LatestComment != "" {
		t.Errorf("LatestComment should be empty for issues event, got %q", payload.LatestComment)
	}
}

func TestParseWebhookPayload_IssueComment(t *testing.T) {
	payload, err := ParseWebhookPayload("issue_comment", []byte(issueCommentCreatedJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "issue_comment" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "issue_comment")
	}
	if payload.IssueNumber != 42 {
		t.Errorf("IssueNumber = %d; want 42", payload.IssueNumber)
	}
	if payload.LatestComment != "I can reproduce this on v1.2.3" {
		t.Errorf("LatestComment = %q", payload.LatestComment)
	}
	if payload.Action != "created" {
		t.Errorf("Action = %q; want %q", payload.Action, "created")
	}
}

func TestParseWebhookPayload_PullRequest(t *testing.T) {
	payload, err := ParseWebhookPayload("pull_request", []byte(pullRequestOpenedJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "pull_request" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "pull_request")
	}
	if payload.IssueNumber != 99 {
		t.Errorf("IssueNumber = %d; want 99", payload.IssueNumber)
	}
	if payload.IssueTitle != "feat: new widget" {
		t.Errorf("IssueTitle = %q", payload.IssueTitle)
	}
	if payload.Action != "opened" {
		t.Errorf("Action = %q; want %q", payload.Action, "opened")
	}
}

func TestParseWebhookPayload_PullRequestReview(t *testing.T) {
	payload, err := ParseWebhookPayload("pull_request_review", []byte(pullRequestReviewSubmittedJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.EventType != "pull_request_review" {
		t.Errorf("EventType = %q; want %q", payload.EventType, "pull_request_review")
	}
	if payload.IssueNumber != 99 {
		t.Errorf("IssueNumber = %d; want 99", payload.IssueNumber)
	}
	if payload.LatestComment != "Looks good to me, just one nit." {
		t.Errorf("LatestComment = %q", payload.LatestComment)
	}
	if payload.Action != "submitted" {
		t.Errorf("Action = %q; want %q", payload.Action, "submitted")
	}
}

func TestParseWebhookPayload_Unsupported(t *testing.T) {
	_, err := ParseWebhookPayload("push", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for unsupported event type, got nil")
	}
}

func TestParseWebhookPayload_InvalidJSON(t *testing.T) {
	_, err := ParseWebhookPayload("issues", []byte(`not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestDeriveAction(t *testing.T) {
	tests := []struct {
		eventType string
		action    string
		want      string
	}{
		{"issues", "opened", "start"},
		{"issues", "labeled", "start"},
		{"issues", "closed", ""},
		{"issues", "edited", ""},
		{"issue_comment", "created", "steer"},
		{"issue_comment", "edited", ""},
		{"issue_comment", "deleted", ""},
		{"pull_request", "opened", "start"},
		{"pull_request", "synchronize", "steer"},
		{"pull_request", "closed", ""},
		{"pull_request", "edited", ""},
		{"pull_request_review", "submitted", "steer"},
		{"pull_request_review", "dismissed", ""},
		{"push", "created", ""},
		{"unknown", "anything", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		got := DeriveAction(tt.eventType, tt.action)
		if got != tt.want {
			t.Errorf("DeriveAction(%q, %q) = %q; want %q", tt.eventType, tt.action, got, tt.want)
		}
	}
}

func TestComposeMessage_WithBody(t *testing.T) {
	payload := &WebhookPayload{
		IssueNumber: 42,
		IssueTitle:  "Bug: nil panic",
		IssueBody:   "Steps to reproduce the issue.",
	}
	got := ComposeMessage(payload)
	want := "Issue #42: Bug: nil panic\n\nSteps to reproduce the issue."
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}

func TestComposeMessage_WithComment(t *testing.T) {
	payload := &WebhookPayload{
		IssueNumber:   42,
		IssueTitle:    "Bug: nil panic",
		IssueBody:     "Steps to reproduce the issue.",
		LatestComment: "I can reproduce this on v1.2.3",
	}
	got := ComposeMessage(payload)
	want := "Issue #42: Bug: nil panic\n\nSteps to reproduce the issue.\n\nLatest comment:\nI can reproduce this on v1.2.3"
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}

func TestComposeMessage_NoBody(t *testing.T) {
	payload := &WebhookPayload{
		IssueNumber: 7,
		IssueTitle:  "Feature request",
	}
	got := ComposeMessage(payload)
	want := "Issue #7: Feature request"
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}

func TestComposeMessage_CommentOnlyNoBody(t *testing.T) {
	payload := &WebhookPayload{
		IssueNumber:   7,
		IssueTitle:    "Feature request",
		LatestComment: "Great idea!",
	}
	got := ComposeMessage(payload)
	want := "Issue #7: Feature request\n\nLatest comment:\nGreat idea!"
	if got != want {
		t.Errorf("ComposeMessage() = %q; want %q", got, want)
	}
}

func TestParseWebhookPayload_IssuesLabeled(t *testing.T) {
	payload, err := ParseWebhookPayload("issues", []byte(issuesLabeledJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.Action != "labeled" {
		t.Errorf("Action = %q; want %q", payload.Action, "labeled")
	}
	if payload.IssueNumber != 7 {
		t.Errorf("IssueNumber = %d; want 7", payload.IssueNumber)
	}
	if payload.RepoOwner != "octocat" {
		t.Errorf("RepoOwner = %q; want %q", payload.RepoOwner, "octocat")
	}
}

func TestParseWebhookPayload_PullRequestSynchronize(t *testing.T) {
	payload, err := ParseWebhookPayload("pull_request", []byte(pullRequestSynchronizeJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.Action != "synchronize" {
		t.Errorf("Action = %q; want %q", payload.Action, "synchronize")
	}
}

func TestParseWebhookPayload_RawBodySet(t *testing.T) {
	body := []byte(issuesOpenedJSON)
	payload, err := ParseWebhookPayload("issues", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(payload.RawBody) != string(body) {
		t.Error("RawBody not preserved in parsed payload")
	}
}
