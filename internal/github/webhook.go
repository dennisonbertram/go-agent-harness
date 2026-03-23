package github

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// WebhookPayload holds normalized data from a GitHub webhook event.
type WebhookPayload struct {
	EventType     string
	DeliveryID    string
	RepoOwner     string
	RepoName      string
	IssueNumber   int
	IssueTitle    string
	IssueBody     string
	Action        string // GitHub action field ("opened", "created", etc.)
	LatestComment string
	RawBody       []byte
}

// issuesEvent is the JSON shape for GitHub "issues" webhook events.
type issuesEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	} `json:"issue"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// issueCommentEvent is the JSON shape for GitHub "issue_comment" webhook events.
type issueCommentEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	} `json:"issue"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// pullRequestEvent is the JSON shape for GitHub "pull_request" webhook events.
type pullRequestEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	} `json:"pull_request"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// pullRequestReviewEvent is the JSON shape for GitHub "pull_request_review" webhook events.
type pullRequestReviewEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	} `json:"pull_request"`
	Review struct {
		Body string `json:"body"`
	} `json:"review"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// ParseWebhookPayload decodes a GitHub webhook JSON body for the given event type.
// Supported event types: "issues", "issue_comment", "pull_request", "pull_request_review".
// Returns error for unsupported event types.
func ParseWebhookPayload(eventType string, body []byte) (*WebhookPayload, error) {
	switch eventType {
	case "issues":
		var ev issuesEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil, fmt.Errorf("failed to parse issues event: %w", err)
		}
		return &WebhookPayload{
			EventType:   eventType,
			RepoOwner:   ev.Repository.Owner.Login,
			RepoName:    ev.Repository.Name,
			IssueNumber: ev.Issue.Number,
			IssueTitle:  ev.Issue.Title,
			IssueBody:   ev.Issue.Body,
			Action:      ev.Action,
			RawBody:     body,
		}, nil

	case "issue_comment":
		var ev issueCommentEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil, fmt.Errorf("failed to parse issue_comment event: %w", err)
		}
		return &WebhookPayload{
			EventType:     eventType,
			RepoOwner:     ev.Repository.Owner.Login,
			RepoName:      ev.Repository.Name,
			IssueNumber:   ev.Issue.Number,
			IssueTitle:    ev.Issue.Title,
			IssueBody:     ev.Issue.Body,
			Action:        ev.Action,
			LatestComment: ev.Comment.Body,
			RawBody:       body,
		}, nil

	case "pull_request":
		var ev pullRequestEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil, fmt.Errorf("failed to parse pull_request event: %w", err)
		}
		return &WebhookPayload{
			EventType:   eventType,
			RepoOwner:   ev.Repository.Owner.Login,
			RepoName:    ev.Repository.Name,
			IssueNumber: ev.PullRequest.Number,
			IssueTitle:  ev.PullRequest.Title,
			IssueBody:   ev.PullRequest.Body,
			Action:      ev.Action,
			RawBody:     body,
		}, nil

	case "pull_request_review":
		var ev pullRequestReviewEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil, fmt.Errorf("failed to parse pull_request_review event: %w", err)
		}
		return &WebhookPayload{
			EventType:     eventType,
			RepoOwner:     ev.Repository.Owner.Login,
			RepoName:      ev.Repository.Name,
			IssueNumber:   ev.PullRequest.Number,
			IssueTitle:    ev.PullRequest.Title,
			IssueBody:     ev.PullRequest.Body,
			Action:        ev.Action,
			LatestComment: ev.Review.Body,
			RawBody:       body,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported GitHub event type: %q", eventType)
	}
}

// DeriveAction maps a GitHub event type and action to a trigger action string.
//
// Mapping:
//
//	"issues" opened/labeled         → "start"
//	"issue_comment" created         → "steer"
//	"pull_request" opened           → "start"
//	"pull_request" synchronize      → "steer"
//	"pull_request_review" submitted → "steer"
//
// Any unrecognised combination returns an empty string.
func DeriveAction(eventType, action string) string {
	switch eventType {
	case "issues":
		switch action {
		case "opened", "labeled":
			return "start"
		}
	case "issue_comment":
		if action == "created" {
			return "steer"
		}
	case "pull_request":
		switch action {
		case "opened":
			return "start"
		case "synchronize":
			return "steer"
		}
	case "pull_request_review":
		if action == "submitted" {
			return "steer"
		}
	}
	return ""
}

// ComposeMessage builds the prompt message from a webhook payload.
// Format: "Issue #N: <title>\n\n<body>" with optional latest comment appended.
// For pull requests the prefix remains "Issue #N" for consistency with the trigger system.
func ComposeMessage(payload *WebhookPayload) string {
	var sb strings.Builder
	sb.WriteString("Issue #")
	sb.WriteString(strconv.Itoa(payload.IssueNumber))
	sb.WriteString(": ")
	sb.WriteString(payload.IssueTitle)
	if payload.IssueBody != "" {
		sb.WriteString("\n\n")
		sb.WriteString(payload.IssueBody)
	}
	if payload.LatestComment != "" {
		sb.WriteString("\n\nLatest comment:\n")
		sb.WriteString(payload.LatestComment)
	}
	return sb.String()
}
