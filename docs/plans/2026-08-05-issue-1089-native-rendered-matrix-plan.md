# Plan: Issue #1089 rendered native acceptance

## Context

- Governing issue: #1089.
- Problem: ToolWalk and reducer tests are not rendered installed-app proof.
- Constraints: exact `93bfc883` base, isolated app/daemon/port/workspace only; no existing process ownership; terminal slash commands are N/A because the native composer has no slash UI.

## Scope

- In scope: hash-bound native overlay/cases/evidence validation, artifact digest checks, and a driver launcher that requires actual AX/OCR/screenshot collection.
- Out of scope: UI redesign, slash parsing, #1010 cron/callback repair, or any product defect discovered by a driver.

## Test Plan (TDD)

- Red: reject empty, partial, all-failed, and duplicate-pass proof manifests;
  reject a rendered artifact that is tampered with or escapes through a symlink;
  reject an arbitrary driver or a non-loopback daemon URL.
- Green: require exactly one final native PASS for every applicable #1086 case;
  validate canonical regular artifacts, digest-bound repository driver, and
  launcher-owned nonce/temp-root/app-build/child-daemon provenance.
- Full: focused/race Go package, `cd macapp && swift test`, and `./scripts/test-regression.sh`.

## Impact Map

See `2026-08-05-issue-1089-native-rendered-matrix-impact-map.md`.

## Rollback

This is additive acceptance tooling. Revert the runner/launcher; preserve failed
proof packs and create a separate defect issue for product failures. Do not
weaken generic cross-surface report history to satisfy the native qualifying
manifest's exact-one-PASS contract.
