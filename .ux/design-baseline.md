# Design baseline — GoCode vs. Codex (ChatGPT.app)

Captured 2026-07-27 from the two apps running live on this machine.

- **Bar**: Codex surface inside `ChatGPT.app` (pid 70417, `com.openai.codex`). Window
  `1728 × 1079 pt`. Screenshot: `/tmp/gauntlet/codex.png` (3456 × 2158 px @2x).
- **Work**: `GoCode` (pid 11504), SwiftUI, source `macapp/Sources/GoCodeUI/`. Window
  `1167 × 987 pt`. Screenshot: `/tmp/gauntlet/gocode.png` (2334 × 1974 px @2x).

## How the numbers were obtained — read this before quoting any figure

Two different methods, and they are not equally strong:

- **GoCode**: element frames read from the live accessibility tree via the `nativeui`
  driver (32 elements, saved at `/tmp/gauntlet/gocode-snap.json`), converted from screen
  to window-relative coordinates. These are the app's own reported geometry.
- **Codex**: **no usable accessibility tree.** `nativeui snapshot --app ChatGPT` returns
  exactly 3 elements — the three traffic-light buttons — because the Codex surface is a
  web view that exposes nothing to AX. Every Codex number below is therefore measured by
  **pixel probing the screenshot** (colour-run scans along rows/columns, and ink-extent
  bounding boxes over thresholded pixels), then divided by 2 for the Retina scale.
  Pixel probing gives edges accurate to about ±1 pt and cannot see anything that shares a
  colour with its neighbour.
- Text sizes below are **ink extents** (topmost to bottommost lit pixel of a given
  string), not font point sizes. They are comparable to each other only when the strings
  have the same ascender/descender profile; where they do not, that is called out.
- Claims labelled **[judgement]** are visual reads with no measurement behind them.

The two windows are different sizes, so absolute pt comparisons are only meaningful where
noted; ratios and percentages are given where absolute numbers would mislead.

---

## Measured summary

| Property | Codex (pixel-probed) | GoCode (AX + pixel-probed) |
|---|---|---|
| Window | 1728 × 1079 pt | 1167 × 987 pt |
| Title bar height | 56 pt | 52 pt (AX `AXToolbar`) |
| Left navigation width | **348 pt**, labelled | **50 pt**, icon-only |
| Nav row pitch | 37 pt | 38 pt |
| Nav selection indicator | 330 × 36 pt fill, inset 10 pt | 38 × 34 pt fill |
| Conversation content column | 881 pt (51.0% of window width) | 540 pt (46.3% of window width) |
| Right panel footprint | floating card **361 × 240 pt** = **4.6%** of window area | full-height split **543 × 935 pt** = **44.1%** of window area |
| Composer | one surface, **883 × 118 pt** | input **520 × 34 pt** + separate 36 pt control strip |
| Composer bottom inset | 19 pt | 0 pt (runs to window edge) |
| Surface ramp (grey levels) | 24 → 34 → 36 → 45 → 51 (span **27**) | 40 → 43 → 46 → 48 → 51 → 53 (span **13**) |
| Surface hue | pure neutral (R = G = B) | warm (R = B + 3 on every surface) |
| Body-text colour / contrast | `#FFFFFF` on `#181818` = **17.8:1** | `#DFDFDE` on `#2B2A28` = **10.8:1** |
| Foreground ramp | 5 levels: 255 / 222 / 163 / 116 / 97 | 2 levels: 224 / 158 |
| Body ink height (asc→desc) | 16 pt | 11–13 pt |
| Distinct content left edges in main column | 1 (406–410 pt, ±4 pt glyph bearing) | **4** (51, 63, 67, 88 pt) |

---

## 1. Window chrome

**GoCode, measured.** `AXToolbar` is 1167 × 52 pt. It contains the traffic lights at
x = 18/38/58 (window-relative) and two static texts: `uiwalk-ws` at x = 91, and a
1167 × 52 pt full-bleed `AXStaticText` labelled `GoCode` at x = 161 that spans the entire
toolbar. Title bar background is `#2B2A28`, identical to the transcript panel below it —
the colour-run scan finds no boundary between chrome and content at x = 600.

