package tui

import (
	"encoding/json"

	"go-agent-harness/cmd/harnesscli/tui/components/statspanel"
)

// contextUsedTokens returns how much of the model's context window the next
// request will carry: the most recent turn's prompt plus its completion.
//
// This is deliberately not the run's cumulative token total. Every turn re-sends
// the whole conversation, so summing turns grows superlinearly and measures
// spend rather than occupancy — which is how the meter reached 131% of its
// window (issue #1307). Cumulative spend is still reported, by the /cost overlay.
func (m Model) contextUsedTokens() int { return m.contextOccupancyTokens }

// applyUsageDelta folds a usage.delta event into the model: cumulative figures
// drive cost and the stats panel, while the latest turn's own usage drives the
// context meter.
//
// A payload that fails to parse leaves every reading untouched. Zeroing the
// meter on a malformed frame would read as "context freed", which is worse than
// showing a slightly stale number.
func (m *Model) applyUsageDelta(raw []byte) {
	var p struct {
		TurnUsage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"turn_usage"`
		CumulativeUsage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"cumulative_usage"`
		CumulativeCostUSD float64 `json:"cumulative_cost_usd"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}

	m.cumulativeCostUSD = p.CumulativeCostUSD
	m.statusBar.SetCost(m.cumulativeCostUSD)
	// totalTokens stays cumulative: it feeds cost and accounting surfaces.
	m.totalTokens = p.CumulativeUsage.TotalTokens
	m.usageDataPoints = upsertTodayDataPoint(m.usageDataPoints, 1, p.CumulativeCostUSD)
	m.statsPanel = statspanel.New(m.usageDataPoints)

	if occupancy := p.TurnUsage.PromptTokens + p.TurnUsage.CompletionTokens; occupancy > 0 {
		m.contextOccupancyTokens = occupancy
	}
	m.contextGrid.UsedTokens = m.contextUsedTokens()
	m.statusBar.SetContext(m.contextUsedTokens(), m.contextWindowTotal())
	// Keep the /cost overlay's snapshot current even while it is open.
	m.costDisplay = m.costDisplay.Update(costSnapshotFromModel(m))
}
