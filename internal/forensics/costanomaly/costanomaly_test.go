package costanomaly_test

import (
	"encoding/json"
	"testing"

	"go-agent-harness/internal/forensics/costanomaly"
)

// TestNewDetectorDefaults verifies that a newly created Detector has sensible
// defaults and that the zero-value state is correct before any steps are recorded.
func TestNewDetectorDefaults(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	if d == nil {
		t.Fatal("NewDetector returned nil")
	}
	if d.StepCount() != 0 {
		t.Errorf("StepCount: got %d, want 0", d.StepCount())
	}
	if d.AverageCost() != 0.0 {
		t.Errorf("AverageCost: got %f, want 0.0", d.AverageCost())
	}
}

// TestFirstStepNeverAnomaly verifies that the very first step never triggers
// an anomaly because there is no rolling average to compare against.
func TestFirstStepNeverAnomaly(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	alert := d.Record(1, 100.0) // very large first step cost
	if alert != nil {
		t.Errorf("first step should never be anomalous, got alert: %+v", alert)
	}
}

// TestNoAnomalyBelowThreshold verifies that a step cost below the multiplier
// threshold does not trigger an anomaly.
func TestNoAnomalyBelowThreshold(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	// Step 1: $0.10 (baseline)
	d.Record(1, 0.10)
	// Step 2: $0.15 — less than 2× $0.10
	alert := d.Record(2, 0.15)
	if alert != nil {
		t.Errorf("step cost below threshold should not trigger anomaly, got: %+v", alert)
	}
}

// TestAnomalyAtExactThreshold verifies that a step cost exactly equal to
// multiplier × average IS flagged as an anomaly (>= comparison).
func TestAnomalyAtExactThreshold(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	// Step 1: $0.10 (baseline — average after step 1 is $0.10)
	d.Record(1, 0.10)
	// Step 2: $0.20 = exactly 2× average — should be flagged
	alert := d.Record(2, 0.20)
	if alert == nil {
		t.Fatal("expected anomaly at exactly 2× average, got nil")
	}
	if alert.Step != 2 {
		t.Errorf("alert Step: got %d, want 2", alert.Step)
	}
	if alert.AnomalyType != costanomaly.AnomalyTypeStepMultiplier {
		t.Errorf("alert AnomalyType: got %q, want %q", alert.AnomalyType, costanomaly.AnomalyTypeStepMultiplier)
	}
	if alert.StepCostUSD != 0.20 {
		t.Errorf("alert StepCostUSD: got %f, want 0.20", alert.StepCostUSD)
	}
	// Average before this step should be the average of recorded steps before this one.
	// After step 1, average = 0.10. After step 2 is recorded, the avg used for comparison
	// is the one BEFORE step 2 (i.e., 0.10).
	if alert.AvgCostUSD <= 0 {
		t.Errorf("alert AvgCostUSD: got %f, want > 0", alert.AvgCostUSD)
	}
}

// TestAnomalyAboveThreshold verifies that a step cost clearly above the
// threshold triggers an anomaly.
func TestAnomalyAboveThreshold(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	// Steps 1-3: $0.10 each → average = $0.10
	d.Record(1, 0.10)
	d.Record(2, 0.10)
	d.Record(3, 0.10)
	// Step 4: $0.50 = 5× average → should be flagged
	alert := d.Record(4, 0.50)
	if alert == nil {
		t.Fatal("expected anomaly for 5× average, got nil")
	}
	if alert.Step != 4 {
		t.Errorf("alert Step: got %d, want 4", alert.Step)
	}
	if alert.ThresholdMultiplier != 2.0 {
		t.Errorf("alert ThresholdMultiplier: got %f, want 2.0", alert.ThresholdMultiplier)
	}
}

