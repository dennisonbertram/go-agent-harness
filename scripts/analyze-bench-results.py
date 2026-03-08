#!/usr/bin/env python3
"""Analyze terminal bench results and generate a markdown report.

Reads results.json, per-trial results, and harness_telemetry.json files
from a bench run directory. Compares against baseline.json and outputs
a human-readable markdown report.

Usage:
    ./scripts/analyze-bench-results.py [RESULTS_DIR] [-o OUTPUT_FILE]

If RESULTS_DIR is omitted, finds the latest run in .tmp/terminal-bench/.
"""

import argparse
import json
import sys
from datetime import datetime
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
BASELINE_PATH = REPO_ROOT / "benchmarks" / "terminal_bench" / "baseline.json"
DEFAULT_RESULTS_BASE = REPO_ROOT / ".tmp" / "terminal-bench"


def find_latest_results_dir(base: Path) -> Path | None:
    """Find the most recently modified results directory."""
    if not base.exists():
        return None
    candidates = sorted(base.iterdir(), key=lambda p: p.stat().st_mtime, reverse=True)
    for candidate in candidates:
        if candidate.is_dir():
            return candidate
    return None


def find_run_dir(results_dir: Path) -> Path | None:
    """Find the run subdirectory (e.g. 2026-03-08__00-53-33) inside results_dir."""
    for child in results_dir.iterdir():
        if child.is_dir() and "__" in child.name:
            return child
    # Maybe results_dir itself contains results.json
    if (results_dir / "results.json").exists():
        return results_dir
    return None


def load_baseline() -> dict:
    """Load baseline.json if it exists."""
    if BASELINE_PATH.exists():
        data = json.loads(BASELINE_PATH.read_text())
        return data.get("tasks", {})
    return {}


def load_telemetry(run_dir: Path, task_id: str) -> dict | None:
    """Load harness_telemetry.json for a given task trial."""
    # Trial dirs follow pattern: <task_id>/<trial_name>/
    task_dir = run_dir / task_id
    if not task_dir.exists():
        return None
    for trial_dir in task_dir.iterdir():
        if not trial_dir.is_dir():
            continue
        telemetry_path = trial_dir / "harness_telemetry.json"
        if telemetry_path.exists():
            return json.loads(telemetry_path.read_text())
        # Also check agent-logs subdirectory
        agent_logs_telemetry = trial_dir / "agent-logs" / "harness_telemetry.json"
        if agent_logs_telemetry.exists():
            return json.loads(agent_logs_telemetry.read_text())
    return None


def parse_iso(ts: str | None) -> datetime | None:
    """Parse an ISO timestamp string."""
    if not ts:
        return None
    # Handle timezone offset
    ts = ts.replace("+00:00", "+0000").replace("Z", "+0000")
    try:
        return datetime.fromisoformat(ts.replace("+0000", "+00:00"))
    except ValueError:
        return None


def compute_wall_time(result: dict) -> float | None:
    """Compute agent wall time in seconds from timestamps."""
    start = parse_iso(result.get("agent_started_at"))
    end = parse_iso(result.get("agent_ended_at"))
    if start and end:
        return (end - start).total_seconds()
    return None


