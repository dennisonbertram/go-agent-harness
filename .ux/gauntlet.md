# Codex look-and-feel gauntlet — live progress

**Goal:** make `macapp/` indistinguishable from Codex in spirit — neutral dark
palette, spacing rhythm, sidebar, composer, environment-panel conventions.

**Bar:** the real Codex surface in `ChatGPT.app`, screenshotted and measured.
Live numbers outrank anything written in prose, including our own design docs.

**Rule:** code quality counts as winning. Duplicated styling or a one-off magic
number is a finding of the same weight as a pixel mismatch.

---

## Decomposition

Five pieces, each judged independently, each with a builder and a separate
fresh-context critic:

| # | Piece | State |
|---|---|---|
| 0 | Token layer — spacing, typography, radius, icon scales | shipped |
| 1 | Colour ramp & contrast | **closed** — every surface an exact RGB match (critic-verified) |
| 2 | Sidebar | rebuilt as a conversation list in r7 |
| 3 | Composer | r6–r7: hugs content, even gutters, radius 20pt, no hairline |
| 4 | Environment / status panel | shipped; footprint still source-verified only |
| 5 | Transcript rhythm & type hierarchy | r7: bound to the scale, pitch 16.0 → 25.0pt |

---

## Round 0 — what already shipped

Measured against Codex, merged before this loop began:

| Gap | Before | After | Codex |
|---|---|---|---|
| Palette | 13 grey levels, warm-tinted | **27 levels (24→51), neutral** | 27 levels (24→51) |
| Foreground ramp | 2 levels | **4 levels** | 5 levels |
| Composer | 520×34 field + separate strip, 0pt inset | **898×68 single card, 19pt inset** | 883×118 card, 19pt inset |
| Inspector | permanent, 44.1% of window | **collapsed by default** | floating, 4.6% |
| Sidebar | 50pt icon-only, SF Symbol names as a11y labels | **220pt labelled + sectioned** | 348pt labelled |

---

## Round 1 — measured result

Critic recaptured **both** apps today (Codex was still running, so it was
re-shot rather than reused) and measured GoCode at two window sizes.

### Closed since round 0

