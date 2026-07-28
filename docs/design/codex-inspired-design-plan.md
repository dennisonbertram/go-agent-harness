# Plan: Codex-inspired look & feel + incremental settings for macapp

Compares the current `macapp/` (SwiftUI, HarnessKit) against the Codex app reference
in `docs/design/codex-app-reference.md`, then lays out two incremental tracks: (A)
visual design system, (B) settings surface parity. Current-state facts below come
from a direct code survey on 2026-07-27 (`macapp/Sources/GoCodeUI/*`,
`docs/design/native-macos-app.md`) — cited inline, not assumed.

## Current state (baseline)

- **Screens**: `AppShell.swift` (icon-only far-left rail, 50pt, no labels — Chat/
  Activity/Sessions/Checkpoints/Settings), `ChatView.swift` (transcript+composer),
  `ActivityView.swift` (tool feed), `SessionsView.swift` (conversation picker +
  `CheckpointsView` rewind cards), `DiffView.swift`, `SettingsView.swift` (4 tabs:
  Providers/Models/Project/Access), `ModelSettingsView.swift`.
- **Theming**: none. No `Theme.swift`/`DesignSystem.swift` exists; every color call
  is a system semantic (`Color.accentColor`, `.secondary`, `.quaternary`) — the app
  purely follows macOS system appearance. Confirmed as the current design doc's
  explicit not-yet-built item ("Theme picker — the app follows system appearance
  today", `native-macos-app.md:124`).
- **Settings**: Providers, Models, Project, Access. No Appearance, no Keyboard
  shortcuts, no Hooks, no Environments, no Git, no Usage.
- **Design doc's own direction** (`native-macos-app.md` §4) already calls for a
  two-pane layout, one-line tool-activity rows, checkpoint cards with inline
  Restore, a status strip, a composer model chip, and (eventually) "a settings
  window, an inspector sidebar, and a command palette" — this plan is compatible
  with that direction, not a replacement for it.

## Track A — Look & feel

Goal: get visually closer to Codex's dark-first, tunable, chip-heavy aesthetic
without a rewrite. Ordered so each phase ships independently and is visible.

**A0. Design-token foundation** (prerequisite for everything else)
Add `macapp/Sources/GoCodeUI/DesignSystem/Theme.swift`: a single `enum Theme` or
`struct ThemeTokens` exposing semantic colors (`background`, `surface`,
`surfaceElevated`, `accent`, `foreground`, `foregroundSecondary`) that today just
forward to the existing system semantics 1:1 — i.e. zero visual change on day one,
but every view stops hardcoding `.secondary`/`.quaternary` directly and reads the
token instead. This is what makes A1–A4 possible without a re-audit each time.

**A1. Accent + dark-first pass**
Codex uses one consistent accent blue (`#339CFF`) across both themes rather than a
per-theme accent. Pick one accent for GoCode (or keep `Color.accentColor` but pin
it via `NSApp.appearance` override for the app's own chrome), apply through the
token layer from A0. No new toggle yet — just a single deliberate look, matching
the "dark-first" note in the reference doc.

**A2. Sidebar upgrade: icon rail → labeled sections**
`AppShell.swift:169-206`'s icon-only rail is the closest analog to Codex's full
sidebar (workspace switcher, New chat, Pinned, Projects-as-folders). Incremental
step: keep the rail's current items but add text labels + section grouping (e.g.
group Sessions/Checkpoints under a "History" label), matching Codex's
category-grouped-by-kind pattern rather than a flat icon strip. Full nested
project/folder tree is a bigger lift — defer to a later phase once/if GoCode's
project model supports multiple open projects per window.

**A3. Chip-style activity rows**
`ActivityView.swift` currently renders tool calls as plain rows. Restyle to rounded
pill/chip elements (matching Codex's "Read files, ran commands" chips) — this is a
pure `ActivityView` restyle, no new data needed, since the tool-call data already
exists.

**A4. Environment/session sidebar card grouping**
Codex's right-sidebar groups by *kind* (Changes / branch / Commit or push /
Subagents / Background processes / Sources). GoCode's equivalent data is scattered
across `ActivityView`, `SessionsView`, `CheckpointsView`. Introduce a single
right-side panel (or a `Checkpoints`+`Activity` tab reorganized into cards) with the
same "Changes / Subagents / Background processes" card taxonomy, backed by data
GoCode already has (rewind points ≈ Changes/checkpoints; tool activity ≈
Subagents/Background processes). This directly reuses A0's tokens.

Concrete reference (observed live in Codex, own screenshot):

```
Environment

Changes
+566 -126,137

Local

promote/opus-5-prod

Commit or push

Promote: Claude Opus 5 default run model + API robustness/perf (dev → main)
```

GoCode's version should extend this taxonomy with cards Codex doesn't have but
GoCode's backend already does: a **Subagents** card (active/queued agent runs — the
harness already models sub-agent workflow steps per `internal/workflow/`), a
**Crons** card (scheduled runs — `CronCreate`/`CronList`/`CronDelete` already exist
as harness tools), and a **Callbacks** card (pending approval/webhook callbacks —
the plan-mode approval broker and `/v1/runs/{id}/approve|deny` already produce this
data). These three map directly to backend capability that already exists per
CLAUDE.md, so — unlike B7/B8 — this isn't gated on new backend design, only on
surfacing existing state in the client. Add this as a first sub-step inside A4:
land the card layout with Changes/Local/branch/Commit-or-push first (data already
in `CheckpointsView`), then add Subagents/Crons/Callbacks cards once each backend
endpoint is confirmed to expose the needed list/status data over HTTP.

