---
name: fly-deploy
description: "Deploy containers globally to Fly.io with multi-region support, managed Postgres, volumes, and Firecracker microVMs using the fly CLI. Trigger: when deploying to Fly.io, using flyctl, fly launch, fly deploy, managing Fly machines, Fly Postgres, multi-region deployment"
version: 1
argument-hint: "[launch|deploy|status|logs|ssh|scale|secrets|postgres]"
allowed-tools:
  - bash
  - read
  - write
  - edit
  - glob
  - grep
---
# Fly.io Deployment

You are now operating in Fly.io deployment mode using the `fly` CLI (flyctl).

## Prerequisites

```bash
# Install flyctl
# macOS
brew install flyctl

# Linux / macOS alternative
curl -L https://fly.io/install.sh | sh

# Authenticate
fly auth login

# Or use API token (preferred for CI/automation)
export FLY_API_TOKEN="your-token-here"
# Generate at: https://fly.io/user/personal_access_tokens

# Verify authentication
fly auth whoami
```

## Initializing a New App

```bash
# Launch a new app (detects Dockerfile, prompts for config)
fly launch

# Launch without interactive prompts
fly launch --yes

# Launch without deploying (generate fly.toml only)
fly launch --no-deploy

# Launch with a specific name and region
fly launch --name my-app --region iad --yes

# Launch and override org
fly launch --org my-org --yes
```

## Application Configuration (`fly.toml`)

```toml
app = "my-app"
primary_region = "iad"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8080"
  ENVIRONMENT = "production"

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 1

  [[http_service.checks]]
    grace_period = "10s"
    interval = "30s"
    method = "GET"
    path = "/health"
    timeout = "5s"

[[vm]]
  cpu_kind = "shared"
  cpus = 1
  memory_mb = 256
```

## Deploying

```bash
# Deploy current project
fly deploy

# Deploy with a specific strategy
fly deploy --strategy rolling       # rolling update (default)
fly deploy --strategy canary        # canary release
fly deploy --strategy bluegreen     # blue-green deployment
fly deploy --strategy immediate     # replace all at once

# Deploy and wait for health checks to pass
fly deploy --wait-timeout 120s

# Build and deploy a specific Docker image
fly deploy --image registry.fly.io/my-app:v1.2.3

# Deploy to a specific region only
fly deploy --regions iad

# Verbose output for debugging
fly deploy --verbose
```

## Checking Status

```bash
# App status (shows machines and their health)
fly status

# Status in JSON format (for parsing)
fly status --json

# List all machines
fly machine list

# Machine list in JSON
fly machine list --json

# Check a specific machine
fly machine status <machine-id>

# List all apps in the org
fly apps list
```

## Viewing Logs

```bash
# Stream live logs
fly logs

# Logs in JSON format (for parsing/filtering)
fly logs --json

# Logs from a specific region
fly logs --region iad

# Logs from a specific machine
fly logs --machine <machine-id>

# Filter logs with grep
fly logs | grep -i error

# Parse structured JSON logs
fly logs --json | jq 'select(.level == "error")'
```

## Scaling

```bash
# Scale to N machines (replicas)
fly scale count 3

# Scale per region
fly scale count 2 --region iad
fly scale count 1 --region lhr

# Change VM size (shared CPU)
fly scale vm shared-cpu-1x    # 256 MB RAM
fly scale vm shared-cpu-2x    # 512 MB RAM
fly scale vm shared-cpu-4x    # 1 GB RAM

# Change VM size (dedicated CPU)
fly scale vm performance-1x   # 2 GB RAM
fly scale vm performance-2x   # 4 GB RAM

# Show current VM count and sizes
fly scale show
```

## Multi-Region Configuration

```bash
# Add regions to the app
fly regions add ord lax ams

# Remove a region
fly regions remove lhr

# List active regions
fly regions list

# Set the primary region
fly regions set-primary iad

# Deploy and distribute across regions
fly scale count 2 --region ord
fly scale count 2 --region lax
fly deploy
```

## Secrets Management

```bash
# Set a secret (never logged or shown after set)
fly secrets set DATABASE_URL="postgresql://..."

# Set multiple secrets at once
fly secrets set \
  DATABASE_URL="postgresql://..." \
  API_KEY="sk_live_..." \
  JWT_SECRET="..."

# List secret names (values are never shown)
fly secrets list

# Remove a secret
fly secrets unset DATABASE_URL

# Import secrets from a .env file
fly secrets import < .env.production
```

