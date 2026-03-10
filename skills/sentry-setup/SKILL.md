---
name: sentry-setup
description: "Integrate and manage Sentry error tracking: SDK setup in Go and JavaScript, source maps, releases, performance tracing, error grouping. Trigger: when setting up Sentry, sentry-cli releases, sentry source maps, Sentry SDK integration, Sentry error tracking, Sentry performance monitoring"
version: 1
argument-hint: "[releases|sourcemaps|dsn|performance|go|javascript]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Sentry Error Tracking and Monitoring

You are now operating in Sentry integration mode.

## Installation

```bash
# Install sentry-cli (macOS)
brew install getsentry/tools/sentry-cli

# Install via npm
npm install -g @sentry/cli

# Install via curl
curl -sL https://sentry.io/get-cli/ | bash

# Verify installation
sentry-cli --version

# Authenticate (creates ~/.sentryclirc)
sentry-cli login
```

## Configuration

```bash
# Configure sentry-cli via environment variables (recommended for CI)
export SENTRY_AUTH_TOKEN="sntrys_your_auth_token_here"
export SENTRY_ORG="your-org-slug"
export SENTRY_PROJECT="your-project-slug"
export SENTRY_DSN="https://key@sentry.io/project-id"

# Or create .sentryclirc in project root:
# [defaults]
# org = your-org-slug
# project = your-project-slug
# url = https://sentry.io/
```

## Release Management

```bash
# Create a new release
sentry-cli releases new "1.2.3"

# Tag a release with commits from the repo
sentry-cli releases set-commits "1.2.3" --auto

# Associate commits manually (from a specific git range)
sentry-cli releases set-commits "1.2.3" \
  --commit "your-org/your-repo@abc123..def456"

# Finalize a release (marks it as deployed)
sentry-cli releases finalize "1.2.3"

# Mark a release as deployed in a specific environment
sentry-cli releases deploys "1.2.3" new \
  --env production \
  --name "deploy-$(date +%Y%m%dT%H%M%S)"

# List recent releases
sentry-cli releases list

# Delete a release
sentry-cli releases delete "1.2.3"
```

## Source Maps (JavaScript/TypeScript)

```bash
# Upload source maps to associate minified code with original source
sentry-cli sourcemaps upload \
  --release "1.2.3" \
  --url-prefix "~/static/js" \
  ./dist/

# Inject debug IDs into source files (Sentry CLI 2.x recommended approach)
sentry-cli sourcemaps inject ./dist/

# Upload after injection
sentry-cli sourcemaps upload ./dist/ --release "1.2.3"

# Validate source map upload
sentry-cli sourcemaps explain --release "1.2.3" --url "~/static/js/main.js"

# Legacy approach: inject and upload in one step
sentry-cli releases files "1.2.3" upload-sourcemaps ./dist \
  --url-prefix "~/static/js" \
  --rewrite
```

### Vite + Sentry Example

```javascript
// vite.config.ts
import { sentryVitePlugin } from "@sentry/vite-plugin";

export default {
  plugins: [
    sentryVitePlugin({
      org: process.env.SENTRY_ORG,
      project: process.env.SENTRY_PROJECT,
      authToken: process.env.SENTRY_AUTH_TOKEN,
      sourcemaps: {
        assets: "./dist/**",
        ignore: ["./node_modules/**"],
      },
      release: {
        name: process.env.RELEASE_VERSION,
        inject: true,
      },
    }),
  ],
  build: {
    sourcemap: true,
  },
};
```

## Go SDK Integration

```bash
# Add Sentry Go SDK
go get github.com/getsentry/sentry-go
```

```go
// main.go — initialize Sentry at startup
package main

import (
    "log"
    "time"

    "github.com/getsentry/sentry-go"
)

func main() {
    err := sentry.Init(sentry.ClientOptions{
        Dsn:              "https://key@sentry.io/project-id",
        Environment:      "production",
        Release:          "myapp@1.2.3",
        TracesSampleRate: 0.1, // 10% of transactions
        AttachStacktrace: true,
        Debug:            false,
    })
    if err != nil {
        log.Fatalf("sentry.Init: %v", err)
    }
    // Flush buffered events before the program terminates
    defer sentry.Flush(2 * time.Second)

    // Capture a message
    sentry.CaptureMessage("application started")
}
```

```go
// Capture errors with context
func processOrder(orderID string) error {
    err := db.FindOrder(orderID)
    if err != nil {
        sentry.WithScope(func(scope *sentry.Scope) {
            scope.SetTag("order_id", orderID)
            scope.SetLevel(sentry.LevelError)
            sentry.CaptureException(err)
        })
        return err
    }
    return nil
}

// HTTP middleware example
func sentryMiddleware(next http.Handler) http.Handler {
    return sentryhttp.New(sentryhttp.Options{
        Repanic: true,
    }).Handle(next)
}
```

