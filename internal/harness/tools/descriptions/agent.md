Delegate a complex task to an autonomous sub-agent. The sub-agent gets its own independent tool-calling loop with a fresh step budget, plans its approach, executes multiple tool calls as needed, and returns a consolidated result.

WHEN TO USE: Delegate with this tool whenever a task involves MULTIPLE steps that span different tools or files. This is strongly preferred over doing multi-step work yourself because delegation preserves your step budget for coordination. Examples of tasks to delegate:
- "Research the codebase structure, analyze patterns, and create a summary" (explore + analyze + write)
- "Find all files matching a pattern and refactor them" (search + read + edit multiple files)
- "Investigate a bug by reading logs, checking code, and proposing a fix" (multi-source investigation)
- Any task combining information gathering with content creation or code modification

WHEN NOT TO USE: Do NOT delegate if the task is a single atomic operation:
- Reading one file (use read)
- Running one shell command (use bash)
- Searching for a pattern (use grep)
- Answering a factual question with no tool use needed

PROMPT FORMAT: The "prompt" parameter must be self-contained. Include all file paths, package names, and requirements since the sub-agent does not share your conversation history.

MODEL SELECTION: The optional "model" parameter overrides which LLM the sub-agent uses for its run. When omitted, the runner uses its configured default model. Use this to route tasks to cost-efficient or capability-matched models — for example, delegate lightweight research to a cheaper model and complex multi-step reasoning to a more capable one.