**Codex, measured.** Title bar is 56 pt. It carries traffic lights (x = 15/35/55),
a sidebar-toggle and back/forward pair on the left, a breadcrumb (folder glyph +
`Check Clay MCP access` + `···`, title ink 189 × 16 pt starting at x = 396), and four
panel-layout toggles right-aligned. Background `#181818`, matching the page — also
seamless, but the page it matches is 10 grey levels darker than the sidebar beside it,
so the seam is intentional rather than accidental.

**Gap.** GoCode's title bar is decorative: 998 pt of its 1167 pt width is a single static
`GoCode` label, and the only actionable content in it is the traffic lights. Codex's title
bar is the app's primary control surface — navigation, current-location breadcrumb, a
per-conversation `···` menu, and four layout toggles.

**Remedy.** Replace the centred `GoCode` label with a breadcrumb built from data the app
already has: project name → conversation title, plus an overflow `···`. Move the right
panel's show/hide into a toolbar toggle on the right side. No new backend data required.

---

## 2. Sidebar / rail

**GoCode, measured (AX frames, window-relative).** A 50 pt-wide rail, background
`#302F2D`, holding 5 buttons at y = 62, 109, 147, 185, 223 — pitch **38 pt** after the
first, with a 47 pt gap between item 1 and item 2. Buttons are 38 × 34 pt (item 1) and
13–18 pt glyphs thereafter. Confirmed in source: `AppShell.swift:203` `.frame(width: 50)`,
`:182,:196` `.frame(width: 38, height: 34)`. Selected item fill is `#283B4F`.
A 6th button, `Close`, sits alone at y = 953.

There are **no text labels and no section grouping**. Four of the five rail buttons expose
their raw SF Symbol identifier as their accessibility label —
`bubble.left.and.text.bubble.right`, `chart.bar.doc.horizontal`,
`clock.arrow.circlepath`, `gearshape` — and only one reads as English (`Undo`). The send
button has the same problem (`arrow.turn.up.right`). This is simultaneously an
accessibility defect and direct evidence that no human-readable name exists anywhere in
the navigation.

**Codex, measured (pixel-probed).** A **348 pt** sidebar, background `#222222`, structured
as: workspace switcher `Codex ⌄` (ink 59 × 15 pt at x = 21, y = 67) + search glyph;
five labelled nav rows (`New chat` ink top y = 114 … `Plugins` ink top y = 262, i.e.
4 gaps over 148 pt = **37 pt pitch**); a `Pinned` section header (ink at x = 21, y = 315,
colour `#747474`); a `Projects` section header (y = 451); nested project folders with
their chat rows indented; a `Show more` affordance; and a pinned account row at y = 1045.
The selected chat row is a **330 × 36 pt** `#333333` fill inset 10 pt from the sidebar's
left edge and 8 pt from its right — i.e. an inset rounded pill, not a full-bleed band.
Section headers sit at x = 21 while row labels sit at x = 50, so hierarchy is expressed by
a **29 pt indent step**, not by rules or boxes.

**Gap — the single biggest.** GoCode's navigation is a 50 pt icon-only strip with no
labels, no sections, and no content: it switches between five views. Codex's is a 348 pt
labelled sidebar that is also the **content browser** — it shows the pinned/project/chat
hierarchy, so the user can see and reach every conversation without leaving the main view.
GoCode's equivalent data (conversations, checkpoints) is hidden behind two of the icons.
The width difference (50 vs 348 pt) is a symptom; the structural difference is that one
strip navigates *modes* and the other navigates *content*.

**Remedy, in effort order.**
1. Give every rail button a real `accessibilityLabel` and a `.help()` tooltip. Ten minutes,
   fixes a genuine a11y bug.
2. Widen to ~220–260 pt and add text labels beside the glyphs at the existing 38 pt pitch —
   the pitch already matches Codex's 37 pt, so nothing else needs to move.
3. Group with `#747474`-weight section headers at a 28–30 pt indent step
   (`Chat` / `History`: Sessions + Checkpoints / `Settings`), then inline the session list
   under `History` so the sidebar browses content rather than switching modes.
4. Change the selection indicator from a 38 × 34 pt glyph-sized fill to a full-row inset
   pill (row width − 20 pt, 34–36 pt tall).

---

## 3. Conversation area

**GoCode, measured.** Transcript panel spans x = 51 → 623 (**572 pt**), background
`#2B2A28`. Inside it:

