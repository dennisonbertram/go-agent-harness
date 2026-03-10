---
name: railway-deploy
description: "Deploy applications to Railway: railway up, environment management, domains, secrets/environment variables, logs, service management. Trigger: when deploying to Railway, using railway up, Railway environment variables, Railway domains, Railway logs, Railway CLI"
version: 1
argument-hint: "[up|logs|status|env|domain]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Railway Deployment

You are now operating in Railway deployment mode.

## Installation and Authentication

```bash
# Install Railway CLI
npm install -g @railway/cli

# Login (opens browser)
railway login

# Verify authentication
railway whoami
```

## Project Setup

```bash
# Link current directory to an existing Railway project
railway link

# Initialize a new Railway project
railway init

# Show current project info
railway status
```

## Deploy

```bash
# Deploy from current directory (triggers a build + deploy)
railway up

# Deploy with verbose output
railway up --verbose

# Deploy detached (do not stream logs)
railway up --detach

# Deploy a specific service
railway up --service my-service

# Force a redeploy of the current image (no rebuild)
railway redeploy
```

## Environment Variables and Secrets

```bash
# List all environment variables for current environment
railway variables

# Set an environment variable
railway variables set DATABASE_URL="postgresql://..."
railway variables set SECRET_KEY="$(openssl rand -hex 32)"

# Set multiple variables at once
railway variables set \
  APP_ENV=production \
  LOG_LEVEL=info \
  PORT=8080

# Delete an environment variable
railway variables delete OLD_VAR

# Import from a .env file
railway variables import .env.production
```

## Environments

Railway supports multiple environments (production, staging, etc.).

```bash
# List environments
railway environment

# Switch to a specific environment
railway environment staging

# Create a new environment
# (done via Railway dashboard or CLI prompts)

# Show the current environment
railway status
```

## Domains and Networking

```bash
# Generate a Railway subdomain for the service
railway domain

# Add a custom domain
railway domain add myapp.example.com

# List all domains for the service
railway domain list
```

## Logs

```bash
# Tail deployment logs
railway logs

# Stream logs in real time
railway logs --tail

# View logs for a specific deployment
railway logs --deployment <deployment-id>

# Filter logs by search term
railway logs | grep ERROR
```

## Database Services

Railway can provision databases alongside your app:

```bash
# After adding a Postgres plugin in the dashboard, the
# DATABASE_URL variable is automatically injected.

# Connect to Railway Postgres from CLI
railway run psql $DATABASE_URL

# Run a migration against Railway Postgres
railway run goose -dir migrations postgres "$DATABASE_URL" up

# Dump the Railway database
railway run pg_dump "$DATABASE_URL" > backup.sql
```

## Running One-Off Commands

```bash
# Execute a command in the Railway environment (uses Railway env vars)
railway run go test ./...
railway run ./scripts/migrate.sh
railway run bash -c "echo $DATABASE_URL"
```

## Service Management

```bash
# List all services in the project
railway service

# Show service info
railway service --json

# Restart a service
railway service restart
```

## Monitoring Deployments

```bash
# Check deployment status
railway status

# List recent deployments
railway deployments list 2>/dev/null || railway status

# Verify the deployed commit
railway status --json | jq '.deploymentStatus'
```

## CI/CD Integration

```yaml
# GitHub Actions example
name: Deploy to Railway

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Railway CLI
        run: npm install -g @railway/cli

      - name: Deploy
        env:
          RAILWAY_TOKEN: ${{ secrets.RAILWAY_TOKEN }}
        run: railway up --detach
```

## Post-Deploy Verification

```bash
# Check the service is responding
curl -s -o /dev/null -w "%{http_code}" https://myapp.up.railway.app/health

# Verify the correct commit was deployed
railway status --json | jq '.commitSha'
git rev-parse HEAD

# Monitor initial startup logs
railway logs --tail | head -50
```

## Troubleshooting

```bash
# View build logs for a failed deployment
railway logs --deployment <id>

# Check environment variable is set
railway run printenv APP_ENV

# Validate the Railway config file
cat railway.json 2>/dev/null || cat railway.toml 2>/dev/null

# Force a fresh deployment (clear build cache)
railway up --no-cache 2>/dev/null || railway up
```

## railway.json Configuration

```json
{
  "$schema": "https://railway.app/railway.schema.json",
  "build": {
    "builder": "DOCKERFILE",
    "dockerfilePath": "Dockerfile"
  },
  "deploy": {
    "startCommand": "./server",
    "healthcheckPath": "/health",
    "healthcheckTimeout": 100,
    "restartPolicyType": "ON_FAILURE",
    "restartPolicyMaxRetries": 3
  }
}
```

## Safety Rules

- Never commit `RAILWAY_TOKEN` to source control — use GitHub Secrets or environment-scoped variables.
- Always verify deployed content after `railway up`; a success exit code does not guarantee the new code is live.
- Use environment-specific secrets: production and staging should have separate `DATABASE_URL` values.
