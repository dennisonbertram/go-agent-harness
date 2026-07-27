# Harness TUI — Exhaustive Feature Inventory

Source: `cmd/harnesscli/tui/` (171 files) and `cmd/harnesscli/` (parent CLI). Compiled as a product spec for a macOS native rewrite. All paths absolute under `/Users/dennison/develop/go-code/`.

---

## 0. CLI Flags & Headless Modes (`cmd/harnesscli/`)

The TUI is one mode of a larger CLI. `main.go: dispatch()` routes `os.Args[1:]`; anything not matching a known subcommand falls through to the default headless run command (`run()`).

### Top-level subcommands (`cmd/harnesscli/auth.go: dispatch()`)

| Subcommand | Purpose | Source File |
|---|---|---|
| *(none)* | Default headless run (see below) or `--tui` to launch TUI | `main.go: run()` |
| `auth login` | Generate/store a local API key in `~/.harness/config.json` | `auth.go: runAuthLogin` |
| `auth codex login\|status\|logout` | Manage Codex subscription credential copy (`~/.harness/subscription-auth/codex.json`) | `auth.go: runAuthCodex` |
| `auth kimi login\|status\|logout` | Manage Kimi Code subscription credential copy | `auth.go: runAuthKimi` |
| `acp` | Launch `harness-acp` stdio Agent Client Protocol entrypoint (editor integration, e.g. Zed) | `acp.go: runACP` |
| `plugin install\|list\|uninstall\|update\|trust\|untrust\|marketplace` | Manage installable plugin bundles under `~/.go-harness/plugins` | `plugins.go: runPlugin` |
| `mcp login <name>\|logout\|status` | OAuth login/logout/status for a remote MCP server | `mcp.go: runMCP` |
| `hooks trust\|revoke\|list` | Manage trust for config-driven lifecycle hook files | `hooks.go: runHooks` |
| `service install\|uninstall\|start\|stop\|status` | Install/manage harnessd as an OS service (launchd on darwin, systemd on linux) | `service.go: runService` |
| `list` / `runs` | List runs, filterable by `-status`, `-conversation-id` | `runctl.go: runList` |
| `cancel <run-id>` | Cancel a run (`POST /v1/runs/{id}/cancel`) | `runctl.go: runCancel` |
| `steer <run-id> <prompt>` | Steer a running run mid-turn from the CLI | `runctl.go: runSteer` |
| `status <run-id>` / `show` (alias) | Show a run's status | `runctl.go: runStatus` |
| `continue <run-id> <prompt>` | Continue a completed run with a new prompt; `-no-stream` to skip event streaming | `runctl.go: runContinue` |
| `replay <rollout-path>` | Replay a prior run; `-mode simulate\|fork`, `-fork-step N`, `-detect-drift` | `runctl.go: runReplay` |
| `search <query>` | Search runs; `-status` filter | `runctl.go: runSearch` |
| `viz` | Print/open the run visualizer URL; `-open` opens default browser | `viz.go: runViz` |
| `improve` | Drive the autoresearch/self-improve loop against a target seam | `improve.go: runImprove` |

All run-control subcommands accept `-base-url` (default `http://localhost:8080`).

### Default headless run flags (`main.go: run()`)

| Flag | Default | Meaning |
|---|---|---|
| `-base-url` | `http://localhost:8080` | Harness API base URL |
| `-prompt` | `""` | Prompt to send (required unless `-tui` or `-list-profiles`) |
| `-model` | `""` | Model override for this run |
| `-system-prompt` | `""` | System prompt override |
| `-agent-intent` | `""` | Startup intent for prompt routing (e.g. `code_review`) |
| `-task-context` | `""` | Harness task context injected into startup prompt |
| `-prompt-profile` | `""` | Prompt profile override for model routing |
| `-prompt-custom` | `""` | Custom prompt extension text |
| `-prompt-behavior` | *(repeatable/CSV)* | Behavior extension IDs |
| `-prompt-talent` | *(repeatable/CSV)* | Talent extension IDs |
| `-workspace` | cwd | Workspace directory for the run |
| `-plan-mode` | `false` | Start the run in enforced read-only plan mode |
| `-tui` | `false` | Launch interactive Bubble Tea TUI (experimental) |
| `-resume` | `""` | Resume an existing conversation by ID in the TUI (implies `-tui`) |
| `-list-profiles` | `false` | List available profiles and exit |

### Headless streaming behavior

- Without `-tui`: POSTs a run, prints `run_id=<id>`, then streams SSE events to stdout line-by-line, prints `terminal_event=<event>` on completion.
- **Blocked-run detection**: if `run.waiting_for_user`, `tool.approval_required`, or `plan.approval_required` arrives while stdin is non-interactive, the CLI does NOT hang — it exits with a diagnostic ("stdin is non-interactive, exiting; server-side run left intact") and a suggested `harnesscli continue <run-id> <prompt>` command.
- **SIGINT/SIGTERM during streaming**: best-effort cancels the still-executing server-side run, prints `run <id> cancelled`, exits `130`.
- **Exit code contract** (`exitcodes.go`, public compatibility surface for scripts/CI, documented in `website/docs/reference/exit-codes.md`):

| Code | Meaning |
|---|---|
| `0` | `run.completed` |
| `1` | Client-side error (bad flags, missing prompt, connection/HTTP/stream failure) |
| `2` | `run.failed` |
| `3` | Blocked (waiting for user/approval, non-interactive stdin) |
| `6` | `run.cancelled` (resumable via `continue`) |
| `130` | Interrupted (SIGINT/SIGTERM), conventional `128+SIGINT` |

- HTTP client tuning: request client has a 60s timeout; the SSE stream client has **no** timeout and disabled idle-connection reaping (tool calls can pause silently for minutes). Response bodies capped at 8MB; a single SSE line capped at 16MB (oversized lines drained and skipped, not fatal).

---

## 1. Slash Commands

All commands are dispatched by `cmd_parser.go` (registry) via `cmd_result.go` (result/effect types) into handlers in `model.go`.

