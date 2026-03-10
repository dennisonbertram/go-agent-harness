---
name: vercel-deploy
description: "Deploy frontend apps, Next.js projects, and serverless functions to Vercel with preview URLs and production deployments. Trigger: when deploying to Vercel, using vercel CLI, deploying Next.js, creating preview deployments, managing Vercel environments, adding Vercel domains"
version: 1
argument-hint: "[deploy|--prod|env|logs|domains|inspect]"
allowed-tools:
  - bash
  - read
  - write
  - edit
  - glob
  - grep
---
# Vercel Deployment

You are now operating in Vercel deployment mode using the `vercel` CLI.

## Prerequisites

```bash
# Install Vercel CLI
npm install -g vercel

# Authenticate (one-time, interactive browser flow)
vercel login

# Or use API token (preferred for CI/automation)
export VERCEL_TOKEN="your-token-here"
# Generate at: https://vercel.com/account/tokens

# Verify authentication
vercel whoami
```

## Linking a Project

```bash
# Link current directory to a Vercel project (interactive)
vercel link

# Link to a specific team/org project
vercel link --scope my-team --project my-project

# Pull project settings and env vars locally
vercel env pull .env.local
```

## Preview Deployments

Preview deployments create a unique URL for review before production.

```bash
# Deploy to preview (default behavior)
vercel

# Skip interactive prompts (use project defaults)
vercel --yes

# Deploy a specific directory
vercel ./dist --yes

# Deploy and output only the URL
vercel --yes 2>/dev/null | tail -1
```

## Production Deployments

```bash
# Deploy to production
vercel --prod --yes

# Deploy with confirmation of the production URL
vercel --prod --yes 2>&1 | grep "Production:"

# Deploy a specific branch by pointing to the directory
vercel --prod --yes --archive=tgz
```

## Deployment Inspection

```bash
# List recent deployments
vercel ls

# List deployments with JSON output for parsing
vercel ls --json

# Inspect a specific deployment by URL
vercel inspect https://my-app-abc123.vercel.app

# Get deployment details including build logs URL
vercel inspect <deployment-url> --json
```

## Viewing Logs

```bash
# View logs for the latest production deployment
vercel logs <deployment-url>

# Follow logs in real-time
vercel logs <deployment-url> --follow

# View logs for a specific deployment
vercel logs https://my-app-abc123.vercel.app --follow

# Filter log output
vercel logs <url> --follow 2>&1 | grep -i error
```

## Environment Variable Management

```bash
# Add an environment variable interactively
vercel env add DATABASE_URL

# Add env var to a specific target environment
vercel env add DATABASE_URL production
vercel env add DATABASE_URL preview
vercel env add DATABASE_URL development

# Add env var with a value (pipe input to avoid interactive prompt)
echo "postgresql://..." | vercel env add DATABASE_URL production

# List all environment variables (values are hidden)
vercel env ls

# Remove an environment variable
vercel env rm DATABASE_URL production

# Pull all env vars to a local file
vercel env pull .env.local
vercel env pull .env.production --environment production
```

## Domain Management

```bash
# List all domains for the account
vercel domains ls

# Add a custom domain to a project
vercel domains add my-app.example.com

# Add a domain to a specific project
vercel domains add my-app.example.com --project my-project

# Inspect domain configuration (DNS, cert status)
vercel domains inspect my-app.example.com

# Remove a domain
vercel domains rm my-app.example.com
```

## Removing Deployments

```bash
# Remove a deployment by URL
vercel rm https://my-app-abc123.vercel.app

# Remove all deployments of a project (keep latest)
vercel rm my-project --safe

# Force remove without confirmation
vercel rm my-project --yes
```

## Deployment Patterns

### Pattern 1: Preview Deploy for Code Review

```bash
# 1. Deploy preview
PREVIEW_URL=$(vercel --yes 2>/dev/null | tail -1)
echo "Preview URL: $PREVIEW_URL"

# 2. Run smoke test against preview
curl -f "$PREVIEW_URL/api/health" || echo "Health check failed"

# 3. Share preview URL for review
echo "Share this URL for review: $PREVIEW_URL"
```

### Pattern 2: Production Deployment with Verification

```bash
# 1. Deploy to production
vercel --prod --yes

# 2. Get the production URL
PROD_URL=$(vercel ls --json | jq -r '.[0].url' | head -1)

# 3. Verify the deployment is live
curl -f "https://$PROD_URL" || echo "Production check failed"

# 4. Check that new content is live (not stale cache)
curl -s "https://$PROD_URL/api/version" | jq '.version'

# 5. View logs to confirm no errors
vercel logs "https://$PROD_URL" --follow &
LOGS_PID=$!
sleep 15
kill $LOGS_PID
```

### Pattern 3: Feature Branch Preview

```bash
# 1. Checkout feature branch
git checkout feature/my-feature

# 2. Deploy preview
PREVIEW_URL=$(vercel --yes 2>/dev/null | tail -1)

# 3. Output URL for CI comment or review
echo "Feature preview: $PREVIEW_URL"
```

## Vercel Project Configuration (`vercel.json`)

```json
{
  "buildCommand": "npm run build",
  "outputDirectory": "dist",
  "installCommand": "npm ci",
  "framework": "nextjs",
  "regions": ["iad1", "sfo1"],
  "rewrites": [
    { "source": "/api/(.*)", "destination": "/api/$1" }
  ],
  "headers": [
    {
      "source": "/api/(.*)",
      "headers": [
        { "key": "Cache-Control", "value": "no-store" }
      ]
    }
  ]
}
```

## Parsing Deployment Output

```bash
# Get preview URL from deployment
vercel --yes 2>&1 | grep -E "https://.*vercel\.app"

# Get production URL
vercel --prod --yes 2>&1 | grep "Production:"

# Parse deployment list JSON
vercel ls --json | jq '.[0] | {url, state, created}'

# Check if deployment is ready
vercel ls --json | jq '.[] | select(.state == "READY") | .url' | head -1
```

## Vercel Pricing Reference

| Plan | Cost | Bandwidth | Features |
|------|------|-----------|---------|
| Hobby | Free | 100 GB | 1 user, personal projects |
| Pro | $20/member/month | 1 TB | Teams, analytics, preview protection |
| Enterprise | Custom | Custom | SSO, audit logs, SLA |

## Safety Rules

- Never commit `VERCEL_TOKEN` to source control — use environment variables or CI secrets.
- Always deploy to preview first; verify the preview URL before promoting to production.
- Use `vercel env pull` to sync environment variables locally; never hardcode secrets.
- Verify new content is live after production deploy — Vercel CDN may serve stale cache briefly.
- Use `vercel logs --follow` immediately after production deployment to catch startup errors.
- Domain changes can take up to 48 hours for DNS propagation — plan accordingly.
