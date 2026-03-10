---
name: netlify-deploy
description: "Deploy sites and functions to Netlify: netlify deploy --prod, environment variables, redirects, edge functions, form handling. Trigger: when deploying to Netlify, netlify deploy, Netlify functions, netlify.toml, Netlify edge functions, Netlify redirects, Netlify env variables"
version: 1
argument-hint: "[deploy|env|functions|status|open]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Netlify Deployment

You are now operating in Netlify deployment mode.

## Installation

```bash
# Install Netlify CLI
npm install -g netlify-cli

# Verify installation
netlify --version

# Log in (opens browser for OAuth)
netlify login

# Check authentication status
netlify status
```

## Core Deployment

```bash
# Deploy a draft/preview (does NOT update production URL)
netlify deploy --dir=dist

# Deploy to production (updates the main site URL)
netlify deploy --prod --dir=dist

# Deploy and open the result in browser
netlify deploy --prod --dir=dist --open

# Deploy with a specific message/alias
netlify deploy --alias=feature-branch --dir=dist

# Deploy without a build step (just upload)
netlify deploy --prod --dir=public

# Watch deployment logs
netlify deploy --prod --dir=dist 2>&1 | tee deploy.log
```

## netlify.toml Configuration

```toml
# netlify.toml — place in project root

[build]
  command = "npm run build"
  publish = "dist"
  functions = "netlify/functions"

[build.environment]
  NODE_VERSION = "20"
  NPM_FLAGS = "--prefer-offline"

# Production-specific settings
[context.production]
  command = "npm run build:prod"

[context.deploy-preview]
  command = "npm run build:preview"

[context.branch-deploy]
  command = "npm run build:staging"

# HTTP headers for all routes
[[headers]]
  for = "/*"
  [headers.values]
    X-Frame-Options = "DENY"
    X-Content-Type-Options = "nosniff"
    Referrer-Policy = "strict-origin-when-cross-origin"

# Cache static assets aggressively
[[headers]]
  for = "/assets/*"
  [headers.values]
    Cache-Control = "public, max-age=31536000, immutable"
```

## Redirects and Rewrites

```toml
# In netlify.toml — [[redirects]] rules

# SPA fallback (React Router, Vue Router, etc.)
[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200

# API proxy (avoid CORS by proxying to backend)
[[redirects]]
  from = "/api/*"
  to = "https://api.example.com/:splat"
  status = 200
  force = true

# Permanent redirect (301)
[[redirects]]
  from = "/old-page"
  to = "/new-page"
  status = 301

# Country-based redirect
[[redirects]]
  from = "/"
  to = "/uk/"
  status = 302
  conditions = {Country = ["GB"]}

# Force HTTPS
[[redirects]]
  from = "http://example.com/*"
  to = "https://example.com/:splat"
  status = 301
  force = true
```

Alternatively, use `_redirects` file in the publish directory:

```
# _redirects
/api/*    https://api.example.com/:splat    200
/old      /new                               301
/*        /index.html                        200
```

## Environment Variables

```bash
# List all environment variables for the site
netlify env:list

# Set an environment variable
netlify env:set API_KEY "mysecretkey"

# Set a variable scoped to a specific context
netlify env:set DATABASE_URL "postgres://..." --context production
netlify env:set DATABASE_URL "postgres://..." --context deploy-preview

# Get a specific variable's value
netlify env:get API_KEY

# Unset an environment variable
netlify env:unset API_KEY

# Import environment variables from a .env file
netlify env:import .env.production
```

## Netlify Functions (Serverless)

```bash
# Create a new function
netlify functions:create my-function

# List deployed functions
netlify functions:list

# Invoke a function locally
netlify functions:invoke my-function --payload '{"name":"world"}'

# Start local dev server with functions
netlify dev
```

### Function Example (Node.js)

```javascript
// netlify/functions/hello.js
exports.handler = async (event, context) => {
  const name = event.queryStringParameters?.name || "World";
  return {
    statusCode: 200,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ message: `Hello, ${name}!` }),
  };
};
```

### Function Example (Go)

```go
// netlify/functions/hello/main.go
package main

import (
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
)

func handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    return events.APIGatewayProxyResponse{
        StatusCode: 200,
        Body:       `{"message":"Hello from Go!"}`,
    }, nil
}

func main() { lambda.Start(handler) }
```

## Edge Functions

```javascript
// netlify/edge-functions/greet.js
export default async (request, context) => {
  const url = new URL(request.url);
  const name = url.searchParams.get("name") || "World";
  return new Response(`Hello, ${name}!`, {
    headers: { "Content-Type": "text/plain" },
  });
};

export const config = { path: "/greet" };
```

```toml
# In netlify.toml, declare edge functions:
[[edge_functions]]
  path = "/greet"
  function = "greet"
```

## Form Handling

```html
<!-- Enable Netlify Forms by adding netlify attribute -->
<form name="contact" method="POST" data-netlify="true">
  <input type="hidden" name="form-name" value="contact" />
  <input type="text" name="name" required />
  <input type="email" name="email" required />
  <textarea name="message" required></textarea>
  <button type="submit">Send</button>
</form>
```

```bash
# List form submissions via CLI
netlify api listSiteFormSubmissions --data '{"site_id":"<SITE_ID>","form_id":"<FORM_ID>"}'
```

## Local Development

```bash
# Start local dev server (proxies to Netlify Functions)
netlify dev

# Start with a specific port
netlify dev --port 8888

# Link local directory to existing Netlify site
netlify link --name my-site-name

# Open the Netlify admin panel for the linked site
netlify open

# Show site info
netlify status
```

## CI/CD Integration

```bash
# In CI: use NETLIFY_AUTH_TOKEN and NETLIFY_SITE_ID environment variables
export NETLIFY_AUTH_TOKEN="${NETLIFY_AUTH_TOKEN}"
export NETLIFY_SITE_ID="${NETLIFY_SITE_ID}"

# Build and deploy
npm run build
netlify deploy --prod --dir=dist

# Get deploy URL for smoke testing
DEPLOY_URL=$(netlify deploy --prod --dir=dist --json | jq -r '.url')
curl -f "${DEPLOY_URL}/health" || exit 1
```

## Post-Deploy Verification

```bash
# Check that the new content is live (not cached old version)
SITE_URL="https://my-site.netlify.app"
EXPECTED_VERSION="1.2.3"

# Check for version in deployed content
curl -s "${SITE_URL}/version.json" | jq -r '.version'

# Check deploy status
netlify api getSiteDeploy --data '{"site_id":"<SITE_ID>","deploy_id":"<DEPLOY_ID>"}'

# Health check after deploy
curl -f "${SITE_URL}/health" || echo "Health check failed"
```

## Best Practices

- Use `netlify deploy` (draft) before `netlify deploy --prod` to preview changes.
- Store `NETLIFY_AUTH_TOKEN` and `NETLIFY_SITE_ID` as CI secrets — never commit them.
- Use `netlify.toml` for all configuration (not the Netlify UI) to keep settings in version control.
- Set environment variables per context (production vs deploy-preview vs branch-deploy).
- Use the `_redirects` file or `[[redirects]]` in `netlify.toml` for the SPA fallback — never configure it in the UI only.
- Use Netlify Functions for backend logic instead of exposing API keys in client-side code.
- Enable branch deploys for staging environments to test before promoting to production.
