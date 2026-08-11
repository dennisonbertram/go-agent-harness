# Plan: Issue #1260 resumed TUI live reply

Issue: #1260. Scope is the TUI bridge/reducer boundary only.

1. Record a deterministic red: a delayed conversation callback event followed
   by its old terminal finalizes the fresh local accumulator; the fresh
   same-ID conversation event is rejected and its local copy is deduplicated.
2. Preserve `run_id` on actual terminal bridge messages, and reject a terminal
   whose run owner is not the current run. Keep ownerless bridge lifecycle
   sentinels on their existing reconnect path.
3. Scope terminal finalization and a pre-`RunStartedMsg` assistant accumulator
   by run ID; do not let a prior-run terminal suppress another run's final.
4. Verify same-ID conversation/run copies, conversation copies before and after
   start, terminal-before-final, and late prior-run terminal order. Run focused
   normal/race suites and the repository regression gate.
5. Before merge, obtain a real 100x30 PTY resumed callback conversation showing
   preserved history, a fresh typed message, visible assistant reply, idle
   completion, and matching API transcript.

Rollback: revert this isolated bridge/model/test/docs PR. There is no schema,
data repair, API change, credential change, or deployment migration.
