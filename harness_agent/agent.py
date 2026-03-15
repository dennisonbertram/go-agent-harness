"""
HarnessAgent — Harbor BaseAgent adapter for go-agent-harness.

Implements the LLM tool-calling loop using either the Anthropic or OpenAI API.
Provider is selected from the model name prefix (anthropic/ or openai/).
All bash commands are routed through environment.exec() so they run
inside the Harbor container rather than on the host machine.
"""

from __future__ import annotations

import json
import os
from typing import Any

from harbor.agents.base import BaseAgent
from harbor.environments.base import BaseEnvironment, ExecResult
from harbor.models.agent.context import AgentContext

from harness_agent.prompts import load_system_prompt

# ---------------------------------------------------------------------------
# SDK imports (optional — fall back to raw httpx)
# ---------------------------------------------------------------------------

try:
    import anthropic as _anthropic_sdk
    _HAS_ANTHROPIC_SDK = True
except ImportError:
    _HAS_ANTHROPIC_SDK = False

try:
    import openai as _openai_sdk
    _HAS_OPENAI_SDK = True
except ImportError:
    _HAS_OPENAI_SDK = False

try:
    import httpx as _httpx
    _HAS_HTTPX = True
except ImportError:
    _HAS_HTTPX = False

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

_ANTHROPIC_API_URL = "https://api.anthropic.com/v1/messages"
_ANTHROPIC_API_VERSION = "2023-06-01"
_OPENAI_API_URL = "https://api.openai.com/v1/chat/completions"
_MAX_TURNS = 100
_MAX_OUTPUT_CHARS = 20_000  # truncate bash output to prevent context explosions

_BASH_TOOL_ANTHROPIC: dict[str, Any] = {
    "name": "bash",
    "description": (
        "Execute a bash command in the working environment. "
        "Returns stdout and stderr combined."
    ),
    "input_schema": {
        "type": "object",
        "properties": {
            "command": {
                "type": "string",
                "description": "The bash command to execute.",
            }
        },
        "required": ["command"],
    },
}

_BASH_TOOL_OPENAI: dict[str, Any] = {
    "type": "function",
    "function": {
        "name": "bash",
        "description": (
            "Execute a bash command in the working environment. "
            "Returns stdout and stderr combined."
        ),
        "parameters": {
            "type": "object",
            "properties": {
                "command": {
                    "type": "string",
                    "description": "The bash command to execute.",
                }
            },
            "required": ["command"],
        },
    },
}

# Unified response shape (normalised from either API):
# {
#   "content": [{"type": "text", "text": "..."}, {"type": "tool_use", "id": "...", "name": "bash", "input": {...}}],
#   "stop_reason": "tool_use" | "end_turn",
#   "usage": {"input_tokens": N, "output_tokens": N}
# }

_BASH_TOOL = _BASH_TOOL_ANTHROPIC  # kept for compat, not used directly


# ---------------------------------------------------------------------------
# HarnessAgent
# ---------------------------------------------------------------------------


