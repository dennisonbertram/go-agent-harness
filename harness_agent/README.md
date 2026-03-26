# Harbor Agent Adapter

A [Harbor](https://github.com/harbor-ai/harbor) framework adapter that lets
go-agent-harness compete on the Terminal-Bench 2.0 public leaderboard.

The Python package lives at `harness_agent/` (not `harbor/`) to avoid
shadowing the installed `harbor` framework package.

## What it does

`harness_agent/agent.py` provides `HarnessAgent`, a `BaseAgent` subclass that:

- Calls the **Anthropic Messages API** directly (via the `anthropic` Python SDK,
  or raw `httpx` as fallback).
- Exposes a single **`bash` tool** to the LLM.
- Routes every bash call through `environment.exec()` so it runs inside the
  Harbor-managed container, not on the host.
- Runs up to **200 turns** before stopping.
- Populates `AgentContext` with token counts and estimated cost in USD.

## Prerequisites

```bash
pip install anthropic harbor
export ANTHROPIC_API_KEY="sk-ant-..."
```

## Quick start

Run from the **project root** so that `harness_agent/` is on the Python path:

```bash
# Run 5 tasks from the terminal-bench 2.0 dataset with claude-sonnet-4-6
./harness_agent/run_bench.sh

# Run 20 tasks with opus
./harness_agent/run_bench.sh anthropic/claude-opus-4-6 20
```

Or invoke harbor directly (from the project root):

```bash
harbor run \
  -d terminal-bench@2.0 \
  --agent-import-path harness_agent.agent:HarnessAgent \
  -m anthropic/claude-sonnet-4-6 \
  -n 5
```

## System prompt

The agent loads `prompts/compiled/system_prompt.txt` relative to the project
root if that file exists.  Otherwise it falls back to a built-in default and
prints a warning to stderr.

There is currently no built-in `harnessd` prompt-compilation subcommand in this
repository. If you maintain a compiled prompt file, generate or update
`prompts/compiled/system_prompt.txt` using your external prompt build workflow
before benchmark runs.

## Leaderboard submission

Follow the official Terminal-Bench submission guide.  The key flag is
`--agent-import-path harness_agent.agent:HarnessAgent`.  Run from the
project root so that `harness_agent/` is on the Python path.

## File layout

```
harness_agent/
  __init__.py      — package metadata
  agent.py         — HarnessAgent implementation
  prompts.py       — system prompt loader
  run_bench.sh     — convenience wrapper around `harbor run`
  README.md        — this file
```
