# App Discovery: go-agent-harness CLI/TUI

Generated: 2026-03-23

## Application Type

CLI tool with an interactive terminal TUI (Text User Interface). Two modes:

1. **Streaming CLI mode** (`harnesscli --prompt "..."`) — fires a single run, streams events to stdout as JSON, exits.
2. **Interactive TUI mode** (`harnesscli --tui`) — launches a full-screen BubbleTea interface for multi-turn conversations.

There is also a non-interactive flag `--list-profiles` that lists available profiles and exits.

---

## Tech Stack

- Language: Go
- TUI Framework: BubbleTea (Elm-style architecture, value semantics — all component `Model` types return copies, no mutexes)
- Styling: lipgloss
- Markdown rendering: glamour (via `messagebubble` and `streamrenderer` components)
- Terminal rendering: ansi, termenv
- Server communication: SSE (Server-Sent Events) for streaming run output
- Config persistence: `cmd/harnesscli/config` package (starred models, gateway, API keys)

---

## User Roles

- **Operator**: Runs `harnessd` server, configures profiles (project-level `.harness/profiles/` or user-global `~/.harness/profiles/`), manages API keys in environment.
- **Developer/User**: Uses `harnesscli` TUI or CLI to interact with AI agents, manage runs, select profiles and models.

---

## Feature Map

### TUI Overlays & Panels

The TUI `activeOverlay` field tracks which overlay is open. Valid overlay names and their components:

