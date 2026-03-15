"""
System prompt loader for the HarnessAgent.

Tries to load a compiled system prompt from prompts/compiled/system_prompt.txt
relative to the project root (one level up from this file's parent directory).
Falls back to a built-in default prompt if the file is not found.
"""

import sys
from pathlib import Path

DEFAULT_SYSTEM_PROMPT = """\
You are an expert software engineer working in a terminal environment.
You have access to a bash tool to execute commands. Use it to complete the given task.
When you have completed the task, simply stop calling tools.
Be methodical: read existing code before modifying it. Run tests to verify your work.
Use git to understand the codebase. Write minimal, correct changes.\
"""

# Project root is the parent of the harbor/ package directory
_PACKAGE_DIR = Path(__file__).parent
_PROJECT_ROOT = _PACKAGE_DIR.parent
_COMPILED_PROMPT_PATH = _PROJECT_ROOT / "prompts" / "compiled" / "system_prompt.txt"


def load_system_prompt() -> str:
    """
    Load the system prompt for the HarnessAgent.

    Returns the contents of prompts/compiled/system_prompt.txt if it exists,
    otherwise returns the DEFAULT_SYSTEM_PROMPT and prints a warning.
    """
    if _COMPILED_PROMPT_PATH.exists():
        try:
            prompt = _COMPILED_PROMPT_PATH.read_text(encoding="utf-8").strip()
            if prompt:
                return prompt
            # File exists but is empty — fall through to default
            print(
                f"Warning: {_COMPILED_PROMPT_PATH} is empty; using default system prompt.",
                file=sys.stderr,
            )
        except OSError as exc:
            print(
                f"Warning: Could not read {_COMPILED_PROMPT_PATH}: {exc}; "
                "using default system prompt.",
                file=sys.stderr,
            )
    else:
        print(
            f"Warning: {_COMPILED_PROMPT_PATH} not found; using default system prompt.",
            file=sys.stderr,
        )

    return DEFAULT_SYSTEM_PROMPT
