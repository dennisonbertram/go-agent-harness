---
name: linear-workflow
description: "Manage Linear project management: create and update issues, cycle management, GitHub integration, status updates via GraphQL API. Trigger: when creating Linear issues, updating Linear status, Linear cycles, Linear project management, Linear GraphQL API, Linear workflow"
version: 1
argument-hint: "[create-issue|update-status|list-issues|cycles|teams]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Linear Project Management

You are now operating in Linear workflow management mode.

## Authentication

```bash
# Linear uses personal API keys or OAuth tokens.
# Set your API key as an environment variable:
export LINEAR_API_KEY="lin_api_your_key_here"

# Verify access by fetching your user info:
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ viewer { id name email } }"}' | jq '.data.viewer'
```

## GraphQL API Basics

All Linear operations use the GraphQL API at `https://api.linear.app/graphql`.

```bash
# Helper function for Linear API calls
linear_query() {
    local query="$1"
    curl -s https://api.linear.app/graphql \
        -H "Authorization: ${LINEAR_API_KEY}" \
        -H "Content-Type: application/json" \
        -d "{\"query\": ${query}}"
}
```

## Teams and Projects

```bash
# List all teams
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ teams { nodes { id name key } } }"}' \
  | jq '.data.teams.nodes'

# List all projects in a team
TEAM_ID="your-team-id"
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"{ team(id: \\\"${TEAM_ID}\\\") { projects { nodes { id name } } } }\"}" \
  | jq '.data.team.projects.nodes'

# Get workflow states (issue statuses) for a team
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"{ team(id: \\\"${TEAM_ID}\\\") { states { nodes { id name type } } } }\"}" \
  | jq '.data.team.states.nodes'
```

## Issue Management

### Create an Issue

```bash
# Create a basic issue via GraphQL mutation
TEAM_ID="your-team-id"
TITLE="Fix authentication timeout bug"
DESCRIPTION="Users are being logged out unexpectedly after 5 minutes. Expected session length is 24 hours."

curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"mutation CreateIssue(\$input: IssueCreateInput!) { issueCreate(input: \$input) { success issue { id identifier title url } } }\",
    \"variables\": {
      \"input\": {
        \"teamId\": \"${TEAM_ID}\",
        \"title\": \"${TITLE}\",
        \"description\": \"${DESCRIPTION}\",
        \"priority\": 2
      }
    }
  }" | jq '.data.issueCreate.issue'
```

Priority values: `0` = No priority, `1` = Urgent, `2` = High, `3` = Medium, `4` = Low.

### Create an Issue with Labels and Assignee

```bash
# First, get label IDs
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"{ team(id: \\\"${TEAM_ID}\\\") { labels { nodes { id name } } } }\"}" \
  | jq '.data.team.labels.nodes'

# Get team member IDs
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"{ team(id: \\\"${TEAM_ID}\\\") { members { nodes { id name } } } }\"}" \
  | jq '.data.team.members.nodes'

# Create issue with labels and assignee
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"mutation CreateIssue(\$input: IssueCreateInput!) { issueCreate(input: \$input) { success issue { id identifier title url } } }\",
    \"variables\": {
      \"input\": {
        \"teamId\": \"${TEAM_ID}\",
        \"title\": \"Add rate limiting to /api/login\",
        \"description\": \"Implement IP-based rate limiting (10 req/min) on the login endpoint.\",
        \"priority\": 1,
        \"labelIds\": [\"label-id-1\", \"label-id-2\"],
        \"assigneeId\": \"user-id\"
      }
    }
  }" | jq '.data.issueCreate.issue'
```

### List Issues

```bash
# List issues in a team (recent 20)
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"{ team(id: \\\"${TEAM_ID}\\\") { issues(first: 20, orderBy: updatedAt) { nodes { id identifier title state { name } assignee { name } priority createdAt } } } }\"
  }" | jq '.data.team.issues.nodes'

# Filter issues by state (e.g., only In Progress)
STATE_ID="your-state-id"
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"{ issues(filter: { team: { id: { eq: \\\"${TEAM_ID}\\\" } }, state: { id: { eq: \\\"${STATE_ID}\\\" } } }) { nodes { id identifier title assignee { name } } } }\"
  }" | jq '.data.issues.nodes'

# Search issues by keyword
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "{ issueSearch(query: \"authentication timeout\") { nodes { id identifier title state { name } } } }"
  }' | jq '.data.issueSearch.nodes'
```

### Update an Issue

