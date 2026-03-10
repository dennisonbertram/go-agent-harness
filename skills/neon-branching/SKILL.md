---
name: neon-branching
description: "Manage Neon serverless Postgres database branches: branch-per-PR workflow, neon branch create/delete, connection strings, reset branch from parent. Trigger: when using Neon database, neon branch create, neon branch delete, Neon branching, database branching, branch-per-PR, Neon Postgres"
version: 1
argument-hint: "[branch create|delete|list|get] [--name branch-name] [--project-id proj-id]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Neon Database Branching

You are now operating in Neon serverless Postgres branching mode.

## Installation and Setup

```bash
# Install Neon CLI
npm install -g neonctl

# macOS (via Homebrew)
brew install neonctl

# Authenticate with Neon (opens browser)
neonctl auth

# Authenticate with API key (CI/CD)
neonctl auth --token $NEON_API_KEY

# Verify authentication
neonctl me

# Set default project (avoids --project-id on every command)
neonctl set-context --project-id <project-id>

# List your projects
neonctl projects list
```

## Branch Management

```bash
# List all branches in a project
neonctl branches list
neonctl branches list --project-id <project-id>

# Get branch details
neonctl branches get main
neonctl branches get my-feature-branch

# Create a branch from the default (main) branch
neonctl branches create --name my-feature-branch

# Create a branch from a specific parent branch
neonctl branches create --name feature-db-v2 --parent staging

# Create a branch from a specific point in time (point-in-time restore)
neonctl branches create \
  --name restore-to-yesterday \
  --parent main \
  --parent-timestamp 2024-01-15T10:00:00Z

# Create a branch from a specific LSN (Log Sequence Number)
neonctl branches create \
  --name restore-to-lsn \
  --parent main \
  --parent-lsn 0/18B5F28

# Delete a branch
neonctl branches delete my-feature-branch

# Rename a branch
neonctl branches rename my-feature-branch --name my-renamed-branch

# Set branch as primary (makes it the default branch)
neonctl branches set-primary my-branch

# Reset branch to match parent HEAD (discard all changes)
neonctl branches reset my-feature-branch --parent
```

## Connection Strings

```bash
# Get connection string for a branch (uses default compute)
neonctl connection-string my-feature-branch

# Get connection string for the main branch
neonctl connection-string main

# Get connection string with specific role and database
neonctl connection-string my-branch \
  --role-name myuser \
  --database-name mydb

# Get pooled connection string (PgBouncer)
neonctl connection-string my-branch --pooled

# Output as environment variable format
neonctl connection-string my-branch --output env

# Store connection string in shell variable
BRANCH_DB_URL=$(neonctl connection-string my-feature-branch)
export DATABASE_URL="$BRANCH_DB_URL"

# Output connection string components separately
neonctl connection-string my-branch --output json | jq '{
  host: .host,
  port: .port,
  database: .dbname,
  user: .user,
  password: .password
}'
```

## Branch-per-PR Workflow

This is the primary Neon use case: create a fresh database branch for each pull request, run migrations, test, then delete the branch when the PR closes.

### Manual Workflow

```bash
# 1. Create branch when PR opens (use PR number as branch name)
PR_NUMBER=42
BRANCH_NAME="pr-${PR_NUMBER}"

neonctl branches create --name "$BRANCH_NAME" --parent main

# 2. Get connection string
DB_URL=$(neonctl connection-string "$BRANCH_NAME")
export DATABASE_URL="$DB_URL"

# 3. Run migrations on the branch
npm run db:migrate
# or: goose up
# or: flyway migrate

# 4. Run integration tests
npm test

# 5. Delete branch when PR closes
neonctl branches delete "$BRANCH_NAME"
```

### GitHub Actions Automation

```yaml
# .github/workflows/preview-db.yml
name: Preview Database Branch

on:
  pull_request:
    types: [opened, synchronize, reopened, closed]

jobs:
  setup-db-branch:
    if: github.event.action != 'closed'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Neon CLI
        run: npm install -g neonctl

      - name: Create database branch
        id: create-branch
        env:
          NEON_API_KEY: ${{ secrets.NEON_API_KEY }}
          PROJECT_ID: ${{ vars.NEON_PROJECT_ID }}
        run: |
          BRANCH_NAME="pr-${{ github.event.number }}"

          # Create branch (idempotent: delete if exists, then create)
          neonctl branches delete "$BRANCH_NAME" 2>/dev/null || true
          neonctl branches create \
            --name "$BRANCH_NAME" \
            --parent main \
            --project-id "$PROJECT_ID"

          # Get connection string
          DB_URL=$(neonctl connection-string "$BRANCH_NAME" \
            --project-id "$PROJECT_ID" \
            --role-name neondb_owner \
            --database-name neondb)

          echo "db_url=$DB_URL" >> $GITHUB_OUTPUT
          echo "branch_name=$BRANCH_NAME" >> $GITHUB_OUTPUT

      - name: Run migrations
        env:
          DATABASE_URL: ${{ steps.create-branch.outputs.db_url }}
        run: npm run db:migrate

      - name: Run tests
        env:
          DATABASE_URL: ${{ steps.create-branch.outputs.db_url }}
        run: npm test

      - name: Comment PR with branch URL
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: '🗄️ Preview database branch: `${{ steps.create-branch.outputs.branch_name }}`'
            })

  cleanup-db-branch:
    if: github.event.action == 'closed'
    runs-on: ubuntu-latest
    steps:
      - name: Delete database branch
        env:
          NEON_API_KEY: ${{ secrets.NEON_API_KEY }}
          PROJECT_ID: ${{ vars.NEON_PROJECT_ID }}
        run: |
          npm install -g neonctl
          BRANCH_NAME="pr-${{ github.event.number }}"
          neonctl branches delete "$BRANCH_NAME" \
            --project-id "$PROJECT_ID" || true
```