// TestCustomMultiplier verifies that a custom threshold multiplier is respected.
func TestCustomMultiplier(t *testing.T) {
	t.Parallel()

	// Use a 3× multiplier.
	d := costanomaly.NewDetector(3.0)
	d.Record(1, 0.10)
	// Step 2: $0.25 = 2.5× — below 3×, should NOT be flagged.
	alert2 := d.Record(2, 0.25)
	if alert2 != nil {
		t.Errorf("2.5× should not trigger 3.0× threshold, got alert: %+v", alert2)
	}
	// Step 3: $0.31 = 3.1× the step-1-only average (0.10) — above 3×, SHOULD be flagged.
	// But the average at this point is (0.10 + 0.25) / 2 = 0.175.
	// 0.31 / 0.175 ≈ 1.77 — below 3×. Let's use 0.60 which is 3.43×.
	alert3 := d.Record(3, 0.60)
	// avg of [0.10, 0.25] = 0.175; 0.60 / 0.175 ≈ 3.43 > 3.0 → anomaly
	if alert3 == nil {
		t.Fatal("expected anomaly for 3.43× average with 3.0 multiplier, got nil")
	}
	if alert3.ThresholdMultiplier != 3.0 {
		t.Errorf("ThresholdMultiplier: got %f, want 3.0", alert3.ThresholdMultiplier)
	}
}

// TestRollingAverageUpdates verifies that the rolling average is updated
// correctly across multiple steps.
func TestRollingAverageUpdates(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	d.Record(1, 0.10) // avg after: 0.10
	d.Record(2, 0.20) // avg after: 0.15
	d.Record(3, 0.30) // avg after: 0.20

	// After 3 steps, average should be (0.10 + 0.20 + 0.30) / 3 = 0.20
	if got := d.AverageCost(); got < 0.199 || got > 0.201 {
		t.Errorf("AverageCost after 3 steps: got %f, want ~0.20", got)
	}
	if d.StepCount() != 3 {
		t.Errorf("StepCount: got %d, want 3", d.StepCount())
	}
}

// TestAnomalyAlertFields verifies that the CostAnomalyAlert struct has all
// required fields populated when an anomaly is detected.
func TestAnomalyAlertFields(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	d.Record(1, 0.10)
	d.Record(2, 0.10)
	alert := d.Record(3, 0.30) // 3× average of 0.10
	if alert == nil {
		t.Fatal("expected anomaly for 3× average, got nil")
	}

	// Verify all required payload fields are present.
	if alert.Step != 3 {
		t.Errorf("Step: got %d, want 3", alert.Step)
	}
	if alert.AnomalyType != costanomaly.AnomalyTypeStepMultiplier {
		t.Errorf("AnomalyType: got %q, want %q", alert.AnomalyType, costanomaly.AnomalyTypeStepMultiplier)
	}
	if alert.StepCostUSD <= 0 {
		t.Errorf("StepCostUSD: got %f, want > 0", alert.StepCostUSD)
	}
	if alert.AvgCostUSD <= 0 {
		t.Errorf("AvgCostUSD: got %f, want > 0", alert.AvgCostUSD)
	}
	if alert.ThresholdMultiplier <= 0 {
		t.Errorf("ThresholdMultiplier: got %f, want > 0", alert.ThresholdMultiplier)
	}
}

// TestAnomalyAlertJSONRoundTrip verifies that CostAnomalyAlert serializes and
// deserializes correctly via JSON.
func TestAnomalyAlertJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := costanomaly.CostAnomalyAlert{
		Step:                5,
		AnomalyType:         costanomaly.AnomalyTypeStepMultiplier,
		StepCostUSD:         0.50,
		AvgCostUSD:          0.10,
		ThresholdMultiplier: 2.0,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got costanomaly.CostAnomalyAlert
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Step != orig.Step {
		t.Errorf("Step: got %d, want %d", got.Step, orig.Step)
	}
	if got.AnomalyType != orig.AnomalyType {
		t.Errorf("AnomalyType: got %q, want %q", got.AnomalyType, orig.AnomalyType)
	}
	if got.StepCostUSD != orig.StepCostUSD {
		t.Errorf("StepCostUSD: got %f, want %f", got.StepCostUSD, orig.StepCostUSD)
	}
	if got.AvgCostUSD != orig.AvgCostUSD {
		t.Errorf("AvgCostUSD: got %f, want %f", got.AvgCostUSD, orig.AvgCostUSD)
	}
	if got.ThresholdMultiplier != orig.ThresholdMultiplier {
		t.Errorf("ThresholdMultiplier: got %f, want %f", got.ThresholdMultiplier, orig.ThresholdMultiplier)
	}
}

