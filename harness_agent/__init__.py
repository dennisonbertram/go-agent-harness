"""
Harbor framework adapter for go-agent-harness.

Provides a HarnessAgent that implements the Harbor BaseAgent interface,
running the LLM tool-calling loop against the Anthropic Messages API
with all bash commands routed through the Harbor container environment.
"""

__version__ = "0.1.0"
__author__ = "go-agent-harness"
