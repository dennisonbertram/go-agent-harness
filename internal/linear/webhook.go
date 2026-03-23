package linear

import (
	"encoding/json"
	"fmt"
	"strings"
)

// LinearWebhookPayload holds normalized Linear event data.
type LinearWebhookPayload struct {
	EventType   string // "Issue", "Comment"
	Action      string // "create", "update", "delete"
	IssueID     string // data.id (for Issue events)
	Identifier  string // data.identifier (e.g. "ENG-123")
	Title       string // data.title
	Description string // data.description
	TeamID      string // data.teamId
	OrgID       string // organizationId
	CommentBody string // data.body (for Comment events)
	RawBody     []byte
}

// linearIssueEvent is the JSON shape for Linear "Issue" webhook events.
type linearIssueEvent struct {
	Type           string `json:"type"`
	Action         string `json:"action"`
	OrganizationID string `json:"organizationId"`
	Data           struct {
		ID          string `json:"id"`
		Identifier  string `json:"identifier"`
		Title       string `json:"title"`
		Description string `json:"description"`
		TeamID      string `json:"teamId"`
	} `json:"data"`
}

// linearCommentEvent is the JSON shape for Linear "Comment" webhook events.
type linearCommentEvent struct {
	Type           string `json:"type"`
	Action         string `json:"action"`
	OrganizationID string `json:"organizationId"`
	Data           struct {
		ID      string `json:"id"`
		Body    string `json:"body"`
		IssueID string `json:"issueId"`
		Issue   struct {
			Identifier string `json:"identifier"`
			Title      string `json:"title"`
		} `json:"issue"`
	} `json:"data"`
}

// linearEventType is used to peek at the "type" field before full parsing.
type linearEventType struct {
	Type string `json:"type"`
}

// ParseWebhookPayload decodes a Linear webhook JSON body.
// Supports "Issue" and "Comment" event types.
func ParseWebhookPayload(body []byte) (*LinearWebhookPayload, error) {
	var peek linearEventType
	if err := json.Unmarshal(body, &peek); err != nil {
		return nil, fmt.Errorf("failed to parse linear event payload: %w", err)
	}

	switch peek.Type {
	case "Issue":
		var ev linearIssueEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil, fmt.Errorf("failed to parse linear Issue event: %w", err)
		}
		return &LinearWebhookPayload{
			EventType:   ev.Type,
			Action:      ev.Action,
			IssueID:     ev.Data.ID,
			Identifier:  ev.Data.Identifier,
			Title:       ev.Data.Title,
			Description: ev.Data.Description,
			TeamID:      ev.Data.TeamID,
			OrgID:       ev.OrganizationID,
			RawBody:     body,
		}, nil

	case "Comment":
		var ev linearCommentEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return nil, fmt.Errorf("failed to parse linear Comment event: %w", err)
		}
		return &LinearWebhookPayload{
			EventType:   ev.Type,
			Action:      ev.Action,
			IssueID:     ev.Data.IssueID,
			Identifier:  ev.Data.Issue.Identifier,
			Title:       ev.Data.Issue.Title,
			OrgID:       ev.OrganizationID,
			CommentBody: ev.Data.Body,
			RawBody:     body,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported Linear event type: %q (supported: Issue, Comment)", peek.Type)
	}
}

// DeriveAction maps a Linear event type and action to a trigger action string.
//
// Mapping:
//
//	Issue.create          → "start"
//	Issue.update          → "steer"
//	Comment.create        → "steer"
//
// Any unrecognised combination returns an empty string.
func DeriveAction(eventType, action string) string {
	switch eventType {
	case "Issue":
		switch action {
		case "create":
			return "start"
		case "update":
			return "steer"
		}
	case "Comment":
		if action == "create" {
			return "steer"
		}
	}
	return ""
}

// ComposeMessage builds the trigger message from a Linear payload.
func ComposeMessage(payload *LinearWebhookPayload) string {
	var sb strings.Builder

	switch payload.EventType {
	case "Issue":
		if payload.Identifier != "" {
			sb.WriteString(payload.Identifier)
			sb.WriteString(": ")
		}
		sb.WriteString(payload.Title)
		if payload.Description != "" {
			sb.WriteString("\n\n")
			sb.WriteString(payload.Description)
		}
	case "Comment":
		if payload.Identifier != "" {
			sb.WriteString(payload.Identifier)
			if payload.Title != "" {
				sb.WriteString(": ")
				sb.WriteString(payload.Title)
			}
			sb.WriteString("\n\n")
		}
		sb.WriteString("New comment:\n")
		sb.WriteString(payload.CommentBody)
	default:
		// Fallback: just use what we have.
		sb.WriteString(payload.Title)
	}

	return sb.String()
}
