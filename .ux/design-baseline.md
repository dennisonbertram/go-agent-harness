# Design baseline — GoCode vs. Codex (ChatGPT.app)

**The bar.** Every number here was measured from the two apps running live. Live
numbers outrank anything written in prose, including our own design docs —
`docs/design/codex-app-reference.md` has been wrong twice.

- **Reference**: the Codex surface inside `ChatGPT.app`. Window `1728 × 1079 pt`.
  Capture: `/tmp/gauntlet/codex.png` (3456 × 2158 px @2x).
- **Challenger**: `GoCode`, SwiftUI, source `macapp/Sources/GoCodeUI/`. Window
  `1162 × 772 pt`. Capture: `/tmp/gauntlet/r7-judged.png` (2324 × 1544 px @2x).

Last measured after **round 7** (merged in #970), by an independent
fresh-context critic doing a blind A/B.

## How to read the numbers — before quoting any figure

- **All figures are POINTS = raw px ÷ 2.** Never compare raw pixel values across
  the two captures; the windows are different sizes.
- The 2x assumption is **verified, not assumed**: macOS traffic-light circles
  measure 24 raw px in both captures, which is 12.0pt as specified.
- **Codex exposes no usable accessibility tree** — it is a web view, and
  `nativeui snapshot --app ChatGPT` returns only the three traffic-light
  buttons. Every Codex figure is therefore **pixel-probed** from the screenshot:
  colour-run scans along rows and columns, and ink-extent boxes over thresholded
  pixels. Edges are accurate to roughly ±1pt, and the method cannot see anything
  sharing a colour with its neighbour.
- GoCode figures come from either the same pixel probing or the live
  accessibility tree via `nativeui`. **The instrument is stated per measurement**
  rather than blurred, because the two are not equally strong.
- Type sizes are **cap heights** unless stated. Where a pt size is derived from a
  cap height it is marked as derived — cap ÷ 0.70 for SF.

---

## Closed — measured as matching. Do not touch these.

| Property | Codex | GoCode | Note |
|---|---|---|---|
| Composer max width | 882.5pt | 883.0pt | best-calibrated number in the build |
| Transcript ↔ composer left alignment | 0.5pt apart | 1.0pt apart | shared column |
| User bubble fill | `#363636` | `#363636` | exact |
| User bubble height | 45.5pt | 45.5pt | exact |
| User bubble right edge vs composer | flush | flush (0.0pt) | exact |
| Body text cap height | 12.5pt | 12.5pt | exact |
| Sidebar surface | `#222222` | `#222222` | exact |
| Content surface | `#181818` | `#181818` | exact |
| Composer surface | `#2D2D2D` | `#2D2D2D` | exact |
| Selected-row fill | `#333333` | `#333333` | exact |
| Selected-row height | 36.0pt | 36.0pt | exact |
| Composer corner radius | 23.0pt | 20.9pt | effectively closed |
| Send button diameter | 33.5pt | 34.0pt | effectively closed |
| Composer bottom margin | 20.0pt | 18.5pt | good |
| Conversation-header title cap | 13.5pt | 13.0pt | good |
| Section label case | mixed | mixed | correct grammar; size and grey still off |

## Open gaps, ranked by how fast they give the app away

| # | Gap | Codex | GoCode | Kind |
|---|---|---|---|---|
| 1 | Full-width app-name titlebar | both columns reach y=0 | `#222222` band y=0→52 across the whole window | structural |
| 2 | Two stacked header bars | one 55pt bar, title on the traffic-light row | 52pt titlebar + separate header; first message 169.5pt down vs 94.0pt | structural |
| 3 | Nav pill steals the selected state | `#333333` reserved for the active conversation; nav rows unfilled | nav pill and conversation row both `#333333`, 31.5pt apart | structural |
| 4 | Every hairline missing | 3 rules: `#3C3C3C` under header, `#393939` above footer, `#2B2B2B` under tool row | zero rules anywhere | structural |
| 5 | Two message actions share one glyph | 4 distinct glyphs, gaps 19.0/19.5/18.0pt | copy-message and copy-conversation both `doc.on.doc`; gaps 37pt | bug + cosmetic |
| 6 | Sidebar type a step too small | row cap 13.0pt; label cap 11.0pt at `#747474` | row cap 9.5pt; label cap 9.0pt at `#A3A3A3` | cosmetic |
| 7 | Sidebar footer | avatar + name + help, 55.5pt band | five bare icons, 16pt band, unbalanced | structural |
| 8 | Composer control vocabulary | chips and value controls; no checkbox anywhere | literal checkbox plus a bare word button | structural |
| 9 | Composer height / placeholder hierarchy | 117.5pt; placeholder cap 12.0pt, larger than its labels | 89.0pt; placeholder cap 9.5pt, smaller than its labels | cosmetic |
| 10 | Turn-to-turn spacing | 62.5pt | 20.0pt | cosmetic |
| 11 | Top-right controls | bordered pill + filled square + 2 icons, ink `#FFFFFF` | 2 bare icons at `#595959`, plus an orphan chevron | structural |
| 12 | User bubble radius | 17.0pt | 10.5pt | cosmetic |
| 13 | Sidebar pills clipped | 9.5pt left / 9.0pt right, symmetric | 1.0pt left / 15.5pt right | cosmetic |
| 14 | Status dot side | right-aligned, ~31pt from the sidebar's right edge | left of the row icon, a third indent level | cosmetic |
| 15 | Conversation-header insets | icon 18.0pt from pane edge, gap 13.0pt, hangs 40.5pt left of the column | 30.5pt, 9.0pt, flush | cosmetic |

**Explicitly not a gap:** sidebar width is 348.0pt vs 220.0pt, but the windows
differ in size — as a fraction that is 20.1% vs 18.9%, within noise for a
resizable pane. Do not "fix" it.

**Unverified:** whether GoCode suppresses the work-summary row above an answer,
or simply had no tool work on the measured turn. Not determinable from a still.

## Lessons this baseline exists to prevent

- **A token set is not a token consumed.** The transcript rendered at the macOS
  13pt default for a whole round while every token test passed, because the
  message views set no font and inherited it. Assert that something *reads* the
  scale, not that the scale holds the right number.
- **Blunt instructions produce over-corrections.** "Remove the hairline above the
  composer" removed every hairline in the app. "Add air above the first message"
  overshot the reference by 25pt. State the target value, not just the direction.
- **Verify against pixels, not intent.** Round 7 reported the sidebar reaching
  the top of the window; a colour probe at y=5 disproved it.
- **`.task` fires once.** The conversation rail stayed empty for entire sessions
  because it fetched on appear and nothing refreshed it.