| Overlay Name | Component | What It Does |
|---|---|---|
| `"help"` | `helpdialog.Model` | Modal dialog showing all slash commands (with descriptions) and keyboard shortcuts, organized in tabs. Navigated with Tab/Shift+Tab or h/l. |
| `"context"` | `contextgrid.Model` | Visual bar showing token count vs. total context window (default 200k tokens) as a percentage progress bar. |
| `"stats"` | `statspanel.Model` | Activity heatmap showing run count and cumulative USD cost over time. Toggle period with `r` (week/month/year). |
| `"model"` | `modelswitcher.Model` | Two-level model browser: Level 0 = provider list (OpenAI, Anthropic, Google, DeepSeek, xAI, Groq, Qwen, Kimi); Level 1 = models for that provider. Type to search. Press `s` to star/unstar. Enter drills in or opens config panel. |
| `"model"` (config panel) | Inline in `model.go` | Per-model config panel (Level 2): gateway selection (Direct vs OpenRouter), API key entry, reasoning effort (for reasoning-capable models). |
| `"apikeys"` | Inline in `model.go` | Provider API key management. List of providers with configured status. Enter enters input mode; type the key value; Enter confirms. Ctrl+U clears input. |
| `"provider"` | Inline in `model.go` | Gateway routing selection: "Direct" (each model's native provider) vs "OpenRouter" (route all via openrouter.ai). |
| `"profiles"` | `profilepicker.Model` | Profile picker list. Profiles come from GET /v1/profiles. Shows name, description, model, tool count, source tier (project/user/built-in). |
| `"subagents"` | Inline (viewport append) | Loads from GET /v1/subagents and appends formatted lines to the viewport. Not a modal overlay — it injects into the chat view. |

Additional always-visible components (not overlays):

| Component | Location | What It Does |
|---|---|---|
| `statusbar.Model` | Bottom bar | Shows active model name and cumulative cost in USD. Displays transient status messages (auto-dismiss after 3 seconds). |
| `thinkingbar.Model` | Above input area | Shows streaming reasoning/thinking text while a run is active. |
| `inputarea.Model` | Bottom input | Multi-line text input with cursor, history (up to 100 entries), tab-completion, and `@` mention support. Shows `❯` prompt symbol. |
| `viewport.Model` | Main scroll area | Scrollable conversation view. Supports `AppendLines`, `AppendChunk`, `ReplaceTailLines` (for live-streaming tool output replacement). |
| `slashcomplete.Model` | Dropdown above input | Autocomplete dropdown that appears when input starts with `/`. Filtered in real time. Tab or Enter accepts suggestion. Up/Down navigates. Auto-executes on single-item accept. |
| `planoverlay.Model` | Full-screen | Plan mode overlay. States: Hidden, Pending (awaiting user approval), Approved, Rejected. Shows plan markdown with scroll support. |
| `permissionprompt.Model` | Modal | Tool permission prompt. Options: "Yes (allow once)", "No (deny)", "Allow all (this session)". Tab-amend mode lets user edit the resource path before confirming. |
| `interruptui.Model` | Banner | Interrupt confirmation banner. States: Hidden, Confirm ("Press Ctrl+C again to stop..."), Waiting ("Stopping... (waiting for current tool to finish)"), Done. |
| `sessionpicker.Model` | Modal list | Past session picker showing session ID, start time, model, turn count, and first 60 chars of last message. |
| `messagebubble.Model` | Inline in viewport | Renders user and assistant messages with role-appropriate styling and glamour markdown. |
| `tooluse.Model` | Inline in viewport | Renders tool call blocks: tool name, status (running/completed/error), args/params, streamed output, duration, expandable full view. |
| `diffview` | Within tooluse | Renders unified diffs with syntax highlighting when tool output is a diff. |
| `transcriptexport` | Background cmd | Exports conversation to timestamped markdown files (e.g. `transcript-20260323-114704.md`). |

---

### TUI Slash Commands

Registered in `cmd/harnesscli/tui/cmd_parser.go` via `builtinCommandEntries()`:

| Command | Description |
|---|---|
| `/clear` | Clear conversation history (resets viewport, transcript, assistant text accumulator) |
| `/context` | View context window usage (opens context grid overlay) |
| `/export` | Export conversation to markdown (writes timestamped file to default export dir) |
| `/help` | Show help dialog (opens help overlay with commands and keybindings tabs) |
| `/keys` | Manage provider API keys (opens apikeys overlay with provider list and key input) |
| `/model` | Select AI model (opens two-level model browser overlay, fetches models from server) |
| `/quit` | Quit the TUI |
| `/stats` | Show cost and token statistics (opens stats panel heatmap overlay) |
| `/subagents` | View active subagent processes (fetches GET /v1/subagents, appends to viewport) |
| `/profiles` | View and select a profile for next run (opens profile picker overlay, fetches GET /v1/profiles) |

Tab-completion is active for all slash commands. The dropdown opens as soon as `/` is typed, filters as you type, and auto-executes when a unique full command name is accepted.

---

### CLI Flags (harnesscli non-TUI mode)

Defined in `cmd/harnesscli/main.go`:

| Flag | Description |
|---|---|
| `--base-url` | harness API base URL (default: `http://localhost:8080`) |
| `--prompt` | Prompt text to send to harness (required in non-TUI mode) |
| `--model` | Model override for this run |
| `--system-prompt` | System prompt override for this run |
| `--agent-intent` | Startup intent for prompt routing (e.g. `code_review`) |
| `--task-context` | Harness task context injected into startup prompt |
| `--prompt-profile` | Prompt profile override for model routing |
| `--prompt-custom` | Custom prompt extension text |
| `--tui` | Launch interactive BubbleTea TUI |
| `--list-profiles` | List available profiles and exit |
| `--prompt-behavior` | Behavior extension IDs (repeatable or comma-separated) |
| `--prompt-talent` | Talent extension IDs (repeatable or comma-separated) |

---

### Keyboard Shortcuts

Defined in `cmd/harnesscli/tui/keys.go` via `DefaultKeyMap()`:

| Key(s) | Action |
|---|---|
| `enter` | Submit message / confirm selection |
| `shift+enter` / `ctrl+j` | Insert newline in input |
| `up` / `ctrl+p` | Scroll up one line (or navigate overlay/dropdown up) |
| `down` / `ctrl+n` | Scroll down one line (or navigate overlay/dropdown down) |
| `pgup` | Page up (half viewport height) |
| `pgdn` | Page down (half viewport height) |
| `/` | Begin slash command (opens autocomplete dropdown) |
| `@` | Mention file |
| `esc` | Multi-priority interrupt: close overlay input mode > close overlay > cancel active run > clear input |
| `ctrl+c` | Quit (if no run active); cancel active run (if run active) |
| `ctrl+s` | Copy last assistant response to clipboard |
| `ctrl+o` | Plan mode toggle / expand active tool call |
| `ctrl+e` | Open editor mode |
| `ctrl+h` / `?` | Open help dialog |

**Overlay-specific keys:**

| Key(s) | Context | Action |
|---|---|---|
| `tab` / `right` / `l` | Help dialog open | Next tab |
| `shift+tab` / `left` / `h` | Help dialog open | Previous tab |
| `r` | Stats overlay open | Toggle period (week/month/year) |
| `j` / `k` / `up` / `down` | Model overlay open (not config panel) | Navigate provider or model list |
| `s` | Model overlay at level 1 or in search | Toggle star on selected model |
| Any printable char | Model overlay open (no slash, not config) | Accumulate into search query |
| `backspace` / `delete` | Model overlay with search active | Delete last search character |
| `j` / `k` | Provider/gateway overlay | Navigate gateway options |
| `j` / `k` / `up` / `down` | API keys overlay | Navigate provider list |
| `enter` | API keys overlay | Enter key input mode / confirm key |
| `ctrl+u` | API key input mode | Clear input |

---

### Interactive UI Elements

1. **Input area** (`inputarea.Model`) — multiline text input with history navigation (up/down keys cycle previous messages), tab-completion for slash commands, `@`-mention for file paths.
2. **Slash command autocomplete dropdown** (`slashcomplete.Model`) — appears on `/`, filters on typing, navigated with up/down, accepted with Enter or Tab.
3. **Viewport** (`viewport.Model`) — scrollable chat area with live-streaming tool output replacement (tail-splice, not append).
4. **Tool call blocks** (`tooluse.Model`) — collapsible/expandable. `ctrl+o` toggles the currently active tool call. Shows running spinner, elapsed time, arg summary, and full output when expanded. Error state shown in red.
5. **Thinking bar** (`thinkingbar.Model`) — shows reasoning text from `assistant.thinking.delta` events above the input area while active.
6. **Status bar** (`statusbar.Model`) — always-visible bottom bar; shows current model and cumulative cost; flashes 3-second status messages.
7. **Model switcher** (`modelswitcher.Model`) — two-level browser. At level 0: provider list. At level 1: model list for that provider. Config panel at level 2 (inline): gateway, API key, reasoning effort. Stars persist to config file.
8. **Profile picker** (`profilepicker.Model`) — scrollable list (max 10 visible rows) of available profiles. Selection takes effect on the next run.
9. **Permission prompt** (`permissionprompt.Model`) — modal with "Yes (allow once)", "No (deny)", "Allow all (this session)". Tab enters amend mode to edit the resource path before confirming.
10. **Plan overlay** (`planoverlay.Model`) — full-screen markdown display of proposed plan. User approves or rejects; emits `PlanApprovedMsg` or `PlanRejectedMsg`.
11. **Interrupt banner** (`interruptui.Model`) — two-stage interrupt confirmation shown when the user presses Ctrl+C during an active run.
12. **Session picker** (`sessionpicker.Model`) — list of past sessions with ID, start time, model, turn count, and last message preview.
13. **Help dialog** (`helpdialog.Model`) — tabbed modal with Commands tab, Keybindings tab, and About tab.
14. **Context grid** (`contextgrid.Model`) — progress bar showing token usage as percentage of context window (default 200k).
15. **Stats panel** (`statspanel.Model`) — activity heatmap with run count and USD cost, toggling between week/month/year periods.
16. **Transcript export** (`transcriptexport`) — background export to timestamped markdown files; status message confirms path.

---

### Server API (what the TUI talks to)

Routes registered in `internal/server/http.go`:

| Endpoint | Method | Purpose |
|---|---|---|
| `/healthz` | GET | Health check |
| `/v1/runs` | POST | Start a new agent run |
| `/v1/runs/{id}` | GET | Get run by ID |
| `/v1/runs/{id}/events` | GET (SSE) | Stream run events |
| `/v1/runs/{id}/approve` | POST | Approve a pending tool action |
| `/v1/runs/{id}/deny` | POST | Deny a pending tool action |
| `/v1/conversations/` | GET | List conversations |
| `/v1/conversations/search` | GET | Search conversations |
| `/v1/conversations/cleanup` | POST | Delete old conversations |
| `/v1/conversations/{id}` | DELETE | Delete conversation |
| `/v1/conversations/{id}/messages` | GET | Get conversation messages |
| `/v1/conversations/{id}/runs` | GET | List runs for a conversation |
| `/v1/conversations/{id}/export` | GET | Export conversation as JSONL |
| `/v1/conversations/{id}/compact` | POST | Compact conversation history |
| `/v1/models` | GET | List available models |
| `/v1/agents` | POST | Invoke agent execution |
| `/v1/subagents` | GET/POST | List or create subagents |
| `/v1/subagents/{id}` | GET/DELETE | Get or delete subagent |
| `/v1/subagents/{id}/wait` | POST | Wait until terminal subagent status |
| `/v1/subagents/{id}/cancel` | POST | Request subagent cancellation |
| `/v1/providers` | GET | List providers with configured status |
| `/v1/providers/{name}/key` | PUT | Set provider API key (admin) |
| `/v1/summarize` | POST | Summarize text |
| `/v1/cron/jobs` | GET/POST | Cron job management |
| `/v1/cron/jobs/{id}` | GET/PATCH/DELETE | Cron job by ID |
| `/v1/cron/jobs/{id}/pause` | POST | Pause cron job |
| `/v1/cron/jobs/{id}/resume` | POST | Resume cron job |
| `/v1/skills` | GET | List skills |
| `/v1/skills/{name}` | GET | Skill by name |
| `/v1/skills/{name}/verify` | POST | Mark skill as verified |
| `/v1/recipes` | GET | List recipes |
| `/v1/search/code` | POST | Code search |
| `/v1/mcp/servers` | GET/POST | List or connect MCP servers |
| `/v1/profiles` | GET | List profiles |
| `/v1/profiles/{name}` | GET/POST/PUT/DELETE | Get/create/update/delete profile |
| `/v1/external/trigger` | POST | External webhook trigger |
| `/v1/webhooks/github` | POST | GitHub webhook |
| `/v1/webhooks/slack` | POST | Slack webhook |
| `/v1/webhooks/linear` | POST | Linear webhook |

The TUI uses: `/v1/runs` (POST), `/v1/runs/{id}/events` (SSE), `/v1/models`, `/v1/providers`, `/v1/providers/{name}` (key set), `/v1/subagents`, `/v1/profiles`.

---

## Navigation Structure

```
TUI Launch (harnesscli --tui)
│
├── Main Chat View (default)
│   ├── Viewport (scrollable conversation history)
│   ├── Thinking bar (above input, visible during active run)
│   ├── Input area (always focused when no overlay open)
│   └── Status bar (always visible at bottom)
│
├── Slash Command Autocomplete Dropdown (when input starts with "/")
│   └── Accept with Enter/Tab → executes command immediately
│
├── Overlays (mutually exclusive, Esc to close)
│   ├── /help → Help dialog (tabs: Commands | Keybindings | About)
│   ├── /context → Context grid (token usage bar)
│   ├── /stats → Stats panel heatmap (period toggle: r)
│   ├── /model → Model switcher
│   │   ├── Level 0: Provider list (j/k to navigate, Enter to drill in)
│   │   ├── Level 1: Model list (j/k, s to star, type to search, Enter for config)
│   │   └── Level 2: Config panel (gateway, API key, reasoning effort)
│   │       └── If provider unconfigured → redirects to /keys overlay
│   ├── /keys → API keys overlay (j/k to select provider, Enter to input key)
│   ├── /profiles → Profile picker (j/k or up/down, Enter to select)
│   └── provider overlay → Gateway selection (Direct vs OpenRouter)
│
└── Modal Interrupts (highest priority, block chat input)
    ├── Permission prompt (pending tool approval)
    ├── Plan overlay (pending plan approval/rejection)
    └── Interrupt banner (Ctrl+C confirmation during active run)
```

Escape priority (highest to lowest):
1. API keys overlay: exit key-input mode first, then close overlay
2. Model overlay: exit key-input mode → exit config panel → clear search → exit to provider list → close overlay
3. Profiles overlay: close picker
4. Any other overlay: close it
5. Active run: cancel run
6. Non-empty input: clear input text
7. Otherwise: no-op

---

## Data Entities

| Entity | Where Managed | Fields |
|---|---|---|
| **Run** | Server-side | RunID, prompt, model, provider, reasoning effort, profile, conversation ID |
| **Conversation** | Multi-turn linkage via `conversationID` | First run ID becomes conversation ID; subsequent turns pass it |
| **Profile** | `/v1/profiles` | Name, description, model, allowed tool count, source tier (project/user/built-in) |
| **Model** | `modelswitcher.DefaultModels` + server fetch | ID, display name, provider, provider label, reasoning mode, availability, starred |
| **Provider** | `/v1/providers` | Name, configured (bool), API key env var name |
| **Gateway** | `gatewayOptions` (local) | ID (`""` = Direct, `"openrouter"` = OpenRouter), label, description |
| **Tool call** | `toolViews` map in `Model` | CallID, tool name, status, args, params, result, error, timer, duration, expanded state |
| **Transcript entry** | `transcript []TranscriptEntry` | Role (user/assistant), content, timestamp |
| **Session** | `sessionpicker.SessionEntry` | ID, started at, model, turn count, last message |
| **Usage/Cost** | Accumulated from `usage.delta` SSE events | Input tokens, output tokens, cost USD (cumulative per session) |
| **API Keys** | Persisted in config file; env vars at startup | Provider key → value map |
| **Starred models** | Persisted in config file | List of model IDs |

---

## Error States

| Error | How It Surfaces |
|---|---|
| Run failed (`run.failed` SSE event) | Error lines appended to viewport; run state cleared |
| SSE stream error | "⚠ stream error: ..." appended to viewport; polling continues |
| Tool call error | Tool block renders in error state with red error text and optional hint |
| Export failed | Status bar: "Export failed" (3-second transient) |
| Copy unavailable | Status bar: "Copy unavailable" (3-second transient) |
| Unknown slash command | Status bar: hint message (3-second transient) |
| Run interrupted by user | Status bar: "Interrupted" (3-second transient) |
| Model provider not configured | Model switcher redirects to /keys overlay with cursor pre-positioned on the relevant provider |
| API key set failed | (Server returns error; no dedicated TUI error state — falls through to status message) |
| Subagents load failed | Status bar: "Load subagents failed: <error>" |
| Profiles load failed | `ProfilesLoadedMsg.Err` is non-nil; picker shows empty state |
| Input cleared via Esc | Status bar: "Input cleared" (3-second transient) |
| Conversation cleared | Status bar: "Conversation cleared" (3-second transient) |

---

## Recommended Story Topics

1. **First Launch & Chat** — Starting the TUI with `harnesscli --tui`, sending the first message, watching streamed assistant response appear in the viewport, seeing cumulative cost update in the status bar.

2. **Tool Execution Flow** — Watching tool call blocks appear (`tool.call.started`), streaming output chunks replace the tail, timer tracking, block transitions to completed/error state, toggling expand with `ctrl+o`.

3. **Permission & Safety Controls** — The permission prompt modal with "Yes (allow once)", "No (deny)", "Allow all (this session)" options, Tab-amend flow to edit the resource path before approving.

4. **Model & Provider Selection** — Opening `/model`, navigating the two-level provider/model browser, starring a model with `s`, entering the config panel, setting reasoning effort, handling an unconfigured provider (redirect to `/keys`).

5. **Profile Selection & Isolation** — Opening `/profiles`, seeing profile entries with name/description/model/tool-count/source-tier, selecting a profile (it applies to the next run only, not persisted), what "source tier" means.

6. **Conversation Management** — Using `/clear` to reset the viewport and transcript, using `/export` to save a timestamped markdown transcript, conversation ID linkage across multi-turn runs.

7. **Planning Mode (Extended Thinking)** — Triggering plan mode via `ctrl+o`, the plan overlay showing pending plan markdown, approving with Enter or rejecting, `PlanApprovedMsg`/`PlanRejectedMsg` flow.

8. **Cost & Context Awareness** — Using `/stats` to view the activity heatmap (toggle week/month/year with `r`), using `/context` to see the token progress bar, status bar cost counter updating on `usage.delta` events.

9. **Slash Commands & Autocomplete** — Typing `/` to trigger the dropdown, filtering by typing, navigating with up/down, accepting with Enter/Tab, the auto-execute behavior when a full command name is the only match.

10. **Error Recovery & Interrupts** — Pressing `ctrl+c` during a run to see the interrupt banner (Confirm → Waiting → Done states), pressing `esc` to cancel instead, run failure rendering in the viewport.

11. **Keyboard-Driven Navigation** — All shortcuts: `ctrl+s` to copy last response, `pgup`/`pgdn` to page through history, `ctrl+p`/`ctrl+n` for line-by-line scroll, vim-style `j`/`k` inside overlays, `ctrl+u` to clear key input.

12. **Session Resumption & Export** — The session picker showing past sessions with start time/model/turn count/last message preview, picking a previous session to resume, `/export` producing a timestamped markdown transcript.
