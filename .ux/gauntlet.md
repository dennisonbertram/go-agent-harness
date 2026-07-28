# Gauntlet Loop — live progress

Two goals, judged against real artifacts rather than descriptions.

**Goal 1 — every tool works when driven through the app's UI.**
Bar: the transcript. A tool passes only when the reply proves it ran.

**Goal 2 — the app looks like Codex.**
Bar: the real Codex surface inside `ChatGPT.app`, running on this machine.

---

## Merged this session

| PR | What |
|---|---|
| #947 | provider catalog, honest pricing, four behaviour gaps |
| #948 | block-level markdown rendering |
| #952 | **conversation-scoped SSE stream — callback and cron runs are now visible** |
| #953 | diff memoisation, bounded completion scan, export/rewind cleanup, shared card styling |
| #954 | six correctness findings incl. the binary-resolution fallback |
| #955 | **design tokens — neutral 27-level palette, matching Codex's measured 27** |
| #956 | **single-card composer at Codex's 19pt inset, collapsed inspector, labelled rail** |

---

## Goal 2 — design, round 0 baseline

Measured, not impressionistic. One caveat that shapes everything: **GoCode exposes a
real accessibility tree; Codex does not** — it is a web view returning three elements.
So GoCode's numbers are its own reported geometry and Codex's are pixel-probed from a
screenshot at 2x. Different instruments, stated rather than blurred.

| # | Gap | Measured |
|---|---|---|
| 1 | **No palette.** Nothing reads as layered. | GoCode's six surfaces span 13 grey levels (40→53), all warm-tinted. Codex's five span 27 (24→51), pure neutral. GoCode's *darkest* surface is 16 levels lighter than Codex's page background. |
| 2 | **Composer is scattered.** | GoCode: 520×34 field, send button outside it, controls on a separate strip, 0 bottom inset. Codex: one 883×118 elevated card holding everything, inset 19. |
| 3 | **Right panel is permanent and opaque.** | 543×935 = **44.1% of the window** to show two JSON strings. Codex's equivalent floats at 361×240 = **4.6%**. |
| 4 | **No type hierarchy.** | Four sizes inside a 4pt range, a 2-level foreground ramp. Codex: 16pt primary, a 5-level ramp, 1.65× the body contrast. |
| 5 | **Icon-only rail.** | 50pt, no labels or sections — and four rail buttons expose their **SF Symbol identifier as the accessibility label** (`gearshape`, `bubble.left.and.text.bubble.right`). A real a11y defect and proof no human-readable name exists. |

Cheap runner-up: four different content left edges in the transcript column
(51/63/67/88pt) against Codex's single edge.

**Not a gap despite appearances:** conversation column width is comparable once
normalised (Codex 51.0% of window, GoCode 46.3%). The right panel's problem is that
it is permanent and opaque, not that it is wide.

**Correction to our own reference doc:** `codex-app-reference.md` §2 describes
transcript activity as chips. In the state captured today Codex renders it as plain
text above a hairline rule — GoCode is actually the chippier of the two. The running
app beat the document, which is the entire point of using it as the bar.

### Round 1 result — measured, merged

| Gap | Before | After | Codex |
|---|---|---|---|
| 1 palette | 13 grey levels, warm-tinted | **27 levels (24→51), neutral** | 27 levels (24→51) |
| 1 foreground ramp | 2 levels | **4 levels** | 5 levels |
| 2 composer | 520×34 field + separate 36pt strip, 0pt inset | **898×68 single card, 19pt inset** | 883×118 card, 19pt inset |
| 3 inspector | permanent, 44.1% of window | **collapsed by default (0%)**, 30.8% when opened | floating, 4.6% |
| 5 rail | 50pt, icon-only, SF Symbol names as a11y labels | **220pt, labelled, sectioned, real a11y labels** | 348pt labelled sidebar |

Both builders measured their own output by sampling live pixels rather than reading
their source, so these are results not intentions. The two branches collided in nine
hunks across `ChatView` and `AppShell`; resolved by keeping the layout structure and
the palette tokens together, verified by sampling `24,24,24` and measuring the single
composer card in one running build.

Round 2 needs a fresh critic pass against Codex — blocked on the display (below).

---

## Goal 1 — tool walk through the UI

Two false starts, both worth recording because each would have produced a confident
wrong answer:

1. **Plan mode was on.** The first batch returned no replies at all — every run was
   correctly blocked awaiting approval. A walker trusting an empty transcript would
   have reported 23 false failures. The harness behaved properly; the walker did not.
2. **Replies were offset by one tool.** The transcript read was picking up the previous
   exchange because the new-conversation click had not settled. Results looked
   plausible and were systematically wrong — `read` showing `ls`'s answer.

A third, found by reading the app rather than running it: **every tool in the four
priority areas is `TierDeferred`**, so the model cannot call it until `find_tool`
activates it for that run. Prompts that just say "call cron_list" test nothing.

All three are now handled. The walk is being re-run.

---

## Standing findings not yet fixed

- **`Activity → Runs` is permanently dead.** The supervisor sets `HARNESS_CONVERSATION_DB`
  but never `HARNESS_RUN_DB`, so `GET /v1/runs` answers 501 for every app-launched
  daemon — removing the one surface where a fired callback's run could be found.
- **No menu bar commands and no keyboard shortcuts at all.** No `.commands {}`, no
  `Settings` scene, so ⌘, is inert.
- **Activity rows are inert.** `/v1/tasks` returns an `actions` array and the client
  decodes it; no view reads it. Not one background task can be acted on.
- **No UI at all** for cron CRUD, subagent control, workflows, or callback cancellation —
  all tool-only.
- Issue #951 findings 1, 2, 8, 10 (god object, no client protocol, stringly-typed
  vocabularies, duplicated catalog state) remain open.
