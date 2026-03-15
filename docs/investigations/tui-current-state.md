# TUI Current State Analysis

## BubbleTea Dependency Status
- **bubbletea**: NOT in go.mod
- **glamour**: v1.0.0 (in go.mod)
- **lipgloss**: v1.1.1-0.20250404203927-76690c660834 (transitive, via glamour)
- **bubbles**: NOT in go.mod

## Codebase Structure

### Production CLI: `cmd/harnesscli/`
- Pure flag-based tool (no interactivity)
- Streaming via raw SSE parsing + line-by-line output
- 348-line main.go with blocking I/O model
- No TUI framework, no interactive components

### Experimental Demo CLI: `.claude/worktrees/agent-ac17ff79/demo-cli/`
- Interactive multi-turn support (readline-based)
- Basic ANSI colors (no styling library)
- 171-line display module with manual escape codes

## Rendering Pipeline
- **Input**: SSE stream (HTTP event-stream)
- **Parsing**: Manual `event:` / `data:` line parsing (main.go:301-320)
- **Processing**: Sequential JSON decode → event dispatch (main.go:322-331)
- **Output**: Formatted text to stdout with ANSI codes
- **No state model** or reactive component system

## Event Stream Architecture
- **54 event types** defined in `internal/harness/events.go`
- **Terminal events**: `run.completed`, `run.failed` (IsTerminalEvent at line 324-326)
- **Delta streaming**: assistant text, thinking, tool output arrive as continuous deltas
- **Forensics events**: context window snapshots, tool decisions, causal graphs

## Current Display Functions (cmd/harnesscli/main.go or display.go)
- `PrintDelta()` - raw text pass-through
- `PrintThinkingDelta()` - dimmed text
- `PrintToolStart()/PrintToolComplete()` - tool lifecycle messages
- `PrintQuestion()` - numbered choice display
- `PrintUsage()` - cumulative cost (dimmed text)
- `PrintPrompt()` - colored prompt prefix
- `PrintBanner()` - welcome message

## Critical Gaps for Full BubbleTea TUI

| Feature | Current State | Needed |
|---------|---------------|--------|
| Screen redraws | Static output only | Full Model/Update/View loop |
| Input handling | Blocking stdin reads | Non-blocking event queue |
| Async events | Goroutine SSE reader | Channels feeding TUI model |
| Layout control | Printf-based | Lipgloss flex layout |
| Component state | Per-function display | Unified Model struct |
| Key bindings | None | Key message routing |
| Window resizing | Not handled | Automatic via BubbleTea |
| Spinner/progress | Hardcoded text | Bubbles spinner/progress |
| Status line | Static footer text | Live updating footer |
| Scrolling | Not supported | Bubbles viewport |

## Architecture Integration Challenge

**Current**: SSE stream → Parse JSON → Print line → Move on
**BubbleTea needed**: SSE stream → Channel → `Model.Update()` receives Msg → View rerenders

The blocking SSE reader at `main.go:214-274` is the **fundamental architectural blocker** — must be converted to a non-blocking channel-based pattern to feed the BubbleTea event loop.

## Key File Locations

| Component | Path | Lines |
|-----------|------|-------|
| CLI entry | `cmd/harnesscli/main.go` | 112-175 |
| Event streaming | `cmd/harnesscli/main.go` | 214-274 |
| SSE parsing | `cmd/harnesscli/main.go` | 276-320 |
| Event definitions | `internal/harness/events.go` | 1-543 |
| Terminal events | `internal/harness/events.go` | 324-326 |