```bash
ISSUE_ID="issue-uuid"
NEW_STATE_ID="in-progress-state-uuid"

# Update issue status
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"mutation UpdateIssue(\$id: String!, \$input: IssueUpdateInput!) { issueUpdate(id: \$id, input: \$input) { success issue { id identifier title state { name } } } }\",
    \"variables\": {
      \"id\": \"${ISSUE_ID}\",
      \"input\": {
        \"stateId\": \"${NEW_STATE_ID}\"
      }
    }
  }" | jq '.data.issueUpdate.issue'

# Add a comment to an issue
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"mutation CreateComment(\$input: CommentCreateInput!) { commentCreate(input: \$input) { success comment { id body } } }\",
    \"variables\": {
      \"input\": {
        \"issueId\": \"${ISSUE_ID}\",
        \"body\": \"Fixed in commit abc1234. Deployed to staging for verification.\"
      }
    }
  }" | jq '.data.commentCreate.comment'
```

## Cycle Management

```bash
# List all cycles for a team
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{\"query\":\"{ team(id: \\\"${TEAM_ID}\\\") { cycles { nodes { id number name startsAt endsAt completedAt } } } }\"}" \
  | jq '.data.team.cycles.nodes'

# Get issues in the active cycle
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"{ team(id: \\\"${TEAM_ID}\\\") { activeCycle { id number startsAt endsAt issues(first: 50) { nodes { id identifier title state { name } assignee { name } } } } } }\"
  }" | jq '.data.team.activeCycle'

# Add an issue to the active cycle
CYCLE_ID="cycle-uuid"
curl -s https://api.linear.app/graphql \
  -H "Authorization: ${LINEAR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"query\": \"mutation AddIssueToCycle(\$issueId: String!, \$cycleId: String!) { cycleCreate(input: { issueId: \$issueId, cycleId: \$cycleId }) { success } }\",
    \"variables\": {
      \"issueId\": \"${ISSUE_ID}\",
      \"cycleId\": \"${CYCLE_ID}\"
    }
  }" | jq '.'
```

## GitHub Integration

Linear can link GitHub PRs and commits to issues automatically.

```bash
# Mention Linear issue in a git commit message to auto-link
git commit -m "fix: resolve authentication timeout issue

Fixes LIN-123. Increased session TTL from 5m to 24h.
See https://linear.app/your-org/issue/LIN-123"

# In PR title or body, reference the issue:
# "fix: authentication timeout (LIN-123)"
# Linear will automatically transition the issue state when PR is merged.
```

### GitHub Actions Integration

```yaml
# .github/workflows/linear-status-update.yml
name: Update Linear Issue Status
on:
  pull_request:
    types: [closed]

jobs:
  update-linear:
    if: github.event.pull_request.merged == true
    runs-on: ubuntu-latest
    steps:
      - name: Extract Linear Issue ID from PR title
        id: linear
        run: |
          ISSUE=$(echo "${{ github.event.pull_request.title }}" | grep -oP 'LIN-\d+' | head -1)
          echo "issue=${ISSUE}" >> $GITHUB_OUTPUT

      - name: Mark issue as Done
        if: steps.linear.outputs.issue != ''
        run: |
          # Get Done state ID for your team
          DONE_STATE_ID="${{ secrets.LINEAR_DONE_STATE_ID }}"
          ISSUE_IDENTIFIER="${{ steps.linear.outputs.issue }}"

          curl -s https://api.linear.app/graphql \
            -H "Authorization: ${{ secrets.LINEAR_API_KEY }}" \
            -H "Content-Type: application/json" \
            -d "{
              \"query\": \"mutation { issueUpdate(id: \\\"${ISSUE_IDENTIFIER}\\\", input: { stateId: \\\"${DONE_STATE_ID}\\\" }) { success } }\"
            }"
```

## Workflow Automation Patterns

```bash
# Auto-create issues from failing CI jobs
create_ci_failure_issue() {
    local job_name="$1"
    local run_url="$2"

    curl -s https://api.linear.app/graphql \
      -H "Authorization: ${LINEAR_API_KEY}" \
      -H "Content-Type: application/json" \
      -d "{
        \"query\": \"mutation CreateIssue(\$input: IssueCreateInput!) { issueCreate(input: \$input) { success issue { identifier url } } }\",
        \"variables\": {
          \"input\": {
            \"teamId\": \"${LINEAR_TEAM_ID}\",
            \"title\": \"CI Failure: ${job_name}\",
            \"description\": \"Automated issue from failed CI run: ${run_url}\",
            \"priority\": 2,
            \"labelIds\": [\"${CI_LABEL_ID}\"]
          }
        }
      }" | jq '.data.issueCreate.issue'
}
```

## Best Practices

- Store `LINEAR_API_KEY` in CI secrets — never commit it to source code.
- Include the Linear issue identifier (e.g., `LIN-123`) in commit messages and PR titles for automatic linking.
- Use priority `1` (Urgent) sparingly; reserve it for production incidents and security issues.
- Create issues at the point of discovery, not just before starting work — this captures the full backlog.
- Use labels consistently across the team to enable filtering (e.g., `bug`, `feature`, `tech-debt`).
- Automate status transitions via GitHub integration to keep Linear in sync with code state without manual updates.
- Use cycles for sprint planning; keep cycle scope realistic (do not add more than team capacity).
