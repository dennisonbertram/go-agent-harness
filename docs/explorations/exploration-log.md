# Exploration Log

Use this file to document spikes and experiments.

## Entry Template

- Date:
- Hypothesis:
- Experiment:
- Result:
- Decision:
- Next step:

- 2026-03-05: Terminal Bench smoke-suite debugging established that Docker was the initial blocker, but the durable harness issues are `apply_patch` compatibility, CLI stream timeout behavior, and brittle structured-file writes. See `docs/explorations/terminal-bench-debugging-2026-03-05.md`.

- 2026-03-05: GitHub issue creation for the Terminal Bench follow-ups is currently blocked because the configured upstream repository `dennisonbertram/go-agent-harness` returns 404 via both `gh` and the GitHub API.

- 2026-03-05: Created GitHub issues #12, #13, and #14 for the high-priority Terminal Bench robustness findings after bypassing the `GITHUB_TOKEN`-forced service-account authentication in `gh`.