class HarnessAgent(BaseAgent):
    """
    Harbor-compatible agent that drives the Anthropic or OpenAI API
    and routes all bash tool calls into the Harbor container environment.
    """

    SUPPORTS_ATIF: bool = False

    def __init__(self, *args: Any, reasoning_effort: str | None = None, **kwargs: Any):
        super().__init__(*args, **kwargs)
        # e.g. "low" | "medium" | "high" — passed via --ak reasoning_effort=high
        self.reasoning_effort = reasoning_effort

    @staticmethod
    def name() -> str:
        return "harness-agent"

    def version(self) -> str | None:
        return "0.1.0"

    async def setup(self, environment: BaseEnvironment) -> None:
        """No special setup required for this agent."""
        self.logger.info("HarnessAgent.setup() called — nothing to do")

    async def run(
        self,
        instruction: str,
        environment: BaseEnvironment,
        context: AgentContext,
    ) -> None:
        """
        Run the tool-calling loop until the LLM stops calling tools or
        the MAX_TURNS hard limit is reached.
        """
        # Parse provider/model from Harbor's model_name format
        raw_model = self.model_name or "openai/gpt-4.1"
        if "/" in raw_model:
            provider, model = raw_model.split("/", maxsplit=1)
        else:
            provider, model = "openai", raw_model

        provider = provider.lower()

        if provider == "anthropic":
            api_key = os.environ.get("ANTHROPIC_API_KEY", "")
            if not api_key:
                raise EnvironmentError("ANTHROPIC_API_KEY not set.")
            call_fn = _call_anthropic
        else:  # default: openai
            api_key = os.environ.get("OPENAI_API_KEY", "")
            if not api_key:
                raise EnvironmentError("OPENAI_API_KEY not set.")
            call_fn = _call_openai

        system_prompt = load_system_prompt()

        # Provider-specific message histories.
        # For Anthropic: messages in Anthropic format (content blocks with
        #   tool_use / tool_result types).
        # For OpenAI: messages in OpenAI Chat Completions format (role=tool,
        #   tool_calls on assistant messages).  Sending Anthropic-format
        #   content blocks to OpenAI causes a validation error on turn 2+.
        if provider == "anthropic":
            # Single shared list in Anthropic format.
            anthropic_messages: list[dict[str, Any]] = [
                {"role": "user", "content": instruction},
            ]
        else:
            # Separate list in OpenAI format.  The system message is prepended
            # inside _call_openai(), so we only store user/assistant/tool turns
            # here.
            oai_messages: list[dict[str, Any]] = [
                {"role": "user", "content": instruction},
            ]

        total_input_tokens = 0
        total_output_tokens = 0
        total_cache_tokens = 0

        self.logger.info(
            "HarnessAgent starting: provider=%s model=%s max_turns=%d",
            provider, model, _MAX_TURNS
        )

        for turn in range(_MAX_TURNS):
            self.logger.debug("Turn %d/%d", turn + 1, _MAX_TURNS)

            # ---- call LLM ------------------------------------------------
            try:
                if provider == "anthropic":
                    response = await _call_anthropic(
                        api_key=api_key,
                        model=model,
                        system=system_prompt,
                        messages=anthropic_messages,
                    )
                else:
                    response, raw_oai_message = await _call_openai(
                        api_key=api_key,
                        model=model,
                        system=system_prompt,
                        messages=oai_messages,
                        reasoning_effort=self.reasoning_effort,
                    )
            except Exception as exc:  # pylint: disable=broad-except
                self.logger.error("API error on turn %d: %s", turn + 1, exc)
                break

            # ---- accumulate token usage ----------------------------------
            usage = response.get("usage", {})
            total_input_tokens += usage.get("input_tokens", 0)
            total_output_tokens += usage.get("output_tokens", 0)
            total_cache_tokens += usage.get("cache_read_input_tokens", 0)

            # ---- parse content blocks ------------------------------------
            content_blocks: list[dict[str, Any]] = response.get("content", [])
            stop_reason: str = response.get("stop_reason", "end_turn")

            # ---- check for tool use --------------------------------------
            tool_use_blocks = [b for b in content_blocks if b.get("type") == "tool_use"]

            if not tool_use_blocks or stop_reason == "end_turn":
                # No tools to execute — record the assistant turn and stop.
                if provider == "anthropic":
                    anthropic_messages.append({"role": "assistant", "content": content_blocks})
                else:
                    oai_messages.append(raw_oai_message)
                self.logger.info(
                    "Agent finished after %d turn(s) (stop_reason=%s)",
                    turn + 1,
                    stop_reason,
                )
                break

            # ---- execute tool calls --------------------------------------
            tool_results: list[dict[str, Any]] = []
            # OpenAI tool result messages (one per tool call)
            oai_tool_result_messages: list[dict[str, Any]] = []

            for block in tool_use_blocks:
                tool_name: str = block.get("name", "")
                tool_use_id: str = block.get("id", "")
                tool_input: dict[str, Any] = block.get("input", {})

                if tool_name == "bash":
                    command: str = tool_input.get("command", "")
                    self.logger.debug("Executing bash: %s", command[:200])

                    result_text = await _exec_bash(
                        environment=environment,
                        command=command,
                        logger=self.logger,
                    )
                else:
                    # Unknown tool — return an error so the LLM can recover
                    self.logger.warning("Unknown tool requested: %s", tool_name)
                    result_text = f"Error: unknown tool '{tool_name}'."

                # Anthropic-format result (used for Anthropic provider)
                tool_results.append(
                    {
                        "type": "tool_result",
                        "tool_use_id": tool_use_id,
                        "content": result_text,
                    }
                )
                # OpenAI-format result (used for OpenAI provider)
                oai_tool_result_messages.append(
                    {
                        "role": "tool",
                        "tool_call_id": tool_use_id,
                        "content": result_text,
                    }
                )

            # Append turns in provider-specific format.
            if provider == "anthropic":
                # Assistant turn with tool_use blocks, then user turn with results.
                anthropic_messages.append({"role": "assistant", "content": content_blocks})
                anthropic_messages.append({"role": "user", "content": tool_results})
            else:
                # OpenAI assistant message (raw, includes tool_calls field),
                # then one role=tool message per result.
                oai_messages.append(raw_oai_message)
                oai_messages.extend(oai_tool_result_messages)

        else:
            self.logger.warning("Reached MAX_TURNS (%d) limit.", _MAX_TURNS)

        # ---- populate context -------------------------------------------
        context.n_input_tokens = total_input_tokens
        context.n_output_tokens = total_output_tokens
        context.n_cache_tokens = total_cache_tokens if total_cache_tokens else None

        # Estimate cost using Anthropic's published pricing for claude-opus-4-6.
        # Harbor uses this for leaderboard cost tracking.  If the model is not
        # recognised we skip cost estimation rather than raise.
        cost = _estimate_cost(model, total_input_tokens, total_output_tokens)
        if cost is not None:
            context.cost_usd = cost

        self.logger.info(
            "HarnessAgent done: input_tokens=%d output_tokens=%d cost_usd=%s",
            total_input_tokens,
            total_output_tokens,
            f"{cost:.6f}" if cost is not None else "unknown",
        )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


