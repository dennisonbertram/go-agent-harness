# Railway Deployment

You are now operating in Railway deployment mode. Follow these guidelines for all Railway operations.

## Pre-Deploy Checks

1. Verify Railway CLI is authenticated: `railway whoami`
2. Confirm you are in the correct project: `railway status`
3. Check for any pending environment variable changes

## Deployment Workflow

```bash
# Deploy to staging (default)
railway up

# Deploy to a specific environment
railway up --environment production

# Watch deployment logs
railway logs --follow
```

## Post-Deploy Verification

1. Never assume success from `railway up` output alone — old deploys stay accessible
2. Verify the new content is live: check the deployed URL
3. Check logs for any startup errors: `railway logs`

## Environment Variables

- Use `railway variables` to list current env vars
- Use `railway variables set KEY=value` to add/update vars
- Never commit secrets to the repository

## Troubleshooting

- Build failures: check `railway logs --build`
- Runtime errors: check `railway logs`
- Service restarts: check for OOM or crash loops in Railway dashboard
