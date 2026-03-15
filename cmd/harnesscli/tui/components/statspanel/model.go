package statspanel

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// DataPoint represents a single activity record.
type DataPoint struct {
	Date  time.Time
	Count int     // number of runs/messages
	Cost  float64 // USD cost
}

// Period controls the time range of the heatmap.
type Period int

const (
	PeriodWeek  Period = iota // last 7 days
	PeriodMonth               // last 30 days
	PeriodYear                // last 365 days
)

// periodDays returns the number of days for each Period value.
func periodDays(p Period) int {
	switch p {
	case PeriodWeek:
		return 7
	case PeriodMonth:
		return 30
	case PeriodYear:
		return 365
	default:
		return 30
	}
}

// periodLabel returns the human-readable label for a Period.
func periodLabel(p Period) string {
	switch p {
	case PeriodWeek:
		return "last 7 days"
	case PeriodMonth:
		return "last 30 days"
	case PeriodYear:
		return "last 365 days"
	default:
		return "last 30 days"
	}
}

// Model holds stats and display state.
// All methods use value receivers so the model is safe for concurrent use
// without any synchronization — each goroutine has its own copy.
type Model struct {
	data   []DataPoint
	period Period
	width  int
}

// New creates a new Model with the given data points.
func New(data []DataPoint) Model {
	pts := make([]DataPoint, len(data))
	copy(pts, data)
	return Model{data: pts, period: PeriodWeek}
}

// SetWidth returns a copy of the Model with the given display width.
func (m Model) SetWidth(w int) Model {
	m.width = w
	return m
}

// SetPeriod returns a copy of the Model with the given Period.
func (m Model) SetPeriod(p Period) Model {
	m.period = p
	return m
}

// TogglePeriod cycles through Week → Month → Year → Week and returns the new Model.
func (m Model) TogglePeriod() Model {
	switch m.period {
	case PeriodWeek:
		m.period = PeriodMonth
	case PeriodMonth:
		m.period = PeriodYear
	default:
		m.period = PeriodWeek
	}
	return m
}

// ActivePeriod returns the current Period.
func (m Model) ActivePeriod() Period {
	return m.period
}

// View renders the heatmap as a plain-text string.
func (m Model) View() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	return render(m.data, m.period, w)
}

// intensityBlock returns the block character for the given count given the sorted
// non-zero counts slice (used for percentile bucketing).
func intensityBlock(count int, sorted []int) rune {
	if count <= 0 || len(sorted) == 0 {
		return '░'
	}
	n := len(sorted)
	// Use the upper-bound index (first element strictly greater than count) so
	// equal values rank at the top of their group. This ensures that when all
	// counts are equal they all map to 100% and render as the highest intensity.
	rank := sort.SearchInts(sorted, count+1) // index of first element > count
	// Percentile (0..100).
	pct := float64(rank) / float64(n) * 100.0
	switch {
	case pct <= 25:
		return '░'
	case pct <= 75:
		return '▒'
	case pct <= 95:
		return '▓'
	default:
		return '█'
	}
}

// truncateDate strips time-of-day, normalised to UTC so map keys are comparable.
func truncateDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// render is the pure rendering function.
func render(data []DataPoint, period Period, width int) string {
	days := periodDays(period)
	label := periodLabel(period)

	// Build a map from truncated date → aggregated count and cost.
	type agg struct {
		count int
		cost  float64
	}
	byDay := make(map[time.Time]agg, len(data))
	for _, dp := range data {
		if dp.Date.IsZero() {
			continue
		}
		k := truncateDate(dp.Date)
		a := byDay[k]
		a.count += dp.Count
		a.cost += dp.Cost
		byDay[k] = a
	}

	// Build the ordered list of days in the window (oldest first).
	now := truncateDate(time.Now())
	window := make([]time.Time, days)
	for i := 0; i < days; i++ {
		window[i] = now.AddDate(0, 0, -(days - 1 - i))
	}

	// Collect non-zero counts for percentile computation.
	var nonZeroCounts []int
	for _, d := range window {
		if a, ok := byDay[d]; ok && a.count > 0 {
			nonZeroCounts = append(nonZeroCounts, a.count)
		}
	}
	sort.Ints(nonZeroCounts)

	// Build the heatmap grid: 7 rows (Mon–Sun).
	// Number of columns = ceil(days / 7).
	numCols := (days + 6) / 7

	// grid[row][col] where row 0=Mon … row 6=Sun.
	grid := make([][]rune, 7)
	for r := 0; r < 7; r++ {
		grid[r] = make([]rune, numCols)
		for c := 0; c < numCols; c++ {
			grid[r][c] = '░'
		}
	}

	// Assign each day in the window to its (row, col) in the grid.
	for i, d := range window {
		col := i / 7
		if col >= numCols {
			continue
		}
		// Go weekday: Sunday=0 … Saturday=6. Remap so Mon=0 … Sun=6.
		wd := int(d.Weekday())
		row := (wd + 6) % 7
		a := byDay[d]
		grid[row][col] = intensityBlock(a.count, nonZeroCounts)
	}

	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	var sb strings.Builder

	// Header line.
	sb.WriteString(fmt.Sprintf("Activity (%s)  [r to toggle period]\n\n", label))

	// Heatmap rows (Mon → Sun).
	for r := 0; r < 7; r++ {
		sb.WriteString(dayNames[r])
		sb.WriteString("  ")
		for c := 0; c < numCols; c++ {
			sb.WriteRune(grid[r][c])
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Totals: aggregate over the window only.
	var totalRuns int
	var totalCost float64
	for _, d := range window {
		a := byDay[d]
		totalRuns += a.count
		totalCost += a.cost
	}
	sb.WriteString(fmt.Sprintf("Total runs: %d   Total cost: $%.2f\n", totalRuns, totalCost))

	return sb.String()
}
