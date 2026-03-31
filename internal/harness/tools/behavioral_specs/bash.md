## When to Use
- Running build commands (go build, make, npm run)
- Executing scripts and binaries
- Installing packages or managing dependencies
- Starting/stopping background processes
- System operations requiring shell access
- Chaining multiple shell commands in a single invocation

## When NOT to Use
- Reading a file — use the read tool instead of cat/head/tail
- Writing a file — use the write tool instead of echo/tee
- Editing a file — use the edit tool instead of sed/awk
- Searching file contents — use the grep tool
- Finding files by pattern — use the glob tool
- Performing git status — use the git_status tool
- Long-running interactive processes that need streaming output

## Behavioral Rules
1. Never use `git push --force` or `git reset --hard` without explicit user instruction
2. Never use `git commit --amend` to rewrite history unless the user asks
3. Never skip git hooks with `--no-verify`
4. Never delete files or directories with `rm -rf` on the workspace root or parent paths
5. Avoid `sleep` commands — use background jobs and polling instead
6. Prefer parallel bash calls when commands are independent
7. Quote file paths containing spaces to avoid shell word-splitting bugs
8. When running go test, use -v flag to get individual test names and counts

## Common Mistakes
- **PreferDedicatedTool**: Using `cat file.go` instead of the read tool; using `grep` via bash instead of the grep tool
- **ForcePush**: Running `git push --force` or `git push -f` without explicit instruction
- **SleepPolling**: Using `sleep 5 && check_status` in a loop instead of background jobs with polling
- **AmendingHistory**: Running `git commit --amend` to fix a failed commit hook instead of creating a new commit
- **GlobViaFind**: Using `find . -name "*.go"` instead of the glob tool

## Examples
### WRONG
```bash
cat internal/config/config.go
```

### RIGHT
Use the read tool with file_path parameter to read the file.

### WRONG
```bash
sleep 10 && curl http://localhost:8080/healthz
```

### RIGHT
Start the server as a background job with run_in_background=true, then use job_output to poll for the health endpoint becoming available.