## SSH Access

```bash
# Open an interactive shell in a running machine
fly ssh console

# Run a one-off command (non-interactive)
fly ssh console -C "ls /app"

# SSH to a specific machine
fly ssh console --machine <machine-id>

# SSH to a specific region
fly ssh console --region lax

# Copy files from a machine
fly ssh sftp get /app/logs/app.log ./app.log
```

## Volumes (Persistent Storage)

```bash
# Create a volume in a region
fly volumes create myapp_data --region iad --size 10

# List volumes
fly volumes list

# Extend a volume (increase size)
fly volumes extend <volume-id> --size 20

# Show volume details
fly volumes show <volume-id>

# Attach volume in fly.toml
# [[mounts]]
#   source = "myapp_data"
#   destination = "/data"
```

## Managed Postgres

```bash
# Create a managed Postgres cluster
fly postgres create --name myapp-db --region iad

# Create with specific plan
fly postgres create --name myapp-db --region iad --initial-cluster-size 1 --vm-size shared-cpu-1x --volume-size 10

# Attach Postgres to an app (injects DATABASE_URL secret)
fly postgres attach myapp-db --app my-app

# Connect to Postgres with psql
fly postgres connect -a myapp-db

# View Postgres status
fly postgres status -a myapp-db

# List Postgres clusters
fly postgres list

# Failover (promote replica)
fly postgres failover -a myapp-db

# Detach Postgres from app
fly postgres detach myapp-db --app my-app
```

## Deployment Patterns

### Pattern 1: Single-Region Production Deploy

```bash
# 1. Launch or deploy
fly deploy

# 2. Verify status
fly status --json | jq '.Machines[] | {id, state, region}'

# 3. Check logs for errors
fly logs 2>&1 | head -50

# 4. Report app URL
fly status | grep "Hostname:"
```

### Pattern 2: Multi-Region Deployment

```bash
# 1. Launch app
fly launch --yes

# 2. Add regions
fly regions add ord lax ams

# 3. Scale in each region
fly scale count 2 --region ord
fly scale count 2 --region lax
fly scale count 1 --region ams

# 4. Deploy globally
fly deploy

# 5. Verify all regions are healthy
fly status --json | jq '.Machines[] | {region, state}'
```

### Pattern 3: App with Managed Postgres

```bash
# 1. Create Postgres cluster
fly postgres create --name myapp-db --region iad

# 2. Create the app
fly launch --name myapp --region iad --no-deploy

# 3. Attach Postgres (sets DATABASE_URL automatically)
fly postgres attach myapp-db --app myapp

# 4. Deploy (DATABASE_URL is available in the container)
fly deploy

# 5. Verify database connection
fly ssh console -C "psql \$DATABASE_URL -c 'SELECT 1'"
```

## VM Sizing Reference

| VM Size | CPUs | RAM | Price/Month |
|---------|------|-----|-------------|
| shared-cpu-1x | 1 shared | 256 MB | ~$1.94 |
| shared-cpu-2x | 2 shared | 512 MB | ~$3.88 |
| shared-cpu-4x | 4 shared | 1 GB | ~$7.76 |
| performance-1x | 1 dedicated | 2 GB | ~$31 |
| performance-2x | 2 dedicated | 4 GB | ~$62 |

## Free Tier

- 3 shared-cpu-1x VMs with 256 MB RAM
- 3 GB persistent volume storage
- 160 GB outbound data transfer
- Shared IPv4, dedicated IPv6

## Safety Rules

- Never commit `FLY_API_TOKEN` to source control — use environment variables or CI secrets.
- Use `fly secrets set` for sensitive values; never pass them via environment variables in `fly.toml`.
- Test locally with Docker before deploying to Fly.io.
- Use `fly deploy --strategy rolling` (default) to avoid downtime during updates.
- Always check `fly status` after deployment to confirm machines are healthy.
- SSH (`fly ssh console`) can be used for emergency debugging — avoid running destructive commands.
- Volumes cannot be moved between regions — plan region placement before creating volumes.
- Postgres `fly postgres failover` promotes a replica — only use in emergency, not routine maintenance.
