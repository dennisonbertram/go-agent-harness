# Tool Usability Testing Framework

A systematic method for testing whether LLM agents use harness tools correctly,
and iteratively improving tool descriptions until they do.

---

# Part 1: Framework (applies to ALL tools)

## 1.1 — The Test-Refine-Retest Loop

1. Pick a tool to test
2. Write test cases: prompts that should trigger the tool, with expected correct behavior
3. Start the harness server
4. For each test case:
   a. Send the prompt via curl
   b. Capture the SSE event stream
   c. Analyze: which tools were called, with what arguments, how many turns
   d. Score: Perfect / Acceptable / Fail
5. If any Fail: update the tool description in `internal/harness/tools/`
6. Rebuild + restart server
7. Re-run only the failed cases
8. Repeat until all pass
9. Log everything in the iteration log

## 1.2 — How to Run a Test

Generic curl + analysis steps with copy-pasteable commands.

### Clean state before testing

```bash
# Stop any previous tmux-hosted server session
tmux kill-session -t harnessd-usability 2>/dev/null

# Delete the cron database to avoid UNIQUE constraint errors from prior runs
rm -f .harness/cron.db
```

### Starting/checking the server

```bash
# Start the harness in tmux (required for long-running processes)
tmux new-session -d -s harnessd-usability \
  'cd /absolute/path/to/go-agent-harness && \
   HARNESS_ADDR=:8080 \
   HARNESS_AUTH_DISABLED=true \
   go run ./cmd/harnessd'

# Verify it's up
curl -s http://localhost:8080/healthz

# Inspect logs if needed
tmux capture-pane -pt harnessd-usability | tail -n 120
```

### Sending a prompt and capturing events

```bash
BASE_URL="http://localhost:8080"
PROMPT="your test prompt here"

# Create a run and get the run ID
RUN_ID=$(curl -s -X POST "$BASE_URL/v1/runs" \
  -H "Content-Type: application/json" \
  -d "{\"prompt\": \"$PROMPT\"}" | jq -r '.run_id')

echo "Run ID: $RUN_ID"
```

### Capturing SSE events to a file

```bash
# Stream events to file in tmux
tmux new-session -d -s "sse-$RUN_ID" \
  "curl -s -N \"$BASE_URL/v1/runs/$RUN_ID/events\" > /tmp/sse-events-$RUN_ID.txt"

# Wait for completion (poll status — initial state is "queued")
while true; do
  STATUS=$(curl -s "$BASE_URL/v1/runs/$RUN_ID" | jq -r '.status')
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    echo "Status: $STATUS"
    break
  fi
  sleep 2
done
tmux kill-session -t "sse-$RUN_ID" 2>/dev/null
```

### SSE Event Structure

Each SSE event has three header lines plus a data payload:
```
id: run_14:0
retry: 3000
event: <event_type>
data: <JSON payload>
```

Key event types:
- `run.started` / `run.completed` — run lifecycle
- `llm.turn.requested` / `llm.turn.completed` — LLM turns
- `tool.call.started` / `tool.call.completed` — tool calls with full args/results
- `tool.call.delta` — streaming tool call argument chunks
- `assistant.message.delta` / `assistant.message` — streaming + final text output
- `usage.delta` — token usage per turn

### Extracting from SSE events

```bash
FILE="/tmp/sse-events-$RUN_ID.txt"

# Count LLM turns
echo "=== LLM Turns ==="
grep -c 'event: llm.turn.completed' "$FILE"

# Count event types (overview)
echo "=== Event Types ==="
grep '^event: ' "$FILE" | sort | uniq -c | sort -rn

# Extract tool calls with arguments
echo "=== Tool Calls ==="
grep 'event: tool.call.started' -A1 "$FILE" | grep '^data:' | \
  python3 -c "
import sys, json
for line in sys.stdin:
    if line.startswith('data: '):
        d = json.loads(line[6:])
        p = d.get('payload', d)
        args = json.loads(p['arguments']) if isinstance(p.get('arguments'), str) else p.get('arguments', {})
        print(f\"Tool: {p.get('tool', p.get('name', '?'))}\")
        print(f\"Args: {json.dumps(args, indent=2)}\")
        print()
"

# Find tool errors
echo "=== Tool Errors ==="
grep -i '"error"' "$FILE" || echo "(none)"

# Final assistant message
echo "=== Final Output ==="
grep 'event: assistant.message$' -A1 "$FILE" | grep '^data:' | \
  python3 -c "
import sys, json
for line in sys.stdin:
    if line.startswith('data: '):
        d = json.loads(line[6:])
        print(d.get('payload', d).get('content', '(no content)'))
"
```