| Command | Args | Behavior | Source File |
|---|---|---|---|
| `/help` | none | Opens Help dialog overlay (tabs: Commands / Keybindings / About) | `model.go`, `components/helpdialog/` |
| `/clear` | none | Clears transcript/viewport, thinking bar, compaction blocks, pending steers; status "Conversation cleared"; keeps conversation ID | `model.go` |
| `/new` | none | Starts a fresh session: clears conversation ID, transcript, viewport, thinking bar, compaction blocks, title | `model.go` |
| `/undo` | `[count]` | No args: opens undo picker overlay (recent-prompt history) to pick how many to drop. `/undo N`: directly `POST /v1/conversations/{id}/undo` to remove last N user prompts. Refuses while a run is active or with no conversation | `model.go`, `api.go`, `components/undopicker/` |
| `/context` | none | Opens context-window token usage grid overlay | `model.go`, `components/contextgrid/` |
| `/export` | none | Exports in-memory transcript to a markdown file (`transcriptexport.NewExporter`); reports written path | `model.go` |
| `/keys` | none | Opens API-keys overlay; fetches provider list; `i` imports/activates a vendor subscription credential; direct key entry via `POST /v1/providers/{provider}/key` | `model.go`, `keys.go`, `api.go` |
| `/model` | none | Opens model-switcher overlay; fetches models/providers; pick model + reasoning effort | `model.go`, `components/modelswitcher/` |
| `/quit` | none | Quits the Bubble Tea program | `model.go` |
| `/stats` | none | Toggles stats overlay (cost/token statistics) | `model.go`, `components/statspanel/` |
| `/cost` | none | Toggles cost overlay (cumulative cost/token usage) | `model.go`, `components/costdisplay/` |
| `/config` | none | Opens read-only config overlay: base_url, model, workspace, max_steps, theme, color_profile, gateway, reasoning_effort | `model.go`, `components/configpanel/` |
| `/subagents` | none | Loads and displays active subagent processes | `model.go` |
| `/tasks` | none | Opens tasks overlay; `GET /v1/tasks` (bash jobs, subagents, cron, callbacks) | `model.go`, `components/taskspanel/` |
| `/hooks` | none | `GET /v1/hooks`; renders loaded + skipped lifecycle hooks as transcript lines | `model.go` |
| `/profiles` | none | Opens profiles overlay; loads run profiles for the next run | `model.go`, `components/profilepicker/` |
| `/theme` | none | Re-scans themes dir; opens theme-picker overlay with active theme marked | `model.go`, `components/themepicker/`, `theme.go`, `themes.go` |
| `/sessions` | none | Opens session-picker overlay (local session store) | `model.go`, `components/sessionpicker/`, `sessionstore.go` |
| `/title` | `[text]` or `clear` | No args: shows current title. `clear`: removes title. `<text>`: sets/persists session title in `sessions.json` | `model.go` |
| `/init` | `[confirm]` | Runs a fixed prompt through the normal run pipeline to generate `AGENTS.md`; refuses to overwrite unless `confirm`; refuses while a run is active | `init_agents.go` |
| `/add-dir` | `[remove] <path>` | No args: lists session-scoped extra directories. `<path>`: attaches a dir to `RunRequest.ExtraDirs`. `remove <path>`: detaches. Not persisted | `add_dir.go` |
| `/feedback` | none | Bundles recent rollout JSONL (newest 5) + redacted config + version info into a local zip under `<config-dir>/feedback/`. Nothing uploaded | `feedback.go` |
| `/rewind` | `<point-id> confirm` | No args: `GET /v1/conversations/{id}/rewind-points`, rendered as a status-bar line (not a picker). With `<point-id> confirm`: `POST /v1/conversations/{id}/rewind` (destructive; literal `confirm` token required; any other 2nd arg rejected) | `model.go`, `api.go` |
| `/fork` | none | `POST /v1/conversations/{id}/fork`; duplicates active conversation, switches into the fork. Requires active conversation | `model.go`, `api.go`, `fork.go` |
| `/search` | `<query>` | Full-text searches current session transcript; opens search overlay | `model.go`, `search.go` |
| `/history` | `<query>` | Searches stored session metadata (last message); opens search overlay with `searchIsHistory=true` | `model.go`, `search.go` |
| `/attach` | none | Status-message hint only: "Attach files by typing @path in your prompt." No overlay/API call | `model.go` |
| `/runs` | none | `GET /v1/runs`; lists recent harness runs | `model.go` |
| `/cancel` | `[run-id]` | `POST /v1/runs/{id}/cancel`; defaults to current active run | `model.go`, `api.go` |
| `/compact` | `[instruction]` | `POST /v1/runs/{id}/compact` (hybrid mode; args become preserve-instruction). Requires active run | `model.go`, `api.go`, `compaction_block.go` |
| `/replay` | `<run-id-or-rollout-path>` | Replays a prior run | `model.go` |
| `/resume` (alias `continue`) | `<run-id> <prompt>` | Continues a completed run: expands `@path` tokens, appends to transcript, `POST /v1/runs/{id}/continue` | `model.go`, `api.go` |
| `/dashboard` | none | Opens multi-run TUI dashboard overlay; loads all runs, starts 2s poll loop | `model.go`, `dashboard.go` |
| `/doctor` | none | Prints a fixed diagnostic hint (`go test ./cmd/harnesscli && bash -n scripts/go-code.sh`); no live checks | `model.go` |
| `/permissions` | none | Opens client-local permissions overlay (session-accumulated tool-permission rules only; no server route backs this) | `model.go`, `components/permissionspanel/` |
| `/workflow` | none / `status <run-id>` / `<name> [json-args]` | No args: `GET /v1/script-workflows`. `status <run-id>`: `GET /v1/script-workflow-runs/{id}`. `<name> [json-args]`: `POST /v1/script-workflows/{name}/runs` | `model.go`, `api.go` |
| `/plugins` | none | Opens plugin-browser overlay listing installed plugin bundles | `model.go`, `plugin_browser.go` |

### Dynamic/extensible commands (names come from user/plugin config, not fixed)

- **Legacy JSON plugins** (`plugin_loader.go: LoadAndRegisterPlugins`) — from `~/.config/harnesscli/plugins`; `handler: bash` (shell command via `plugin/execute*.go`) or `handler: prompt` (expanded prompt template).
- **Installable bundle markdown commands** (`plugin_loader.go: LoadAndRegisterBundleCommands`) — `.md` files under a trusted bundle's `commands/` dir (`~/.go-harness/plugins`, `installable_plugins.go`). Body expands `$ARGUMENTS`, `$0..$n`, `$WORKSPACE`, `$SKILL_DIR`; submitted as a user prompt. Name collisions namespaced as `<bundle>:<name>`.

### Aliases

- `/resume` has alias `continue` (production, `cmd_parser.go`).
- `/q`, `/exit` appear only in a synthetic test fixture (`cmd_parser_test.go`) exercising the generic alias mechanism — **not real product aliases**.

### Not a slash command (parity note)

- Mid-turn steering is **Ctrl+G** only (`POST /v1/runs/{id}/steer`) — there is no `/steer` slash command in the TUI (unlike the headless `harnesscli steer` CLI subcommand).

---

## 2. Keybindings

Primary dispatch in `model.go`'s `Update()`; base bindings defined in `keys.go`. Context precedence for global keys: overlay-specific handler > generic overlay close > run-active handling > idle input handling.

### Global (always available)