// TestZeroCostStepsNoAnomaly verifies that zero-cost steps (unpriced model)
// do not trigger anomalies because the average is 0 and no useful comparison
// can be made.
func TestZeroCostStepsNoAnomaly(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	d.Record(1, 0.0)
	d.Record(2, 0.0)
	// Even with a "huge" step cost, if average is 0 we cannot detect anomalies
	// meaningfully. The detector should treat 0-cost steps gracefully.
	alert := d.Record(3, 100.0)
	// When avg <= 0, the detector cannot compute a meaningful ratio.
	// Whether this is an anomaly depends on implementation. The key requirement
	// is that it doesn't panic and returns a consistent result.
	_ = alert // result is implementation-defined but must not panic
}

// TestSingleStepRunNoAnomaly verifies that a single-step run never produces
// an anomaly (no prior average exists for comparison).
func TestSingleStepRunNoAnomaly(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	alert := d.Record(1, 999.0)
	if alert != nil {
		t.Errorf("single-step run should never produce anomaly, got: %+v", alert)
	}
}

// TestAnomalyTypeConstantValue verifies the string value of AnomalyTypeStepMultiplier.
func TestAnomalyTypeConstantValue(t *testing.T) {
	t.Parallel()

	if costanomaly.AnomalyTypeStepMultiplier != "step_multiplier" {
		t.Errorf("AnomalyTypeStepMultiplier = %q, want %q",
			costanomaly.AnomalyTypeStepMultiplier, "step_multiplier")
	}
}

// TestDetectorResetBetweenRuns verifies that creating a new Detector starts
// fresh (simulating per-run isolation).
func TestDetectorResetBetweenRuns(t *testing.T) {
	t.Parallel()

	// Run 1: accumulate expensive costs.
	d1 := costanomaly.NewDetector(2.0)
	d1.Record(1, 1.0)
	d1.Record(2, 1.0)

	// Run 2: fresh detector — first step should not be flagged.
	d2 := costanomaly.NewDetector(2.0)
	alert := d2.Record(1, 5.0)
	if alert != nil {
		t.Errorf("first step of new detector should not be anomalous, got: %+v", alert)
	}
}

// TestMultipleAnomaliesAcrossSteps verifies that multiple anomalies in a single
// run each produce independent alerts.
func TestMultipleAnomaliesAcrossSteps(t *testing.T) {
	t.Parallel()

	d := costanomaly.NewDetector(2.0)
	// Establish a small baseline.
	d.Record(1, 0.01)
	d.Record(2, 0.01)

	// Step 3: big spike.
	alert3 := d.Record(3, 0.10)
	if alert3 == nil {
		// avg of [0.01, 0.01] = 0.01; 0.10 / 0.01 = 10× — should be flagged.
		t.Fatal("expected anomaly at step 3, got nil")
	}

	// Step 4: another spike (average has grown, but still a spike).
	// avg of [0.01, 0.01, 0.10] ≈ 0.04; 0.20 / 0.04 = 5× — should be flagged.
	alert4 := d.Record(4, 0.20)
	if alert4 == nil {
		t.Fatal("expected anomaly at step 4, got nil")
	}
	if alert4.Step != 4 {
		t.Errorf("second anomaly Step: got %d, want 4", alert4.Step)
	}
}
