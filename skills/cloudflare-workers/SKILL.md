---
name: cloudflare-workers
description: "Deploy and manage Cloudflare Workers, KV namespaces, D1 databases, R2 buckets, and Durable Objects via the wrangler CLI. Trigger: when deploying to Cloudflare, using wrangler, deploying Workers, managing KV namespaces, D1 databases, R2 buckets, Durable Objects, Cloudflare queues"
version: 1
argument-hint: "[deploy|dev|tail|kv|d1|r2|secret|rollback]"
allowed-tools:
  - bash
  - read
  - write
  - edit
  - glob
  - grep
---
# Cloudflare Workers Deployment

You are now operating in Cloudflare Workers deployment mode using the `wrangler` CLI.

## Prerequisites

```bash
# Install wrangler
npm install -g wrangler

# Authenticate (one-time, interactive)
wrangler login

# Or use API token (preferred for CI/automation)
export CLOUDFLARE_API_TOKEN="your-token-here"
# Generate at: https://dash.cloudflare.com/profile/api-tokens
# Required scopes: Workers Scripts:Edit, Workers KV Storage:Edit, D1:Edit, R2:Edit

# Verify authentication
wrangler whoami
```

## Project Structure

```
my-worker/
├── wrangler.toml      # Configuration file
├── src/
│   └── index.ts       # Worker entrypoint
└── package.json
```

### Minimal `wrangler.toml`

```toml
name = "my-worker"
main = "src/index.ts"
compatibility_date = "2024-01-01"

# Optional: bind KV namespace
[[kv_namespaces]]
binding = "MY_KV"
id = "abc123def456"

# Optional: bind D1 database
[[d1_databases]]
binding = "DB"
database_name = "my-database"
database_id = "abc123"

# Optional: bind R2 bucket
[[r2_buckets]]
binding = "MY_BUCKET"
bucket_name = "my-bucket"
```

## Local Development

```bash
# Start local dev server (uses local emulation)
wrangler dev

# Dev against real Cloudflare network (remote bindings)
wrangler dev --remote

# Specify port
wrangler dev --port 8787

# Enable live reload on save
wrangler dev --watch
```

## Deployment

```bash
# Deploy to production
wrangler deploy

# Dry run (validate without deploying)
wrangler deploy --dry-run

# Deploy to a named environment (requires [env.staging] in wrangler.toml)
wrangler deploy --env staging
wrangler deploy --env production

# Deploy and verify output
wrangler deploy 2>&1 | tee deploy.log
grep "Deployed" deploy.log
```

## Multi-Environment Configuration

```toml
# wrangler.toml

name = "my-worker"
main = "src/index.ts"
compatibility_date = "2024-01-01"

[env.staging]
name = "my-worker-staging"
vars = { ENVIRONMENT = "staging" }

[env.production]
name = "my-worker-production"
vars = { ENVIRONMENT = "production" }
```

## Monitoring: Live Log Tailing

```bash
# Tail logs in real-time
wrangler tail

# Always use --format json for parsing
wrangler tail --format json

# Tail a specific environment
wrangler tail --env production --format json

# Filter by status code (e.g., only errors)
wrangler tail --status error

# Filter by sampling rate (0.0 to 1.0)
wrangler tail --sampling-rate 0.1
```

## Version Management and Rollback

```bash
# List deployed versions
wrangler versions list
wrangler versions list --json

# View a specific version's details
wrangler versions view <version-id>

# Upload a new version without activating
wrangler versions upload

# Activate a specific version (gradual rollout)
wrangler versions deploy <version-id>

# Deploy with percentage traffic split
wrangler versions deploy <version-id> --percentage 10

# Rollback to previous version
wrangler rollback
```

## Secrets Management

```bash
# Set a secret (prompts for value, never logged)
wrangler secret put DATABASE_URL

# Set a secret for a specific environment
wrangler secret put DATABASE_URL --env production

# List all secret names (values are never shown)
wrangler secret list

# Delete a secret
wrangler secret delete DATABASE_URL
```

## KV Namespace Operations

```bash
# Create a KV namespace
wrangler kv namespace create MY_NAMESPACE

# List all namespaces
wrangler kv namespace list

# Write a key-value pair
wrangler kv key put mykey "myvalue" --namespace-id <namespace-id>

# Read a value
wrangler kv key get mykey --namespace-id <namespace-id>

# List keys with prefix
wrangler kv key list --namespace-id <namespace-id> --prefix "user:"

# Delete a key
wrangler kv key delete mykey --namespace-id <namespace-id>

# Bulk upload from JSON file
wrangler kv bulk put data.json --namespace-id <namespace-id>
```

