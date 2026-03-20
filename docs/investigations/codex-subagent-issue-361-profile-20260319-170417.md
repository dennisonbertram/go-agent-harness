## 1) Scope completed
- Investigated issue #361 profile surface and startup behavior relevant to “golden-path” for `--profile full`.
- Mapped `internal/profiles/builtins/full.toml` and its runtime effect through the loader/config stack.
- Identified persistence path behavior and provider/env requirements needed to boot `harnessd` after #362 behavior changes.
- Recommending the narrowest stable smoke-test contract for this profile/path/env combo.

## 2) Files inspected
- [docs/investigations/issue-361-grooming.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/investigations/issue-361-grooming.md)
- [docs/investigations/issue-362-grooming.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/investigations/issue-362-grooming.md)
- [docs/plans/issue-362-plan.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/plans/issue-362-plan.md)
- [docs/runbooks/golden-path-deployment.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/runbooks/golden-path-deployment.md)
- [internal/profiles/builtins/full.toml](/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/builtins/full.toml)
- [internal/profiles/profile.go](/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/profile.go)
- [internal/profiles/loader.go](/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/loader.go)
- [cmd/harnessd/main.go](/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnessd/main.go)
- [internal/config/config.go](/Users/dennisonbertram/Develop/go-agent-harness/internal/config/config.go)
- [internal/store/s3backup/s3backup.go](/Users/dennisonbertram/Develop/go-agent-harness/internal/store/s3backup/s3backup.go)
- [scripts/smoke-test.sh](/Users/dennisonbertram/Develop/go-agent-harness/scripts/smoke-test.sh)

## 3) Concrete findings
1. `internal/profiles/builtins/full.toml` currently enables:
   - `meta`: name `full`, description “Default — all tools available”, version `1`
   - `runner.model = "gpt-4.1-mini"`
   - `runner.max_steps = 30`
   - `runner.max_cost_usd = 2.0`
   - `system_prompt = ""` (empty)
   - `tools.allow = []` (empty)

2. Effect of `tools.allow = []`:
   - In profile merge logic, an empty/nil allow-list means **all built-in tools are available** (not restricted).

3. Profile loading path/surface:
   - `--profile full` is expected from CLI and resolves via `ProfilesDir` (`~/.harness/profiles/<name>.toml`) in `cmd/harnessd/main.go`.
   - Builtin TOML files are embedded in `internal/profiles/builtins`, but startup still relies on file-backed profile resolution in the current load path for `--profile` values.

4. Persistence behavior surfaced by startup config:
   - Memory/state SQLite: from `HARNESS_MEMORY_SQLITE_PATH`, default `.harness/state.db` (joined against `HARNESS_WORKSPACE` when relative).
   - Cron DB: fixed to `HARNESS_WORKSPACE/.harness/cron.db` (always used).
   - Run DB: only if `HARNESS_RUN_DB` is set; relative path is workspace-relative.
   - Conversation DB: only if `HARNESS_CONVERSATION_DB` is set; relative path is workspace-relative.
   - S3 backup is opt-in and requires all of `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `S3_BUCKET` to be present.

5. Minimum env vars to start `harnessd` successfully (post-#362):
   - `HARNESS_MODEL_CATALOG_PATH` is needed for non-OpenAI catalog-based provider bootstrap path.
   - One provider credential is required (example from current path/tests): `ANTHROPIC_API_KEY` is sufficient without `OPENAI_API_KEY` when catalog path is set.
   - `HARNESS_WORKSPACE` is recommended/safe for deterministic persistence roots; defaulting to current working context is otherwise possible.
   - `HARNESS_ADDR` is needed only for client-facing endpoint tests, not for mere process boot.

## 4) Risks/blockers
1. Potential golden-path mismatch: if operations assume “full” comes from embedded builtins directly, they can still fail in practice if `~/.harness/profiles/full.toml` is missing.
2. Docs/scripts may not assert persistence path effects explicitly; current smoke script validates endpoints and provider config but not store-path initialization.
3. `HARNESS_RUN_DB` / `HARNESS_CONVERSATION_DB` are optional by code, so persistence behavior appears only when explicitly set; contracts that assume persistence without setting these env vars are fragile.

## 5) Suggested next step
- Adopt this narrow stable smoke-test contract:
  1. Prepare temp workspace (`HARNESS_WORKSPACE`) and required catalog path.
  2. Start server with:
     - `--profile full`
     - `HARNESS_MODEL_CATALOG_PATH=<path>`
     - `ANTHROPIC_API_KEY=<value>` (or other supported provider key)
     - optional but recommended: explicit `HARNESS_MEMORY_SQLITE_PATH`, `HARNESS_RUN_DB`, `HARNESS_CONVERSATION_DB` under workspace
  3. Assert process is running and:
     - `/health` returns 200
     - `/providers` includes at least one provider
     - `/models` returns non-empty list
     - if persistence is part of contract, write/read an object that requires one of the DB-backed stores and verify file exists.

Task status: DONE