| Key | Context | Effect | Source File |
|---|---|---|---|
| Ctrl+C (1st press) | Run active | Shows interrupt confirmation banner; does not cancel yet | `model.go`, `components/interruptui/` |
| Ctrl+C (2nd press) | Run active, banner showing | Confirms interrupt: `POST /v1/runs/{id}/cancel`, closes SSE bridge, `runActive=false` | `model.go` |
| Ctrl+C | Shell command running, no run | Interrupts the shell-mode command | `model.go` |
| Ctrl+C | Idle (no run, no shell cmd) | Hides stale banner, quits the TUI | `model.go` |
| Ctrl+C | Input has text, otherwise idle | Clears input buffer | `components/inputarea/model.go` |
| Esc (1st press) | Interrupt banner visible | Dismisses banner without cancelling; run stays active | `model.go` |
| Esc | Slash-complete dropdown open | Closes only the dropdown, retains typed text | `model.go` |
| Esc | Any overlay open (generic) | Closes that overlay (see per-overlay rows below for special cases) | `model.go` |
| Esc | Run active, no overlay | Cancels the run locally + interrupts active tool call | `model.go` |
| Esc | Shell command running | Interrupts the shell command | `model.go` |
| Esc | Shell mode, input empty | Exits shell mode | `model.go` |
| Esc | Input has text, otherwise idle | Clears input | `model.go` |
| Esc | Fully idle | No-op (does not quit) | `model.go` |
| Enter | Input focused, no overlay/dropdown | Submits input (message / shell command / slash command) | `model.go` |
| Shift+Enter / Ctrl+J | Input focused | Inserts literal newline (multi-line input), no submit | `keys.go`, `components/inputarea/model.go` |
| Ctrl+O | Active tool call present | Expands/collapses that tool call's detail (highest precedence) | `model.go` |
| Ctrl+O | No active tool call, compaction block(s) present | Expands/collapses the most recent compaction block (2nd precedence) | `model.go`, `compaction_block.go` |
| Ctrl+O | Idle (no tool call, no block, no run) | Toggles Plan Mode on/off (3rd precedence, lowest) | `model.go` |
| Ctrl+D | No overlay active | Opens multi-run Dashboard overlay | `model.go`, `keys.go` |
| Ctrl+S | Global | Copies last assistant response to clipboard | `model.go` |
| Ctrl+B | Shell command running | Backgrounds the running shell-mode command | `model.go` |
| Ctrl+B | No shell command running | No-op | `model.go` |
| Ctrl+G | Run active, input non-empty | Sends input as mid-turn steer (`POST /v1/runs/{id}/steer`), clears input, run continues | `model.go` |
| Ctrl+G | No active run | Status hint "No active run to steer"; no network call | `model.go` |
| Ctrl+G | Run active, input empty | Status hint "Type a message to steer into the run" | `model.go` |
| Ctrl+V | No overlay active | Pastes clipboard image as attachment chip (rejects if model is known text-only) | `model.go`, `clipboard_image.go` |
| Ctrl+E | No overlay, `$EDITOR` set | Suspends TUI, opens `$EDITOR` on a temp file with current input, reloads on exit | `model.go` |
| Ctrl+E | No overlay, `$EDITOR` unset | Status "$EDITOR not set" | `model.go` |
| Ctrl+U | No overlay active | Clears input; status "Input cleared" | `model.go` |
| Ctrl+H | Global | Always opens Help overlay | `model.go` |
| `?` | Input empty, no overlay | Opens Help overlay | `model.go` |
| `?` | Input non-empty | Types literal `?` | `model.go` |
| `@` | No overlay active | Inserts `@` (subsequent Tab offers file-path completion) | `model.go` |
| `!` | Input empty, no overlay/shell mode | Enters shell mode (violet border, `!` prompt) | `model.go` |
| `/` | Input box, start of message | Opens slash-command autocomplete dropdown | `model.go` |
| Backspace | Shell mode, input empty | Exits shell mode | `model.go` |
| Backspace | Input box, cursor not at start | Deletes char before cursor | `components/inputarea/model.go` |
| Backspace | Input empty, attachment chip present | Removes most recent image chip | `components/inputarea/model.go` |
| Left/Right | Input box | Moves cursor | `components/inputarea/model.go` |
| Up / Ctrl+P | No overlay/dropdown | Input history: older command | `keys.go`, `model.go` |
| Down / Ctrl+N | No overlay/dropdown | Input history: newer/draft | `keys.go`, `model.go` |
| PgUp / PgDown | Global | Scrolls transcript viewport up/down half a page | `keys.go`, `model.go` |
| Home / End | Global | Scrolls transcript to top/bottom | `keys.go`, `model.go` |
| Tab | Input box, `/`-prefix typed | Autocompletes slash command (single match completes + space; multi-match completes common prefix) | `components/inputarea/model.go` |
| Tab | Input box, `@path` typed | File-path completion (see §6.7) | `filecomplete.go` |
| (reserved, unbound) | Ctrl+R | Comment in `keys.go` reserves this for a future history-search binding | `keys.go` |

### Overlay-specific

| Key | Context | Effect | Source File |
|---|---|---|---|
| Up/Down/`j`/`k` | Any list overlay (model, sessions, profiles, theme, undo, tasks, search, permissions, dashboard, apikeys) | Navigate list selection | `model.go` + respective `components/*` |
| Enter | Overlay list item selected | Confirms selection / drills in (behavior varies per overlay, see §3) | `model.go` |
| Esc | Overlay-specific | Backs out one level, or closes (see §3 for per-overlay nuance) | `model.go` |
| `s` | Model overlay, level 1, no search | Toggles "starred" on highlighted model | `model.go` |
| `/` | Model overlay | Enters cross-provider fuzzy search sub-mode | `model.go` |
| `K` | Model config panel | Enters API-key input mode for that provider | `model.go` |
| `i` | API-keys overlay, subscription-auth provider | Imports vendor CLI subscription credential | `model.go` |
| `r` | Stats overlay | Cycles stats period (Week → Month → Year) | `model.go` |
| `r` | Tasks overlay | Refreshes task list | `model.go` |
| `o`/Enter | Tasks overlay | Opens selected row's output detail | `model.go`, `components/taskspanel/` |
| `x`/Ctrl+K | Tasks overlay | Stops/cancels selected task (cron asks confirm first) | `model.go` |
| `y`/`Y` | Tasks overlay confirm sub-mode | Confirms pending cron delete/cancel | `components/taskspanel/` |
| `n`/`N` | Tasks overlay confirm sub-mode | Cancels the confirm prompt | `components/taskspanel/` |
| `h`/Left | Tasks overlay output-detail sub-mode | Closes detail, returns to list | `components/taskspanel/` |
| `d` | Sessions overlay | Deletes selected session | `components/sessionpicker/` |
| `t`/Enter/Space | Permissions overlay | Toggles selected rule allow/deny | `model.go` |
| `d` | Permissions overlay | Removes selected rule | `model.go` |
| `n` | Dashboard overlay | Enters "new run" input sub-mode | `model.go`, `dashboard.go` |
| `s` | Dashboard overlay, run selected | Enters "steer" input sub-mode | `model.go`, `dashboard.go` |
| `x` | Dashboard overlay, run selected | Cancels the selected run | `model.go` |
| Enter/`p` | Dashboard overlay, run selected | Opens live SSE peek/tail pane | `model.go`, `dashboard.go` |
| `a`/`A`/`y`/`Y`/Enter | Tool-approval / plan-approval overlay | Approves | `approval.go`, `plan_approval.go` |
| `d`/`D`/`n`/`N` | Tool-approval / plan-approval overlay | Denies | `approval.go`, `plan_approval.go` |
| Tab/Right/`l` | Help dialog | Next tab | `model.go` |
| Shift+Tab/Left/`h` | Help dialog | Previous tab | `model.go` |

