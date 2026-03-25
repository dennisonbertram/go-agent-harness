package training

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBaseline(t *testing.T) {
	dir := t.TempDir()
	benchDir := filepath.Join(dir, "benchmarks", "terminal_bench")
	if err := os.MkdirAll(benchDir, 0o755); err != nil {
		t.Fatal(err)
	}

	baseline := map[string]any{
		"tasks": map[string]any{
			"go-race-fix": map[string]any{
				"expected_pass":     true,
				"avg_steps":         10.0,
				"avg_cost_usd":      0.04,
				"avg_wall_time_sec": 36.0,
			},
			"go-rename": map[string]any{
				"expected_pass":     true,
				"avg_steps":         15.0,
				"avg_cost_usd":      0.06,
				"avg_wall_time_sec": 180.0,
			},
			"staging-deploy": map[string]any{
				"expected_pass":     false,
				"avg_steps":         8.0,
				"avg_cost_usd":      0.03,
				"avg_wall_time_sec": 50.0,
			},
		},
	}
	data, _ := json.MarshalIndent(baseline, "", "  ")
	if err := os.WriteFile(filepath.Join(benchDir, "baseline.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewRegressionGuard(RegressionConfig{})

	result, err := g.LoadBaseline(dir)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}

	// 2 out of 3 tasks have expected_pass=true
	if result.PassRate != 2.0/3.0 {
		t.Errorf("PassRate = %f, want %f", result.PassRate, 2.0/3.0)
	}

	// avg_cost_usd across all tasks: (0.04 + 0.06 + 0.03) / 3
	expectedCost := (0.04 + 0.06 + 0.03) / 3.0
	if abs(result.AvgCostUSD-expectedCost) > 0.001 {
		t.Errorf("AvgCostUSD = %f, want ~%f", result.AvgCostUSD, expectedCost)
	}

	// avg_steps across all tasks: (10 + 15 + 8) / 3
	expectedSteps := (10.0 + 15.0 + 8.0) / 3.0
	if abs(result.AvgSteps-expectedSteps) > 0.001 {
		t.Errorf("AvgSteps = %f, want ~%f", result.AvgSteps, expectedSteps)
	}

	// TaskResults: 2 pass, 1 fail
	if len(result.TaskResults) != 3 {
		t.Errorf("TaskResults len = %d, want 3", len(result.TaskResults))
	}
	if !result.TaskResults["go-race-fix"] {
		t.Error("go-race-fix should be true")
	}
	if result.TaskResults["staging-deploy"] {
		t.Error("staging-deploy should be false")
	}
}

func TestLoadBaseline_FileNotFound(t *testing.T) {
	g := NewRegressionGuard(RegressionConfig{})
	_, err := g.LoadBaseline(t.TempDir())
	if err == nil {
		t.Error("expected error for missing baseline.json")
	}
}

func TestLoadBaseline_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	benchDir := filepath.Join(dir, "benchmarks", "terminal_bench")
	if err := os.MkdirAll(benchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(benchDir, "baseline.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := NewRegressionGuard(RegressionConfig{})
	_, err := g.LoadBaseline(dir)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

// Table-driven tests for decision logic
func TestDecisionLogic(t *testing.T) {
	tests := []struct {
		name         string
		baseline     BenchmarkResult
		candidate    BenchmarkResult
		accuracyDrop float64
		costRise     float64
		stepRise     float64
		wantDecision string
	}{
		{
			name:         "merge_no_regression",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.85, AvgCostUSD: 0.05, AvgSteps: 10},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "merge",
		},
		{
			name:         "revert_accuracy_drop",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.70, AvgCostUSD: 0.05, AvgSteps: 10},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "revert",
		},
		{
			name:         "flag_cost_rise",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.07, AvgSteps: 10},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "flag",
		},
		{
			name:         "flag_step_rise",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 13},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "flag",
		},
		{
			name:         "revert_takes_priority_over_flag",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.70, AvgCostUSD: 0.07, AvgSteps: 13},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "revert",
		},
		{
			name:         "exact_threshold_accuracy_no_revert",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.76, AvgCostUSD: 0.05, AvgSteps: 10},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "merge", // -0.04 delta, threshold is -0.05, so no revert
		},
		{
			name:         "just_past_threshold_accuracy_revert",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.74, AvgCostUSD: 0.05, AvgSteps: 10},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "revert", // -0.06 delta is below -0.05 threshold
		},
		{
			name:         "exact_threshold_cost_no_flag",
			baseline:     BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.80, AvgCostUSD: 0.0575, AvgSteps: 10},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "merge", // exactly 15%, threshold > (strict), so no flag
		},
		{
			name:         "zero_baseline_passrate",
			baseline:     BenchmarkResult{PassRate: 0.0, AvgCostUSD: 0.05, AvgSteps: 10},
			candidate:    BenchmarkResult{PassRate: 0.0, AvgCostUSD: 0.05, AvgSteps: 10},
			accuracyDrop: -0.05,
			costRise:     0.15,
			stepRise:     0.20,
			wantDecision: "merge", // no change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewRegressionGuard(RegressionConfig{
				AccuracyDropThreshold: tt.accuracyDrop,
				CostRiseThreshold:     tt.costRise,
				StepRiseThreshold:     tt.stepRise,
			})

			result := g.decide(tt.baseline, tt.candidate)
			if result.Decision != tt.wantDecision {
				t.Errorf("Decision = %q, want %q (accuracy_delta=%.4f cost_delta=%.4f step_delta=%.4f)",
					result.Decision, tt.wantDecision,
					result.AccuracyDelta, result.CostDelta, result.StepDelta)
			}
		})
	}
}

