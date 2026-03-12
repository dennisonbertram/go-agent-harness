// Package costanomaly provides cost anomaly detection for the agent harness.
// It detects when a single step's cost is disproportionately high relative
// to the rolling average of prior steps in the same run.
package costanomaly

// AnomalyType identifies which anomaly condition was triggered.
type AnomalyType string

const (
	// AnomalyTypeStepMultiplier is emitted when a single step costs more than
	// N× the rolling average of all prior steps, where N is the configured
	// multiplier (default 2.0).
	AnomalyTypeStepMultiplier AnomalyType = "step_multiplier"
)

// CostAnomalyAlert describes a detected cost anomaly in a single run step.
type CostAnomalyAlert struct {
	// Step is the 1-based step number that triggered the anomaly.
	Step int `json:"step"`
	// AnomalyType is the category of anomaly detected.
	AnomalyType AnomalyType `json:"anomaly_type"`
	// StepCostUSD is the cost of the triggering step in US dollars.
	StepCostUSD float64 `json:"step_cost_usd"`
	// AvgCostUSD is the rolling average cost per step before the triggering step.
	AvgCostUSD float64 `json:"avg_cost_usd"`
	// ThresholdMultiplier is the configured multiplier threshold used to detect
	// this anomaly (e.g. 2.0 means 2× average).
	ThresholdMultiplier float64 `json:"threshold_multiplier"`
}

// Detector tracks per-step costs for a single run and detects anomalies.
// It is not safe for concurrent use; callers must synchronize externally if
// multiple goroutines need to share a Detector.
type Detector struct {
	multiplier float64
	stepCount  int
	totalCost  float64
}

// NewDetector creates a new Detector with the given threshold multiplier.
// A multiplier of 2.0 means a step that costs >= 2× the rolling average is
// flagged as anomalous. The multiplier must be > 0; callers should use the
// default (2.0) when in doubt.
func NewDetector(multiplier float64) *Detector {
	if multiplier <= 0 {
		multiplier = 2.0
	}
	return &Detector{multiplier: multiplier}
}

// Record records the cost of one step and checks whether it is anomalous.
// It returns a non-nil *CostAnomalyAlert when the step's cost exceeds the
// threshold. It always returns nil for the very first step because there is
// no prior average to compare against.
//
// The comparison uses the rolling average BEFORE the current step is added,
// so the current step does not inflate its own baseline.
func (d *Detector) Record(step int, costUSD float64) *CostAnomalyAlert {
	// Compute the average before adding this step.
	prevAvg := 0.0
	if d.stepCount > 0 {
		prevAvg = d.totalCost / float64(d.stepCount)
	}

	// Always update state, regardless of anomaly detection.
	d.stepCount++
	d.totalCost += costUSD

	// First step: no prior average exists — never an anomaly.
	if d.stepCount == 1 {
		return nil
	}

	// When the previous average is 0 (e.g., all prior steps had zero cost),
	// we cannot compute a meaningful ratio. Skip detection to avoid false
	// positives on unpriced models.
	if prevAvg <= 0 {
		return nil
	}

	// Emit an alert when the step cost is >= multiplier × rolling average.
	if costUSD >= d.multiplier*prevAvg {
		return &CostAnomalyAlert{
			Step:                step,
			AnomalyType:         AnomalyTypeStepMultiplier,
			StepCostUSD:         costUSD,
			AvgCostUSD:          prevAvg,
			ThresholdMultiplier: d.multiplier,
		}
	}

	return nil
}

// StepCount returns the number of steps recorded so far.
func (d *Detector) StepCount() int {
	return d.stepCount
}

// AverageCost returns the current rolling average cost per step.
// Returns 0 when no steps have been recorded.
func (d *Detector) AverageCost() float64 {
	if d.stepCount == 0 {
		return 0
	}
	return d.totalCost / float64(d.stepCount)
}
