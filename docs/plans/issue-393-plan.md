# Implementation Plan: Issue #393 — Profile-Backed Subagent Smoke Test

**Date:** 2026-03-20
**Issue:** test(profiles): add a profile-backed subagent smoke and integration suite
**Status:** Planning

---

## Summary

Extend `scripts/smoke-test.sh` (currently steps 1-8) with new steps 9-15 that validate the profile-backed golden path:
1. Profile list discovery
2. Profile detail fetch
3. Run creation with `profile` field
4. Profile-backed run completion
5. Structured completion validation
6. Run readback

Also update `docs/runbooks/golden-path-deployment.md` to document the new steps.

---

## Files to Modify

- `scripts/smoke-test.sh` — Add steps 9-15
- `docs/runbooks/golden-path-deployment.md` — Add profile-backed section

---

## Key API Facts (from research)

### GET /v1/profiles response
```json
{"profiles": [{"name": "full", "description": "Default — all tools available", "model": "gpt-4.1-mini", "allowed_tool_count": 0, "source_tier": "built-in"}], "count": 1}
```

### GET /v1/profiles/{name} response
```json
{"name": "full", "description": "Default — all tools available", "version": 1, "model": "gpt-4.1-mini", "max_steps": 30, "max_cost_usd": 2.0, "source_tier": "built-in", "created_by": "built-in"}
```

### POST /v1/runs request with profile field
```json
{"prompt": "Reply with exactly: PROFILE_TEST_PASS", "model": "gpt-4.1-mini", "profile": "full"}
```
Note: JSON tag is `"profile"`, Go field is `ProfileName` in RunRequest.

### Completed run response
```json
{"id": "run-abc", "status": "completed", "output": "PROFILE_TEST_PASS", "usage_totals": {...}}
```

---

## New Smoke Test Steps

**Step 9:** `GET /v1/profiles` — verify list returns >=1 profile, extract first profile name
**Step 10:** `GET /v1/profiles/full` — verify required fields (name, model, max_steps)
**Step 11:** `POST /v1/runs` with `"profile": "full"` — create profile-backed run, capture run_id
**Step 12:** Poll until run reaches `completed` status (same timeout as step 7)
**Step 13:** Verify `output` field contains expected content
**Step 14:** `GET /v1/runs/{id}` — verify run is readable from store
**Step 15:** Print PASS/FAIL summary for new steps

---

## Risk Areas

1. Profile "full" must exist (it's a built-in — safe)
2. Run completion may take longer when profile is specified — use same timeout
3. No persistence store needed for step 14 (in-memory store is sufficient)