- User bubble: x = 220 → 607, **387 pt** wide, fill `#253648` — a *blue-tinted* bubble
  (R 37, G 54, B 72), the only strongly hued surface in the window.
- Tool-call row: a filled rounded rect x = 67 → 607, **540 × 29 pt**, fill `#333230`
  (3 levels above the panel behind it), with a green check `#32D44A` at x = 78 and the
  tool name at x = 101.
- Assistant body: text begins at x = 88.
- Per-message affordance: a single 12 × 13 pt `Copy message` button, twice.

**Left-edge alignment is the measurable defect**: within one column the content uses
**four different left edges** — plan banner 51 pt (full-bleed), composer field 63 pt,
tool chip 67 pt, assistant text 88 pt. Right edges disagree too: tool chip and user
bubble both end at 607 pt, the composer field ends at 583 pt.

**Codex, measured.** Content column x = 406 → 1290, **881 pt**. Left edges: separator rule
406, `Worked for 11s` 410, body text 409, composer 408 — a **single** left edge within
±4 pt, and that 4 pt is glyph side-bearing, not layout drift. User bubble x = 916 → 1291,
**375 pt**, fill `#242424` — 12 levels above the `#181818` page, **neutral, not hued**.
Collapsed tool activity renders as plain `#A3A3A3` text (`Worked for 11s ›`, ink
113 × 12 pt) above a hairline rule spanning the full 881 pt column — *not* as a filled
chip in this session's state. Assistant responses carry a four-icon action row
(copy / thumbs up / thumbs down / branch) at 16 pt spacing.

> Note: `docs/design/codex-app-reference.md` §2 describes activity as chip/pill elements.
> In the state captured now, collapsed activity is text + rule. Both may exist in different
> states; only the text + rule form was observed today.

**Gap.** Two things, both measurable. First, GoCode has no shared content edge — four left
margins in one column, versus Codex's one. Second, GoCode's user bubble is the only
saturated surface in the app while Codex's is neutral, which means GoCode spends its single
strongest visual signal on "this message is from me" — the least informative distinction
on screen.

**Remedy.** Define one content inset constant and apply it to the plan banner, tool rows,
assistant text and composer alike (pick 20–24 pt from the panel edge). Desaturate the user
bubble to a neutral 10–14 levels above the transcript background and let the blue accent
carry state instead. Add the response action row (copy / branch / retry) — the copy button
already exists, it just needs siblings and a consistent position.

---

## 4. Composer

**GoCode, measured.** The input field is **520 × 34 pt** (x = 63 → 583, y = 917 → 951),
fill `#353433`. The send button (16 × 13 pt) sits **outside** it at x = 593, in a 40 pt
right gutter, which is why the field's insets are asymmetric — 12 pt left, 40 pt right.
Below it, a separate 36 pt strip carries `Server default ⌄` (125 pt popup at x = 63),
a `Plan mode` checkbox (70 pt at x = 198) and a `New` button at x = 589. That strip has no
background of its own and runs to the bottom window edge: **bottom inset 0 pt**.

**Codex, measured.** One surface: x = 408 → 1290, y = 942 → 1060 — **883 × 118 pt**, fill
`#2D2D2D`, inset **19 pt** from the window bottom and 59 pt from the panel's left edge.
Everything lives inside it: placeholder `Do anything` (`#616161`), `+` attach,
`Approve for me` shield toggle, `Custom Medium ⌄` model/effort selector, mic, and a
circular send button (`#7C7C7C` fill) at the bottom-right corner. Two rows — text on top,
controls beneath — inside a single container.

**Gap.** GoCode's composer is 34 pt of chrome scattered across three non-contiguous
regions (field, right gutter, bottom strip) with no shared background; Codex's is a single
118 pt elevated card that is unmistakably *the thing you type into*. Codex's composer
surface is **3.5× taller** and is the second-most-prominent object on screen; GoCode's
reads as a footer.

**Remedy.** Wrap field + send + model picker + plan toggle in one `RoundedRectangle`
surface, ~100–120 pt tall, with a 16–20 pt bottom inset and symmetric horizontal padding.
Move the send button inside the container's bottom-right. This is a single container
change in `ChatView.swift`, and it is the highest visual-impact-per-line change in the app.

---

## 5. Right panel — activity / environment

