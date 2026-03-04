# Issue Triage Runbook

## Policy

When a bug/problem is identified, create a GitHub issue so remote agents can take ownership quickly.

## Required Artifacts for Each Bug

- Engineering log entry (`docs/logs/engineering-log.md`)
- Regression test
- GitHub issue

## Create Issue with GitHub CLI

```bash
gh issue create --title "<short bug title>" --body-file /tmp/issue.md --label bug
```

## Recommended Issue Body Sections

- Summary
- Impact
- Steps to reproduce
- Expected vs actual behavior
- Suspected root cause
- Proposed fix direction
- Test plan / regression coverage
- Related docs/log entries