async def _exec_bash(
    environment: BaseEnvironment,
    command: str,
    logger: Any,
) -> str:
    """
    Execute *command* in the Harbor container and return a formatted string
    combining stdout, stderr, and (if non-zero) the exit code.
    """
    try:
        result: ExecResult = await environment.exec(command)
    except Exception as exc:  # pylint: disable=broad-except
        logger.error("environment.exec() raised: %s", exc)
        return f"Error executing command: {exc}"

    parts: list[str] = []

    if result.stdout:
        parts.append(result.stdout)

    if result.stderr:
        # Only include stderr when there's content
        if result.stdout:
            parts.append(result.stderr)
        else:
            parts.append(result.stderr)

    output = "\n".join(parts) if parts else ""

    if result.return_code != 0:
        suffix = f"\n[exit code: {result.return_code}]"
        output = (output + suffix) if output else suffix.lstrip("\n")

    # Truncate to prevent context window explosions
    if len(output) > _MAX_OUTPUT_CHARS:
        half = _MAX_OUTPUT_CHARS // 2
        output = (
            output[:half]
            + f"\n\n[... {len(output) - _MAX_OUTPUT_CHARS} chars truncated ...]\n\n"
            + output[-half:]
        )

    return output or "(no output)"


async def _call_anthropic(
    *,
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
) -> dict[str, Any]:
    """Call Anthropic Messages API; returns normalised response dict."""
    if _HAS_ANTHROPIC_SDK:
        client = _anthropic_sdk.AsyncAnthropic(api_key=api_key)
        response = await client.messages.create(
            model=model,
            max_tokens=8192,
            system=system,
            messages=messages,  # type: ignore[arg-type]
            tools=[_BASH_TOOL_ANTHROPIC],  # type: ignore[arg-type]
        )
        return response.model_dump()
    elif _HAS_HTTPX:
        payload = {
            "model": model,
            "max_tokens": 8192,
            "system": system,
            "messages": messages,
            "tools": [_BASH_TOOL_ANTHROPIC],
        }
        headers = {
            "x-api-key": api_key,
            "anthropic-version": _ANTHROPIC_API_VERSION,
            "content-type": "application/json",
        }
        async with _httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(_ANTHROPIC_API_URL, json=payload, headers=headers)
            resp.raise_for_status()
            return resp.json()
    else:
        raise ImportError("Install 'anthropic' or 'httpx': pip install anthropic")


# Models that require max_completion_tokens instead of max_tokens
_OPENAI_COMPLETION_TOKENS_MODELS = ("gpt-5", "o1", "o3", "o4")

def _openai_token_kwarg(model: str, n: int) -> dict[str, Any]:
    """Return the right token limit kwarg for this model."""
    if any(model.startswith(p) for p in _OPENAI_COMPLETION_TOKENS_MODELS):
        return {"max_completion_tokens": n}
    return {"max_tokens": n}