---

## 3. Overlays / Panels / Modals

`Model.View()` renders exactly one overlay at a time in fixed priority order: `askUser` > `toolApproval` > `planApproval` > generic `activeOverlay` (string enum) > default viewport. Swarm panel and slash-complete dropdown are the two exceptions rendered inline (not part of the priority stack).

| Overlay/Panel | Opened By | Dismissed By | Purpose | Source File |
|---|---|---|---|---|
| Help dialog | `/help`, `?`, Ctrl+H | Esc | Tabbed: Commands / Keybindings / About | `components/helpdialog/` |
| Stats panel | `/stats` | Esc | Cost/token usage over week/month/year | `components/statspanel/` |
| Context grid | `/context` | Esc | Context-window token usage grid | `components/contextgrid/` |
| Cost display overlay | `/cost` (toggle) | `/cost` again or Esc | Running token usage + USD cost | `components/costdisplay/` |
| Config panel | `/config` | Esc | Read-only session config view | `components/configpanel/` |
| Model switcher (2-level) | `/model` | Esc (back one level, then close) | Pick model + provider + reasoning effort; level-2 sub-panel for gateway/key/reasoning | `components/modelswitcher/` |
| Provider/gateway picker | From inside model switcher config panel | Esc (back to model list) | Choose Direct vs OpenRouter gateway | `model.go` (`activeOverlay="provider"`) |
| API keys panel | `/keys`; auto-opened from model switcher if provider unconfigured | Esc | List providers, view availability, type new API key | `model.go` |
| Profile picker | `/profiles` | Esc | Choose agent profile for next run | `components/profilepicker/` |
| Theme picker | `/theme` | Esc | Pick color theme, marks active | `components/themepicker/`, `theme.go`, `themes.go` |
| Session picker | `/sessions` | Esc | Browse/switch/delete past sessions | `components/sessionpicker/` |
| Undo picker | bare `/undo` | Esc, or Enter to confirm | Pick a recent prompt to undo back to | `components/undopicker/` |
| Search/history results | `/search <q>` or `/history <q>` | Esc, or Enter to jump to result | Paginated (20/page) search-result list | `search.go` |
| Permissions panel | `/permissions` | Esc | View/toggle/delete session tool-permission rules | `components/permissionspanel/` |
| Tasks panel | `/tasks` | Esc (Detail/Confirm → List first) | Unified bash jobs / subagents / cron / callbacks list; drill into detail or confirm destructive action | `components/taskspanel/` |
| Plugin browser | `/plugins` | Esc | List installed plugin bundles, toggle enable/disable | `plugin_browser.go` |
| Dashboard | `/dashboard` or Ctrl+D | Esc | Multi-run overview by status; peek/steer/cancel/dispatch without leaving overlay | `dashboard.go` |
| Ask-user-question overlay | Server-driven (`AskUserQuestion` tool call pauses run) | Enter (submit), Esc (dismiss; server times out) | Multiple-choice question(s) mid-run, with countdown | `askuser.go` |
| Tool-approval overlay | Server-driven (`tool.approval_required` SSE) | Approve/deny keys, or Esc (cancels whole run) | Pending tool name + pretty-printed args gate | `approval.go` |
| Plan-approval overlay | Server-driven (plan-mode exit request) | Approve/deny keys | Proposed `.harness/plan.md` content, scrollable, optional selectable "approach" options | `plan_approval.go` |
| Swarm panel (inline, non-modal) | Automatic: `agent_swarm` tool call starts | Freezes automatically on tool completion (not user-dismissed) | Live per-member status block rendered in the transcript | `swarm_panel.go` |
| Slash-command autocomplete dropdown | Typing `/` in input | Esc, or accept via Tab/Enter | Fuzzy-filtered command+skill suggestions above input | `components/slashcomplete/` |
| Interrupt confirmation banner | 1st Ctrl+C during active run | 2nd Ctrl+C confirms, Esc dismisses | Two-stage "press again to interrupt" banner | `components/interruptui/` |

Not overlays (status-message or transcript-line driven instead): `/rewind` list, `/fork`, `/hooks`, `/new`, `/title`, `/attach`, `/runs`, `/cancel`, `/compact`, `/replay`, `/resume`, `/doctor`, `/workflow`, `/export`, `/init`, `/add-dir`, `/feedback`.

---

## 4. Persistent UI Regions

| Region | Displays | Updates On | Source File |
|---|---|---|---|
| Transcript / main viewport | Scrollable log: message bubbles, tool-use cards, compaction blocks, welcome hint when empty | New message, tool call start/delta/result, compaction event, scroll keys, resize, `/clear`, session switch | `components/viewport/`, `model.go` |
| User message bubble | `❯ ` prefix + user text, full-width dark background | User submits input | `components/messagebubble/user.go` |
| Assistant message bubble | `⏺ ` dot + optional title + streamed/markdown-rendered response | Each streaming delta; markdown rendered via glamour at completion | `components/messagebubble/assistant.go` |
| Tool-use card (collapsed) | `⏺` dot (green=running/dim=done), `ToolName(args…)`, duration/`✗` | Tool start/delta/result/error events; re-rendered in place | `components/tooluse/collapsed.go`, `model.go` |
| Tool-use card (expanded) | Full args tree, result body (bash output / unified diff / generic), duration+timestamp footer | Ctrl+O while that tool card is active | `components/tooluse/expanded.go`, `bashoutput.go`, `fileop.go`, `errorview.go` |
| Compaction summary block | Collapsed one-liner, or expanded mode/token-count/summary lines | Manual `/compact` result, or `auto_compact.*` SSE events; Ctrl+O toggles most recent block | `compaction_block.go` |
| Thinking bar | "`{Label}…`" (default "Thinking") above input while model streams reasoning | Thinking-delta arrival; cleared by superseding delta | `components/thinkingbar/` |
| Spinner | Rotating glyph + rotating verb/tool-action label + elapsed duration (after 2s) + "(esc to interrupt)"; then "Worked for Ns" | 120ms tick while run active and thinking bar empty; Start/Stop on run begin/end | `components/spinner/`, `animation.go` |
| Interrupt confirmation banner | Amber "press Ctrl+C again" box, then dim "Stopping…" line | 1st/2nd Ctrl+C, tool finishing | `components/interruptui/` |
| Slash-command autocomplete dropdown | Fuzzy command suggestions, `▶` cursor, scroll indicators | Typing `/`+query; Up/Down; Tab/Enter accept | `components/slashcomplete/` |
| Input area (prompt box) | Multiline editable buffer, `❯` prompt (or `!` violet border in shell mode), cursor, history recall | Every keystroke; Up/Down history; `!` toggles shell mode; Enter submits | `components/inputarea/` |
| Separators | Full-width horizontal rule above thinking/input stack and above status bar | Terminal width change | `components/layout/separators.go` |
| Status bar (bottom) | Priority-ordered segments (dropped low→high when width tight): model → session title → context meter → running indicator → cost → permission-mode tag → git branch → workdir → MCP-failure count | Model switch, `/title`, token/cost update, run start/stop, permission/plan toggle, branch/dir change, MCP failures, resize | `components/statusbar/`, `model.go` |
| Transient status message | One-off message fully replacing the status-bar line for ~3s | Any slash-command feedback/error; auto-clears after `StatusMsgDuration` | `model.go` (`statusMsg` field), `animation.go` |
| Session title (in status bar) | Bold title segment when set via `/title` | `/title <name>` sets/clears; session switch loads stored title | `model.go`, `sessionstore.go` |
| Context/token-usage meter (compact) | `◫ NN%/200K`-style meter, warn-color at ≥80% | Every token-count update | `components/statusbar/` |
| Cost (compact, in status bar) | `$X.XXXX` running cost | Every cumulative-cost update | `components/statusbar/` |
| Welcome/empty-state hint | Centered hint: "Type /model to select a model • Type /help for all commands" | Shown only when no model selected + empty viewport + idle thinking bar | `model.go` |