| Gap | Evidence |
|---|---|
| **Palette** (was #1) | GoCode's three opaque surfaces `#181818` / `#222222` / `#2D2D2D` are **byte-identical** to three of Codex's five. Span 13 → 21 levels, foreground rungs 2 → 5, warm tint gone from every surface but one. |
| **Composer structure** (was #2) | One container, 18.0 pt bottom inset (was 0), symmetric 16 pt insets, send inside the card. Only its scale is still wrong. |
| **Sidebar** (was #5) | Labels, `HISTORY` header, 28.0 pt indent (Codex 28.5), 37 pt pitch, full-row pill, and real English accessibility names — the SF-Symbol-identifier defect is gone. |
| **Left edges** (runner-up) | Status text 237.5 and composer 237.0 now agree to 0.5 pt. |

### Top 5 remaining

| # | Gap | GoCode | Codex |
|---|---|---|---|
| 1 | Composer half-height; send disc half-size | 68.5 pt tall, 16.5 pt disc | 117 pt, 33.5 pt |
| 2 | Toolbar is the last non-token surface **and the only warm one** | `#2B2A28`, R>B by 2–3, ~19 levels above page | no chrome band at all — page runs to y=0 |
| 3 | No content max-width | composer 1276.5 pt at a 1530 pt window (97.5%) | holds 883.0 pt, confirmed 3 ways |
| 4 | Nav labels one rung too dim; no true white | `#A3A3A3` 6.31:1; primary 13.32:1 | `#DEDEDE` 11.83:1; primary 17.76:1 |
| 5 | Inspector fixed full-height split | 380 pt = 33.6% when open | 361 × 238.5 content-sized card, 4.6% |

Gaps 1–4 are small precise numbers, which is why round 2 takes all four at once.

### What the critic could not measure, and said so

GoCode's transcript had no content and could not be given any — the send button
reports `enabled: false` even after AX text injection, because the written value
renders without driving the SwiftUI state. So message bubbles, tool rows,
per-message actions and internal left edges are **unverified on this build**. It
declined to carry the old build's numbers forward for them, which is the right
call.

### A doc our own repo got wrong

`docs/design/codex-app-reference.md` §2 describes Codex's collapsed activity as
chips. In two captures 8.5 hours apart it renders as **plain text above a 1 pt
hairline**, page colour sampled either side, no fill. The doc is stale; the
running app is the bar.

## Round 2 — verified closed

A critic re-measured every claim independently rather than taking them on trust.
All five held; nothing was overstated.

| Claim | GoCode measured | Codex measured |
|---|---|---|
| Composer height | 117.0pt | 117.5pt |
| Send disc | 34.0 × 33.5pt | 34.0 × 33.5pt |
| Toolbar chrome | `#181818` = page, zero warmth | `#181818` |
| Content max-width @1530pt | 883.0pt | 882.5pt |
| Nav label contrast | 11.83:1 | 8.83:1 — GoCode now exceeds Codex |

Residual: a 52pt chrome strip still sits above the **sidebar**; Codex's sidebar
runs to y=0.

### The critic caught a hole in my own guard script

`gui-app-target.sh` certified a binary that started **six minutes before** the
commit it was sent to verify — right window, right workspace, stale code. It
matched on workspace name only. The critic noticed, rebuilt, relaunched and
re-measured before reporting a number.

The guard now also rejects a process older than the binary it was launched from.
A check that proves *which app* but not *which build* is worse than no check,
because it reads as certainty.

## Round 3 — shipped, awaiting measurement

Inspector was the biggest remaining gap and unchanged since round 1:
**380 × 848pt fixed split, 33.6% of the window**, against Codex's content-sized
card at **361 × 241pt, 4.7%**. Seven times the footprint.

Now a content-sized overlay grouped by kind — Changes, Subagents, Background
processes — from data the app already has. No branch or commit card: there is no
data for them, and an empty card is worse than an absent one.

## Round 4 — shipped

| Finding | Was | Now | Codex |
|---|---|---|---|
| User bubble | blue `#14273B`, 32pt | neutral `#242424`, **45.5pt** | `#242424`, 45.5pt |
| Tool calls | 29pt filled cards | plain line + 1pt hairline | plain line + hairline |
| Left edges | 5 edges over 36.5pt | **3.0pt across 5** | 4.0pt across 5 |
| Status strip | 30pt persistent | gone, inline while running | inline |

## Round 5 — verdict: does not win, and the top gap was mine

> A person separates the two screenshots in well under a second.

**The biggest gap was a merge that never happened.** I branched round 3 from
`24ef010c` *before* round 2 merged, so rounds 3 and 4 never contained round 2's
four fixes. The critic proved it with `git merge-base --is-ancestor`, then
confirmed by pixel probe — it was measuring a build where the composer was still
69pt and the send disc still 17pt because those fixes were not in the binary.

Rebased. All four are now in the chain; 159 tests pass. That one merge removes
the two loudest tells.

**And it caught a product regression I would have shipped.** Removing the status
strip was right, but the strip carried two *features* that went with it:
`UsageLabel` and `TranscriptText.plain` both had **zero call sites**. No token or
cost readout, no copy-conversation. A functional loss wearing a styling change's
clothes. Being re-homed into the Environment card, with a test that fails if
either goes unreferenced again.

## Round 6 — closed

All four targets landed, but one of my own reports was wrong and the critic
caught it.

| Gap | Before | After |
|---|---|---|
| Rail selection colour | `#1D3045` fill / `#007AFF` text | neutral, via `Theme.selectedRowSurface` |
| Transcript body type | reported as raised to 16.5pt | **it was not** — see below |
| User prompt width | 787pt full-column band | hugs content, capped 374.5pt |
| Sidebar to y=0 | 52pt strip above it | did not land in r6; closed in r7 |

**The type scale was raised and never read.** Markdown paragraphs, list rows,
quote rows and the user bubble set no font at all, so every one inherited the
macOS 13pt system default. Every token test passed the whole time, because the
tokens held the right numbers and nothing consumed them. Setting a token is not
the same as consuming it — the new tests assert the transcript is *wired to*
the scale, not that the scale exists.

Measured after the fix: line pitch 16.0pt → **25.0pt** (reference 26.5pt), from
60% of the reference to 94%.

## Round 7 — closed, pending critic

Driven by an independent critic's ranked measurements. Its top item was one no
previous round had named.

| # | Gap | Before | After |
|---|---|---|---|
| 1 | Sidebar was a nav menu | 7 rows, 468pt void (61% of window height) | conversation list, most recent first |
| 2 | Sidebar stopped short of the top | 52pt content-coloured bar above it | runs to y=0 |
| 3 | No conversation header | window title only | folder glyph + title + overflow, in the content pane |
| 4 | User message alignment | flush left | flush right |
| 5 | Hairline above composer | 1pt edge-to-edge `#2F2F2F` | removed |
| 6 | Transcript vs composer column | 16pt out on both edges | one shared `ConversationColumn` |
| 7 | Message actions | 1 | 4, all wired to real behaviour |
| 8 | Tool rows | two identical `Worked ›` rows | one `Worked for Ns ›` |
| 9 | Section header | tracked uppercase `HISTORY` | sentence case at row size |
| 10 | Divider contrast | `#333333`, ΔL 27 | `#2B2B2B` |
| 11 | Composer radius | 14pt | 20pt |
| 12 | Composer gutters | 8pt top / 18pt bottom | even |

**Round 7 also shipped a bug that only driving the app would find.** The new
rail read conversations once, on appear — so in a new project it stayed empty
for the whole session and every conversation the user started was invisible
until relaunch. That would have made the void *worse* than the one this round
existed to remove. Caught by noticing the Sessions screen listed a conversation
the rail beside it did not. Fixed by refreshing when a run finishes, and
verified live: the rail now fills in without a relaunch.

## Round 8 — in flight (issue #1071)

Ground truth regenerated against a build of current `main` rather than trusting
round 7's numbers. No UI change has landed since round 7, so its measurements
still hold; the gaps below are what the round-7 critic ranked, minus the two
that have since closed.

| # | Gap | Codex | GoCode | Kind |
|---|---|---|---|---|
| 1 | Full-width app-name titlebar | both columns reach y=0 | `#222222` band y=0→52 across the whole window | structural |
| 2 | Two stacked header bars | one 55pt bar; first message at 94pt | 52pt bar + separate header; first message at 169.5pt | structural |
| 3 | Nav pill steals the selected state | `#333333` on the active conversation only | on the nav pill *and* the conversation, 31.5pt apart | structural |
| 4 | Sidebar type a step too small | row cap 13.0pt, label 11.0pt at `#747474` | row cap 9.5pt, label 9.0pt at `#A3A3A3` | cosmetic |
| 5 | Two message actions share a glyph | 4 distinct | `doc.on.doc` twice (`ChatView.swift:214`, `:243`) | **bug** |

Gap 1 was claimed closed in round 7 and was not: a colour probe at y=5 returns
the sidebar colour on both sides of the window. This round is specced to verify
by pixel probe rather than by reading the diff.

Gap 5 was found by the round-7 critic and its fix never landed — confirmed
still present on `main` today. Copy-message and copy-conversation render
identically, which reads as a duplicated button.

### Closed since round 7

| Gap | Before | After | Verified by |
|---|---|---|---|
| Rail rows off-centre | pill 1.0pt left / 15.0pt right | **8.0pt / 8.0pt** | accessibility tree on the running app |

The cause was not the pill: five compact footer rows needed 249pt inside a
204pt column, and the 45pt overflow shifted the whole rail. The test pins the
arithmetic — five rows plus gaps must fit the column — so the next icon added
to that footer fails the test rather than quietly bending the rail again.

### Explicitly not in scope this round

The sixteen properties the critic measured as already matching, listed in
`.ux/design-baseline.md` under "Closed". Two earlier rounds were partly spent
re-fixing things that were already correct, which is why the issue now carries
a do-not-touch list.

## Standing caveats

- The inspector's footprint is **source-verified only**. Synthetic clicks did not
  land that round, so nobody has measured it rendered. Not counted closed.
- One round measured both sides by pixel probe alone (no accessibility tree).
  Instruments are stated per round rather than blurred.
- `docs/design/codex-app-reference.md` §2 describes collapsed activity as chips;
  it rendered as text over a hairline in two captures 8.5 hours apart. That is one
  divergence seen twice, not two errors, and the doc may simply describe a state
  we never hit. The running app is the bar; the doc is not therefore unreliable.