## JavaScript/TypeScript SDK Integration

```bash
# Install Sentry browser SDK
npm install @sentry/browser @sentry/tracing

# Or for React
npm install @sentry/react
```

```javascript
// src/main.ts — initialize at app entry point
import * as Sentry from "@sentry/browser";

Sentry.init({
  dsn: "https://key@sentry.io/project-id",
  environment: import.meta.env.MODE,
  release: import.meta.env.VITE_RELEASE_VERSION,
  integrations: [
    Sentry.browserTracingIntegration(),
    Sentry.replayIntegration({
      maskAllText: true,
      blockAllMedia: true,
    }),
  ],
  tracesSampleRate: 0.1,      // 10% of transactions
  replaysSessionSampleRate: 0.01, // 1% of sessions
  replaysOnErrorSampleRate: 1.0,  // 100% of sessions with errors
});
```

```javascript
// Capture errors manually
try {
  riskyOperation();
} catch (err) {
  Sentry.withScope((scope) => {
    scope.setTag("feature", "checkout");
    scope.setUser({ id: userId, email: userEmail });
    Sentry.captureException(err);
  });
}

// Add breadcrumbs for context
Sentry.addBreadcrumb({
  category: "user-action",
  message: "Clicked submit button",
  level: "info",
});
```

## Performance Tracing

```go
// Go: start a custom transaction
ctx := sentry.StartTransaction(context.Background(), "process-order")
defer ctx.Finish()

// Create a child span
span := sentry.StartSpan(ctx, "db.query", sentry.WithDescription("SELECT orders"))
defer span.Finish()
```

```javascript
// JavaScript: custom performance transaction
const transaction = Sentry.startTransaction({ name: "checkout" });
const span = transaction.startChild({ op: "db", description: "fetchCart" });
try {
  const cart = await db.fetchCart(userId);
  span.setStatus("ok");
} catch (err) {
  span.setStatus("internal_error");
  Sentry.captureException(err);
} finally {
  span.finish();
  transaction.finish();
}
```

## Error Grouping and Fingerprinting

```go
// Override Sentry's automatic grouping by setting a custom fingerprint
sentry.WithScope(func(scope *sentry.Scope) {
    scope.SetFingerprint([]string{"database-connection-error", "postgres"})
    sentry.CaptureException(err)
})
```

```javascript
// JavaScript: custom fingerprint
Sentry.withScope((scope) => {
  scope.setFingerprint(["payment-gateway-timeout", gateway]);
  Sentry.captureException(err);
});
```

## CI/CD Integration

```bash
# Full release workflow in CI (e.g., GitHub Actions)
RELEASE="${GITHUB_REF_NAME}-${GITHUB_SHA:0:8}"

# 1. Create release
sentry-cli releases new "$RELEASE"

# 2. Associate commits
sentry-cli releases set-commits "$RELEASE" --auto

# 3. Upload source maps (if applicable)
sentry-cli sourcemaps inject ./dist/
sentry-cli sourcemaps upload ./dist/ --release "$RELEASE"

# 4. Finalize and mark as deployed
sentry-cli releases finalize "$RELEASE"
sentry-cli releases deploys "$RELEASE" new --env production
```

## Troubleshooting

```bash
# Test that sentry-cli can reach your Sentry instance
sentry-cli info

# List files uploaded for a release (source maps)
sentry-cli releases files "1.2.3" list

# Check that DSN is correct
curl -X POST "https://sentry.io/api/<project-id>/store/" \
  -H "X-Sentry-Auth: Sentry sentry_version=7, sentry_key=<key>" \
  -H "Content-Type: application/json" \
  -d '{"message":"test","level":"error","platform":"javascript"}'

# Enable verbose logging for sentry-cli
sentry-cli --log-level debug releases new "test"
```

## Best Practices

- Always call `sentry.Flush(2 * time.Second)` at program exit in Go to ensure buffered events are sent.
- Set `environment` in SDK init (`production`, `staging`, `development`) — this enables filtering in the Sentry UI.
- Set `release` in SDK init to correlate errors with specific code versions.
- Use `TracesSampleRate` less than 1.0 in production to control costs (start at 0.1).
- Never log or print the DSN in application output — treat it as a semi-secret.
- Use `scope.SetUser()` for PII-compliant user context (id only, not email, in GDPR regions).
- Upload source maps in CI as part of the release pipeline — never skip this step.
- Use `sentry-cli releases set-commits --auto` only when CI has git history (use `fetch-depth: 0` in GitHub Actions).