Note: full-screen versions of cost/context/stats (opened via `/cost`, `/context`, `/stats`) are modal overlays (§3), not persistent chrome — their persistent counterparts are the compact cost/context segments folded into the status bar.

---

## 5. Modes

| Mode | Entered By | Exited By | Behavior While Active | Source File |
|---|---|---|---|---|
| Plan Mode (client toggle) | Ctrl+O while idle (no run, no active tool call, no compaction block) | Ctrl+O again while idle; cleared once a run consumes it | Sets `planMode=true`; status "Plan mode: ON/OFF"; next `POST /v1/runs` sends `plan_mode:true` | `model.go`, `api.go` |
| Plan Mode (server-side run state) | Runner sets `PlanModeActive` when a run starts with `plan_mode:true` | Model calls `plan_exit` tool → `PlanModeExitPending` → `PlanModeInactive` (approved) or back to `PlanModeActive` (denied) | Mutating tool calls fail closed unless the write targets `.harness/plan.md`; exit requires explicit operator approval | `internal/harness/plan_mode.go` |
| Plan-approval overlay | SSE `plan.approval_required` | Approve/deny keys (§2/§3) | Scrollable plan content; optional "approaches" selectable list | `plan_approval.go` |
| Shell Mode | Typing/pasting `!` as first rune on empty input (no overlay active) | Backspace on empty shell input; Esc (1st clears text, 2nd on empty exits); Enter submits command and returns to normal mode | Input border violet/rounded with `!` marker instead of `❯`; typed text is a shell command, not a prompt | `model.go`, `shellmode_*.go` |
| Shell Command Execution (foreground) | Enter in Shell Mode with non-empty command | Process exits; Esc/Ctrl+C interrupts; default 120s timeout auto-kills | Runs `sh -c <command>` locally (never via harnessd bash tool), own process group, output capped 30KB (head+tail) | `shellexec.go`, `shellexec_kill_unix.go`, `shellexec_kill_other.go` |
| Shell Command — Backgrounded | Ctrl+B while shell command running | Runs to completion on its own; single completion card on exit | Detaches from foreground; live streaming stops; input usable immediately; Esc no longer kills it | `shellexec.go` |
| Shell Context Injection (one-shot) | Automatic: any completed (non-interrupted) shell-mode command | Consumed once by next user prompt submission | Wraps last shell command + bounded output (10KB cap) in an XML block prepended to next run prompt; never shown in own bubble | `shellcontext.go` |
| Tool Approval (interactive gate) | SSE `tool.approval_required` | Approve/deny/Esc (cancels whole run) | Blocks normal input routing while active | `approval.go` |
| Ask-User-Question (interactive gate) | Pending question fetched via `GET /v1/runs/{id}/input` | Enter submits, Esc dismisses (server-side deadline still fires) | Option cursor navigation; countdown to deadline | `askuser.go` |
| Server Tool-Approval Mode (run-level policy, not interactively toggled) | Set once at run/config time | Fixed per run/config | `full_auto` (auto-approve tools, plan-exit still gated), `permissions` (rule-gated), `all` (everything gated) | `internal/harness/tools/types.go` |
| Permissions overlay mode | `/permissions` | Esc | Client-local rule browser: toggle/remove rules accumulated this session | `components/permissionspanel/`, `model.go` |
| Model-Switcher Search sub-mode | `/` while model overlay open | Esc clears query then exits | All keystrokes become search query text (including `j`/`k`/`s`) | `model.go` |
| Model Config panel (Level-1) | `K`/Enter on model list within `/model` overlay | Esc closes config panel | Gateway / API-key / reasoning-effort sections; nested Key-Input sub-mode | `model.go` |
| API-Key Input sub-mode | Enter on a non-subscription provider in `/keys` overlay | Enter saves, Esc cancels | Captures raw key string; `i` on subscription providers instead triggers import | `model.go` |
| Dashboard "steer" input sub-mode | `s` on selected run in Dashboard | Enter submits or Esc/close clears | Keystrokes accumulate into dashboard input, shown inline | `dashboard.go` |
| Dashboard "new" input sub-mode | `n` in Dashboard | Enter submits or close clears | Same inline capture but dispatches a brand-new run | `dashboard.go` |
| Mid-turn Steering (action, not persisted mode) | Ctrl+G with active run + non-empty input | Immediate (fires steer POST right away) | Sends input into running turn without cancelling; local echo until `steering.received` confirms | `model.go`, `api.go` |
| External Editor Mode | Ctrl+E, no overlay, `$EDITOR` set | Spawned editor process exits | Suspends TUI via `tea.ExecProcess`; writes input to temp file, execs `$EDITOR`, reloads on exit | `model.go` |
| Tool-Call Expand/Collapse toggle | Ctrl+O with active tool call (highest Ctrl+O priority) | Ctrl+O again | Expands/collapses that tool call's rendered detail | `model.go` |
| Compaction Block Toggle | Ctrl+O, no active tool call, compaction block exists (2nd Ctrl+O priority) | Ctrl+O again | Expands/collapses most recent compaction block | `model.go`, `compaction_block.go` |
| Tasks Panel sub-modes (List/Confirm/Detail) | `/tasks` opens in List; destructive action → Confirm; view output → Detail | Esc in Confirm/Detail backs to List first | Confirm: y/n prompt (cron delete). Detail: scrollable output | `components/taskspanel/` |
| Generic Overlay Mode (umbrella) | Any overlay-opening command/key | Esc (overlay-specific) or completing primary action | While active, most global keys are suppressed/redirected to the overlay's own handler | `model.go` |

