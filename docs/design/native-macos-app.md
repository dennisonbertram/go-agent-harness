# Native macOS app for the harness

Status: in progress. Code lives in `macapp/`.

The terminal UI (`cmd/harnesscli/tui/`) is the product specification. The native
app is not a new product — it is the same harness surfaced natively, and every
TUI capability is tracked to parity in §5.

## 1. Stack decision

**SwiftUI + Swift Package Manager, targeting macOS 14+.**

Three candidates were reviewed:

| Option | Verdict |
|---|---|
| **SwiftUI / AppKit** | **Chosen.** Native text rendering, accessibility, and scrolling for free — all three are load-bearing for an app whose main surface is a long streaming transcript of code. Toolchain already present (Xcode 26.3, Swift 6.2.4). `swift test` runs headlessly, so TDD needs no extra infrastructure. |
| **Vercel Native SDK** (`~/develop/native`) | Rejected. Zig engine with its own renderer and `.native` markup. Interesting, but pre-1.0, brings a second toolchain, and supplies no macOS text/accessibility stack — the exact things this app leans on hardest. No established headless test story. |
| **Osaurus's layout** (`~/develop/fork-osaurus`) | Adopted in part, not wholesale. Its split of "thin app target + fat local SPM package holding all real source" is what makes `swift test` viable, and is copied. Its Xcode-project-per-app-target is deferred: an SPM executable builds and tests entirely from the CLI. An `.xcodeproj` is only needed for entitlements/notarization at ship time. |

### Module layout

```
macapp/
  Sources/HarnessKit/   transport + domain model. No SwiftUI import — stays headlessly testable.
  Sources/GoCodeUI/     SwiftUI views and view models.
  Sources/GoCodeApp/    executable entry point.
  Tests/HarnessKitTests/
  scripts/live-harnessd.sh
```

## 2. Process architecture — one harnessd per project

