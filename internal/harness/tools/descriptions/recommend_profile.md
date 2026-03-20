Recommend the best built-in profile for a task using deterministic keyword heuristics.

Use this tool before calling run_agent when you are unsure which profile to use. The recommendation is based on keyword matching — NOT model inference — so it is fast and free.

Heuristic rules (first match wins):
- "review", "audit", "check", "analyze"     → reviewer
- "research", "search", "find", "investigate" → researcher
- "bash", "shell", "script", "run command"  → bash-runner
- "write file", "create file", "edit file"   → file-writer
- "github", "pull request", "pr", "issue"   → github
- (no match)                                 → full (default)

The response always includes:
- profile_name: the recommended profile
- reason: explanation of which keyword triggered the match (or fallback reason)
- confidence: "high" (keyword matched) or "low" (fallback)

IMPORTANT: This tool only makes a recommendation. It does NOT override an explicit profile choice. If you already know which profile to use, call run_agent directly.