**GoCode, measured.** A permanent opaque split: x = 624 → 1167, **543 pt** wide,
full height below the toolbar (935 pt), background `#282725`. Inside: the tool name `ls`
(ink 10 × 10 pt), a right-aligned `completed` label, and two field groups —
`Arguments` label + a 511 × 35 pt `#2E2D2B` value box, then `Output` label + a second
511 × 35 pt box. The value boxes sit **6 grey levels** above the panel behind them.
Footprint: 543 × 935 = **44.1% of the window's area, permanently, for one tool call.**

**Codex, measured.** A floating `Environment` card at x = 1349 → 1710, y = 70 → 310 —
**361 × 240 pt**, fill `#2D2D2D` (21 levels above the `#181818` page), inset 18 pt from the
right window edge. It is **only as tall as its contents**; the colour scan at y = 600
finds uninterrupted page background all the way to x = 1728. Five rows at a **37 pt
pitch** — matching the sidebar's 37 pt — each an icon + label + optional value:
`Changes` with a `+566` / `−126,137` diff stat in green `#6BC67F` / red `#E75248`,
`Local ⌄`, the branch `promote/opus-5-prod ⌄`, `Commit or push`, and the latest commit with
a GitHub link glyph. Footprint: **4.6% of window area**.

**Gap — the biggest by area.** GoCode reserves 44.1% of the window permanently to display
two JSON strings for a single tool call. Codex spends 4.6% on a card that summarises the
*whole environment* — diff stat, branch, commit, subagents, background processes — grouped
by kind, and gives the space back when it has nothing to say. GoCode's panel is a detail
inspector promoted to permanent chrome; Codex's is a status summary that floats.

**Remedy.** Two steps, either shippable alone.
1. Make the panel collapsible from a toolbar toggle and default it closed. One line of
   state, reclaims 44% of the window immediately.
2. Restructure it from "selected tool call detail" to Codex's kind-grouped card taxonomy —
   Changes / Branch / Subagents / Background processes — sized to content rather than to
   the window. `docs/design/codex-inspired-design-plan.md` §A4 already scopes this and
   names the backing data as already available.

---

## 6. Typography scale

**Measured ink extents** (top-to-bottom lit pixels of the named string; comparable only
within matching ascender/descender profiles, so both profiles are listed separately).

| Profile | Codex | GoCode |
|---|---|---|
| Ascender→descender | body 16, breadcrumb 16, sidebar `Plugins` 16, `Changes` 16 | user bubble 13, tool chip `glob` 11, `Ready to leave plan mode` 11 |
| Cap→baseline (no descender) | `New chat` 13, `Worked for 11s` 12, account name 12 | `Server default` 10, panel title `ls` 10, `completed` 9 |

**Gap.** Like-for-like, Codex's primary UI text is **1.23–1.45× larger** than GoCode's
(16 vs 11–13 pt asc→desc; 12–13 vs 9–10 pt cap→baseline). More damaging than the absolute
size: GoCode's four observed sizes (13 / 11 / 10 / 9) are crowded into a **4 pt range**, so
a section label, a status word and body copy are all nearly the same size — there is no
step large enough to read as hierarchy. Codex uses a **clear two-tier split** — a 16 pt
asc→desc primary tier for anything you read, and a 12 pt tier for metadata — reinforced by
colour rather than by more sizes.

The reference doc records Codex's UI font default as 14 px with size as a first-class user
preference; GoCode hardcodes `.font(.system(size: 16))` / `size: 15` for rail glyphs and
otherwise uses SwiftUI semantic defaults (`.headline`, `.callout`, `.caption`).

**Remedy.** Define three sizes and use only those: body 14, label 12, caption 11. Ship them
as named tokens so the range cannot collapse again. Expressing hierarchy through the
foreground ramp in §8 costs nothing and does more than adding a fourth size.

---

## 7. Spacing rhythm

**Codex, measured.** A single ~36–37 pt row unit repeats across unrelated regions:
sidebar nav pitch **37 pt**, environment-card row pitch **37 pt**, selected-row height
**36 pt**. Content insets are consistent — composer 59 pt from the panel's left edge,
19 pt from the window bottom; environment card 18 pt from the right edge, 14 pt below the
title bar. The sidebar hierarchy indent is a clean 29 pt step (21 → 50).

