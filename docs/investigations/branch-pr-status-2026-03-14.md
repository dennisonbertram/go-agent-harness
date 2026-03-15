# Branch and PR Status — 2026-03-14

## Current Branch

**Branch**: `issue-238-reset-context-tool`

Working tree is clean (no staged or modified tracked files). There are numerous untracked files in `docs/` and `docs/plans/`.

### Recent Commits (last 5)

```
61c8b4c feat(#238): agent-controlled context reset with selective persistence
a88a2a6 fix(#231): deep-copy ToolCalls to prevent caller mutation of runner state
04199e9 fix(#231): add ToolCall.Clone() contract; preserve nil semantics in copyMessages
3785500 fix(#231): clone msgs before passing to ConversationStore; fix SummarizeMessages aliasing
457d9a3 fix(#231): harden setMessages, turnMessages ingestion, nil-preserve, Message.Clone()
```

---

## Test Status

**Result: ALL PASS**

All packages under `./internal/...` and `./cmd/...` passed with no failures.

```
ok  go-agent-harness/internal/harness/tools            9.714s
ok  go-agent-harness/internal/harness/tools/core       1.418s
ok  go-agent-harness/internal/harness/tools/deferred   1.297s
ok  go-agent-harness/internal/harness/tools/descriptions 0.686s
ok  go-agent-harness/internal/harness/tools/recipe     1.736s
ok  go-agent-harness/internal/harness/tools/script     6.064s
ok  go-agent-harness/internal/mcp                      1.694s
ok  go-agent-harness/internal/mcpserver                1.510s
ok  go-agent-harness/internal/observationalmemory      1.946s
ok  go-agent-harness/internal/provider/anthropic       1.778s
ok  go-agent-harness/internal/provider/catalog         1.869s
ok  go-agent-harness/internal/provider/openai          1.792s
ok  go-agent-harness/internal/provider/pricing         1.838s
ok  go-agent-harness/internal/quality/coveragegate     1.865s
ok  go-agent-harness/internal/rollout                  1.677s
ok  go-agent-harness/internal/server                   6.320s
ok  go-agent-harness/internal/skills                   1.593s
ok  go-agent-harness/internal/skills/packs             1.424s
ok  go-agent-harness/internal/store                    11.178s
ok  go-agent-harness/internal/symphd                   1.314s
ok  go-agent-harness/internal/systemprompt             1.301s
ok  go-agent-harness/internal/watcher                  1.328s
ok  go-agent-harness/internal/workspace                6.767s
ok  go-agent-harness/cmd/coveragegate                  1.388s
ok  go-agent-harness/cmd/cronctl                       1.279s
ok  go-agent-harness/cmd/cronsd                        2.850s
ok  go-agent-harness/cmd/forensics                     1.485s
ok  go-agent-harness/cmd/harnesscli                    2.212s
ok  go-agent-harness/cmd/harnessd                      2.035s
ok  go-agent-harness/cmd/symphd                        1.366s
```

No failures, no skipped packages.

---

## Open Pull Requests

| PR # | Title | Branch | Opened |
|------|-------|---------|--------|
| 243 | feat(#238): agent-controlled context reset with selective persistence | `issue-238-reset-context-tool` | 2026-03-14 |
| 242 | feat(#236): deterministic config propagation to subagent workspaces | `issue-236-config-propagation` | 2026-03-14 |
| 241 | feat(#234): per-run tool filtering, system prompt, and permissions forwarding | `issue-234-per-run-tool-filtering` | 2026-03-14 |
| 240 | fix(#233): marshal accounting structs to map before inserting into event payloads | `issue-233-deepclonevalue-struct-clone` | 2026-03-14 |
| 239 | fix(#232): CompactRun no longer overwritten by stale execute() messages | `issue-232-fix-compactrun-stale-messages` | 2026-03-14 |

All 5 open PRs were opened today (2026-03-14). PR #243 corresponds to the current branch.
