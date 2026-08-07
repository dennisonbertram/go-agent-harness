# Plan: Issue #1243 raw SSE header-to-JSON integrity proof

## Context

- Governing GitHub issue: #1243; stacked on #1232 exact head
  `7618cba395cec57146317a989eebfd586fb871ac`.
- Problem: the #1231 acceptance decoder took JSON event identity as sufficient
  and could silently replace an absent JSON ID with the SSE `id:` header. It
  also discarded the `event:` header, so the retained transcript did not prove
  that HTTP SSE framing and JSON envelopes described the same event.
- User impact: an API acceptance PASS must be evidence of the actual event
  stream, not of a permissive test-only reconstruction.

## Scope

- In scope: test-only decoder and proof validation in
  `cmd/harnessd/filesystem_git_api_acceptance_test.go`, deterministic header
  integrity regressions, and the required plan/map/log/index records.
- Out of scope: `internal/server.writeSSE`, endpoint behavior, tool behavior,
  TUI/GUI, scheduler/callback work, or changes to #1232 itself.

## Test-first contract

- First red: `TestFilesystemGitSSEDecoderRejectsUnboundHeaders` calls the new
  decoder contract and fails because the existing decoder loses framing
  provenance.
- Every data-bearing frame must have nonempty `id:` and `event:` headers and a
  JSON event with nonempty `id` and `type`; header ID must equal JSON ID and
  header event type must equal JSON type. A header must never synthesize JSON
  identity.
- Comment-only keepalive pings produce no frame. Valid multi-`data:` payloads
  and concurrent tool lifecycle ordering remain accepted.
- Green gates: focused normal/race `cmd/harnessd`, direct #1231 real daemon
  acceptance, then external-cache `./scripts/test-regression.sh`.

## Implementation checklist

- [x] Verify base head, owner, current decoder and server frame contract.
- [x] Write plan and impact map before implementation.
- [x] Add deterministic red header-integrity tests.
- [x] Preserve header provenance in a test-only decoded-frame type.
- [x] Make the lifecycle validator require provenance/JSON equality.
- [x] Verify focused normal/race, real daemon, and full regression.
- [x] Update logs/indexes, commit, push, and open stacked PR #1244 with
  `Closes #1243`; independent exact-head review remains required.

## Evidence

- Red: `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1243-red go test
  ./cmd/harnessd -run '^TestFilesystemGitSSEDecoderRejectsUnboundHeaders$'
  -count=1` failed because `decodeFilesystemGitSSEFrames` did not exist.
- Focused green: normal and race commands selected the decoder, lifecycle
  table, and real `TestIssue1231FilesystemAndGitToolsUseOneDurableConversation`
  one-daemon/four-turn acceptance; both passed.
- Full green: `TMPDIR=/private/tmp GOCACHE=/private/tmp/gocode-1243-full-cache
  ./scripts/test-regression.sh` passed normal, race, coverage 85.1%, and zero
  uncovered functions. Retained log: `/private/tmp/gocode-1243-full.log`.

## Rollout and rollback

- No deployed artifact, schema, API, or migration changes. Rollback is one
  test/docs commit revert; retained private evidence remains historical.
