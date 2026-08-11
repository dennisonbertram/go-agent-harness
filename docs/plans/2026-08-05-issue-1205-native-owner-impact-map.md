# Issue #1205 Impact Map

## Task

- Issue: #1205; plan: `2026-08-05-issue-1205-native-owner-plan.md`; status: implementation.

## Current Ownership, Callers, and Data Flow

- Entry: `scripts/run-native-gui-acceptance.sh` and `cmd/native-gui-acceptance`; new owner replaces caller-controlled inputs with private-root lifecycle data.
- Source evidence: `rg -n "NATIVE_GUI_DRIVER|HARNESS_BASE_URL|pkill|live-test|gui-walk" scripts macapp cmd internal`; #1089 handoff at `bbfcd044`.
- Conclusion: only the acceptance owner may create/attest daemon, app, endpoint, probe, and artifact paths.

## Config, API, CLI, and Tools

- No caller URL, driver, manifest, or app path is accepted. Operator foreground/TCC opt-in is preflight-only; this slice never foregrounds an app.
- No server wire/API change. The owner’s short-lived `127.0.0.1:0` reservation is the deliberately minimal alternative to an inherited FD interface.

## Persistence and Compatibility

- Private `0700` temporary root only; no migrations, settings, or compatibility contract. Attestation is run-local and removed only when owned cleanup succeeds.

## Lifecycle, Security, and Reliability

- Recorded PID/PPID/start identity/executable digest/endpoint bound every cleanup action. Symlinked paths and dirty source reject before effects. Failure cleanup never discovers or kills by name/port.

## Product and Integration Surfaces

- Native: isolated app child is a lifecycle fixture, not a rendered acceptance scenario. TUI/web/API/provider catalogs: None; fake-provider process only. Existing apps/daemons/workspaces remain untouched.

## Deployment and Operations

- No deployment. Diagnostics are private-root provenance and failure cleanup record. Roll back by removing the additive command.

## Regression Tests

- Zero-effect rejects, fixed-probe enforcement, app/daemon PID mismatch, symlink/dirty source, injected failure cleanup, and sentinel PID/health survival.
- Commands: focused package normal/race, Swift test, and `TMPDIR=/private/tmp ./scripts/test-regression.sh`.

## Documentation and Handoff

- Add native acceptance runbook, plan/map, durable logs, and indexes. State explicitly that this is lifecycle infrastructure, not GUI proof.
