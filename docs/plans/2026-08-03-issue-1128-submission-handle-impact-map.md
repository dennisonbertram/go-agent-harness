# Cross-Surface Impact Map: Issue #1128

## Ownership and Data Flow

- Composer captures `ComposerAction` once: either `.submit` or `.steer(A)`.
- `ProjectSession.submit` returns `RunSession.submit`'s `RunSubmission`; only
  the successful `startRun` response writes its A identity. The per-run stream
  alone reduces the handle transcript and terminal state.
- ToolWalk polls, auto-answers, auto-approves, times out, and judges this
  handle. It returns a displaced result rather than resolving B from shared UI
  state.

## API, Persistence, Compatibility

- No API, wire, schema, or persistence changes. Existing run-specific control
  endpoints remain unchanged.
- Existing callers may discard the newly returned submission value; visible
  Composer and ToolWalk use the ownership-aware path. Rollback is native-only.

## Lifecycle, Clients, and Operations

- A local run records its first timestamped lifecycle frame, allowing a later
  authoritative scheduled continuation to displace it while retaining the
  provisional-replay protection from #1007.
- Reset/load marks an unresolved handle displaced, so a late `startRun`
  response cannot resurrect a torn-down conversation. TUI, harness, providers,
  deployment, cron, and callbacks are unaffected by source search.

## Tests

- Deterministic stubs prove B before response cannot become A; later B causes
  A displacement and zero B cancel; A transcript/terminal excludes B; failure
  and reset do not resurrect A; captured steer never calls submit.
