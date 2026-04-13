# GitHub Issues 504-506: Implementation Readiness Assessment

**Assessment Date:** 2026-03-31  
**Scope:** Three P3 enhancement issues derived from Claude Code prompt learnings  
**Reference:** `docs/investigations/claude-code-prompt-learnings.md` §5, §11, §12

---

## Issue #504: Speculative Pre-Execution

### Issue Summary
After each response, predict the user's likely next request and pre-execute it in an isolated environment using a copy-on-write overlay system. If the user accepts, inject the speculated work. If they diverge, discard.

**Reference:** Section 5 of prompt learnings

### Already Addressed?
**NO** — Codebase analysis findings:
- ✅ **Worktree system exists** (`internal/workspace/worktree.go`) for workspace isolation
- ❌ **No copy-on-write overlay** implementation found
- ❌ **No speculative/prediction agent** code found
- ❌ **No discard mechanism** for speculated changes
- ✅ **Feature flag pattern exists** (models in `internal/systemprompt/catalog.go`, runtime profile system)

### Problem Statement Clarity
**CLEAR** — The issue statement is unambiguous:
- Predicts next user input via forked lightweight agent
- Executes in isolated copy-on-write overlay
- Bounds: max 20 turns, stop at non-read-only operations
- Integrates with worktree abstraction
- Add discard/injection mechanism
- Feature-flag gated (disabled by default)

### Acceptance Criteria
**INCOMPLETE** — Criteria are stated but lack measurable "done" conditions:
- "Prediction agent predicts" — needs success metrics (accuracy %, latency SLA)
- "Copy-on-write works" — needs test coverage (overlay isolation, rollback)
- "Feature-flag gated" — needs config loading test
- Missing: E2E test scenario (predict → user accepts vs. diverges)

**Suggested additions:**
- Prediction accuracy target (70%+ on common follow-ups)
- Overlay isolation verified by comparing file states before/after discard
- Feature flag toggles between enabled/disabled states in config
- Unit tests for overlay creation, injection, and discard

### Scope Assessment
**ATOMIC** — Yes, the issue is a self-contained feature:
- Does not depend on other pending issues
- Requires worktree system (already exists)
- Does not require undercover mode (504) or jitter (506)
- Clear entry and exit points

### Blockers & Dependencies
**NONE identified.** The issue can proceed independently:
- No dependency on #505 (undercover mode)
- No dependency on #506 (cron jitter)
- Depends on: existing worktree infrastructure ✅

### Effort Estimate
**LARGE** (estimated 13-21 dev days)

Breakdown:
- Prediction agent (forked, lightweight): 3-4 days
- Copy-on-write overlay filesystem abstraction: 4-5 days
- Discard/injection mechanism: 2-3 days
- Config + feature flag integration: 1-2 days
- Tests (unit + integration): 2-3 days
- Documentation: 1 day

### Key Files That Will Need to Change
1. **New files:**
   - `internal/harness/speculative/agent.go` — prediction agent
   - `internal/harness/speculative/overlay.go` — copy-on-write filesystem
   - `internal/harness/speculative/cache.go` — speculated work cache
   - `internal/harness/speculative/types.go` — data structures

2. **Modify existing:**
   - `internal/workspace/workspace.go` — add overlay support to Workspace interface
   - `internal/config/config.go` — add `SpeculativeConfig` struct
   - `internal/harness/runner.go` — integrate speculation after response
   - `internal/systemprompt/catalog.go` — register prediction agent model profile

3. **Tests:**
   - `internal/harness/speculative/agent_test.go`
   - `internal/harness/speculative/overlay_test.go`
   - `internal/harness/speculative/integration_test.go`

### TOML Config Fields Needed
```toml
[speculative]
enabled = false              # Feature flag: disabled by default
max_turns = 20               # Max speculation depth
max_messages = 100           # Max messages per speculation
stop_at_write = true         # Stop at non-read-only operations
prediction_model = "gpt-4o-mini"  # Model for next-step prediction
discard_timeout_sec = 300    # Discard speculated work if no user response

[speculative.directories]
overlay_root = ".harness/speculative"  # Where to mount overlays
```

