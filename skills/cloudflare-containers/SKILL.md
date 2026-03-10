---
name: cloudflare-containers
description: "Deploy full Docker containers to Cloudflare's edge network using Firecracker microVMs via Cloudflare Containers (Public Beta). Trigger: deploy container to cloudflare, cloudflare containers, wrangler containers, deploy docker to cloudflare edge, cloudflare container runtime"
version: 1
allowed-tools:
  - bash
  - read
  - write
---

# Cloudflare Containers (Public Beta)

> **Status: Public Beta since June 2025. This skill will be updated when Cloudflare Containers reaches GA.**
>
> Available to all Workers Paid plan users ($5/month). Not yet generally available — global autoscaling and latency-aware routing are pending. Monitor the [Cloudflare changelog](https://developers.cloudflare.com/changelog/) for GA announcement.

Deploy and manage full Linux containers on Cloudflare's edge network. Unlike Workers (V8 isolates), Containers run arbitrary Docker images in Firecracker microVMs — any language, stateful filesystem, long-lived processes.

## How Containers Differ from Workers

| Aspect | Workers | Containers |
|--------|---------|------------|
| Runtime | V8 isolates (JS/TS/WASM) | Full Linux containers (any language) |
| Startup | Sub-millisecond | 2-3 seconds |
| Duration | Short-lived | Long-lived, persistent |
| State | Stateless (use bindings) | Stateful filesystem, in-memory |
| Isolation | V8 sandbox | Firecracker microVM + KVM |
| Languages | JavaScript, TypeScript, WASM | Any (Go, Python, Rust, Java, etc.) |
| Cost model | Per-request + active CPU | Per-second (vCPU + memory + disk) |

Containers are declared in `wrangler.toml` or `wrangler.jsonc` alongside Workers. A Worker acts as the entry point, routing requests to container instances. Containers are modeled as Durable Object classes.

## Prerequisites

1. Cloudflare Workers Paid plan ($5/month)
2. `wrangler` CLI installed and authenticated:
   ```bash
   npm install -g wrangler
   wrangler login
   ```
3. Docker installed locally (for building images)
4. A `Dockerfile` in the project

## Configuration

**`wrangler.jsonc`** — define containers alongside the Worker:

```jsonc
{
  "name": "my-container-app",
  "main": "src/index.js",
  "compatibility_date": "2025-06-01",
  "containers": [
    {
      "class_name": "MyContainer",
      "image": "./Dockerfile",
      "instance_type": "standard-1",
      "max_instances": 10,
      "sleep_after": "5m"
    }
  ],
  "durable_objects": {
    "bindings": [
      {
        "name": "MY_CONTAINER",
        "class_name": "MyContainer"
      }
    ]
  }
}
```

**Scale-to-zero** with `sleep_after`: the container instance hibernates after the configured idle duration. Billing stops during sleep. Startup latency on wake is 2-3 seconds.

## Instance Types

| Type | vCPU | Memory | Use Case |
|------|------|--------|----------|
| lite | Shared | 256 MiB | Lightweight tasks, sidecar processes |
| basic | Shared | 1 GiB | Small apps, scripts |
| standard-1 | 1 | 4 GiB | General purpose (recommended default) |
| standard-2 | 2 | 8 GiB | Compute-heavy workloads |
| standard-4 | 8 | 32 GiB | Heavy workloads, ML inference |
| custom | Configurable | Configurable | Specialized requirements |

Start with `standard-1` for general workloads. Use `lite` or `basic` for tasks that do not need dedicated CPU.

## Worker Entry Point

The Worker routes requests to the container via Durable Objects:

```javascript
// src/index.js
export { MyContainer } from './container';

export default {
  async fetch(request, env) {
    const id = env.MY_CONTAINER.idFromName('instance-1');
    const stub = env.MY_CONTAINER.get(id);
    return stub.fetch(request);
  }
};
```

```javascript
// src/container.js
import { Container } from 'cloudflare:containers';

export class MyContainer extends Container {
  defaultPort = 8080;
  sleepAfter = '5m';

  override async onStart() {
    console.log('Container started');
  }
}
```

## Deploy

```bash
# Build and deploy (builds Docker image, uploads to CF, deploys Worker)
wrangler deploy

# Local development with container simulation
wrangler dev

# Stream logs from running containers
wrangler tail

# Check deployment status
wrangler deployments list
```

## Managing Instances

```bash
# List container instances
wrangler containers list

# View instance details
wrangler containers describe <instance-id>

# Force restart an instance
wrangler containers restart <instance-id>
```

## Rollback

```bash
# List versions
wrangler versions list

# Roll back to previous version
wrangler rollback
```

## Pricing (included with Workers Paid $5/month)

| Resource | Free Included | Overage |
|----------|--------------|---------|
| vCPU | 375 minutes/month | $0.00002/vCPU-second |
| Memory | 25 GB-hours/month | Usage-based |
| Disk | 200 GB-hours/month | Usage-based |
| Billing granularity | 10ms increments | Active CPU (not provisioned) |

Billing is for active CPU time only (since November 2025), not provisioned time. Scale-to-zero means idle containers do not incur ongoing cost.

## Dockerfile Recommendations

Containers run as full Linux environments. Follow standard Docker best practices:

```dockerfile
# Multi-stage build for smaller image
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app ./cmd/server

FROM alpine:3.20
RUN adduser -D appuser
USER appuser
COPY --from=builder /app /app
EXPOSE 8080
CMD ["/app"]
```

Keep images small — Cloudflare pulls them from the registry on deploy. Smaller images deploy faster.

## Beta Limitations (as of March 2026)

These limitations apply during Public Beta and may be resolved at GA:

- Global autoscaling and latency-aware routing are not yet available. Instances run in a limited set of regions.
- Inter-container communication patterns are limited. Containers communicate via HTTP through the Worker routing layer.
- Some regions may have limited or no capacity. Deployments may fail with capacity errors in underserved regions.
- Custom domains for container endpoints require routing through a Worker.
- No direct UDP support — TCP only.

## Key Updates Timeline

- **June 2025**: Public Beta launched. Available to all Paid plan users.
- **November 2025**: Switched to active CPU billing (not provisioned). Significant cost reduction.
- **January 2026**: Custom instance types available.
- **February 2026**: Limits increased to 6 TiB memory, 1,500 vCPU, 30 TB disk per account.

## When to Use Containers vs Workers

Use Containers when:
- You need a language runtime other than JavaScript/TypeScript/WASM
- Your process is long-lived (database, persistent cache, background job)
- You need a stateful filesystem
- You are lifting-and-shifting an existing Docker-based service to the edge

Use Workers when:
- You are writing JavaScript/TypeScript
- You need sub-millisecond cold start
- Your logic is stateless and request-scoped
- You want the simplest possible deployment model

This skill will be updated when Cloudflare Containers reaches GA.