**GoCode, measured.** The rail's 38 pt pitch matches Codex's 37 pt — the one place the
rhythm is already right. Everywhere else it is not: content left edges at 51 / 63 / 67 /
88 pt, right edges at 583 / 607 pt, composer horizontal insets asymmetric at 12 pt left vs
40 pt right, bottom inset 0 pt, right-panel value boxes 35 pt tall against a 29 pt tool
chip in the other column. Source shows the cause — one-off literals: `.padding(10)`,
`.padding(28)`, `.padding(.vertical, 10)`, `cornerRadius: 8`.

**Gap.** GoCode has no spacing scale; every inset is decided locally. Codex has one
repeating row unit and a consistent edge inset, which is most of what "polished" means
here. **[judgement]** — that last clause is an interpretation; the edge numbers above are
measurements.

**Remedy.** Adopt a 4 pt scale with a 36 pt row unit, and one `contentInset` constant used
by every element in the transcript column. Replacing the literal paddings with named
constants is mechanical and is a precondition for §2, §4 and §5 landing cleanly.

---

## 8. Colour and contrast — the highest-leverage fix

**Measured surface ramps.**

- Codex: page `#181818` (24) → sidebar `#222222` (34) → user bubble `#242424` (36) →
  composer and environment card `#2D2D2D` (45) → selected row `#333333` (51).
  Five steps, **span 27 levels**, every one **pure neutral (R = G = B)**.
- GoCode: right panel `#282725` (40) → transcript `#2B2A28` (43) → right-panel value box
  `#2E2D2B` (46) → rail `#302F2D` (48) → tool chip `#333230` (51) → composer field
  `#353433` (53). Six steps, **span 13 levels**, average gap **2.6 levels**, and every
  surface **warm-tinted (R = B + 3)**.

Consequences that follow directly from those numbers: GoCode's darkest surface (40) is
**16 levels lighter** than Codex's page background (24), so nothing in GoCode reads as
"behind" anything. Adjacent surfaces differ by 2–3 levels, below the threshold at which a
boundary reads without a divider — which is why the right panel needs an explicit split and
the title bar disappears into the transcript.

**Measured foreground ramps.**

- Codex: body `#FFFFFF` (255) → sidebar labels `#DEDEDE` (222) → secondary `#A3A3A3` (163)
  → section headers `#747474` (116) → placeholder `#616161` (97). **Five levels.**
- GoCode: everything readable is 222–224 — user bubble text, assistant body, tool chip
  label, `Server default`, monospace values — with a single secondary at `#9E9E9D` (158).
  **Two levels.**

Body-text contrast: Codex `#FFFFFF` on `#181818` = **17.8:1**. GoCode `#DFDFDE` on
`#2B2A28` = **10.8:1**. Codex's primary text carries **1.65× the contrast**. Both pass
WCAG AAA for body text; the difference is that Codex's ramp leaves three usable rungs
*below* primary for hierarchy, and GoCode's leaves one.

**Root cause, confirmed in source.** `AppShell.swift:204` `.background(.quaternary.opacity(0.25))`
and `:239` `.background(.quaternary.opacity(0.35))`. Every GoCode surface is the same
translucent quaternary material at a slightly different opacity over the system window
background. That is precisely a 2–3 level ramp with a warm system tint — the measurements
above are the direct arithmetic consequence of that one pattern. There is no
`DesignSystem/` directory and no `Theme.swift` in `macapp/Sources/GoCodeUI/`.

**Gap.** GoCode has no palette. It has one material used six times.

**Remedy — do this first.** Add explicit opaque surface tokens (`background`, `surface`,
`surfaceElevated`, `surfaceSelected`) and foreground tokens (`fgPrimary` … `fgPlaceholder`,
five rungs), then replace every `.quaternary.opacity(x)` and every bare `.secondary`.
Target a ramp with roughly Codex's span: background ≈ 24, panels ≈ 34, elevated ≈ 45,
selected ≈ 51, neutral rather than warm; foreground 255 / 222 / 163 / 116 / 97.
This is §A0 + §A1 of `docs/design/codex-inspired-design-plan.md`, and it is a single new
file plus mechanical substitutions.

---

## 9. Iconography

**GoCode, measured.** SF Symbols, monochrome, rendered at `size: 16` (rail item 1) and
`size: 15` (rail items 2–5), with AX-reported glyph boxes of 13–18 pt. No labels anywhere.
Four of five rail buttons and the send button expose the SF Symbol identifier as their
accessibility label (§2). The only coloured glyph in the transcript is the green check
`#32D44A` on the tool chip. The send button is `arrow.turn.up.right` — a *reply* arrow, not
a send arrow.

