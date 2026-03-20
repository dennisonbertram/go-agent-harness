# Issue #360 Grooming: TUI Transcript Export Artifacts

## Already Addressed?
**NO** — Still present. `cmd/harnesscli/tui/model.go:974` calls `transcriptexport.NewExporter(".")` writing to CWD.

## Evidence
- Line 974 in model.go: `exporter := transcriptexport.NewExporter(".")`
- 130+ `transcript-*.md` files in git status under `cmd/harnesscli/tui/components/transcriptexport/`
- Tests use `t.TempDir()` so they pass fine, but production writes to `.`

## Clarity
GOOD — Problem is unambiguous: writes to CWD, should write to a runtime-safe path.

## Acceptance Criteria
Adequate:
- Export writes outside repo tree (e.g. `~/.harness/transcripts/` or OS temp)
- User sees the absolute path in status message (already works once path is fixed)
- Repeated exports don't dirty the source tree

## Scope
ATOMIC — single call site change + path resolution helper.

## Blockers
NONE.

## Recommended Labels
- `well-specified`
- `small`
- `bug`
