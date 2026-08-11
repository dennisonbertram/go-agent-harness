package tui

import "testing"

// usageDeltaPayload builds a usage.delta event body with a turn's own usage and
// the run's cumulative total, which are different quantities.
func usageDeltaPayload(turnPrompt, turnCompletion, cumulativeTotal int) string {
	return `{"turn_usage":{"prompt_tokens":` + itoa(turnPrompt) +
		`,"completion_tokens":` + itoa(turnCompletion) +
		`,"total_tokens":` + itoa(turnPrompt+turnCompletion) +
		`},"cumulative_usage":{"total_tokens":` + itoa(cumulativeTotal) +
		`},"cumulative_cost_usd":1.25}`
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestContextUsedTracksOccupancyNotCumulative is the regression test for issue
// #1307: the meter must report what is resident in the context window, not what
// the run has spent in total. Each turn re-sends the whole conversation, so the
// cumulative sum grows superlinearly and is not occupancy.
func TestContextUsedTracksOccupancyNotCumulative(t *testing.T) {
	m := Model{}
	m.contextGrid.TotalTokens = 200000

	// Three turns. Cumulative climbs past the window while each turn's own
	// prompt stays small — exactly the shape that produced a reading above 100%.
	m.applyUsageDelta([]byte(usageDeltaPayload(10000, 500, 10500)))
	m.applyUsageDelta([]byte(usageDeltaPayload(21000, 700, 32200)))
	m.applyUsageDelta([]byte(usageDeltaPayload(33000, 900, 66100)))

	const wantOccupancy = 33000 + 900
	if got := m.contextUsedTokens(); got != wantOccupancy {
		t.Errorf("contextUsedTokens() = %d, want %d (latest turn's prompt+completion)", got, wantOccupancy)
	}
}

// TestCumulativeCostStillTracksWholeRun is the false-positive control: occupancy
// must not be achieved by breaking cost accounting, which is a genuinely
// cumulative figure and belongs to the /cost overlay.
func TestCumulativeCostStillTracksWholeRun(t *testing.T) {
	m := Model{}
	m.applyUsageDelta([]byte(usageDeltaPayload(10000, 500, 10500)))

	if m.cumulativeCostUSD != 1.25 {
		t.Errorf("cumulativeCostUSD = %v, want 1.25", m.cumulativeCostUSD)
	}
}

// TestContextUsedIgnoresMalformedPayload guards the parse boundary: a payload the
// client cannot read must leave the last good reading alone rather than zeroing it.
func TestContextUsedIgnoresMalformedPayload(t *testing.T) {
	m := Model{}
	m.applyUsageDelta([]byte(usageDeltaPayload(12000, 300, 12300)))
	before := m.contextUsedTokens()

	m.applyUsageDelta([]byte(`{"turn_usage":`))

	if got := m.contextUsedTokens(); got != before {
		t.Errorf("malformed payload changed the reading from %d to %d", before, got)
	}
}