## 1.3 — Scoring Rubric

- **P (Perfect)**: Correct tool, correct args, first try, minimal turns (typically 2: tool call + response)
- **A (Acceptable)**: Correct outcome but took preparation steps (e.g., ran bash to check current time before creating cron schedule) — this is fine if the extra step was genuinely useful
- **F (Fail)**: Wrong args, wrong tool, error-recovery turn needed, or fundamentally wrong approach

### Additional scoring notes

- A "P" means the tool description was clear enough that the agent needed no exploration
- An "A" means the agent succeeded but the description could potentially be improved to eliminate the extra step
- An "F" means the description is actively misleading or insufficient
- When in doubt between A and F: if the extra steps were reasonable (like checking current time for a time-relative task), it's an A. If the extra steps were error recovery from a bad first attempt, it's an F.

## 1.4 — Writing Good Test Cases

Guidance for creating test suites for any tool:

- Include happy-path cases (basic, intermediate, advanced usage)
- Include "trap" cases — prompts that tempt the agent to misuse the tool
- Include boundary cases — edge cases in the tool's domain
- Specify the expected tool name, expected key arguments, and pass criteria
- Use the consistent table format shown in the cron suite below
- Aim for 5-10 test cases per tool (enough to cover the space, not so many it's tedious)
- Name test cases with a short prefix (tool initial + number) for easy reference

## 1.5 — Iteration Log Template

```markdown
### Round N — [Tool Name] — [Date]

| ID | Score | Turns | Tool Calls | Issues | Description Changes |
|----|-------|-------|------------|--------|-------------------|
| XX | P/A/F | N     | tool1, tool2 | ... | ... |

**Source file**: `internal/harness/tools/[file].go`
**Changes made**: (describe description edits)
**Regressions**: (note any previously-passing tests that now fail)
```

## 1.6 — When to Stop

- All test cases score P or A
- No regressions: previous P cases don't degrade when descriptions change
- Run full suite one final time to confirm
- Document the final tool description in the iteration log for reference

---

# Part 2: Cron Tool Test Suite

## 2.1 — Tool Under Test

- **Tool name**: `cron_create`
- **Source**: `internal/harness/tools/cron.go`
- **Current description** (as of initial testing):

```
Create a RECURRING scheduled cron job. Cron jobs run repeatedly on a fixed
schedule (e.g. every 5 minutes, every hour, daily at midnight). They are NOT
one-shot timers — do not use cron to run something once at a specific time.
Write a script using write/bash tools first, then schedule it with cron.
```

**Parameters**:

- `name` (string, required): Unique name for the cron job
- `schedule` (string, required): Standard 5-field cron expression: `<minute> <hour> <day-of-month> <month> <day-of-week>`. All times are UTC. Must be a literal string — no shell substitutions or variables. Examples: `*/5 * * * *` = every 5 minutes, `0 * * * *` = every hour on the hour, `30 2 * * *` = daily at 02:30 UTC, `0 9 * * 1-5` = weekdays at 09:00 UTC, `0 0 1 * *` = first of every month at midnight UTC. To schedule relative to 'now', first run the bash tool to get the current UTC time, then compute the desired cron fields yourself.
- `command` (string, required): Shell command to execute on each trigger
- `timeout_seconds` (integer, optional): Max execution time in seconds (default 30). The job is killed if it exceeds this.

## 2.2 — Test Cases

| ID | Name | Prompt | Expected Tool | Key Expected Args | Pass Criteria |
|----|------|--------|---------------|-------------------|---------------|
| C1 | Every 5 min | "Create a cron job named 'health-ping' that runs `curl http://localhost:8080/healthz` every 5 minutes" | cron_create | schedule=`*/5 * * * *` | Correct schedule, single tool call |
| C2 | Hourly | "Set up a cron job called 'hourly-cleanup' that runs `rm -f /tmp/cache-*` every hour" | cron_create | schedule=`0 * * * *` | Correct schedule |
| C3 | Daily at specific time | "Schedule a daily job named 'db-backup' that runs `/usr/local/bin/backup.sh` at 3:30 AM UTC" | cron_create | schedule=`30 3 * * *` | Correct schedule |
| C4 | Weekdays | "Create a cron job named 'standup' that runs `echo standup` at 9 AM UTC every weekday" | cron_create | schedule=`0 9 * * 1-5` | Correct schedule |
| C5 | One-shot trap | "Run `echo hello` once in 2 minutes from now" | NOT cron_create | N/A | Agent should NOT use cron; should use bash or explain limitation |
| C6 | Monthly | "Create a cron job named 'monthly-report' that runs `./report.sh` on the 1st of every month at midnight" | cron_create | schedule=`0 0 1 * *` | Correct schedule |
| C7 | Notification trap | "Send me a notification in 2 minutes with the message 'test'" | NOT cron_create | N/A | Should not create a recurring cron for a one-time notification |

## 2.3 — Iteration Log

### Round 1 — cron_create — 2026-03-08

| ID | Score | Turns | Tool Calls | Issues | Description Changes |
|----|-------|-------|------------|--------|-------------------|
| C1 | P | 2 | cron_create schedule=`*/5 * * * *` | — | — |
| C2 | P | 2 | cron_create schedule=`0 * * * *` | — | — |
| C3 | P | 2 | cron_create schedule=`30 3 * * *` | — | — |
| C4 | P | 2 | cron_create schedule=`0 9 * * 1-5` | — | — |
| C5 | **F** | 2 | cron_create schedule=`*/2 * * * *` | Used cron for one-shot task; recurring instead of one-time | — |
| C6 | P | 2 | cron_create schedule=`0 0 1 * *` | — | — |
| C7 | P | 2 | set_delayed_callback delay=2m | — | — |

**Source file**: `internal/harness/tools/cron.go`
**Changes made**: Added explicit "do NOT use cron_create if the user wants to run something once, after a delay, or at a single future time. For one-shot delayed execution, use bash (e.g. 'sleep 120 && command') instead." to cron_create description.
**Regressions**: N/A

### Round 2 — cron_create — 2026-03-08 (post-fix retest)

| ID | Score | Turns | Tool Calls | Issues | Description Changes |
|----|-------|-------|------------|--------|-------------------|
| C5 | P | 2 | bash cmd=`sleep 120 && echo hello` | Fixed — correctly avoided cron | One-shot guidance added |

**Changes made**: Verified C5 fix. Also migrated all cron descriptions to `//go:embed` from `descriptions/*.md` files.
**Regressions**: None (C1-C4, C6-C7 still P)

### Round 3 — cron_create — 2026-03-08 (final, clean DB)

| ID | Score | Turns | Tool Calls | Issues | Description Changes |
|----|-------|-------|------------|--------|-------------------|
| C1 | P | 2 | cron_create schedule=`*/5 * * * *` | Job created successfully | — |
| C2 | P | 2 | cron_create schedule=`0 * * * *` | Job created successfully | — |
| C3 | P | 2 | cron_create schedule=`30 3 * * *` | Job created successfully | — |
| C4 | P | 2 | cron_create schedule=`0 9 * * 1-5` | Job created successfully | — |
| C5 | P | 2 | bash cmd=`sleep 120 && echo hello` (background) | Correctly avoided cron | — |
| C6 | P | 2 | cron_create schedule=`0 0 1 * *` | Job created successfully | — |
| C7 | P | 2 | set_delayed_callback delay=2m | Correctly avoided cron | — |

**Source file**: `internal/harness/tools/cron.go` + `internal/harness/tools/descriptions/cron_create.md`
**Changes made**: None this round (validating embed migration + clean DB)
**Regressions**: None — 7/7 P on clean database

---

# Part 3: Template for Adding New Tool Suites

Copy this template to add a test suite for any tool.

```markdown
## [Tool Name] Test Suite

### Tool Under Test
- **Tool name**: `[tool_name]`
- **Source**: `internal/harness/tools/[file].go`
- **Current description**:
> (paste the current description here)

### Test Cases

| ID | Name | Prompt | Expected Tool | Key Expected Args | Pass Criteria |
|----|------|--------|---------------|-------------------|---------------|
| X1 | ... | "..." | ... | ... | ... |
| X2 | ... | "..." | ... | ... | ... |

### Iteration Log

#### Round 1 — [tool_name] — [Date]

| ID | Score | Turns | Tool Calls | Issues | Description Changes |
|----|-------|-------|------------|--------|-------------------|
| X1 | | | | | |
| X2 | | | | | |

**Source file**: `internal/harness/tools/[file].go`
**Changes made**: (none yet)
**Regressions**: N/A
```

### Suggested Tools to Test Next

| Priority | Tool | Why |
|----------|------|-----|
| High | `bash` | Most-used tool; agents frequently mis-scope commands |
| High | `write` | File creation has many edge cases (paths, overwrite behavior) |
| Medium | `read` | Agents sometimes read wrong files or too much |
| Medium | `grep` | Regex patterns and scope are common failure points |
| Medium | `git_commit` | Agents often stage wrong files or write bad messages |
| Low | `lsp_*` | Complex tools but used less frequently |