---

## 6. Streaming / Interaction Behaviors

### 6.1 Interrupt handling (Ctrl+C)

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | Ctrl+C (`m.keys.Quit`) | `model.go` |
| State machine | Idle → 1st Ctrl+C while run active shows banner (`StateConfirm`), does not cancel → 2nd Ctrl+C (banner visible) cancels & hides banner | `components/interruptui/`, `model.go` |
| 1st press effect | Shows banner; status "Press ctrl+c again to interrupt (esc to keep going)"; `runActive` stays true; cancel func NOT invoked | `model.go` |
| 2nd press effect | Hides banner; `POST /v1/runs/{id}/cancel` (only if RunID set); closes local SSE bridge; `interruptActiveToolCall()`; `runActive=false` | `model.go`, `ctrlc_server_cancel_test.go` |
| Idle Ctrl+C | Hides stale banner, returns `tea.Quit` | `model.go` |
| Shell-mode running command | 1st kills shell command; further Ctrl+C with no shell running reaches run/idle branches | `model.go` |
| Esc while banner visible | Dismisses without cancelling; status "Interrupt cancelled"; run stays active | `model.go`, `interrupt_two_stage_test.go` |
| Restart after Esc | Fresh Ctrl+C re-shows banner (not sticky/one-shot) | `interrupt_two_stage_test.go` |
| Visual states | `StateHidden` / `StateConfirm` (amber box) / `StateWaiting` (dim "Stopping…") / `StateDone` | `components/interruptui/` |
| Server-side guard | No cancel POST issued if `RunID` empty | `ctrlc_server_cancel_test.go` |

### 6.2 Two-stage Escape

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | Esc (`m.keys.Interrupt`) | `model.go` |
| Priority chain (high→low) | dashboard peek close → interrupt-banner dismiss → slash-dropdown close (retains text) → per-overlay back/close → generic overlay close → active-run cancel → non-empty input clear → no-op | `model.go` |
| Overlay vs run precedence | Both present: Esc closes overlay only; run stays active, no cancel | `escape_test.go` |
| Two presses in sequence | 1st: close overlay. 2nd: clear input (run not active) | `escape_test.go` |
| With slash-dropdown open | Closes only dropdown; input text retained; 2nd Esc needed to clear | `escape_test.go` |
| Cancels run directly (no overlay) | If no overlay and run active, Esc calls `cancelRun()` directly — a single-stage path distinct from the Ctrl+C two-stage flow | `escape_test.go` |
| Idle with text | Clears input; status "Input cleared" | `escape_test.go` |
| Fully idle | No-op, does not quit | `escape_test.go` |
| Ask-user/tool-approval/plan-approval overlays | Intercept Esc themselves before the general handler runs | `model.go` |
| Tasks panel sub-modes | Esc in Confirm/Detail backs to List first; further Esc (List) closes overlay | `model.go`, `components/taskspanel/` |

### 6.3 Mid-turn steering (Ctrl+G)

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | Ctrl+G (`m.keys.Steer`) with input text during active run | `model.go`, `steer_key_test.go` |
| Wire contract | `POST /v1/runs/{id}/steer`, JSON `{"prompt": "..."}`; 202 → `SteerAcceptedMsg` | `api.go` |
| Client-side rejection | Empty/whitespace prompt rejected before any HTTP call (`invalid_prompt`) | `api.go` |
| No active run | Status "No active run to steer"; no network call; input preserved | `model.go` |
| On send | Input cleared immediately; local echo appended synchronously (before HTTP round-trip); status "Steering sent"; run stays active | `model.go` |
| Local echo format | Role `"user"`, content `"steered ⟂ " + message + " (pending)"` | `model.go` |
| Server confirmation | `steering.received` SSE event; matching message strips `"(pending)"`; non-matching appends a new distinct marker line | `model.go`, `steer_events_test.go` |
| Failure cleanup | `SteerErrorMsg` removes the pending echo entry entirely; run stays active; status shows mapped error | `model.go` |
| Error → status mapping | `run_not_active`→"run already finished"; `steering_buffer_full`→"try again shortly"; `not_found`→"run not found"; `invalid_prompt`→"prompt is required" | `model.go` |
| Distinguishing marker | Normal Enter-submitted prompts never carry the "steered" marker | `steer_events_test.go` |

### 6.4 Approval prompts (tool permission gate)

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | SSE `tool.approval_required` during active run | `model.go`, `approval_test.go` |
| Payload | `{call_id, tool, arguments, deadline_at}`; args pretty-printed, capped 800 chars | `approval.go` |
| Overlay priority | Checked right after ask-user in the Update switch; takes priority over everything else except ask-user | `model.go` |
| Key routing | `a`/`A`/`y`/`Y`/Enter approve; `d`/`D`/`n`/`N` deny; Esc cancels whole run; other keys swallowed | `approval.go` |
| Approve | `POST /v1/runs/{id}/approve` (optional `{option: id}` for plan-exit); overlay dismissed before HTTP response returns | `approval.go` |
| Deny | `POST /v1/runs/{id}/deny`; same immediate-dismiss pattern | `approval.go` |
| Esc | `POST /v1/runs/{id}/cancel` — cancels whole run, distinct from denying one call | `approval.go` |
| Failure | `ToolApprovalErrorMsg` renders "tool approval: <err>" in status | `model.go` |
| External resolution | `tool.approval_granted`/`tool.approval_denied` SSE (decided by another client) clears a lingering overlay | `model.go` |
| Visual layout | Bordered box: tool name, args, "[a]pprove [d]eny (esc cancels the run)", optional deadline countdown | `approval.go` |
| Related: permissions panel | Separate persistent-rule browser (`/permissions`), not a per-call gate | `components/permissionspanel/` |

### 6.5 Ask-user-question tool UI

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | Server signals pending question; TUI fetches `GET /v1/runs/{id}/input` → `AskUserPendingMsg` | `askuser.go` |
| Question shape | `{Question, Header, Options:[{Label,Description}], MultiSelect}` | `askuser.go` |
| State | `{active, runID, callID, questions, deadlineAt, qIdx, selectedIdx}`; multiple questions handled sequentially via `qIdx` | `askuser.go` |
| Key routing | Up/Down move option cursor (clamped); Enter submits `{question: label}` and dismisses; Esc dismisses without answering | `askuser.go` |
| Multi-select | Not implemented — shows "⚠ [multi-select not supported, selecting one]" warning; only single choice possible | `askuser.go` |
| Submission | `POST /v1/runs/{id}/input` `{"answers": {question: label}}`; failure → status "ask user: <err>" | `askuser.go`, `model.go` |
| Deadline/timeout | `tea.Tick` fires `AskUserTimeoutMsg` at deadline (immediate if already past); dismisses only if RunID+CallID still match; viewport gets "⚠ question timed out" | `askuser.go`, `model.go` |
| Visual layout | Bordered box, question text, option rows with `▶` cursor, footer hints, optional deadline countdown | `askuser.go` |