KV is eventually consistent (~60s). Best for: config, feature flags, cached reads.

## D1 Database Operations

```bash
# Create a D1 database
wrangler d1 create my-database

# List all D1 databases
wrangler d1 list

# Execute SQL (local emulation)
wrangler d1 execute my-database --command "SELECT * FROM users LIMIT 10;"

# Execute SQL on production (always use --remote for production)
wrangler d1 execute my-database --remote --command "SELECT COUNT(*) FROM users;"

# Execute a SQL file
wrangler d1 execute my-database --remote --file ./schema.sql

# Apply migrations
wrangler d1 migrations apply my-database --remote

# Show migration status
wrangler d1 migrations list my-database
```

D1 is strongly consistent SQLite. Best for: structured data, relational queries.

## R2 Bucket Operations

```bash
# Create an R2 bucket
wrangler r2 bucket create my-bucket

# List all buckets
wrangler r2 bucket list

# Upload an object
wrangler r2 object put my-bucket/path/to/file.txt --file ./local-file.txt

# Download an object
wrangler r2 object get my-bucket/path/to/file.txt --file ./downloaded.txt

# Delete an object
wrangler r2 object delete my-bucket/path/to/file.txt
```

R2 has zero egress fees. Best for: files, media, backups.

## Queues Operations

```bash
# Create a queue
wrangler queues create my-queue

# List queues
wrangler queues list

# Send a message to a queue (for testing)
wrangler queues send my-queue '{"event": "test", "data": {}}'
```

## Worker Deletion

```bash
# Delete a Worker (irreversible!)
wrangler delete

# Delete a specific environment's worker
wrangler delete --env staging
```

## Deployment Patterns

### Pattern 1: Standard Deploy and Verify

```bash
# 1. Deploy
wrangler deploy --env production

# 2. Tail logs to verify health (press Ctrl+C when satisfied)
wrangler tail --env production --format json &
TAIL_PID=$!

# 3. Wait a few seconds for traffic
sleep 10

# 4. Stop tailing
kill $TAIL_PID

# 5. Report deployment URL (from deploy output)
```

### Pattern 2: Multi-Environment Pipeline

```bash
# 1. Deploy to staging first
wrangler deploy --env staging

# 2. Verify staging works
curl -f https://my-worker-staging.workers.dev/health

# 3. Deploy to production only if staging passes
wrangler deploy --env production

# 4. Verify production
curl -f https://my-worker-production.workers.dev/health
```

### Pattern 3: Blue-Green with Traffic Splitting

```bash
# 1. Upload new version without activating
wrangler versions upload

# 2. Get the new version ID
NEW_VERSION=$(wrangler versions list --json | jq -r '.[0].id')

# 3. Route 10% of traffic to new version
wrangler versions deploy "$NEW_VERSION" --percentage 10

# 4. Monitor logs for errors
wrangler tail --format json | jq 'select(.outcome == "exception")'

# 5. Promote to 100% if healthy
wrangler versions deploy "$NEW_VERSION" --percentage 100

# OR rollback if problems found
wrangler rollback
```

## Cloudflare Storage Binding Reference

| Binding | Best For | Consistency | Notes |
|---------|---------|-------------|-------|
| KV | Config, feature flags, cached reads | Eventually (~60s) | $0.50/M reads, $5/M writes |
| R2 | Files, media, backups | Strong | Zero egress, $0.015/GB-month |
| D1 | Structured SQL data | Strong (SQLite) | $0.001/M reads |
| Durable Objects | Real-time, WebSockets, per-entity state | Strong (single-threaded) | $0.15/M requests |
| Queues | Background jobs, async processing | At-least-once | $0.40/M after 1M free |
| Workers AI | LLM inference, embeddings | N/A | Per-token billing |

## Free Tier Limits

- 100,000 requests/day
- 10ms CPU time per invocation
- KV: 100k reads/day, 1k writes/day, 1 GB storage
- D1: 5M rows read/day, 100k rows written/day, 5 GB storage
- R2: 10 GB storage, 1M Class A ops, 10M Class B ops

## Safety Rules

- Never commit `CLOUDFLARE_API_TOKEN` to source control — use environment variables or `wrangler secret put`.
- Always deploy to staging before production.
- Always tail logs after deployment to verify no errors.
- Use `--dry-run` to validate the deployment bundle before pushing to production.
- Use `wrangler rollback` immediately if errors spike after deployment.
- D1 migrations with `--remote` modify live production data — test locally first.