async def _call_openai(
    *,
    api_key: str,
    model: str,
    system: str,
    messages: list[dict[str, Any]],
    reasoning_effort: str | None = None,
) -> tuple[dict[str, Any], dict[str, Any]]:
    """Call OpenAI Chat Completions API.

    Returns a 2-tuple of:
      (normalised_response, raw_oai_assistant_message)
    """
    oai_messages = [{"role": "system", "content": system}] + messages
    token_kwarg = _openai_token_kwarg(model, 16384)

    extra: dict[str, Any] = {}
    if reasoning_effort:
        extra["reasoning_effort"] = reasoning_effort

    if _HAS_OPENAI_SDK:
        client = _openai_sdk.AsyncOpenAI(api_key=api_key)
        response = await client.chat.completions.create(
            model=model,
            **token_kwarg,
            messages=oai_messages,  # type: ignore[arg-type]
            tools=[_BASH_TOOL_OPENAI],  # type: ignore[arg-type]
            tool_choice="auto",
            **extra,
        )
        raw = response.model_dump()
        return _normalise_openai(raw), _extract_oai_assistant_message(raw)
    elif _HAS_HTTPX:
        payload: dict[str, Any] = {
            "model": model,
            **token_kwarg,
            "messages": oai_messages,
            "tools": [_BASH_TOOL_OPENAI],
            "tool_choice": "auto",
            **extra,
        }
        headers = {
            "Authorization": f"Bearer {api_key}",
            "content-type": "application/json",
        }
        async with _httpx.AsyncClient(timeout=120.0) as client:
            resp = await client.post(_OPENAI_API_URL, json=payload, headers=headers)
            resp.raise_for_status()
            raw = resp.json()
        return _normalise_openai(raw), _extract_oai_assistant_message(raw)
    else:
        raise ImportError("Install 'openai' or 'httpx': pip install openai")


def _extract_oai_assistant_message(raw: dict[str, Any]) -> dict[str, Any]:
    """Return the raw OpenAI-format assistant message dict.

    This preserves the native tool_calls structure that OpenAI expects to
    receive back in subsequent turns.  Concretely:
      {
          "role": "assistant",
          "content": <text or None>,
          "tool_calls": [{"id": ..., "type": "function", "function": {...}}, ...]
      }
    """
    choice = raw.get("choices", [{}])[0]
    message = choice.get("message", {})
    result: dict[str, Any] = {"role": "assistant"}
    result["content"] = message.get("content")  # may be None
    tool_calls = message.get("tool_calls")
    if tool_calls:
        result["tool_calls"] = tool_calls
    return result


def _normalise_openai(raw: dict[str, Any]) -> dict[str, Any]:
    """Convert OpenAI response to the same shape as Anthropic response."""
    choice = raw.get("choices", [{}])[0]
    message = choice.get("message", {})
    finish_reason = choice.get("finish_reason", "stop")

    content_blocks: list[dict[str, Any]] = []

    # Text content
    if message.get("content"):
        content_blocks.append({"type": "text", "text": message["content"]})

    # Tool calls → tool_use blocks
    for tc in message.get("tool_calls") or []:
        fn = tc.get("function", {})
        try:
            arguments = json.loads(fn.get("arguments", "{}"))
        except json.JSONDecodeError:
            arguments = {"command": fn.get("arguments", "")}
        content_blocks.append({
            "type": "tool_use",
            "id": tc.get("id", ""),
            "name": fn.get("name", ""),
            "input": arguments,
        })

    # Map finish_reason → stop_reason
    stop_reason = "tool_use" if finish_reason == "tool_calls" else "end_turn"

    usage = raw.get("usage", {})
    return {
        "content": content_blocks,
        "stop_reason": stop_reason,
        "usage": {
            "input_tokens": usage.get("prompt_tokens", 0),
            "output_tokens": usage.get("completion_tokens", 0),
        },
    }


# ---------------------------------------------------------------------------
# Cost estimation
# ---------------------------------------------------------------------------

# Pricing per million tokens (as of 2026-03, approximate)
_PRICING: dict[str, dict[str, float]] = {
    # Anthropic
    "claude-opus-4-6": {"input": 15.0, "output": 75.0},
    "claude-sonnet-4-6": {"input": 3.0, "output": 15.0},
    "claude-haiku-3-5": {"input": 0.8, "output": 4.0},
    "claude-3-5-sonnet-20241022": {"input": 3.0, "output": 15.0},
    "claude-3-5-haiku-20241022": {"input": 0.8, "output": 4.0},
    "claude-3-opus-20240229": {"input": 15.0, "output": 75.0},
    # OpenAI
    "gpt-4.1": {"input": 2.0, "output": 8.0},
    "gpt-4.1-mini": {"input": 0.4, "output": 1.6},
    "gpt-4o": {"input": 2.5, "output": 10.0},
    "gpt-4o-mini": {"input": 0.15, "output": 0.60},
    "o3": {"input": 10.0, "output": 40.0},
    "o4-mini": {"input": 1.1, "output": 4.4},
}


def _estimate_cost(
    model: str, input_tokens: int, output_tokens: int
) -> float | None:
    """Return estimated cost in USD, or None if the model is not in the table."""
    # Try exact match first, then prefix match
    pricing = _PRICING.get(model)
    if pricing is None:
        for key, val in _PRICING.items():
            if model.startswith(key) or key.startswith(model):
                pricing = val
                break

    if pricing is None:
        return None

    cost = (input_tokens / 1_000_000) * pricing["input"]
    cost += (output_tokens / 1_000_000) * pricing["output"]
    return cost