### Vercel Integration Pattern

```bash
# In vercel.json or deployment scripts
# Neon integrates with Vercel for automatic preview branch creation

# Manual equivalent with Vercel CLI
vercel env add DATABASE_URL preview "$(neonctl connection-string pr-42)"
```

## Compute Endpoints

Each branch can have multiple compute endpoints. By default, endpoints use autosuspend.

```bash
# List endpoints for a branch
neonctl endpoints list --branch my-feature-branch

# Create an additional endpoint for a branch (e.g., read replica)
neonctl endpoints create \
  --branch my-feature-branch \
  --type read_only

# Get endpoint details
neonctl endpoints get <endpoint-id>

# Start a suspended endpoint
neonctl endpoints start <endpoint-id>

# Suspend an endpoint (saves compute costs)
neonctl endpoints suspend <endpoint-id>

# Delete an endpoint
neonctl endpoints delete <endpoint-id>

# Update compute size
neonctl endpoints update <endpoint-id> --cu-min 0.25 --cu-max 2
```

## Database and Role Management

```bash
# List databases in a branch
neonctl databases list --branch my-feature-branch

# Create a database in a branch
neonctl databases create mydb --branch my-feature-branch

# Delete a database
neonctl databases delete mydb --branch my-feature-branch

# List roles
neonctl roles list --branch my-feature-branch

# Create a role
neonctl roles create myuser --branch my-feature-branch

# Get role password (for connection strings)
neonctl roles get myuser --branch my-feature-branch
```

## Point-in-Time Restore

```bash
# Restore a branch to a specific timestamp
# (Creates a new branch, doesn't modify existing)
neonctl branches create \
  --name restored-20240115 \
  --parent main \
  --parent-timestamp 2024-01-15T10:00:00Z

# After restoring, swap connection strings in your app
# or reset the production branch from the restored branch

# Reset production from a restored point (DESTRUCTIVE)
# This replaces production's data with the restored branch's data
neonctl branches reset production --parent restored-20240115

# Alternative: rename branches to swap
neonctl branches rename production --name production-backup
neonctl branches rename restored-20240115 --name production
```

## Using Neon via REST API

```bash
# Base URL
NEON_API="https://console.neon.tech/api/v2"
AUTH="Authorization: Bearer $NEON_API_KEY"

# List branches
curl -s -H "$AUTH" \
  "$NEON_API/projects/$PROJECT_ID/branches" | jq '.branches[] | {id, name, created_at}'

# Create a branch
curl -s -X POST -H "$AUTH" -H "Content-Type: application/json" \
  "$NEON_API/projects/$PROJECT_ID/branches" \
  -d '{
    "branch": {"name": "my-branch", "parent_id": "main-branch-id"},
    "endpoints": [{"type": "read_write"}]
  }' | jq .

# Delete a branch
curl -s -X DELETE -H "$AUTH" \
  "$NEON_API/projects/$PROJECT_ID/branches/$BRANCH_ID"

# Get connection URI
curl -s -H "$AUTH" \
  "$NEON_API/projects/$PROJECT_ID/connection_uri?branch_id=$BRANCH_ID&role_name=neondb_owner&database_name=neondb" | \
  jq -r '.uri'
```

## Cost Management

```bash
# Autosuspend prevents charges when branch is idle
# Branches suspend automatically after inactivity (default: 5 minutes)

# Check current compute state
neonctl endpoints list | jq '.endpoints[] | {id, current_state, autosuspend_duration_seconds}'

# Branches share storage — only incremental changes cost extra
# Delete unused PR branches promptly to avoid storage costs

# Estimate branch storage
neonctl branches get my-branch | jq '.branch.logical_size'

# Check project usage/billing
neonctl projects get $PROJECT_ID | jq '.project | {
  data_storage_bytes_hour: .data_storage_bytes_hour,
  compute_time_seconds: .compute_time_seconds
}'
```

## Troubleshooting

```bash
# Verbose output for debugging
neonctl --debug branches list

# Check CLI version
neonctl --version

# Re-authenticate if token expired
neonctl auth

# Check project ID
neonctl projects list | jq '.projects[] | {id, name}'

# Common issues:
# "Branch not found" — check name with: neonctl branches list
# "Endpoint is starting" — branches may take 1-3s to start after autosuspend
# "Too many connections" — use pooled connection string with --pooled flag
# "Auth token expired" — run: neonctl auth
# Connection timeout — check endpoint state: neonctl endpoints list
```
