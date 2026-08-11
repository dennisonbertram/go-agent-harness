package tooluse

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// groupHintStyle renders the expand hint faintly so the summary stays quiet.
var groupHintStyle = lipgloss.NewStyle().Faint(true)

// Group collapses a run of tool calls into a single transcript line.
//
// A tool-heavy turn emits dozens of calls and, rendered one per line, buries the
// assistant's actual answer (issue #1308). Collapsed, the group costs one line;
// expanded, every member renders in full.
//
// Failures are never hidden. A failed call renders its error whatever the toggle
// says, because an error the user has to go looking for is worse than a noisy
// transcript.
//
// Group is an immutable value type — every mutation returns a new Group.
type Group struct {
	calls    []Model
	expanded bool
	width    int
}

// NewGroup returns an empty Group rendering at the given width.
func NewGroup(width int) Group {
	if width <= 0 {
		width = defaultWidth
	}
	return Group{width: width}
}

// Add returns a new Group with the call inserted, replacing any existing member
// with the same CallID. Upsert rather than append because a call is rendered
// first when it starts and again when it completes or fails, and the second
// render is an update, not a second call.
//
// The expanded state is carried over, so a group the user opened does not snap
// shut when the next call lands.
func (g Group) Add(call Model) Group {
	call.Width = g.width
	next := g
	next.calls = append([]Model(nil), g.calls...)
	for i, existing := range next.calls {
		if existing.CallID == call.CallID {
			next.calls[i] = call
			return next
		}
	}
	next.calls = append(next.calls, call)
	return next
}

// Toggle returns a new Group with the expanded flag flipped.
func (g Group) Toggle() Group {
	next := g
	next.expanded = !g.expanded
	return next
}

// IsExpanded reports whether the group renders its members in full.
func (g Group) IsExpanded() bool { return g.expanded }

// Len returns how many calls the group holds.
func (g Group) Len() int { return len(g.calls) }

// HasFailure reports whether any member call failed.
func (g Group) HasFailure() bool {
	for _, c := range g.calls {
		if isFailedStatus(c.Status) {
			return true
		}
	}
	return false
}

// View renders the group: one summary line when collapsed, every member when
// expanded. A single call is never summarised — a one-item summary is strictly
// worse than the item itself.
func (g Group) View() string {
	switch {
	case len(g.calls) == 0:
		return ""
	case len(g.calls) == 1:
		return g.calls[0].View()
	case g.expanded:
		return g.expandedView()
	default:
		return g.collapsedView()
	}
}

// expandedView renders every member call, one after another.
func (g Group) expandedView() string {
	parts := make([]string, 0, len(g.calls))
	for _, c := range g.calls {
		if rendered := strings.TrimRight(c.View(), "\n"); rendered != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.Join(parts, "\n")
}

// collapsedView renders the summary line, followed by any failures. The summary
// is always exactly one line so the group's cost is predictable.
func (g Group) collapsedView() string {
	summary := fmt.Sprintf("%s%d tool calls (%s)", dotPrefix, len(g.calls), g.toolNameSummary())
	line := dimStyle.Render(summary) + groupHintStyle.Render("  ctrl+o expand")

	failures := g.failureLines()
	if len(failures) == 0 {
		return line
	}
	return line + "\n" + strings.Join(failures, "\n")
}

// failureLines renders each failed member so errors survive collapsing.
func (g Group) failureLines() []string {
	var out []string
	for _, c := range g.calls {
		if !isFailedStatus(c.Status) {
			continue
		}
		if rendered := strings.TrimRight(c.View(), "\n"); rendered != "" {
			out = append(out, rendered)
		}
	}
	return out
}

// toolNameSummary lists the distinct tool names in the group, most frequent
// first, so the summary says what happened rather than only how much.
func (g Group) toolNameSummary() string {
	counts := make(map[string]int, len(g.calls))
	order := make([]string, 0, len(g.calls))
	for _, c := range g.calls {
		name := c.ToolName
		if name == "" {
			continue
		}
		if _, seen := counts[name]; !seen {
			order = append(order, name)
		}
		counts[name]++
	}
	if len(order) == 0 {
		return "tools"
	}
	// Stable ordering: by descending count, then by first appearance.
	first := make(map[string]int, len(order))
	for i, name := range order {
		first[name] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] > counts[order[j]]
		}
		return first[order[i]] < first[order[j]]
	})

	const maxNames = 3
	shown := order
	suffix := ""
	if len(shown) > maxNames {
		shown = shown[:maxNames]
		suffix = ", …"
	}
	return strings.Join(shown, ", ") + suffix
}

// isFailedStatus reports whether a status string marks a failed call.
func isFailedStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error":
		return true
	default:
		return false
	}
}
