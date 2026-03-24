# Issue #413 Implementation Plan — feat(integrations): add Slack and Linear trigger adapters

**Date**: 2026-03-23
**Branch**: issue-413-slack-linear-trigger-adapters (based on #412)

## Summary

Add Slack and Linear webhook adapters mirroring the GitHub adapter from #412. Both reuse the shared `ExternalTriggerEnvelope`, `ValidatorRegistry`, and `dispatchTriggerEnvelope` from #411/#412.

## Architecture

Two new packages (`internal/slack/`, `internal/linear/`) + two new HTTP endpoints (`POST /v1/webhooks/slack`, `POST /v1/webhooks/linear`), mirroring `internal/github/` pattern exactly.

## Slack Adapter (`internal/slack/`)

### Slack Webhook Events to Support
- `app_mention` event → action="steer" or "continue"
- `message` event (in channel) → action="steer" or "continue"
- `event_callback` (outer envelope) with nested event type

### Slack Payload Structure
```json
{"type":"event_callback","event_id":"Ev123","event":{"type":"app_mention","text":"...","channel":"C123","ts":"12345.67890"},"team_id":"T123","api_app_id":"A123"}
```

### Signature Format (from validator.go)
- Signature in envelope: `"{timestamp}:{v0=hex}"` (packed format)
- `X-Slack-Request-Timestamp` header + `X-Slack-Signature` header
- Handler packs them as `timestamp:v0=hex` into `X-Trigger-Signature` header

### ThreadID for Slack
- Use `channel_id + ":" + thread_ts` (or just `channel_id` for top-level messages)
- This gives per-channel or per-thread isolation

### `internal/slack/webhook.go`
- `SlackWebhookPayload` struct
- `ParseWebhookPayload(body []byte) (*SlackWebhookPayload, error)`
- `ComposeMessage(payload *SlackWebhookPayload) string`

### `internal/slack/adapter.go`
- `SlackAdapter` struct
- `ParseWebhookRequest(r *http.Request) (*trigger.ExternalTriggerEnvelope, error)`
  - Reads `X-Slack-Request-Timestamp` + `X-Slack-Signature`
  - Packs as `timestamp:signature` into `X-Trigger-Signature` header equivalent in envelope
  - Sets Source="slack", ThreadID=channel+ts

## Linear Adapter (`internal/linear/`)

### Linear Webhook Events to Support
- `Issue` action=`create` → action="start"
- `Issue` action=`update` → action="steer" or "continue"
- `Comment` action=`create` → action="steer" or "continue"

### Linear Payload Structure
```json
{"type":"Issue","action":"create","data":{"id":"issue-uuid","identifier":"ENG-123","title":"...","description":"...","teamId":"team-uuid"},"organizationId":"org-uuid","webhookId":"hook-uuid"}
```

### Signature Format (from validator.go)
- `X-Linear-Signature` header: raw hex HMAC-SHA256
- Set directly as `X-Trigger-Signature`

### ThreadID for Linear
- Use `data.identifier` (e.g., "ENG-123") as ThreadID — stable per issue

### `internal/linear/webhook.go`
- `LinearWebhookPayload` struct
- `ParseWebhookPayload(body []byte) (*LinearWebhookPayload, error)`
- `DeriveAction(eventType, action string) string`
- `ComposeMessage(payload *LinearWebhookPayload) string`

### `internal/linear/adapter.go`
- `LinearAdapter` struct
- `ParseWebhookRequest(r *http.Request) (*trigger.ExternalTriggerEnvelope, error)`

## Server Handler Files

### `internal/server/http_slack_webhook.go`
- `POST /v1/webhooks/slack` → `handleSlackWebhook`
- Mirrors `handleGitHubWebhook` exactly

### `internal/server/http_linear_webhook.go`
- `POST /v1/webhooks/linear` → `handleLinearWebhook`
- Mirrors `handleGitHubWebhook` exactly

## Files to Create/Modify

| File | Change |
|------|--------|
| `internal/slack/webhook.go` | New — Slack payload parsing |
| `internal/slack/webhook_test.go` | New |
| `internal/slack/adapter.go` | New — Slack adapter |
| `internal/slack/adapter_test.go` | New |
| `internal/linear/webhook.go` | New — Linear payload parsing |
| `internal/linear/webhook_test.go` | New |
| `internal/linear/adapter.go` | New — Linear adapter |
| `internal/linear/adapter_test.go` | New |
| `internal/server/http_slack_webhook.go` | New — POST /v1/webhooks/slack |
| `internal/server/http_slack_webhook_test.go` | New |
| `internal/server/http_linear_webhook.go` | New — POST /v1/webhooks/linear |
| `internal/server/http_linear_webhook_test.go` | New |
| `internal/server/http.go` | Add slackAdapter, linearAdapter fields + route registration |
| `cmd/harnessd/main.go` | Init adapters from env: SLACK_SIGNING_SECRET, LINEAR_WEBHOOK_SECRET |

## Commit Strategy
```
feat(#413): add Slack and Linear trigger adapters on shared trigger layer
```

## Notes on Slack Signature Packing

The `SlackValidator` in `internal/trigger/validator.go` expects `env.Signature` to be packed as `"timestamp:v0=hex"`. The Slack adapter must pack the two headers into this format before setting the signature in the envelope OR pass it via a header that the handler reads.

Check how `handleGitHubWebhook` passes the signature header to `dispatchTriggerEnvelope` and mirror that pattern for Slack.
