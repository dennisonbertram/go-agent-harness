# Issue #363 Grooming: TUI Core Rendering Seams

## Already Addressed?
**PARTIALLY** — Rendering components (messagebubble, tooluse, diffview) are built and tested but NOT wired into the root model. Root model renders events directly to viewport via raw string append.

## Evidence
- `cmd/harnesscli/tui/components/messagebubble/model.go` View() returns `""`
- `cmd/harnesscli/tui/components/diffview/model.go` View() returns `""`
- `cmd/harnesscli/tui/components/tooluse/model.go` View() returns `""`
- Root model does `m.vp.AppendLine("⏺ " + p.Tool + ...)` — bypasses component layer entirely
- Thinking deltas collected but not displayed (comment in model.go says "skip for now")

## Clarity
UNCLEAR (3/5) — "rendering seams" is vague. Doesn't specify which components, what "finishing" means, or whether this is rewiring vs new functionality.

## Acceptance Criteria
NOT PROVIDED — Issue lacks specifics:
- messagebubble.Model.View() should dispatch to AssistantBubble/UserBubble
- diffview.Model.View() should bridge to View.Render()
- tooluse.Model.View() should use collapsed/expanded sub-components
- thinking indicator should show extended thinking content

## Scope
NOT ATOMIC — four independent component rewires plus thinking display. Recommend splitting into focused issues.

## Blockers
- Soft dependency on #364 (per tracker order)
- Risk: viewport rendering behavior (autoscroll, wrapping) may change

## Recommended Labels
- `needs-clarification`
- `medium`