### Suggested Labels
- `enhancement`
- `ux-improvement`
- `p3-low-priority`
- `feature-flag-gated`
- `speculative-execution`

### Implementation Notes
- Use the existing worktree infrastructure as a foundation
- Consider using FUSE or similar for copy-on-write if available
- The prediction agent should be a lightweight fork (2-3 turns max)
- Log all speculation attempts and discards for audit trail
- Consider rate-limiting speculation to avoid runaway resource usage

---

## Issue #505: Undercover Mode for External Repo Contributions

### Issue Summary
Add an undercover mode that strips all agent attribution when contributing to external repositories — no model IDs, no Co-Authored-By lines, commit messages written as a human developer would.

**Reference:** Section 12 of prompt learnings

### Already Addressed?
**NO** — Codebase analysis findings:
- ✅ **Git commit infrastructure exists** (`internal/training/applier.go` lines 210-214)
- ❌ **No Co-Authored-By stripping** code found
- ❌ **No undercover mode config** in `internal/config/config.go`
- ❌ **No public repo auto-detection** code
- ✅ **Profile system exists** (config layers, named profiles at `~/.harness/profiles/`)

Current git commit logic:
```go
msg := fmt.Sprintf("training: auto-apply %d findings", result.FindingsApplied)
if _, err := a.runGit("commit", "-m", msg); err != nil {
    return fmt.Errorf("git commit: %w", err)
}
```

No author stripping or attribution removal is implemented.

### Problem Statement Clarity
**CLEAR** — The issue statement is unambiguous:
- Auto-detect public repos (check git remote URL)
- Strip from commits: Co-Authored-By lines, model identifiers, agent codenames
- Modify commit message style to match human conventions
- Feature-flag gated

