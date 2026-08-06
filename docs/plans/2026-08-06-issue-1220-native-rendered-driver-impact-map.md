# Cross-Surface Impact Map: Issue #1220 native rendered-driver foundation

## Task

- Task / issue: #1220, parent #1089.
- Plan link: `2026-08-06-issue-1220-native-rendered-driver-plan.md`.
- Owner: native GUI acceptance lane.
- Status: in implementation; no rendered PASS yet.

## Current Ownership, Callers, and Data Flow

- Entry points: `scripts/run-native-gui-acceptance.sh` ->
  `cmd/native-gui-acceptance` -> `nativegui.RenderedDriver` -> #1205 `Owner`.
- Source of truth: the owner attestation supplies runtime/artifact roots, exact
  child PIDs, endpoint, nonce, and probe digest. The proof manifest records live
  run/conversation IDs and typed artifact hashes.
- Consumers: operator diagnostics now; #1089 proof runner later. #1086 inventory
  validation remains separate and is not satisfied by this single scenario.
- Similar abstractions searched: #1205 owner, #1208 scenario manifest,
  `nativegui.Manifest`, `scripts/guidrive.swift`, `gui-walk-one.sh`, ToolWalk,
  HarnessKit live tests, conversation runs/messages/events APIs.
- Search evidence: `rg -n 'AXUI|Accessibility|CGPreflight|screencapture|nativegui|GOCODE_INITIAL_PROMPT|v1/conversations' macapp scripts internal cmd docs`.
- Duplication conclusion: reuse the owner and API routes; replace the shared,
  fixed-coordinate/broad-`pkill` scripts with an attested-PID adapter.

## Config, API, CLI, and Tools

- User-facing config: none. The public command retains only the explicit
  foreground opt-in and accepts no URL, PID, path, manifest, or driver.
- Defaults/fallbacks: permission uncertainty is unavailable; no environment
  variable can attest TCC availability.
- Environment: owner supplies only isolated fake-provider/workspace/base-URL and
  fixed prompt values to its own children.
- API/wire: existing health, conversations, runs, messages, and conversation SSE
  responses are read unchanged. No endpoint or schema changes.
- CLI/tools: one native acceptance command gains real foundation behavior; no
  harness tool catalog or production command changes.
- Errors: explicit accessibility-unavailable, screen-recording-unavailable,
  launch, scenario, capture, correlation, and cleanup failures.

## Persistence and Compatibility

- Persistence: private owner-created workspace/global SQLite only; no migration.
- Artifacts: separate `0700` retained directory, exact regular-file paths,
  SHA-256, byte length, nonce, and run/conversation correlation.
- Compatibility: additive acceptance infrastructure. Existing product stores,
  app paths, HTTP clients, and #1089 v2 evidence schema are unchanged.
- Partial rollout: unsupported/non-Darwin or ungranted hosts fail before spawn.

## Lifecycle, Security, and Reliability

- Ownership: serialized owner creates both children and both roots, records
  handles, and stops only those handles. No discovery, attach, reuse, or broad kill.
- Cancellation: context cancellation propagates through probes/capture; cleanup
  runs on all post-spawn exits with bounded child shutdown.
- Permissions: non-prompting preflight only. No `AXIsProcessTrustedWithOptions`
  prompt dictionary, `CGRequestScreenCaptureAccess`, System Settings automation,
  or prompt clicking.
- Privacy/secrets: fake provider, disposable workspace, generated nonce/prompts,
  no credentials or user data. AX/screenshot scope is the attested app PID/window.
- Failure/recovery: artifacts remain for diagnosis; cleanup failure joins the
  primary error and prevents PASS. Hash/correlation validation is idempotent.

## Product and Integration Surfaces

- Server/runtime: existing fake-provider daemon and read-only API probes.
- Native app: real owner-created app window, composer input, transcript render,
  and same-conversation continuation. No SwiftUI behavior change intended.
- TUI/web: none after repository search; this is native-only acceptance.
- Provider/model/tools: fixed fake model and core `ls`; cron/callback fixtures are
  not executed in this slice.
- External automation: none; no network beyond the private loopback daemon.
- UX/accessibility: driver uses the app's accessibility roles/labels and retains
  AX evidence; it does not rely on fixed screen coordinates.

## Deployment and Operations

- Deployment: opt-in local test infrastructure only; no production rollout.
- Diagnostics: proof manifest, screenshot, AX, raw SSE, API/store JSON, daemon
  log, app log, hashes, child IDs, endpoint, and cleanup status.
- Rollback: revert the PR; remove only the exact retained artifact directory.
- Runbook: document prerequisites, expected fail-closed states, invocation,
  artifact interpretation, and explicit non-claims.

## Regression Tests

- First red: required-TCC state currently trusts `HARNESS_NATIVE_TCC_STATE` and
  a four-file validator accepts unrelated content without typed correlation.
- Acceptance tests: admission-before-effects, owner root separation, exact child
  cleanup, one core two-message contract, manifest correlation and hashes.
- Negative/security: prompt-required/unavailable, unknown state, symlink/traversal,
  duplicate/empty/wrong-hash/wrong-kind, mixed IDs, missing rendered markers.
- Integration: injected platform/API/capture adapter drives the complete contract;
  opt-in real macOS execution only with existing grants.
- Cross-surface: nativegui and command normal/race, Swift tests, full regression.

## Documentation and Handoff

- Before code: issue comment, this plan, impact map, plan index, intent log.
- After green: native runbook plus engineering/observational/system logs and indexes.
- Public docs/release notes: none; this is acceptance infrastructure, not product functionality.