### 6.6 Diff view

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | Completed tool result that "looks like a unified diff" (`diff --git `, `--- `, `@@ ` prefix match) AND the tool card is Expanded (Ctrl+O) | `components/tooluse/model.go` |
| Non-diff/bash tools | Fall through to generic expanded rendering or `BashOutput` | `components/tooluse/model.go` |
| Rendering | Dashed-border box, header `╌╌╌ path/to/file ╌╌╌`; per-line `+`/`-`/context with line numbers | `components/diffview/view.go` |
| Truncation | `MaxLines` default 40; footer `╌╌╌ [+N more lines] ╌╌╌` | `components/diffview/view.go` |
| Empty diff | Zero lines renders as empty string | `components/diffview/view.go` |
| Theming | `Styles{Add, Remove, Hunk, Border}` threaded from active theme's diff palette | `components/diffview/`, `theme_components.go` |
| Width default | Falls back to 80 columns if unset | `components/diffview/model.go` |

### 6.7 Todo/task panel (`/tasks`)

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | `/tasks` command | `tasks_overlay_814_test.go` |
| Data source | `GET /v1/tasks` unifying bash jobs, subagents, cron jobs, delayed callbacks | `api_tasks_test.go`, `components/taskspanel/` |
| Open→Loading | Resets to loading state, shows "Loading tasks…" until data arrives | `components/taskspanel/model.go` |
| Loaded rows | `TYPE | STATUS | AGE | LABEL` columns; age formatted `5s`/`2m5s`/`1h3m` | `components/taskspanel/view.go` |
| Fetch error | "Failed to load tasks: <err>" | `tasks_overlay_814_test.go` |
| Empty | "No background tasks." | `tasks_overlay_814_test.go` |
| Row actions | `o`/Enter view output (`GET /v1/jobs/{id}/output` or `GET /v1/subagents/{id}`); `x`/Ctrl+K stop (bash_job→kill, subagent→cancel, cron→delete with confirm, callback→cancel); `r` refresh | `model.go`, `api_tasks_test.go` |
| Confirm mode | Cron delete only: "Delete cron job \"X\"? This cannot be undone." y/n | `components/taskspanel/view.go` |
| Detail mode | Scrollable output; Up/Down scroll, `h`/Left/Esc back | `components/taskspanel/view.go` |
| Related: swarm panel | Separate transient inline panel (not `/tasks` overlay): on `agent_swarm` tool start shows launched/completed counts + per-item status; freezes on tool completion | `swarm_panel.go` |

### 6.8 Compaction summary block

| Aspect | Detail | Source File |
|---|---|---|
| Triggers | Manual `/compact [instruction]`, or automatic `auto_compact.started`/`auto_compact.completed` SSE events | `compact_command_test.go`, `model.go` |
| Manual wire contract | `POST /v1/runs/{id}/compact` `{"mode":"hybrid","instruction": args}`; no active run → usage hint, no HTTP call | `compact_command_test.go` |
| Success result | Collapsed block "Compacted context — N messages removed"; expanded reveals mode + summary | `compaction_block_test.go` |
| Server error | No block added; status "compact failed: <err>" | `compact_command_test.go` |
| Auto-compact started | Adds in-progress block "Auto-compacting context — ~N tokens (mode)…"; tracked via `pendingAutoCompactID` | `model.go` |
| Auto-compact completed (matched) | Updates same block: "Auto-compacted context — before → after tokens (mode)"; error case → "Auto-compaction failed" | `model.go` |
| Auto-compact completed (unmatched) | Appends a new completed block if started event was missed (e.g. reconnect) | `model.go` |
| Rendering | Collapsed = `"▸ " + title`; expanded = `"▾ " + title` + wrapped detail lines prefixed `"⎿  "` | `compaction_block.go` |
| Ctrl+O precedence | Active tool call wins first; else most recent compaction block toggles; else (idle) toggles Plan Mode | `model.go`, `compaction_block_test.go` |
| Only latest block toggles | No "toggle/collapse all"; older blocks retain their state | `compaction_block.go` |
| State reset | `clearCompactionBlocks()` on new session / `/clear` / session switch | `compaction_block.go` |

### 6.9 Cost display

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | `/cost` toggles overlay; Esc closes | `cost_config_overlay_test.go` |
| Data shown | Input tokens (↑), output tokens (↓), cumulative USD cost, current model name | `components/costdisplay/view.go` |
| Format | `↑ 1,234 in  ↓ 567 out  $0.0123  [model-name]` | `components/costdisplay/view.go` |
| Data source | `usage.delta` SSE: `cumulative_usage.total_tokens`, `cumulative_cost_usd`; also refreshes on model switch | `cost_config_overlay_test.go` |
| No model | Brackets omitted entirely if `Model==""` | `view.go` |

### 6.10 Model picker

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | `/model` opens at Level-0 (provider list) | `model_command_test.go` |
| Structure | Level-0 providers (OpenAI, Anthropic, Google, DeepSeek, xAI, Groq, Qwen, Kimi, + live OpenRouter/Together); Level-1 models within provider | `components/modelswitcher/model.go` |
| Navigation | Up/Down cursor (wraps); Enter drills in or opens config panel; Esc steps back a level then closes | `model_command_test.go` |
| Search | `/` enters cross-provider fuzzy search: exact > prefix > substring > subsequence fuzzy | `components/modelswitcher/model.go` |
| Starring | `s` toggles starred (floats to top); persisted | `components/modelswitcher/model.go` |
| Live catalog | `GET /v1/models` at startup; falls back to hardcoded `DefaultModels` if offline (marked) | `fetch_models_modalities_internal_test.go` |
| OpenRouter models | Separately fetched from `openrouter.ai/api/v1/models` when OR key configured; slug `provider/model-id` | `openrouter_models_test.go` |
| Availability indicator | `●`/`○` per provider/model by API-key presence; unavailable still selectable, dimmed "(unavailable)" | `components/modelswitcher/model.go` |
| Reasoning effort | Reasoning-capable models show `[R]` badge; config panel offers Default/Low/Medium/High | `model.go` |
| Config panel | 3 sections: Gateway (←/→), API Key (`K` to edit inline), Reasoning Effort | `model.go` |
| Pricing/context window | **Not shown anywhere** — only `modalities` (text/image) is fetched, used solely for the image-paste pre-flight gate | `fetch_models_modalities_internal_test.go` |
| Gateway routing | Separate "Routing Gateway" overlay from the config panel: Direct vs OpenRouter | `model_gateway_test.go` |
| API keys | `/keys` opens separate provider list; inline key entry; subscription-auth providers show "connected" + `i` import | `model_apikeys_test.go` |

