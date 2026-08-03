# #1005 impact map

| Surface | Impact |
| --- | --- |
| Ownership/callers | `CallbackManager` remains the sole tool-facing owner; harnessd creates its durable store. |
| Data/persistence | New SQLite callback table holds ID, scope, prompt, due time, state, timestamps, and reserved metadata fields. |
| API/TUI | Existing callback tools and their tenant/conversation filtering remain compatible; no new endpoint. |
| Lifecycle | Persist-before-ack; bind/recover after runner is constructed; shutdown stops timers only. |
| Security | Scope fields are persisted verbatim and no new credentials/network calls occur. |
| Provider/catalog | None; dispatch retry/idempotency intentionally deferred to #1006. |
| Deployment/config | Default relative workspace path `.harness/callbacks.db`; failed open/migration stops startup. |
| Compatibility | Empty/legacy DB migrates automatically; in-memory constructor remains valid for unit callers. |
| Tests/docs | SQLite/store/restart tests plus harnessd wiring tests; server docs record durable default. |
