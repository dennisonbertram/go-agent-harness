# Issue #42 Grooming: conversation-persistence: Add JSONL backup streaming to S3/Elasticsearch

## Summary
Stream conversation data to S3 and/or Elasticsearch as a durable backup with full-text indexing.

## Already Addressed?
**NOT ADDRESSED** — No S3 or Elasticsearch integration exists. No JSONL backup streaming implemented.

## Clarity Assessment
Clear motivation but missing: backup interval/trigger, granularity (per message vs per run), retry policy, metrics, and whether ES is mandatory or optional.

## Acceptance Criteria
Needs clarification on: backup trigger (real-time vs periodic), granularity, ES requirement, retry behavior, metrics.

## Scope
Medium-Large.

## Blockers
**BLOCKED on issue #36** (JSONL export) — the issue explicitly depends on #36 as a prerequisite. Do not start until #36 is closed.

## Effort
**Large** (8-12h after #36 is done) — External dependencies (AWS SDK, ES client), background service, concurrency, security.

## Label Recommendations
Current: `enhancement`. Recommended: `enhancement`, `blocked`, `large`

## Recommendation
**blocked** — Wait for issue #36 to be closed. Then clarify backup trigger, granularity, retry policy before implementation.
