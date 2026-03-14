package training

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RegressionConfig controls the regression guard thresholds.
type RegressionConfig struct {
	AccuracyDropThreshold float64 // default: -0.05 (-5%) -> auto-revert
	CostRiseThreshold     float64 // default: +0.15 (+15%) -> flag
	StepRiseThreshold     float64 // default: +0.20 (+20%) -> flag
	BenchmarkCmd          string  // command to run benchmarks, default: "make bench"
	BenchmarkDir          string  // directory to run in
	RepoPath              string
	TimeoutSecs           int // default: 300
}

// BenchmarkResult holds the results of a benchmark run.
type BenchmarkResult struct {
	PassRate    float64         `json:"pass_rate"`
	AvgCostUSD  float64        `json:"avg_cost_usd"`
	AvgSteps    float64        `json:"avg_steps"`
	TaskResults map[string]bool `json:"task_results"`
	RawOutput   string         `json:"raw_output,omitempty"`
}

// RegressionResult describes the outcome of a regression check.
type RegressionResult struct {
	Baseline      BenchmarkResult `json:"baseline"`
	Candidate     BenchmarkResult `json:"candidate"`
	AccuracyDelta float64         `json:"accuracy_delta"`
	CostDelta     float64         `json:"cost_delta"`
	StepDelta     float64         `json:"step_delta"`
	Decision      string          `json:"decision"` // "merge" | "revert" | "flag"
	Reason        string          `json:"reason"`
	Reverted      bool            `json:"reverted"`
}

// RegressionGuard runs benchmarks and decides whether to merge or revert.
type RegressionGuard struct {
	cfg RegressionConfig
}

// NewRegressionGuard creates a RegressionGuard with defaults applied.
func NewRegressionGuard(cfg RegressionConfig) *RegressionGuard {
	if cfg.AccuracyDropThreshold == 0 {
		cfg.AccuracyDropThreshold = -0.05
	}
	if cfg.CostRiseThreshold == 0 {
		cfg.CostRiseThreshold = 0.15
	}
	if cfg.StepRiseThreshold == 0 {
		cfg.StepRiseThreshold = 0.20
	}
	if cfg.TimeoutSecs == 0 {
		cfg.TimeoutSecs = 300
	}
	if cfg.RepoPath == "" {
		cfg.RepoPath = "."
	}
	return &RegressionGuard{cfg: cfg}
}

// Check runs the benchmark on the current branch and compares to baseline.
// If regression detected: auto-reverts the git branch.
func (g *RegressionGuard) Check(ctx context.Context, branchName string, baseline BenchmarkResult) (*RegressionResult, error) {
	candidate, err := g.RunBenchmark(ctx)
	if err != nil {
		return nil, fmt.Errorf("run benchmark: %w", err)
	}

	result := g.decide(baseline, *candidate)
	result.Baseline = baseline
	result.Candidate = *candidate

	switch result.Decision {
	case "revert":
		if err := g.Revert(ctx, branchName); err != nil {
			return result, fmt.Errorf("auto-revert failed: %w", err)
		}
		result.Reverted = true
	case "merge":
		// Caller decides whether to merge. We don't auto-merge in Check.
	case "flag":
		// Flagged for human review. No automatic action.
	}

	return result, nil
}

// decide computes deltas and decides merge/revert/flag.
func (g *RegressionGuard) decide(baseline, candidate BenchmarkResult) *RegressionResult {
	result := &RegressionResult{}

	// Accuracy delta: candidate - baseline (negative means regression).
	result.AccuracyDelta = candidate.PassRate - baseline.PassRate

	// Cost delta: relative change. (candidate - baseline) / baseline.
	if baseline.AvgCostUSD > 0 {
		result.CostDelta = (candidate.AvgCostUSD - baseline.AvgCostUSD) / baseline.AvgCostUSD
	}

	// Step delta: relative change.
	if baseline.AvgSteps > 0 {
		result.StepDelta = (candidate.AvgSteps - baseline.AvgSteps) / baseline.AvgSteps
	}

	// Decision logic (revert takes priority).
	if result.AccuracyDelta < g.cfg.AccuracyDropThreshold {
		result.Decision = "revert"
		result.Reason = fmt.Sprintf("accuracy dropped by %.2f%% (threshold: %.2f%%)",
			result.AccuracyDelta*100, g.cfg.AccuracyDropThreshold*100)
		return result
	}

	if result.CostDelta > g.cfg.CostRiseThreshold {
		result.Decision = "flag"
		result.Reason = fmt.Sprintf("cost increased by %.2f%% (threshold: %.2f%%)",
			result.CostDelta*100, g.cfg.CostRiseThreshold*100)
		return result
	}

	if result.StepDelta > g.cfg.StepRiseThreshold {
		result.Decision = "flag"
		result.Reason = fmt.Sprintf("steps increased by %.2f%% (threshold: %.2f%%)",
			result.StepDelta*100, g.cfg.StepRiseThreshold*100)
		return result
	}

	result.Decision = "merge"
	result.Reason = "no regression detected"
	return result
}