**A5. Appearance settings tab**
Once A0–A1 exist, expose them: new `AppearanceTab` in `SettingsView.swift`
alongside the existing 4 tabs — UI font size slider, code font size slider,
reduce-motion toggle, and (if A1 chose a fixed accent) an accent color picker.
This is the first user-facing settings change under Track B, so A5 and B1 should
ship together.

**Explicitly deferred / lower value**: the theme-editor's literal JSON-diff preview
(cute but not functional), translucent-sidebar contrast slider (needs A0+A1 solid
first), and a full System/Light/Dark picker (SwiftUI mostly gets this for free from
system appearance already — only worth it once GoCode has its own non-default
theme to switch *to*).

## Track B — Settings, incremental

Ordered by (a) user value, (b) whether the backend capability already exists per
`CLAUDE.md` / prior harnessd work, so early phases are UI-only wiring and later
phases require backend design first. Items marked **[needs backend check]** are
not confirmed to exist server-side — verify before scoping, don't assume.

**B1. Appearance** (ships with A5) — font sizes, reduce motion, accent. Pure
client-local state, no server dependency.

**B2. Configuration (approval policy + sandbox)**
Backend already exists: `internal/harness/permission_rules.go` and
`internal/harness/plan_mode.go` implement plan-mode gating and permission-rule
matching. Add a `ConfigurationTab` exposing: approval-policy selection and a
read-only view of active permission rules for the current project. This turns an
existing server capability into a settings surface, mirroring Codex's
Configuration page — the highest-value low-risk phase, since no new backend design
is needed, only a client for what's already there.

**B3. Hooks (read-only first)**
Backend already exists: `GET /v1/hooks` lists loaded + skipped hooks
(`docs/design/plugins.md`). Add a read-only `HooksTab` listing name/event/kind/
source/matcher, plus skipped-hook reasons — directly mirrors Codex's Hooks page
("From Config" / "From Projects" grouping) using data the server already computes.
Trust/revoke stays CLI-only for now (`harnesscli hooks trust|revoke|list`), matching
existing design intent that trust management is deliberately offline.

**B4. Usage** (if/when harnessd tracks token usage per project)
**[needs backend check]** — confirm whether run/token accounting already exists
anywhere (cost tracking mentioned in catalog/models.json pricing data) before
committing to this phase. If it exists, a read-only Usage tab (tokens this
session/project, no billing — GoCode isn't a paid product) is a small lift.

**B5. Keyboard shortcuts (documentation, not remapping)**
Lower effort version of Codex's fully-remappable list: a static, searchable
reference of GoCode's existing keyboard shortcuts (whatever's already wired in
`GoCodeApp.swift`/`AppShell.swift`), no remapping capability yet. Remapping is a
bigger lift (needs a shortcuts-persistence layer) — defer.

**B6. Plugins (read-only inventory)**
**[needs backend check]** — `internal/plugins` bundle system exists per CLAUDE.md,
but confirm what's introspectable via HTTP today vs. CLI-only. If there's a routes
gap, this phase is blocked on adding a `GET /v1/plugins`-equivalent before any UI
work — don't build a client for an endpoint that doesn't exist.

**B7. Git**
**[needs backend check, likely not started]** — Codex's Git settings (branch
prefix, force-push policy, draft-PR default, commit/PR instruction injection)
assume the app itself drives git/GitHub operations. Confirm whether harnessd has
any git-authoring capability today (vs. just being *run inside* a git repo). If
not, this is a backend feature request, not a settings-UI task — flag to the user
rather than silently scoping only the UI half.

**B8. Environments / Worktrees**
**Architecture mismatch, not just a missing feature.** Per existing memory,
harnessd is process-level — "no per-run workspace field; one server per project
directory" — whereas Codex's Environments/Worktrees model assumes the app manages
*multiple* per-project worktrees with their own setup/cleanup scripts and lifecycle.
Porting this needs a design decision (does GoCode want multi-worktree-per-project?)
before any settings UI is meaningful. Recommend treating this as an open question
for the user, not a scoped phase, until that's resolved.

## Suggested sequencing

1. A0 → A1 → A5 + B1 (Appearance) — one shippable unit, all client-local, no
   backend risk.
2. A2 (sidebar labels) and A3 (activity chips) — pure visual, can run in parallel,
   independent of B-track.
3. B2 (Configuration) — highest-value settings addition, backend already exists.
4. B3 (Hooks, read-only) — same rationale, backend already exists.
5. A4 (environment/session card sidebar) — visually the biggest lift, do once A0–A3
   are proven out.
6. B4/B5/B6 — smaller, gate each on the "[needs backend check]" verification step
   before scoping UI work.
7. B7/B8 — flag to the user as open architecture questions; do not scope as UI-only
   tasks.

## Non-goals (carried over from existing design doc + this comparison)

Image paste, full plugins/hooks/workflow *editing* UI (browsing/read-only is in
scope per B3/B6, editing is not), a command palette (still on the original
roadmap, orthogonal to this plan), and Codex's literal theme-diff-as-JSON preview
(cute, not functional — explicitly deferred in Track A).
