1. **Scope completed**
- Audited existing smoke/manual test assets, integration seams, and runbook/docs context relevant to issue #361 and adjacent issue #362.
- Focused on what already exists to support a low-friction live harness smoke path without introducing long-running infra.

2. **Files inspected**
- [docs/investigations/issue-361-grooming.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/investigations/issue-361-grooming.md)
- [docs/investigations/issue-362-grooming.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/investigations/issue-362-grooming.md)
- [scripts/smoke-test.sh](/Users/dennisonbertram/Develop/go-agent-harness/scripts/smoke-test.sh)
- [docs/runbooks/golden-path-deployment.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/runbooks/golden-path-deployment.md)
- [docs/runbooks/terminal-bench-periodic-suite.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/runbooks/terminal-bench-periodic-suite.md)
- [scripts/run-terminal-bench.sh](/Users/dennisonbertram/Develop/go-agent-harness/scripts/run-terminal-bench.sh)
- [docs/testing/harness-smoke-test-2026-03-18.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/testing/harness-smoke-test-2026-03-18.md)
- [docs/testing/harness-smoke-test-post-rename-2026-03-18.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/testing/harness-smoke-test-post-rename-2026-03-18.md)
- [docs/testing/manual-curl-smoke-test-v2.md](/Users/dennisonbertram/Develop/go-agent-harness/docs/testing/manual-curl-smoke-test-v2.md)
- [cmd/harnessd/main.go](/Users/dennisonbertram/Develop/go-agent-harness/cmd/harnessd/main.go)
- [internal/profiles/loader.go](/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/loader.go)
- [internal/profiles/profile.go](/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/profile.go)
- [internal/profiles/builtins/full.toml](/Users/dennisonbertram/Develop/go-agent-harness/internal/profiles/builtins/full.toml)
- [scripts/test-multiturn.sh](/Users/dennisonbertram/Develop/go-agent-harness/scripts/test-multiturn.sh)
- [scripts/curl-run.sh](/Users/dennisonbertram/Develop/go-agent-harness/scripts/curl-run.sh)

3. **Concrete findings**
- Existing closest-to-suite asset is `[scripts/smoke-test.sh]`; it already:
  - starts `harnessd` with profile + ephemeral port
  - waits/readies server
  - validates `/healthz`, `/v1/providers`, `/v1/models`
  - creates a run and polls status
  - validates SSE stream events
- Current status: documented as manual/informational smoke, intentionally not in default regression because it requires live provider credentials.
- `[docs/investigations/issue-361-grooming.md]` confirms intent for “automated regression + live smoke suite,” but marks the current state as blocked by undefined profile selection and dependency on #362/provider setup.
- Deployment seam is strong and reusable:
  - Profile loading is explicit and centralized in `[cmd/harnessd/main.go]` (`--profile` and config precedence).
  - Provider/model seams can be exercised via env/config/model catalog (`HARNESS_MODEL_CATALOG_PATH`, profile definitions), so smoke can stay live without hardcoding provider internals.
  - DB and feature seams are toggleable via env, allowing low-state tests.
- The “canonical” deployment profile is already `full` (golden path docs), with an existing command pattern to pass through to smoke validation (`[docs/runbooks/golden-path-deployment.md]`).
- Heavy `terminal-bench` path exists (`[scripts/run-terminal-bench.sh]`), but it is broader and infra-heavy versus lightweight smoke intent.

4. **Risks/blockers**
- Primary blocker remains #361 scope dependency on #362: credentials/provider availability can make a fully repeatable live suite brittle.
- Current smoke script is manual-oriented and not formally integrated into CI/regression orchestration.
- No single dedicated “smoke-suite package” directory currently exists; behavior is spread across scripts + docs.
- Provider selection ambiguity across environments (especially model/provider mapping in catalog/profile) can create nondeterministic failures unless a fixed test profile is defined.

5. **Suggested next step**
- For new regression coverage, best location is `scripts/smoke-test.sh` as the base entrypoint plus a small companion runner under `scripts/` (e.g., a dedicated `scripts/smoke-live.sh` or `scripts/ci-smoke.sh`) with a short doc in `docs/testing/` or `docs/runbooks/`.
- Also add a dedicated “live-smoke profile” reference in `docs/runbooks/` and keep it explicitly opt-in (`requires HARNESS_SMOKE_LIVE=1` + profile/provider env contract) to avoid long-running infra entanglement.

Task status: DONE