# Native GUI Acceptance

Issue #1089 validates actual rendered macOS evidence; `ToolWalk`, reducer
tests, an HTTP acknowledgement, and assistant prose are insufficient.

Run only with an explicitly owned, isolated app bundle, daemon, port,
workspace, and artifact directory. Do not reuse, kill, or attach to an existing
GoCode or harnessd process. A foreground interaction requires the operator's
approval before it is attempted.

The external driver must produce screenshot, AX/OCR snapshot, raw SSE, and
API/store probe artifacts plus a #1086 suite manifest containing ordered
messages/actions, run/conversation/event IDs, independent postconditions,
cleanup, and redaction declarations. Then invoke:

```bash
NATIVE_GUI_DRIVER="$PWD/path/to/tracked-native-gui-driver" \
HARNESS_BASE_URL=http://127.0.0.1:PORT \
./scripts/run-native-gui-acceptance.sh
```

The launcher creates the nonce, temporary collection root, artifact root, and
manifest path. It accepts only an executable tracked under this repository,
exports those values as `NATIVE_GUI_COLLECTION_*`, and the driver must copy
them into `collection` in its manifest along with its exact app build SHA and
child daemon PID, loopback port, and URL. The manifest must bind each evidence
row to that app/daemon identity and state verified cleanup. Do not supply or
reuse an artifact root or manifest path from an older run.

The validator recomputes SHA-256 values from canonical regular files and
rejects symlink escapes, empty/partial/all-failed/duplicate case evidence,
missing collection provenance, arbitrary drivers, non-loopback URLs, resolver
drift, or missing cleanup. A qualifying manifest has exactly one final PASS for
every applicable native case; generic cross-surface reports retain their own
history semantics. Native composer has no slash parser: each terminal-only
`tui_command` must therefore have a hash-bound N/A mapping with source
reference and UX rationale, never fabricated GUI execution.
