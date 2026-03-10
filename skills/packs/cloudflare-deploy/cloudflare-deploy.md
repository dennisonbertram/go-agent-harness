# Cloudflare Deployment

You are now operating in Cloudflare deployment mode. Follow these guidelines for all Cloudflare operations.

## Pre-Deploy Checks

1. Verify Wrangler authentication: `wrangler whoami`
2. Confirm correct account: check `wrangler.toml` for `account_id`
3. Review any changes to Worker scripts or Pages configuration

## Workers Deployment

```bash
# Deploy a Worker
wrangler deploy

# Deploy to a specific environment
wrangler deploy --env production

# Tail logs in real time
wrangler tail
```

## Pages Deployment

```bash
# Deploy a Pages project
wrangler pages deploy ./dist

# Deploy with a specific project name
wrangler pages deploy ./dist --project-name my-app
```

## KV and Storage

```bash
# List KV namespaces
wrangler kv namespace list

# Put a value
wrangler kv key put --namespace-id <id> KEY "value"

# List keys
wrangler kv key list --namespace-id <id>
```

## Environment Variables / Secrets

```bash
# Set a secret (prompted securely)
wrangler secret put SECRET_NAME

# List secrets
wrangler secret list
```

## Post-Deploy Verification

1. Check the Worker URL is responding: `curl https://<worker>.<zone>.workers.dev`
2. Review Cloudflare dashboard for error rates
3. Check `wrangler tail` for any runtime errors
