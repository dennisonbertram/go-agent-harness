# Plan: Issue #1175 bootstrap fixture canonical paths

## Context

- Governing issue: #1175.
- Problem: macOS temporary paths may be spelled `/var/...` while Git reports `/private/var/...`, causing a strict fixture assertion to fail before provenance is checked.
- Scope: test-fixture path identity only; production bootstrap security is unchanged.

## Test Plan

- Red: existing bootstrap provenance tests reproduce the path identity failure on macOS.
- Green: canonicalize the fixture root before deriving expected child paths; preserve all clean-child, fetched-origin, inherited-Git-env, and mismatched-metadata assertions.

## Risks

- Canonicalization could hide a real mismatch. It is limited to the fixture's OS symlink identity; the script still compares Git's registered canonical child worktree exactly.