`harness.RunRequest` gained a per-run `workspace_path` field (issue #1372):
`harnesscli --workspace`/the TUI send it, and the server roots that run's
tools there (validated absolute, existing) instead of the process-level
`HARNESS_WORKSPACE`. `extra_dirs` still grants a run access *beyond* the
effective root (the TUI's `/add-dir`) but does not relocate it.

Consequence at the time this decision was made (pre-#1372): opening a second
project meant a second harnessd, since the workspace root was process-level
only. The app supervises one child process per project window — spawn,
health-check, shut down — the way Osaurus supervises its embedded server. This
was a required epic, not an optimisation. Whether `workspace_path` changes that
architecture (a single harnessd could now serve multiple projects by sending a
different `workspace_path` per run) is a design question this document does not
resolve — flagged here rather than answered, since it is a native-app
architecture decision, not a factual doc/code mismatch.

## 3. Wire contract

Documented in `.native-spec/harness-api.md` (routes, request/response shapes,
scopes) and `.native-spec/tui-inventory.md` (the feature spec). Both are
generated from source, not from memory.

- Transport: JSON over HTTP; `Authorization: Bearer <key>` when auth is enabled.
- Streaming: `GET /v1/runs/{id}/events`, `text/event-stream`, envelope
  `{id, run_id, type, timestamp, payload}`. `id` is `<run id>:<seq>`; resume a
  dropped stream by sending it back as `Last-Event-ID`.
- 79 canonical event types exist (`AllEventTypes()`, `internal/harness/events.go`;
  see `docs/design/event-catalog.md`). `HarnessEventType`
  names the ~42 the UI reacts to and preserves the rest as `.other(name)`, so a
  server that gains new events does not break an older app.

### Verification posture

Stubbed transport proves the client is self-consistent, not that it agrees with
the server. So `HarnessKit` is tested twice:

- Unit tests against a `URLProtocol` stub, plus an SSE parser tested against a
  byte-for-byte capture of a real run stream (`Fixtures/run-toolcall-golden.sse`),
  including a chunk-boundary-invariance property.
- Live tests against a real harnessd (`macapp/scripts/live-harnessd.sh`), driving an
  actual run to completion over SSE. Skipped unless `HARNESS_TEST_BASE_URL` is
  set, so the default suite stays hermetic.

## 4. UI shape

Grounded in current agentic-coding tools (Replit Agent, v0, Base44, Google AI
Studio) via Mobbin. The convergent pattern, and what each maps to here:

| Pattern | Mapped to |
|---|---|
| Two-pane split: agent conversation left (~⅓), work surface right (~⅔) | Transcript + composer / diff + file + output |
| Tool activity collapsed to one-line rows with an icon ("Edited `schema.ts`") | `tool.call.started` / `.completed` |
| Checkpoint card with a **Restore** button inline in the transcript | Rewind points (`/v1/conversations/{id}/rewind-points`) |
| Status strip above the composer ("Paused — agent is waiting for your response") | `run.waiting_for_user`, `tool.approval_required` |
| Model chip inside the composer | `/model` |
| Thin far-left icon rail | Sessions, tasks, dashboard, keys, plugins |

The TUI's overlay-stack model does **not** carry over. Twenty modal overlays is
a terminal constraint; on macOS these become a settings window, an inspector
sidebar, and a command palette.

## 5. Parity checklist

Source: `.native-spec/tui-inventory.md`. Shipped items are covered by tests.

### Foundation
- [x] SSE parsing + event model
- [x] HTTP client: run control, catalog, conversations, tasks
- [x] harnessd process supervisor (one per project)
- [x] App shell: window, two-pane layout, icon rail
- [x] CI: build, test, lint

### Core loop
- [x] Streaming transcript with markdown and fenced code blocks
- [x] Tool activity rows (`tool.call.*`, `tool.output.delta`)
- [x] Composer: submit, multiline, prompt history
- [x] Run status + spinner + two-stage interrupt
- [x] Approval gate (`tool.approval_required`)
- [x] AskUserQuestion UI
- [x] Mid-turn steering
- [x] Cost + token display (never renders unpriced as $0.00)

### Sessions
- [x] Conversation/session picker with search · Fork · Rewind · Undo
- [x] Compaction summary block · Export (Markdown/JSONL)

### Configuration
- [x] Model picker (with price and image support) · API keys + subscription import
- [x] Config view · Profiles · Add-dir

### Advanced
- [x] Plan mode · Diff view · Tasks + todos · Runs list
- [x] `@`-mention file expansion + completion

### Not yet built
- [ ] Image paste (needs `attachments` on the run request and a modality gate)
- [ ] Plugins browser, hooks viewer, script workflows
- [ ] Theme picker (the app follows the system appearance today)
- [ ] Client-local permission rules panel
- [ ] Conversation title editing (no server route exists; the TUI keeps titles
      in a local sessions file)

## 6. Server behaviours the app has to work around

### Installation-relative resources

harnessd resolves its prompt catalog by walking up from its **working
directory**, and its model catalog from `<workspace>/catalog/models.json` or a
cwd-relative `catalog/models.json`. For a supervised server both of those are
the user's project, which contains neither — so the server either exited at
startup or came up with an empty model catalog. The supervisor now pins
`HARNESS_PROMPTS_DIR`, `HARNESS_MODEL_CATALOG_PATH` and
`HARNESS_PRICING_CATALOG_PATH` to the installation root, found by walking up
from the binary.

### Conversation persistence is opt-in

Without `HARNESS_CONVERSATION_DB` every conversation route answers 501, which
silently disables sessions, fork, undo and rewind. The supervisor configures a
per-project store at `<workspace>/.harness/conversations.db`.

### Known server-side issue found while building this

`scripts/run-bench-smoke.sh`, the repo's key-free smoke, fails on `main`.

`HARNESS_PROVIDER=fake` installs the fake provider as the runner's *default*,
but per-run `Runner.resolveProvider` (`internal/harness/runner.go:3152`) prefers
`providerRegistry.GetClientForModel(model)` and only falls back to the default
when that lookup fails. Since the default model resolves to a real catalog
provider, smoke runs hit that provider and fail (observed: `codex-subscription`
HTTP 403). Filed separately.

Workaround used by `macapp/scripts/live-harnessd.sh`: set `HARNESS_MODEL` to a
name no catalog provider serves, forcing resolution to miss and fall back to the
fake provider. Live runs must then send `allow_fallback: true`.