### Acceptance Criteria
**PARTIALLY CLEAR** — High-level goals stated, but "done" conditions lack specifics:
- "Strip attribution" — needs a whitelist of patterns to remove
- "Match human conventions" — needs examples of human vs. agent commit messages
- "Auto-detect" — needs logic for detecting public repos (github.com, gitlab.com, etc.)
- Missing: Test cases for various git remotes (ssh, https, git://)

**Suggested additions:**
- List of attribution patterns: `Co-Authored-By`, model IDs like `claude-opus-4.6`, agent codenames
- Commit message prefix rules: human style = no "auto-apply", no internal jargon
- Test matrix: github.com (public), bitbucket.org (public), company git (private)
- Flag to override auto-detection if needed

### Scope Assessment
**ATOMIC** — Yes, a self-contained feature:
- Does not depend on other pending issues
- Concerns only git commit and config logic
- Clear entry points (before commit) and exit points (clean commit)

### Blockers & Dependencies
**NONE identified.** The issue can proceed independently:
- No dependency on #504 (speculative execution)
- No dependency on #506 (cron jitter)
- Depends on: git infrastructure (already exists) ✅

### Effort Estimate
**MEDIUM** (estimated 8-13 dev days)

Breakdown:
- Public repo detection logic: 1-2 days
- Commit message rewriting: 2-3 days
- Config + profile integration: 1-2 days
- Author env var stripping: 1 day
- Tests (various git remotes): 2-3 days
- Documentation: 1 day

### Key Files That Will Need to Change
1. **New files:**
   - `internal/git/detector.go` — public repo detection
   - `internal/git/committer.go` — undercover-aware committing
   - `internal/git/message_transformer.go` — human-style message rewriting

2. **Modify existing:**
   - `internal/config/config.go` — add `UndercoverConfig` struct
   - `internal/training/applier.go` — call committer.Commit instead of runGit("commit")
   - `internal/harness/runner.go` or git tool — integrate undercover logic
   - Any other code that calls `git commit`

3. **Tests:**
   - `internal/git/detector_test.go` (test remotes: github.com, gitlab.com, self-hosted, private)
   - `internal/git/committer_test.go` (test message sanitization)
   - `internal/git/message_transformer_test.go`

### TOML Config Fields Needed
```toml
[undercover]
enabled = false                    # Feature flag: disabled by default
auto_detect_public_repos = true    # Auto-detect and strip for public repos
force_undercover = false           # Override: always strip, even for private repos
strip_patterns = [                 # Patterns to remove from messages/commits
    "auto-apply",
    "claude-",
    "Co-Authored-By",
    "trained-by",
]
human_style_templates = true       # Rewrite messages to match human conventions
log_detection_results = true       # Log what was detected/stripped for audit

[undercover.git]
public_repo_domains = [            # List of public hosting domains
    "github.com",
    "gitlab.com",
    "bitbucket.org",
    "gitea.io",
]
```

### Suggested Labels
- `enhancement`
- `open-source-friendly`
- `p3-low-priority`
- `feature-flag-gated`
- `git-integration`

### Implementation Notes
- Detection can be as simple as checking if `git remote -v` contains known public domains
- Commit messages should avoid markers like "auto-apply", "training:", "generated by"
- Consider stripping GIT_AUTHOR_NAME/GIT_AUTHOR_EMAIL env vars when undercover is enabled
- Need to be careful not to break legitimate bot commits (e.g., dependabot)
- Document the feature clearly: this is for external OSS contributions, not internal repos

---

## Issue #506: Default Jitter for Scheduled Task Execution

### Issue Summary
When scheduling recurring tasks, automatically jitter away from common minute marks (:00, :30) to prevent thundering-herd API load.

**Reference:** Section 11 of prompt learnings

### Already Addressed?
**PARTIALLY** — Codebase analysis findings:
- ✅ **Cron scheduler exists** (`internal/cron/scheduler.go`, `internal/cron/executor.go`)
- ✅ **Clock abstraction exists** (`internal/cron/clock.go`) for time control
- ❌ **No jitter implementation** found in scheduler
- ❌ **No jitter config** in `internal/config/config.go`
- ❌ **No jitter logging** (users don't see actual execution time)

Current scheduler logic (line 84-86 of scheduler.go):
```go
entryID, err := s.cron.AddFunc(job.Schedule, func() {
    s.fireJob(j)
})
```

No transformation of the cron schedule to add jitter.

### Problem Statement Clarity
**CLEAR** — The issue statement is unambiguous:
- Add random jitter (1-5 min) by default
- Avoid :00 and :30 minute marks
- Make jitter configurable/disableable
- Log the jittered time so users know when it will actually fire

### Acceptance Criteria
**MOSTLY CLEAR** — Goals are stated, but lacks precise "done" conditions:
- "Avoid :00 and :30" — needs to define how (e.g., jitter 1-14, 16-29, 31-44, 46-59)
- "Configurable" — needs config field examples
- "Log jittered time" — needs to specify where/format of log output
- Missing: Test verification that :00 and :30 are never used

**Suggested additions:**
- Unit test: 10k generated schedules should have 0% landing on :00 or :30
- Log example: `[CRON] Job scheduled at 14:05 (originally 14:00, jittered +5m)`
- Config: `cron.jitter_min_sec=60, cron.jitter_max_sec=300`
- Verification: Audit log of all scheduled execution times

### Scope Assessment
**ATOMIC** — Yes, a self-contained feature:
- Does not depend on other pending issues
- Only touches cron scheduler configuration
- Clear entry (when schedule is created) and exit (execution time)

### Blockers & Dependencies
**NONE identified.** The issue can proceed independently:
- No dependency on #504 (speculative execution)
- No dependency on #505 (undercover mode)
- Depends on: cron infrastructure (already exists) ✅

### Effort Estimate
**SMALL** (estimated 3-5 dev days)

Breakdown:
- Jitter algorithm + minute-mark avoidance: 1 day
- Config integration: 0.5 days
- Scheduler modification: 1 day
- Logging: 0.5 day
- Tests (distribution, minute-mark avoidance): 1 day
- Documentation: 0.5 day

### Key Files That Will Need to Change
1. **New files:**
   - `internal/cron/jitter.go` — jitter algorithm, minute-mark avoidance

2. **Modify existing:**
   - `internal/cron/scheduler.go` — apply jitter before adding cron entry
   - `internal/config/config.go` — add `CronJitterConfig` struct
   - `internal/cron/types.go` — extend Job struct if needed

3. **Tests:**
   - `internal/cron/jitter_test.go` (unit tests for algorithm)
   - `internal/cron/scheduler_test.go` — verify jitter applied in integration

### TOML Config Fields Needed
```toml
[cron]
jitter_enabled = true              # Feature flag: enabled by default
jitter_min_sec = 60                # Minimum jitter: 60 seconds (1 minute)
jitter_max_sec = 300               # Maximum jitter: 300 seconds (5 minutes)
avoid_minute_marks = [0, 30]       # Avoid these minute marks
log_jittered_times = true           # Log actual execution time
```

### Suggested Labels
- `enhancement`
- `performance`
- `p3-low-priority`
- `infrastructure`
- `cron-scheduler`

### Implementation Notes
- Use a deterministic PRNG seeded by job ID to avoid always jittering the same jobs the same way (for fairness across the fleet)
- When users request "every hour", transform `0 * * * *` to `0 * * * *` + jitter
- When users request "at 9am", transform `0 9 * * *` to `0 9 * * *` + jitter
- Log format: include original schedule and jittered minute in job creation response
- Consider using a probabilistic model to ensure even distribution across the 60-minute window
- Test with 10,000+ random jitters to verify distribution

---

## Summary Table

| Issue | Already Addressed | Clarity | Acceptance Criteria | Atomic | Blockers | Effort | Suggested Labels |
|-------|-------------------|---------|---------------------|--------|----------|--------|------------------|
| **504** Speculative Pre-Execution | ❌ No | ✅ Clear | ⚠️ Incomplete | ✅ Yes | None | **LARGE** (13-21d) | enhancement, ux-improvement, p3, feature-flag |
| **505** Undercover Mode | ❌ No | ✅ Clear | ⚠️ Partial | ✅ Yes | None | **MEDIUM** (8-13d) | enhancement, open-source, p3, feature-flag |
| **506** Cron Jitter | ⚠️ Partial | ✅ Clear | ✅ Clear | ✅ Yes | None | **SMALL** (3-5d) | enhancement, performance, p3, infrastructure |

---

## Recommendations for Grooming

### High-Priority Clarifications Needed
1. **Issue #504** — Add acceptance criteria with measurable success metrics:
   - Prediction accuracy target (e.g., 70%+ of predictions match user input)
   - Overlay isolation test procedure
   - Feature flag disabled by default, testable in integration test

2. **Issue #505** — Document expected commit message transformations:
   - Before: "training: auto-apply 3 findings"
   - After: "Apply improvements to system prompt"
   - Provide 5-10 real examples

3. **Issue #506** — Add specifics on jitter distribution:
   - Document how minute-mark avoidance works (which ranges are used)
   - Specify log format with example
   - Define test coverage (e.g., 10k jitters, 0% on :00/:30)

### Recommended Priority & Sequencing
1. **#506 (Cron Jitter)** — Start here (SMALL effort, no dependencies)
2. **#505 (Undercover Mode)** — Next (MEDIUM, no dependencies, increases user comfort with external contributions)
3. **#504 (Speculative)** — Last (LARGE, most complex, highest value but slowest ROI)

### Config Strategy
All three issues use feature flags disabled by default:
- `speculative.enabled = false`
- `undercover.enabled = false`
- `cron.jitter_enabled = true` (exception: jitter enabled by default per Claude Code pattern)

Consider a unified feature flag section in config for easier discovery:
```toml
[features]
speculative_execution = false
undercover_mode = false
cron_jitter = true
```

### Testing Strategy
- **#504:** Mocked prediction agent, fake overlay filesystem, integration test of accept/discard flow
- **#505:** Git remote URL fixtures (github.com, gitlab, internal), commit message golden files
- **#506:** Distribution analysis (10k samples), minute-mark histogram, deterministic seeding

---

## Next Steps
1. Comment on each issue with proposed acceptance criteria
2. Label with `needs-grooming-refinement` until criteria are explicit
3. Once refined, move to `ready-for-implementation` and assign story points
4. Consider pairing with existing team member for architecture review before starting #504

