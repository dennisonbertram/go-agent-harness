# Issue #412 Implementation Plan — feat(github): add GitHub trigger ingestion and source-context hydration

**Date**: 2026-03-23
**Branch**: issue-412-github-trigger-ingestion (based on #411 branch)

## Summary

Add a GitHub-specific webhook adapter that sits on top of the trigger infrastructure from #411. It ingests GitHub issue/comment/review webhook events, hydrates task context, and routes to the existing `handleExternalTrigger` logic. No runner primitives changed.

## Architecture

New `internal/github/` package + dedicated HTTP endpoint `POST /v1/webhooks/github`.

## New Package: `internal/github/`

### `internal/github/webhook.go`
- `WebhookPayload` struct: EventType, DeliveryID, RepoOwner, RepoName, IssueNumber, IssueTitle, IssueBody, Action, LatestComment, RawBody
- `ParseWebhookPayload(eventType string, body []byte) (*WebhookPayload, error)`
- `DeriveAction(eventType, action string) string`
  - issues.opened/labeled → "start"
  - issue_comment.created → "steer" (caller routes by run state)
  - pull_request.opened → "start", synchronize → "steer"
  - pull_request_review.submitted → "steer"
- `ComposeMessage(payload *WebhookPayload) string`
  - Format: "Issue #N: <title>\n\n<body>\n\nLatest comment:\n<comment>"

### `internal/github/adapter.go`
- `GitHubAdapter` struct with `Secret string`
- `NewGitHubAdapter(secret string) *GitHubAdapter`
- `ParseWebhookRequest(r *http.Request) (*trigger.ExternalTriggerEnvelope, error)`
  - Reads X-GitHub-Event, X-GitHub-Delivery, X-Hub-Signature-256 headers
  - Reads raw body (for HMAC)
  - Sets Signature = X-Hub-Signature-256 header value (used by GitHubValidator)
  - Sets Source="github", SourceID=delivery ID, ThreadID=issue/PR number as string

## New HTTP Handler: `internal/server/http_github_webhook.go`

### `POST /v1/webhooks/github`
1. Read raw body, extract GitHub headers
2. Call `s.githubAdapter.ParseWebhookRequest(r)` → ExternalTriggerEnvelope
3. Delegate to existing `handleExternalTriggerEnvelope(w, r, env)` (refactor existing handler to accept envelope)
4. Return appropriate response

## Files to Create/Modify

| File | Change |
|------|--------|
| `internal/github/webhook.go` | New — payload types, parsing, action derivation, message composition |
| `internal/github/webhook_test.go` | New — parse tests for all 4 event types + compose + derive action |
| `internal/github/adapter.go` | New — GitHub adapter, request parsing |
| `internal/github/adapter_test.go` | New — adapter tests including HMAC validation |
| `internal/server/http_github_webhook.go` | New — HTTP handler for /v1/webhooks/github |
| `internal/server/http_github_webhook_test.go` | New — handler tests |
| `internal/server/http_external_trigger.go` | Refactor to expose inner routing func |
| `internal/server/http.go` | Add githubAdapter field, register route |
| `cmd/harnessd/main.go` | Initialize GitHubAdapter from GITHUB_WEBHOOK_SECRET env var |

## Supported Events (Phase 1)
1. `issues` (opened, labeled) → start
2. `issue_comment` (created) → steer/continue based on run state
3. `pull_request` (opened) → start; (synchronize) → steer
4. `pull_request_review` (submitted) → steer/continue

## Testing Strategy

**Write tests first (fail before implement):**
- Parse tests for each event type with realistic JSON fixtures
- Action derivation for all event+action combinations
- Message composition with/without comment
- Adapter: valid/invalid signature, missing headers
- Handler: start/steer/continue happy paths, 401/400/409 error paths
- Regression: existing trigger + direct API endpoints unaffected

## Commit Strategy
```
feat(#412): add github webhook ingestion and source-context hydration
```
