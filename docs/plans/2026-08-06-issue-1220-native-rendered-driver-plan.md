# Plan: Issue #1220 owner-created native rendered-driver foundation

## Context

- Governing issue: #1220; parent rendered-matrix issue: #1089.
- Existing #1205 owns a private daemon/app lifecycle and #1208 owns deterministic
  fake-provider fixtures, but neither drives the rendered app or produces proof.
- The inherited worktree contains a useful fail-closed skeleton. Its environment
  TCC attestation and untyped four-file validator are not acceptable evidence.

## Success Contract

One opt-in command may run exactly one deterministic core two-message scenario
only when the current driver process already has both Accessibility and Screen
Recording permission. It starts only owner-created app/daemon children, drives
the owner-created app, and retains a manifest plus screenshot, AX snapshot, raw
conversation SSE, API/store response, and child logs under an owner-created
private artifact root. The manifest binds the nonce, child identities, one
conversation, two runs, artifact hashes, and cleanup result. Any missing,
duplicate, empty, escaped, uncorrelated, or invalid artifact prevents PASS.

## Scope

In scope:

- non-prompting Darwin TCC preflight with an injectable test abstraction;
- lifecycle-owned retained artifact root and explicit cleanup attestation;
- one fixed core scenario using the #1208 fake-provider turns;
- rendered composer continuation plus screenshot/AX capture;
- raw conversation SSE and API/store correlation;
- deterministic proof-manifest validation and operator diagnostics.

Out of scope:

- cron/callback rendered scenarios, the full #1089 matrix, #1010 convergence;
- permission requests, TCC prompt acceptance, user app/daemon discovery or reuse;
- broad process killing, installed-app replacement, credentials, production data;
- product UI redesign or new production API/persistence behavior.

## Ownership and Design

1. `RenderedDriver` performs the permission gate before calling the lifecycle.
   Darwin uses `AXIsProcessTrusted()` and
   `CGPreflightScreenCaptureAccess()` only; it never calls request APIs.
2. `Owner` creates a disposable runtime root and a separate `0700` retained
   artifact root. It records only children it starts and stops only those handles.
3. The owner launches the fake daemon and app with a fixed initial prompt. Once
   healthy, the scenario adapter targets the attested app PID, not a discovered
   GoCode process, and submits the fixed second prompt through the rendered app.
4. The collector polls the private API for the sole conversation and two
   completed runs, captures AX/screenshot, downloads raw conversation SSE and
   messages/runs, writes a proof manifest, and validates it before PASS.
5. Runtime cleanup is always attempted; failure artifacts are retained. Cleanup
   failure is part of the final error and can never be represented as PASS.

## Strict TDD Sequence

1. Red: permission state must distinguish Accessibility and Screen Recording and
   refuse lifecycle start for either missing capability.
2. Red: owner must create distinct private runtime/artifact roots and return
   cleanup evidence without accepting caller-selected resources.
3. Red: validator rejects wrong kind sets, duplicate/symlink/escaped/empty files,
   wrong hashes, missing two-run identity, mixed conversations, and absent
   nonce/prompt/assistant markers.
4. Green: implement the platform-neutral scenario/manifest contract and Darwin
   adapter behind injected seams; provide a non-Darwin unavailable implementation.
5. Green: wire the fixed command and preserve fail-closed operator output.

## Verification

- `TMPDIR=/private/tmp go test ./internal/acceptance/nativegui ./cmd/native-gui-acceptance -count=1`
- focused race tests for owner/driver packages;
- `cd macapp && TMPDIR=/private/tmp swift test`
- `TMPDIR=/private/tmp ./scripts/test-regression.sh` in tmux.
- Opt-in real execution only if the non-prompting probe reports both grants.
  A permissions failure is an expected environmental result, not GUI proof.

## Rollback

Revert the isolated driver commit. It has no schema migration, production data,
service deployment, or user-owned process cleanup. Retained private artifacts may
be removed by their recorded exact path after inspection.