### 6.11 Session picker

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | `/sessions` | `model_session_test.go` |
| List source | `sessions.json` (max 100, oldest evicted); fields ID, StartedAt, Model, TurnCount, LastMsg (60-rune truncated), optional Title | `sessionstore.go` |
| Row display | Title (or 8-char ID) · date · model · turn count · last message | `components/sessionpicker/view.go` |
| Navigation | Up/Down or j/k, wraps, scrolls after 10 rows | `components/sessionpicker/model.go` |
| Resume | Enter → `SessionSelectedMsg`; switches conversationID, clears viewport, appends system message naming resumed session | `model_session_test.go` |
| Delete | `d` removes entry immediately, no confirmation | `components/sessionpicker/model.go` |
| Close | Esc closes without changing conversation | `model_session_test.go` |
| Persistence | Atomic write (temp+rename), 0600 perms; corrupt JSON silently resets to empty | `sessionstore.go` |
| `--resume` flow | `TUIConfig.ResumeConversationID` seeds conversationID at startup; fetch errors reported without crash | `resume_conversation_test.go` |

### 6.12 Fork

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | `/fork`, no arguments | `model.go` |
| Precondition | Requires active conversation; else "No active conversation to fork" | `fork_test.go` |
| Behavior | `POST /v1/conversations/{id}/fork` (no body); server duplicates full history under new ID | `api.go` |
| On success | Switches conversationID to fork immediately (transcript unchanged); session-store entry added with `LastMsg: "forked from <src>"`; status "Forked <src> → <new>; you are now in the fork" | `fork_test.go`, `model.go` |
| On failure | Conversation ID unchanged; status "Fork failed: <err>"; no session entry | `fork_test.go` |

### 6.13 Rewind

| Aspect | Detail | Source File |
|---|---|---|
| Trigger (list) | `/rewind` no args → `GET /v1/conversations/{id}/rewind-points` | `rewind_api_test.go` |
| Display | **Not a picker** — rendered as a status-bar line: "Rewind points: p1 (step 2, write), p2 (step 5, edit), ..." | `model.go` |
| Trigger (restore) | `/rewind <point-id> confirm` — literal `confirm` mandatory; missing/wrong 2nd arg rejected client-side, no HTTP call | `rewind_api_test.go` |
| Restore call | `POST /v1/conversations/{id}/rewind` `{"point_id": "..."}` | `api.go` |
| Result | Success: "Rewind complete: N files restored, M messages truncated". Failure (e.g. 409 externally-modified files): "Rewind failed: <error>" | `model.go` |
| Destructiveness gate | No modal confirmation — the `confirm` literal in the command IS the gate | `rewind_test.go` |

### 6.14 Add-dir

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | `/add-dir` (list), `/add-dir <path>` (add), `/add-dir remove <path>` (remove) | `add_dir.go` |
| Path resolution | Relative paths resolve against session workspace; canonicalized via `Abs`+`Clean` | `add_dir.go` |
| Validation | Must exist and be a directory | `add_dir_test.go` |
| Dedup | Adding same resolved path twice → "Already added <path>" | `add_dir_test.go` |
| Collision rule | `remove` only treated as subcommand when followed by a path arg | `add_dir.go` |
| Scope | Session-scoped only, not persisted; affects file-tool confinement (`ExtraDirs`) | `add_dir.go` |
| Wire effect | Marshaled as `extra_dirs` on run-creation request; omitted (not `[]`) when none | `add_dir_internal_test.go` |

### 6.15 File path tab-completion

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | Tab while an `@` token looks path-like (`/`, `~/`, `./`, `../` prefix) | `filecomplete.go` |
| Behavior | Lists matching dir entries; directories get trailing `/`; prefix-filters by partial basename; `~/` tilde-expanded | `filecomplete.go` |
| Limits | Max 20 suggestions; bare `@mention`/`user@example.com` do NOT trigger | `filecomplete_test.go` |
| Slash-command Tab | Separate: Tab on `/xxx` completes unique match + trailing space | `tabcomplete_test.go` |
| Resize survival | Both completion providers survive terminal resize | `tabcomplete_test.go` |

### 6.16 @-mention / file expansion

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | `@path` or `@"quoted path"` in submitted prompt, expanded at submit time | `fileexpand.go` |
| Inserted content | File contents wrapped `<file path="...">CDATA[...]</file>`, replacing the token; path/content escaped against injection | `fileexpand.go` |
| Limits | Max file size 1MB (humanized error); max 10 `@path` tokens per prompt; binary files rejected (null-byte check first 512B); symlinks rejected; non-regular files rejected | `fileexpand.go` |
| Trailing punctuation | Trailing `,.;:!?)` trimmed for prose usage | `fileexpand.go` |
| Non-matches | Email addresses, SSH refs (`git@github.com:org/repo`), bare `@mentions` left untouched | `fileexpand_test.go` |
| Failure UX | Run aborted, status message shown, original input text restored for correction/resubmit | `fileexpand_model_test.go` |

### 6.17 Image paste (Ctrl+V)

| Aspect | Detail | Source File |
|---|---|---|
| Trigger | Ctrl+V, only when no overlay active | `model.go` |
| Pre-flight gate | Checks selected model's fetched `modalities`; rejects immediately if known text-only; unknown/offline models allowed by default (server enforces at send time) | `clipboard_image.go` |
| Platform support | macOS: `osascript` reads `PNGf` clipboard class (hex-encoded AppleScript data, decoded in-process — `pbpaste` can't read image flavors). Linux: `wl-paste` (Wayland, preferred) or `xclip` (X11). Other/headless: unsupported | `clipboard_image.go` |
| Format | PNG only, verified via magic bytes (`\x89PNG\r\n\x1a\n`) | `clipboard_image.go` |
| Storage | `os.MkdirTemp` dir as `clipboard.png`, 0600 perms | `clipboard_image.go` |
| UI feedback | Success: attachment chip `[image #N]` in input area; status "image attached ... backspace on empty input removes it" | `clipboard_paste_internal_test.go` |
| Error feedback | Typed error hints: "no image on clipboard", "unavailable in headless mode", "unsupported on this platform (macOS: osascript; Linux: wl-paste/xclip)" | `clipboard_image.go` |
| Removal | Backspace on empty input removes most recent chip | `clipboard_paste_internal_test.go` |
| Resize survival | Chips persist across `WindowSizeMsg` (input recreated but chips preserved) | `clipboard_paste_internal_test.go` |
| Send path | Each chip's temp file read + base64-encoded as `{"type":"image","media_type":"image/png","data":"..."}` in the run request; temp files deleted after successful submit; unreadable file aborts submit, restores text, keeps chip | `paste_image_send_internal_test.go` |
| No-op | Ctrl+V does nothing while any overlay is open | `clipboard_paste_test.go` |

---

## Gaps flagged for the native rewrite decision

- **No pricing / context-window size** surfaced anywhere in the model picker or config panel — only `modalities` is fetched and used solely for the image-paste text-only gate.
- **Rewind points are plain status-bar text**, not a navigable list/picker — worth deciding whether the native app upgrades this to a real selector.
- **Permissions overlay is client-local only** — there is no server route backing it; it reflects only rules accumulated during the current session.
