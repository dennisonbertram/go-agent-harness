# Cross-Surface Impact Map: Issue #1186

## Task

- Task / issue: #1186 typed public cron validation errors.
- Plan: `2026-08-05-issue-1186-cron-validation-plan.md`.
- Status: planned, red-first implementation pending.

## Current Ownership, Callers, and Data Flow

- Entry points: `internal/server/http_cron.go` POST/PATCH invoke `tools.CronClient`; `cmd/harnessd/main.go` selects embedded or remote adapter; `internal/cron/client.go` consumes cronsd errors.
- Source of truth: raw `internal/cron/server.go` already validates and writes `400 validation_error`; `cronClientAdapter` and `embeddedCronAdapter` translate at the harness seam.
- Search evidence: `rg -n 'validation_error|CreateJob|UpdateJob|writeCronJobError|parseError' internal/cron internal/server cmd/harnessd` identified flattening at `Client.parseError`, adapters, and facade error rendering.
- Conclusion: add one typed error identity with existing not-found/conflict identities; no parallel validation implementation.

## Config, API, CLI, and Tools

- API: POST/PATCH `/v1/cron/jobs` classify known caller-validation failures as existing `400 {error:{code:"validation_error"}}`; JSON shape is unchanged.
- Config/defaults/CLI/tools: none. Existing cron agent tool receives a useful client response via its existing HTTP path.
- Error states: 404/409 and actual 5xx retain their identities and status codes.

## Persistence and Compatibility

- No schema, migration, cache, or persisted-job format changes.
- Invalid creates remain non-persistent. Older harnessd versions retain their old generic-error behavior; newer harnessd remains wire-compatible with older clients.

## Lifecycle, Security, and Reliability

- No scheduling, retries, cancellation, or concurrency ownership change.
- Auth/scope checks remain before writes; validation classification must not turn authorization or store errors into 400.
- Remote transport preserves only authenticated cronsd's explicit 400 validation response; dependency/network failures remain 5xx.

## Product and Integration Surfaces

- Harness/server: affected POST/PATCH responses.
- TUI/GUI/web: no direct code change; existing Activity and tool error views gain the correct actionable server classification.
- Provider/model/external automation: none; no executor or provider routing change.

## Deployment and Operations

- Normal binary deployment, no feature flag. Monitor HTTP 400 versus 500 cron-write rates; unexpected 500s still expose their dependency message.
- Rollback: revert this isolated classification change; no records need repair and no runbook/public tutorial text changes.

## Regression Tests

- First red: public facade typed validation create/update response tests expect 400 but receive current 500; remote adapter test expects preserved identity but sees flattened HTTP error.
- Acceptance: invalid schedule/type/shell/harness config/timeout/status return 400 and leave embedded storage empty; valid create/update remain 2xx.
- Negative: explicitly preserve typed not-found/conflict plus arbitrary dependency error mapping.
- Commands: focused normal/race server, cron, and harnessd packages; canonical-temp full regression; direct embedded and remote HTTP acceptance.

## Documentation and Handoff

- This plan/map precede code. After green verification, update plans/log indexes plus engineering, observational, system, and long-term-thinking logs.
- Public tutorial is unchanged because its documented contract already matches the repaired behavior.
