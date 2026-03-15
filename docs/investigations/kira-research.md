# KIRA Research: Terminus-KIRA Agent Harness Analysis

**Date**: 2026-03-12
**Source**: https://github.com/krafton-ai/KIRA
**Developed by**: KRAFTON AI + Ludo Robotics

---

## 1. What is KIRA?

KIRA (Terminus-KIRA) is a Python-based agent harness built on top of Terminus 2, designed to benchmark and improve LLM agent performance on [Terminal-Bench](https://github.com/terminal-bench/terminal-bench) — a suite of Linux terminal tasks that require multi-step command execution, verification, and state management.

**The core problem it solves**: Frontier models perform poorly on agentic terminal tasks when using naive text-based response parsing (JSON/XML in-context extraction). KIRA replaces parsing-based tool invocation with the LLM provider's native `tools` parameter, which produces structured function calls directly — eliminating fragile regex/JSON extraction from free-form model outputs.

**Performance results on Terminal-Bench**:
- Codex 5.3: 75.5%
- Claude Opus 4.6: 75.7%
- Gemini 3.1 Pro: 74.8%

These are significant improvements over baseline Terminus 2 achieved purely through harness-level changes, with no model fine-tuning.

---

## 2. Architecture and Design Patterns

### 2.1 Repository Structure

```
terminus_kira/
├── __init__.py
└── terminus_kira.py       # Main agent class

prompt-templates/
└── terminus-kira.txt      # System prompt template with {instruction} and {terminal_state} vars

run-scripts/
├── run_docker.sh
├── run_daytona.sh
└── run_runloop.sh

anthropic_caching.py       # Prompt caching utility
pyproject.toml
```

### 2.2 Core Design: Native Tool Calling vs. ICL Parsing

The foundational architectural decision is bypassing in-context learning (ICL) for structured output extraction. Traditional harnesses prompt the model to produce JSON/XML and then parse it out of the response text. KIRA passes tool definitions directly to the LLM API's `tools` parameter via `litellm.acompletion`, causing the model to return native function call objects.

This matters because:
- Eliminates parse failures from malformed JSON/XML in model responses
- Structured tool call arguments are type-checked by the SDK
- The model "knows" it's calling a function, not producing freeform text — this changes how it reasons

### 2.3 Three-Tool Minimal Surface

KIRA defines a deliberately minimal tool surface:

| Tool | Purpose |
|------|---------|
| `execute_commands` | Runs shell commands with planning context and analysis |
| `task_complete` | Signals completion; triggers double-confirmation checklist |
| `image_read` | Reads terminal screenshots via base64 + multimodal LLM call |

This is a significant insight: keeping the tool surface minimal focuses model reasoning. More tools = more decision surface = more opportunity to pick the wrong one.

### 2.4 Agent Loop (`_run_agent_loop`)

The loop runs up to `_max_episodes` iterations:

```
while episodes < max:
    1. Check token pressure → proactive summarization if approaching limit
    2. Call LLM with tools parameter
    3. Extract tool calls from response
    4. Dispatch: execute_commands | image_read | task_complete
    5. Append tool results to message history
    6. Log step trajectory with metrics
```

Context overflow handling:
- `ContextLengthExceededError` → unwind history, summarize, retry
- `OutputLengthExceededError` → inject instruction to be more concise, retry

### 2.5 Marker-Based Polling for Command Completion

**This is the most novel implementation detail.** Instead of waiting a fixed `duration_sec` for a command to complete, KIRA injects a unique sentinel marker (`echo __KIRA_MARKER_<uuid>__`) after each command in the execution batch:

```bash
some_long_command
echo __KIRA_MARKER_abc123__
```

It then polls the terminal output stream, checking for the marker string. As soon as the marker appears, it exits the wait loop — the command finished early. This reduces latency significantly for fast commands without sacrificing correctness for slow ones.

**Go harness relevance**: The current bash tool in go-agent-harness runs commands and waits for process exit. A marker-based streaming poll could be applied to any long-running bash tool invocation to enable earlier-exit and partial output streaming.

### 2.6 Double-Confirmation Completion Pattern

`task_complete` implements a two-call verification pattern:

1. **First call**: Model calls `task_complete` → harness responds with a multi-perspective QA checklist (test engineer view, QA view, end-user view)
2. **Second call**: Model must call `task_complete` again after verifying against the checklist

This forces the model to self-review from multiple angles before committing. The checklist includes perspectives like: "would a test engineer accept this?", "are there edge cases uncovered?", "does the user experience match the requirement?"

**Go harness relevance**: This pattern maps cleanly onto any task that has a discrete "done" signal. For go-agent-harness, a `task_complete` tool analog with a forced confirmation round could prevent premature termination.

### 2.7 Output Truncation: 30 KB Limit

All command output is hard-capped at 30 KB before being returned to the model. The rationale:
- Large outputs (e.g., `find /`, stack traces, log files) rapidly consume context window
- Models don't meaningfully benefit from the 31st kilobyte of output — they should be prompted to page/grep instead

**Go harness relevance**: The current go-agent-harness bash tool should have a configurable output size cap. 30 KB is a reasonable default to evaluate.

### 2.8 Prompt Caching (`anthropic_caching.py`)

The `add_anthropic_caching()` utility applies Anthropic's ephemeral prompt caching to the most recent N messages (configurable, typically last 3). It works by:
1. Deep-copying the message list
2. For the trailing N messages: if content is a string, convert to `[{type: "text", text: ..., cache_control: {type: "ephemeral"}}]`; if already a list, append `cache_control` to each item
3. This is transparent to the model and reduces repeated re-encoding costs

This is model-specific (only applied when model name contains "anthropic" or "claude").

### 2.9 Context Summarization Under Pressure

When cumulative token count approaches the model's context limit, KIRA proactively summarizes the conversation history before the next LLM call. It does not wait for the API to return a `ContextLengthExceededError` — it anticipates and prevents it. The summarization itself is a separate LLM call that condenses prior steps into a single compressed message injected back into the history.

---

## 3. What's Novel or Interesting

### 3.1 Harness-Level Performance Gains Without Model Changes

The most significant finding is that you can push benchmark scores from ~60% to ~75% by improving the harness alone. This is a strong signal that the quality of the agent loop, tool design, and context management matters as much as — or more than — model selection for many tasks. KIRA proves this empirically across three different frontier models all showing similar gains.

### 3.2 Marker-Based Polling is Underused in the Industry

Most agent harnesses treat bash execution as a black box: run command, wait for exit, return output. KIRA's sentinel marker trick is simple but effective for reducing wait latency. It works without modifying the command itself (the marker echo is appended, not injected into the command), so it's transparent to the task.

### 3.3 Multi-Perspective QA as a Forced Checklist

Using different "role perspectives" in a completion checklist (test engineer, QA, user) to force the model to examine its output from different angles is a prompt engineering pattern that's easy to implement and measurably reduces premature task completion.

### 3.4 Proactive Summarization vs. Reactive Recovery

Most harnesses handle context overflow reactively (catch the error, summarize, retry). KIRA monitors cumulative token counts and acts proactively before hitting the limit. This avoids wasted API calls and the latency of an error round-trip.

### 3.5 Minimal Tool Surface as a Design Principle

Three tools. Not thirty. The minimalism appears intentional and correlated with better performance — the model has fewer choices to make wrong.

---

## 4. Applicability to go-agent-harness

The go-agent-harness is architecturally similar (event-driven HTTP backend, LLM tool-calling loop, SSE streaming) but more general-purpose. Here are specific ideas worth borrowing, ordered by implementation effort:

### High Value, Low Effort

**4.1 Output size cap on bash/exec tools**
Add a configurable `MaxOutputBytes` (suggest 30 KB default) to the bash tool. Truncate with a clear message: `"[output truncated at 30KB — use grep/head/tail to narrow results]"`. This directly reduces context bloat.

**4.2 Proactive context summarization**
Track cumulative token count in the runner step loop. When approaching a configurable threshold (e.g., 80% of model context window), trigger a summarization step before the next LLM call. Currently go-agent-harness has no context management — long runs will hit limits reactively rather than proactively.

**4.3 Double-confirmation for task completion**
A `task_complete` tool that requires two consecutive calls with a synthesized checklist between them. The checklist could be generic (correctness, tests passing, edge cases) or domain-specific per task type.

### Medium Value, Medium Effort

**4.4 Marker-based polling for bash tool**
After running a command, append `echo __HARNESS_DONE_<run-id>__` and poll stdout for the marker rather than blocking on process exit. This enables:
- Earlier return for fast commands
- Incremental streaming of partial output
- Integration with SSE — stream output as it arrives rather than buffering

**4.5 Per-step trajectory logging with token metrics**
KIRA logs each step with token counts and cost estimates. go-agent-harness tracks cost at a high level (pricing package) but not per-step in a structured trajectory log. Adding a structured step log (step N, tool called, tokens in/out, cost, duration) would enable post-run analysis and debugging.

### Lower Priority / Research Interest

**4.6 Native provider tool-calling verification**
If go-agent-harness ever supports providers that do ICL-based tool calling (non-OpenAI function-calling format), KIRA's experience confirms native `tools` parameter is strongly superior. Keep this in mind for any new provider adapters.

**4.7 Role-perspective QA prompts**
The multi-perspective checklist pattern could be applied to any long-running harness run as a "checkpoint" tool triggered at configurable intervals or at user request (via mid-run steering).

---

## 5. What KIRA Does Not Address

- **No HTTP server / REST API layer** — KIRA is a library, not a server. It runs as a process per benchmark task.
- **No multi-tenancy or run isolation** — one agent per process, no concurrency model.
- **No SSE streaming** — outputs are batch-returned, not streamed to a client.
- **No GUI frontend** — CLI / script invocation only.
- **No persistent memory across runs** — each run starts fresh.
- **Python-only** — not directly portable but all ideas are language-agnostic.

These are areas where go-agent-harness already has significant advantages.

---

## 6. Summary

KIRA is a focused, benchmark-optimized agent harness for terminal tasks. Its core contributions are:
1. Native tool calling eliminates parsing fragility
2. Marker-based polling reduces command wait latency
3. Output truncation prevents context bloat
4. Proactive summarization avoids reactive context overflow handling
5. Double-confirmation completion prevents premature task termination
6. Multi-perspective QA checklists improve solution quality

The most immediately applicable ideas for go-agent-harness are the **30 KB output cap**, **proactive context summarization**, and **marker-based bash polling**. The double-confirmation completion pattern is also worth exploring as an optional task mode.

---

*Sources*: https://github.com/krafton-ai/KIRA, https://raw.githubusercontent.com/krafton-ai/KIRA/main/README.md, https://raw.githubusercontent.com/krafton-ai/KIRA/main/terminus_kira/terminus_kira.py