**Codex, measured/[judgement].** Icons are consistently paired with text in the sidebar
and the environment card (measured: label ink begins at x = 50 in the sidebar, 29 pt right
of the section-header edge, i.e. the glyph occupies a fixed leading slot). The send control
is a **filled circular button** — a `#7C7C7C` disc measured at the composer's bottom-right —
which is the only filled control in the composer; every other icon is an unfilled line
glyph. Line weight and optical size look uniform across the app **[judgement]** — this is
an impression, not a measurement, since the AX tree gave no glyph frames.

**Gap.** GoCode's icons carry navigation meaning with no text and no accessible names.
Codex uses icons as *accents beside labels*, and reserves the one filled, high-contrast
control for the single primary action.

**Remedy.** Labels and accessibility names on every rail item (§2); swap
`arrow.turn.up.right` for `arrow.up` in a filled circular button as the composer's only
filled control.

---

## 10. State and status

**Codex, measured.** State is encoded in colour against an otherwise neutral field:
a blue unread dot `#91C2FA` on `Scheduled` and on the `Explore Smithers capabilities`
chat row; a spinner glyph on an in-progress chat row; a diff stat rendered as
`+566` in green `#6BC67F` and `−126,137` in red `#E75248` inside the environment card;
`Worked for 11s ›` as expandable secondary text at `#A3A3A3`; a GitHub glyph marking the
commit row as a link-out. Because every *surface* is neutral (§8), each of these hues is
unambiguous — colour in this UI means "state", nothing else.

**GoCode, measured.** In the captured state: a green check `#32D44A` on the tool chip, and
the word `completed` in `#9E9E9D` — **the exact same grey as the `Arguments` and `Output`
field labels beside it**. The run's status is therefore typographically and chromatically
indistinguishable from a static form label. The other coloured elements on screen are the
blue user bubble `#253648` and the blue plan banner `#2F3740`, neither of which encodes
run state.

**Gap.** GoCode spends colour on message ownership and mode chrome, and renders actual
status in the same grey as inert labels. Codex spends colour exclusively on status and
keeps every container neutral.

**Remedy.** Reserve hue for state: `completed` green, `running` accent-blue with motion,
`failed` red; neutralise the user bubble (§3). Then add the states Codex shows that GoCode
already has data for — elapsed time on the tool row (`Worked for 11s` equivalent), and a
diff stat once the checkpoint/rewind data is surfaced.

---

## Ranked gaps — visual impact per unit of effort

1. **No palette — six surfaces spanning 13 grey levels, all from one translucent material.**
   Codex spans 27 levels across five neutral surfaces and 158 levels across five foreground
   rungs; GoCode has 13 and 66 respectively. *Effort: one new tokens file + mechanical
   substitution of `.quaternary.opacity(x)` and `.secondary`. Nothing else on this list
   looks right until this lands.*
2. **The composer is a 34 pt field split across three regions with 0 pt bottom inset.**
   Codex's is one 883 × 118 pt elevated card holding every control, inset 19 pt.
   *Effort: one container in `ChatView.swift`. Largest single-change visual delta.*
3. **The right panel is a permanent opaque split consuming 44.1% of window area to show one
   tool call.** Codex's environment card is 4.6% and sized to its contents.
   *Effort: a collapse toggle is one line of state; the card-taxonomy rebuild is larger and
   already scoped as §A4 of the design plan.*
4. **Type sits in a 4 pt range (13/11/10/9) with a 2-level foreground ramp, so there is no
   hierarchy.** Codex reads at 16 pt asc→desc primary with 5 foreground rungs.
   *Effort: three size tokens; the foreground rungs come free with item 1.*
5. **Navigation is a 50 pt icon-only rail with no labels, no sections, and SF Symbol
   identifiers as its accessibility names.** Codex uses a 348 pt labelled sidebar that also
   browses content. *Effort: labels + a11y names are trivial and fix a real defect; widening
   to a labelled, sectioned sidebar is the largest item here.*

Runner-up, cheap and worth doing alongside item 4: **four different content left edges
(51 / 63 / 67 / 88 pt) in the transcript column** versus Codex's single 406–410 pt edge.
One shared inset constant fixes it.
