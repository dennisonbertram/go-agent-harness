# Codex app (ChatGPT desktop) — UX & design reference

Source: live observation of the "Codex" surface inside `ChatGPT.app` (macOS desktop,
build `26.727.11326` as of 2026-07-27), documented for `macapp/` design borrowing.
Captured jointly: some via automated screenshot, most via the user clicking through
while dictating on-screen text back verbatim. Not derived from source code — OpenAI's
app is closed-source, so this is observation only, and marked `*` where a section
exists but its contents were never opened.

## 1. Information architecture

### Main window
- **Left sidebar** (workspace switcher "Codex ⌄" + search): New chat, Pull requests*,
  Sites*, Scheduled* (unread-dot indicator), Plugins, then **Pinned** (pinned
  projects/chats) and **Projects** (folders of chat threads, `+` to add, "Show more"
  to expand). Bottom: account row (avatar, name, help).
- **Main panel**: breadcrumb (folder icon + title + `...` menu) → transcript (user
  bubbles, assistant text, collapsible tool/subagent activity chips e.g. "Worked for
  11s ›", named subagent pills like "H3 independent audit — updated") → per-response
  actions (copy, thumbs up/down, branch/share) → live-run affordances only while a
  task is in flight (step progress "Step 4/7 · 1 file changed +122 −0", a "Pursuing
  goal: <text> <elapsed>" banner with edit/pause/delete/expand) → composer.
- **Composer**: placeholder "Do anything", `+` attach, "Approve for me" toggle, a
  model/effort selector (seen both as "Custom Medium ⌄" and "5.6 Sol Extra High ⌄"),
  mic, submit arrow.
- **Right sidebar ("Environment")**: gear icon; Changes (diff stat +/−); Local
  (expandable, contents unopened); git branch row (expandable); Commit or push;
  latest commit/PR line (GitHub-icon link out); Subagents (running/done counts,
  avatar stack); Background processes (live shell jobs, e.g. a `cadical` SAT-solver
  run, "Background terminal"); Sources (linked external refs, `+`, "View all").

### Settings (`Cmd+,`, opens as a standalone "Back to app" page, own search box)

**Personal**
- **General** — Permissions (Default permissions, Auto-review, Full access — each
  with an inline risk-warning sentence + "Learn more" link where relevant); General
  (default file-open destination, language, show-in-menu-bar, bottom-panel toggle,
  default terminal location segmented control, prevent-sleep, Speed, import-from-
  other-AI-apps, open-source licenses); Composer (context-window usage, send
  shortcut, follow-up behavior w/ ⌘⏎ inversion, popout hotkey, default-to-standalone);
  Notifications (completion / permission-needed / question-needed, independently
  toggled).
- **Profile** — read-only stats card: lifetime/peak tokens, longest chat, current/
  longest streak, a token-activity heatmap (Daily/Weekly/Cumulative toggle, GitHub-
  contribution-graph style), activity insights (Fast Mode %, most-used reasoning
  level, skills explored/used, total chats), "most used plugins" leaderboard. Share /
  Private / Edit controls top-right.
- **Appearance** — Theme picker (System/Light/Dark) with a live two-pane JSON-style
  diff preview of the light vs. dark `ThemeConfig` object (literally shows
  `surface`, `accent`, `contrast` as code with red/green diff gutters — a distinctive
  "config as visual proof" pattern). Separate Light/Dark editors: accent/background/
  foreground hex swatches, UI font, code font, translucent-sidebar toggle, contrast
  slider. Also: pointer-cursor-on-hover, dock icon picker, reduce motion, UI/code
  font size in px, diff markers (color vs +/−), font smoothing.
- **Voice** — General (mic source); Voice chat (voice picker, global hotkey, "screen
  context" — inspects foreground app, macOS-permission-gated); Dictation (hold-to-
  dictate hotkey, toggle-dictation hotkey, keep-dictation-bar-visible, custom
  dictionary, recent-dictations recovery list).
- **Configuration** — scoped per-project (dropdown selector), "Open config.toml"
  deep link. Approval policy + Sandbox settings (dropdowns, observed set to "Never
  ask for approval" / "Full access"). Model features (available reasoning-effort
  levels, multi-select; "Ultra in model picker slider" toggle). Workspace
  Dependencies (Codex dependencies toggle, Diagnose action, Reset-and-install
  action, current bundle version footer).
- **Personalization** — Personality tone dropdown ("Friendly") with a caveat banner
  ("not supported by every model"); Custom instructions (free-text, per-host, Save
  button, real example shown: a context7 MCP instruction block); Memory (Enable
  memories, Chronicle research preview — screen-context-augmented, allow-memory-
  from-tool-assisted-chats, Reset memories).
- **Pets*** — not opened (sidebar entry exists; a "Show pet" shortcut exists in
  Keyboard shortcuts, suggesting a virtual-pet/mascot overlay feature).
- **Keyboard shortcuts** — searchable, ~90 remappable actions, roughly grouped:
  navigation/tabs/chats, environment actions (9 numbered generic slots), Git actions
  (commit/push, branch, draft PR, PR, merge, open-on-GitHub), workspace (open
  folder, skills, MCP config, import), composer (attach, reasoning-effort cycle,
  model picker, dictation, voice, send, Fast/Plan mode toggle, Local/Worktree
  toggle), copy actions (Markdown, path, deeplink, session ID, working dir),
  browser controls, voice-chat controls, numbered "go to chat 1–9" slots.
- **Usage & billing** — plan name/price; credits balance + auto-reload; two
  independent weekly usage meters (general vs. a specific model, each with its own
  reset date and "% left"); cancel-plan note pointing to web billing.
- **Account*** — not opened (external-link icon, opens outside the app).

**Integrations**
- **Appshots*** — not opened.
- **Plugins** — tabbed: Plugins (15) / Apps (8) / MCPs (13) / Skills (50) /
  Marketplace (1). MCP tab: server list, each row = name + settings gear + enable
  toggle, some needing "Authenticate"; "+ Add server".
- **Browser** — controls the *built-in* browser (distinct from Chrome, under
  Computer use): control toggle, web/local link-open destinations, clear browsing
  data, annotation screenshots (flagged as usage-increasing), password manager,
  saved contact info, download location + ask-where-to-save, download history, site
  permissions (camera/mic) + per-site overrides, approval-before-opening-sites
  policy, and a flagged "Developer mode" (full CDP access, explicit elevated-risk
  copy).
- **Computer use** — per-app control toggles: "Any App", Chrome (via extension),
  Microsoft Excel (add-in); allow-use-while-locked; always-hide-picture-in-picture;
  "Always-allowed apps" list (e.g. Finder).

**Coding**
- **Hooks** — grouped by source: "From Config" (User config, N hooks), "From
  Projects" (per-project hook counts, with a "needs review" flag state visible).
- **Connections*** — not opened.
- **Git** — Branch prefix (`codex/`); PR merge method dropdown; "Always force push"
  (`--force-with-lease`); "Create draft pull requests" default; Review delivery
  (`/review` in current chat vs. separate review chat); Commit instructions and
  Pull request instructions — free-text blocks injected into ChatGPT's commit-
  message / PR-description generation prompts.
- **Environments** — per-project list ("tells ChatGPT how to set up worktrees for a
  project"), each entry = project name + owner + environment file
  (`environment-N.toml`). Drilling into one project's environment: Setup script (runs
  on worktree creation) + Cleanup script (runs before worktree cleanup), both plain
  shell `cd`-into-worktree-then-run-project-script; **Actions** — named one-off
  commands surfaced in the chat header (e.g. Doctor Verify, Start API, Start Full
  Stack, Status, Typecheck, Test), each just a labeled shell command.
- **Worktrees** — worktree root dir (blank = default); auto-delete-old-worktrees
  toggle (recommended on) + auto-delete limit (count-based, snapshotted/restorable
  before deletion); live list of managed worktrees per project with linked
  conversations shown per worktree (or "No conversations linked").

**Archived**
- **Archived chats** — search box + chats grouped by originating project, each row
  = title + archive timestamp.

## 2. Visual design system

- **Palette**: near-black backgrounds (`#181818` dark bg, `#1A1C1F` dark
  foreground-on-light... — exact tokens only confirmed for the *light* theme editor:
  accent `#339CFF`, background `#FFFFFF`, foreground `#1A1C1F`; dark theme shown with
  the same accent `#339CFF`, background `#181818`, foreground `#FFFFFF`). A single
  blue accent (`#339CFF`) is reused across both themes rather than an accent that
  flips per-theme.
- **Typography**: UI font defaults to the system stack
  (`-apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`); code font is a
  monospace stack (`ui-monospace, "SFMono-Regular", Menlo, Consolas, monospace`).
  Both are user-adjustable in px (UI default 14px, code default 12px) — i.e. font
  size is a first-class preference, not baked in.
- **Contrast/translucency as sliders, not toggles-only**: "Translucent sidebar" is a
  toggle but paired with a numeric contrast slider (45 light / 60 dark) — so the
  sidebar's vibrancy is tunable, not fixed.
- **Dark-first**: the app's default/observed running state throughout this session
  was dark; settings pages, chat panels, and the environment sidebar are all
  near-black with white/gray text and blue accents for links and active states.
- **Diff-as-UI motif**: recurring pattern of showing real diffs/deltas as first-class
  UI, not just in a code viewer — the Changes stat in the Environment sidebar
  (+/− line counts), the theme-editor's literal red/green code diff between
  light and dark configs, and "Diff markers: colors vs +/−" as a user preference.
  Diff/delta framing shows up as a design language, not just a git feature.
- **Chips/pills for background activity**: subagent runs, tool calls, and file
  operations render as small rounded pill/chip elements inline in the transcript
  ("Read files, ran commands", "H3 independent audit — updated") rather than as
  plain log lines — keeps a busy multi-agent transcript scannable.
- **Environment sidebar groups by kind, not recency**: the right-sidebar card
  structure ("Changes / Local / branch / Commit or push / Subagents / Background
  processes / Sources") groups by *kind of thing*, not chronologically — useful
  precedent for macapp's own environment/session sidebar.
- **Live-run chrome is additive, not modal**: progress ("Step 4/7"), the active
  goal banner, and background-process list all appear as extra rows/banners within
  the existing layout rather than as overlays or separate windows — the layout
  doesn't restructure itself just because a run is in flight.
- **Config-scoping is visually explicit**: Configuration and Environments settings
  both start with a project-picker dropdown before showing any fields, so the user
  never has to guess whether a setting is global or per-project — the picker itself
  is the signal.
- **Risk-tiered copy, not just permission toggles**: elevated-risk settings (Full
  access, Full CDP access, Locked use) all pair the toggle with a plain-language
  sentence naming the specific risk (data loss, leaks, sensitive browser internals)
  plus a "Learn more" link — risk framing is written per-setting, not left to a
  generic warning icon.

## 3. Known gaps (not fabricated, not yet observed)

Never opened this session: Pull requests, Sites, Scheduled, Pets, Account, Appshots,
Connections tabs/pages, and the app's own onboarding/first-run flow. UI-automation
(System Events click-through) was unreliable in this environment — clicks
intermittently no-op'd, hung, or (once) landed on an unrelated foreground app — so
remaining coverage requires a human driving the app directly, screenshotting each
page.