// RunBenchmark executes the benchmark command and parses results.
func (g *RegressionGuard) RunBenchmark(ctx context.Context) (*BenchmarkResult, error) {
	benchCmd := g.cfg.BenchmarkCmd
	if envCmd := os.Getenv("HARNESS_BENCHMARK_CMD"); envCmd != "" {
		benchCmd = envCmd
	}

	// If no benchmark command is configured, return a placeholder result.
	if benchCmd == "" {
		return &BenchmarkResult{
			PassRate:    0,
			AvgCostUSD:  0,
			AvgSteps:    0,
			TaskResults: map[string]bool{},
			RawOutput:   "no benchmark command configured",
		}, nil
	}

	timeout := time.Duration(g.cfg.TimeoutSecs) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	benchDir := g.cfg.BenchmarkDir
	if benchDir == "" {
		benchDir = g.cfg.RepoPath
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", benchCmd)
	cmd.Dir = benchDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("benchmark command failed: %w\n%s", err, out)
	}

	// Try to parse output as JSON BenchmarkResult.
	var result BenchmarkResult
	if jsonErr := json.Unmarshal(out, &result); jsonErr == nil {
		result.RawOutput = string(out)
		return &result, nil
	}

	// Fallback: return raw output with zero metrics.
	return &BenchmarkResult{
		RawOutput: string(out),
	}, nil
}

// baselineFile represents the on-disk baseline.json format.
type baselineFile struct {
	Tasks map[string]baselineTask `json:"tasks"`
}

type baselineTask struct {
	ExpectedPass bool    `json:"expected_pass"`
	AvgSteps     float64 `json:"avg_steps"`
	AvgCostUSD   float64 `json:"avg_cost_usd"`
}

// LoadBaseline reads benchmarks/terminal_bench/baseline.json and computes
// aggregate metrics from it.
func (g *RegressionGuard) LoadBaseline(repoPath string) (*BenchmarkResult, error) {
	path := filepath.Join(repoPath, "benchmarks", "terminal_bench", "baseline.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}

	var bf baselineFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}

	result := &BenchmarkResult{
		TaskResults: make(map[string]bool),
	}

	var totalCost, totalSteps float64
	var passCount int
	taskCount := len(bf.Tasks)

	for name, task := range bf.Tasks {
		result.TaskResults[name] = task.ExpectedPass
		if task.ExpectedPass {
			passCount++
		}
		totalCost += task.AvgCostUSD
		totalSteps += task.AvgSteps
	}

	if taskCount > 0 {
		result.PassRate = float64(passCount) / float64(taskCount)
		result.AvgCostUSD = totalCost / float64(taskCount)
		result.AvgSteps = totalSteps / float64(taskCount)
	}

	return result, nil
}

// Revert deletes the training branch.
func (g *RegressionGuard) Revert(ctx context.Context, branchName string) error {
	_, err := g.runGit("branch", "-D", branchName)
	if err != nil {
		return fmt.Errorf("delete branch %s: %w", branchName, err)
	}
	return nil
}

// Merge merges the training branch into main with --no-ff.
func (g *RegressionGuard) Merge(ctx context.Context, branchName string) error {
	if _, err := g.runGit("checkout", "main"); err != nil {
		return fmt.Errorf("checkout main: %w", err)
	}
	if _, err := g.runGit("merge", "--no-ff", branchName); err != nil {
		return fmt.Errorf("merge %s: %w", branchName, err)
	}
	return nil
}

// runGit runs a git command in the repo path.
func (g *RegressionGuard) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.cfg.RepoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}
