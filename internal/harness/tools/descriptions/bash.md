Run a shell command in the workspace. Use this for executing build commands, running scripts, installing packages, managing processes, and performing system operations that require shell access.

IMPORTANT: Do NOT use bash when a dedicated tool exists for the task:
- To read a file, use the "read" tool instead of cat/head/tail.
- To write a file, use the "write" tool instead of echo/tee/cat.
- To edit a file, use the "edit" tool instead of sed/awk.

Parameters:
- command (required): The shell command to execute.
- timeout_seconds (optional): Max execution time in seconds (1-3600, default 30). Foreground commands are capped at 300s. Background commands allow up to 3600s.
- run_in_background (optional): Set to true to run the command as a background job. Returns a shell_id immediately. Use job_output to read its output later, and job_kill to terminate it. Use background mode for long-running processes (servers, watchers, builds over 30s).
- working_dir (optional): Working directory relative to the workspace root.
- description (optional): Human-readable note describing what this command does.

When you need exact test counts or per-test results, use verbose flags (e.g. -v, --verbose). Many test runners produce summarized output by default that omits individual test names.

Dangerous commands (rm -rf /, sudo, shutdown, reboot, fork bombs) are rejected by safety policy.

INTERPRETING Go TEST OUTPUT:
When running `go test` without the -v flag, Go reports only a single summary line per package:
  ok  	some/package	0.200s
This "ok" line means the entire package passed — it does NOT mean only one test ran. The
actual number of tests is hidden in non-verbose mode. Do NOT interpret an "ok" line as
"1 test passed."

To see individual test names, counts, and results, always use `go test -v`:
  go test -v ./...
  go test -v -run TestSpecificFunction ./path/to/package

Use -v whenever you need to:
- Count how many tests ran in a package
- Identify which specific tests passed or failed
- Report accurate test results to the user