func TestRunBenchmark_NoCmd(t *testing.T) {
	g := NewRegressionGuard(RegressionConfig{
		BenchmarkCmd: "",
	})

	result, err := g.RunBenchmark(context.Background())
	if err != nil {
		t.Fatalf("RunBenchmark: %v", err)
	}
	// Without a command, returns a placeholder result
	if result.PassRate != 0 {
		t.Errorf("PassRate = %f, want 0 for placeholder", result.PassRate)
	}
}

func TestRevert(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create a branch to revert
	cmd := exec.Command("git", "branch", "training/test-branch")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}

	g := NewRegressionGuard(RegressionConfig{
		RepoPath: dir,
	})

	if err := g.Revert(context.Background(), "training/test-branch"); err != nil {
		t.Fatalf("Revert: %v", err)
	}

	// Verify branch no longer exists
	cmd = exec.Command("git", "branch", "--list", "training/test-branch")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if strings.Contains(string(out), "training/test-branch") {
		t.Error("branch should have been deleted")
	}
}

func TestMerge(t *testing.T) {
	dir := t.TempDir()
	initGitRepoWithBranch(t, dir, "trunk")
	baseBranch := currentGitBranch(t, dir)

	// Create and switch to a training branch, make a commit
	for _, args := range [][]string{
		{"checkout", "-b", "training/merge-test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Create a file and commit on the branch
	testFile := filepath.Join(dir, "training-change.txt")
	if err := os.WriteFile(testFile, []byte("training change"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "training-change.txt"},
		{"commit", "-m", "training change"},
		{"checkout", baseBranch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	g := NewRegressionGuard(RegressionConfig{
		RepoPath: dir,
	})

	if err := g.Merge(context.Background(), "training/merge-test"); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	if got := currentGitBranch(t, dir); got != baseBranch {
		t.Fatalf("current branch = %q, want %q", got, baseBranch)
	}

	// Verify the file exists on the repository base branch now
	if _, err := os.Stat(testFile); err != nil {
		t.Error("training-change.txt should exist on the base branch after merge")
	}
}

func TestCheck_MockBenchmark(t *testing.T) {
	dir := t.TempDir()
	initGitRepoWithBranch(t, dir, "trunk")
	baseBranch := currentGitBranch(t, dir)

	// Create a training branch from the repository base branch.
	for _, args := range [][]string{
		{"checkout", "-b", "training/check-test"},
		{"checkout", baseBranch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	g := NewRegressionGuard(RegressionConfig{
		RepoPath:              dir,
		BenchmarkCmd:          "", // empty = placeholder mode
		AccuracyDropThreshold: -0.05,
		CostRiseThreshold:     0.15,
		StepRiseThreshold:     0.20,
	})

	baseline := BenchmarkResult{
		PassRate:   0.80,
		AvgCostUSD: 0.05,
		AvgSteps:   10,
	}

	result, err := g.Check(context.Background(), "training/check-test", baseline)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	// With placeholder benchmark (PassRate=0), accuracy drops below threshold
	if result.Decision != "revert" {
		t.Errorf("Decision = %q, want revert (placeholder benchmark has 0 pass rate)", result.Decision)
	}
	if !result.Reverted {
		t.Error("expected Reverted=true")
	}
}

func TestRegressionConfig_Defaults(t *testing.T) {
	g := NewRegressionGuard(RegressionConfig{})

	if g.cfg.AccuracyDropThreshold != -0.05 {
		t.Errorf("AccuracyDropThreshold = %f, want -0.05", g.cfg.AccuracyDropThreshold)
	}
	if g.cfg.CostRiseThreshold != 0.15 {
		t.Errorf("CostRiseThreshold = %f, want 0.15", g.cfg.CostRiseThreshold)
	}
	if g.cfg.StepRiseThreshold != 0.20 {
		t.Errorf("StepRiseThreshold = %f, want 0.20", g.cfg.StepRiseThreshold)
	}
	if g.cfg.TimeoutSecs != 300 {
		t.Errorf("TimeoutSecs = %d, want 300", g.cfg.TimeoutSecs)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