def generate_report(run_dir: Path, results: dict, metadata: dict, baseline: dict) -> str:
    """Generate the full markdown report."""
    lines: list[str] = []
    task_results = results.get("results", [])
    n_resolved = results.get("n_resolved", 0)
    n_total = len(task_results)
    accuracy = results.get("accuracy", 0)

    # Header
    run_id = metadata.get("run_id", run_dir.name)
    commit = metadata.get("commit_hash", "unknown")[:8]
    start_time = metadata.get("start_time", "")
    end_time = metadata.get("end_time", "")

    lines.append(f"# Terminal Bench Report: {run_id}")
    lines.append("")
    lines.append(f"- **Commit**: `{commit}`")
    lines.append(f"- **Accuracy**: {n_resolved}/{n_total} ({accuracy:.1%})")
    if start_time and end_time:
        s = parse_iso(start_time)
        e = parse_iso(end_time)
        if s and e:
            total_mins = (e - s).total_seconds() / 60
            lines.append(f"- **Total wall time**: {total_mins:.1f} min")
    lines.append("")

    # --- Pass/Fail vs Baseline ---
    lines.append("## Pass/Fail vs Baseline")
    lines.append("")

    regressions = []
    improvements = []
    new_tasks = []

    lines.append("| Task | Result | Baseline | Delta |")
    lines.append("|------|--------|----------|-------|")

    for tr in sorted(task_results, key=lambda r: r["task_id"]):
        task_id = tr["task_id"]
        passed = tr.get("is_resolved", False)
        result_str = "PASS" if passed else "FAIL"
        bl = baseline.get(task_id)

        if bl is None:
            baseline_str = "N/A"
            delta = "NEW"
            new_tasks.append(task_id)
        else:
            expected = bl.get("expected_pass", True)
            baseline_str = "PASS" if expected else "FAIL"
            if passed and not expected:
                delta = "IMPROVED"
                improvements.append(task_id)
            elif not passed and expected:
                delta = "**REGRESSION**"
                regressions.append(task_id)
            else:
                delta = "---"

        lines.append(f"| {task_id} | {result_str} | {baseline_str} | {delta} |")

    lines.append("")

    if regressions:
        lines.append(f"> **REGRESSIONS**: {', '.join(regressions)}")
        lines.append("")
    if improvements:
        lines.append(f"> **IMPROVEMENTS**: {', '.join(improvements)}")
        lines.append("")

    # --- Efficiency Table ---
    lines.append("## Efficiency")
    lines.append("")
    lines.append("| Task | Difficulty | Steps | Tokens | Cost | Wall Time | Status |")
    lines.append("|------|-----------|-------|--------|------|-----------|--------|")

    total_tokens = 0
    total_cost = 0.0
    total_steps = 0

    for tr in sorted(task_results, key=lambda r: r["task_id"]):
        task_id = tr["task_id"]
        telemetry = load_telemetry(run_dir, task_id)
        bl = baseline.get(task_id, {})
        difficulty = bl.get("difficulty", "?")
        wall = compute_wall_time(tr)
        wall_str = f"{wall:.0f}s" if wall else "N/A"

        if telemetry:
            steps = telemetry.get("steps_taken", "?")
            prompt_tok = telemetry.get("total_prompt_tokens", 0)
            comp_tok = telemetry.get("total_completion_tokens", 0)
            tokens = prompt_tok + comp_tok
            cost = telemetry.get("total_cost_usd", 0)
            status = telemetry.get("status", "?")
            total_tokens += tokens
            total_cost += cost
            if isinstance(steps, int):
                total_steps += steps
            lines.append(
                f"| {task_id} | {difficulty} | {steps} | {tokens:,} | ${cost:.3f} | {wall_str} | {status} |"
            )
        else:
            # Use framework token counts as fallback
            input_tok = tr.get("total_input_tokens", 0)
            output_tok = tr.get("total_output_tokens", 0)
            tokens = input_tok + output_tok
            total_tokens += tokens
            tok_str = f"{tokens:,}" if tokens > 0 else "N/A"
            status = "pass" if tr.get("is_resolved") else "fail"
            lines.append(
                f"| {task_id} | {difficulty} | N/A | {tok_str} | N/A | {wall_str} | {status} |"
            )

    lines.append("")
    lines.append(f"**Totals**: {total_steps} steps, {total_tokens:,} tokens, ${total_cost:.3f}")
    lines.append("")

    # --- Tool Usage Patterns ---
    lines.append("## Tool Usage Patterns")
    lines.append("")

    any_tool_data = False
    for tr in sorted(task_results, key=lambda r: r["task_id"]):
        task_id = tr["task_id"]
        telemetry = load_telemetry(run_dir, task_id)
        if not telemetry:
            continue
        tool_calls = telemetry.get("tool_calls", [])
        if not tool_calls:
            continue
        any_tool_data = True

        # Count tool frequency
        tool_freq: dict[str, int] = {}
        for tc in tool_calls:
            name = tc.get("tool_name", "unknown")
            tool_freq[name] = tool_freq.get(name, 0) + 1

        sorted_tools = sorted(tool_freq.items(), key=lambda x: -x[1])
        tool_summary = ", ".join(f"{name}({count})" for name, count in sorted_tools[:8])
        lines.append(f"- **{task_id}**: {tool_summary}")

    if not any_tool_data:
        lines.append("_No telemetry tool call data available. Enable harness telemetry to populate._")
    lines.append("")

    # --- Failure Analysis ---
    failed = [tr for tr in task_results if not tr.get("is_resolved", False)]
    if failed:
        lines.append("## Failure Analysis")
        lines.append("")

        for tr in failed:
            task_id = tr["task_id"]
            lines.append(f"### {task_id}")
            lines.append("")

            # Show which tests failed
            parser_results = tr.get("parser_results", {})
            failed_tests = [t for t, r in parser_results.items() if r != "passed"]
            if failed_tests:
                lines.append(f"- **Failed tests**: {', '.join(failed_tests)}")

            telemetry = load_telemetry(run_dir, task_id)
            if telemetry:
                steps = telemetry.get("steps_taken", "?")
                status = telemetry.get("status", "?")
                lines.append(f"- **Steps taken**: {steps}")
                lines.append(f"- **Status**: {status}")

                tool_calls = telemetry.get("tool_calls", [])
                if tool_calls:
                    last_calls = tool_calls[-3:]
                    last_tools = [tc.get("tool_name", "?") for tc in last_calls]
                    lines.append(f"- **Last tool calls**: {', '.join(last_tools)}")
            else:
                wall = compute_wall_time(tr)
                if wall:
                    lines.append(f"- **Agent wall time**: {wall:.0f}s")

            lines.append("")

    # --- Test Detail ---
    lines.append("## Test Results Detail")
    lines.append("")

    for tr in sorted(task_results, key=lambda r: r["task_id"]):
        task_id = tr["task_id"]
        parser_results = tr.get("parser_results", {})
        passed_count = sum(1 for v in parser_results.values() if v == "passed")
        total_count = len(parser_results)
        icon = "PASS" if tr.get("is_resolved") else "FAIL"
        lines.append(f"**{task_id}** [{icon}] ({passed_count}/{total_count} tests)")

        for test_name, test_result in parser_results.items():
            mark = "x" if test_result == "passed" else " "
            lines.append(f"  - [{mark}] {test_name}")

        lines.append("")

    # --- Cost Summary ---
    lines.append("## Cost Summary")
    lines.append("")

    if total_cost > 0:
        lines.append(f"- **Total bench cost**: ${total_cost:.3f}")
        lines.append(f"- **Avg cost per task**: ${total_cost / n_total:.3f}")
        if n_resolved > 0:
            lines.append(f"- **Cost per passing task**: ${total_cost / n_resolved:.3f}")
    else:
        lines.append("_No cost data available. Enable harness telemetry to populate._")

    lines.append("")
    lines.append("---")
    lines.append(f"_Generated by analyze-bench-results.py at {datetime.now().isoformat(timespec='seconds')}_")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="Analyze terminal bench results")
    parser.add_argument(
        "results_dir",
        nargs="?",
        help="Path to bench results directory (default: latest in .tmp/terminal-bench/)",
    )
    parser.add_argument("-o", "--output", help="Write report to this file (in addition to stdout)")
    args = parser.parse_args()

    if args.results_dir:
        results_dir = Path(args.results_dir)
    else:
        results_dir = find_latest_results_dir(DEFAULT_RESULTS_BASE)

    if not results_dir or not results_dir.exists():
        print("No results directory found.", file=sys.stderr)
        sys.exit(1)

    run_dir = find_run_dir(results_dir)
    if not run_dir:
        print(f"No run directory found in {results_dir}", file=sys.stderr)
        sys.exit(1)

    results_path = run_dir / "results.json"
    if not results_path.exists():
        print(f"results.json not found in {run_dir}", file=sys.stderr)
        sys.exit(1)

    results = json.loads(results_path.read_text())
    metadata_path = run_dir / "run_metadata.json"
    metadata = json.loads(metadata_path.read_text()) if metadata_path.exists() else {}
    baseline = load_baseline()

    report = generate_report(run_dir, results, metadata, baseline)
    print(report)

    if args.output:
        output_path = Path(args.output)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(report + "\n")
        print(f"\nReport written to {output_path}", file=sys.stderr)


if __name__ == "__main__":
    main()
