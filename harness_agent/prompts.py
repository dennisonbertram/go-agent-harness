"""
System prompt loader for the HarnessAgent.

Tries to load a compiled system prompt from prompts/compiled/system_prompt.txt
relative to the project root (one level up from this file's parent directory).
Falls back to a built-in default prompt if the file is not found.
"""

import sys
from pathlib import Path

DEFAULT_SYSTEM_PROMPT = """\
You are an expert software engineer working in a sandboxed terminal environment.
You MUST use the bash tool to complete tasks — do NOT write answers as plain text.
Every task requires you to execute commands in the terminal.

Your workflow:
1. Use bash to explore the environment: ls, cat files, read the task requirements
2. Use bash to implement the solution: write files, run commands, install packages
3. Use bash to verify your work: run tests, check output, validate results
4. Only stop calling bash when the task is fully complete and verified

IMPORTANT: You cannot complete tasks by describing what to do. You must DO it using bash.\
"""

_BASH_DIRECTIVE = """\
You MUST use the bash tool to complete tasks — do NOT write answers as plain text.
Every task requires you to execute commands in the terminal.
Only stop calling bash when the task is fully complete and verified.\
"""

# Project root is the parent of the harbor/ package directory
_PACKAGE_DIR = Path(__file__).parent
_PROJECT_ROOT = _PACKAGE_DIR.parent
_COMPILED_PROMPT_PATH = _PROJECT_ROOT / "prompts" / "compiled" / "system_prompt.txt"


def load_system_prompt() -> str:
    """
    Load the system prompt for the HarnessAgent.

    Returns the contents of prompts/compiled/system_prompt.txt if it exists,
    prepended with the bash-directive block to ensure the agent always uses
    the bash tool rather than answering in plain text.

    Falls back to DEFAULT_SYSTEM_PROMPT (which already contains the directive)
    if the compiled file is not found or is empty.
    """
    if _COMPILED_PROMPT_PATH.exists():
        try:
            prompt = _COMPILED_PROMPT_PATH.read_text(encoding="utf-8").strip()
            if prompt:
                # Prepend the directive so it overrides any softer instructions
                # that might be in the compiled prompt.
                return _BASH_DIRECTIVE + "\n\n" + prompt
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
