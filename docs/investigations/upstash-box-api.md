# Upstash Box API Investigation

**Date**: 2026-03-12
**Sources**: https://upstash.com/docs/box, https://upstash.com/docs/llms-full.txt, https://upstash.com/blog/upstash-box

Upstash Box is a developer-preview service that provides isolated cloud containers (backed by Alpine Linux Docker containers) with optional built-in AI agents. Each box has a filesystem, shell, git, and configurable runtime. Boxes do not expire but auto-freeze after 1 hour of idle time.

---

## Question 1: Is there a REST API for creating/destroying boxes? What does it look like?

**Short answer**: There is no publicly documented raw REST API. All documented control-plane operations are exposed only through the TypeScript SDK (`@upstash/box`). The underlying HTTP transport exists (the SDK wraps it), but no curl examples, endpoint URLs, or OpenAPI spec are published in the docs as of March 2026.

The TypeScript SDK control-plane interface looks like this:

```typescript
import { Box, ClaudeCode } from "@upstash/box"

// Create
const box = await Box.create({
  runtime: "node",           // "node" | "python" | "go" | others
  agent: {
    model: ClaudeCode.Opus_4_6,
    apiKey: process.env.ANTHROPIC_API_KEY,
  },
})

// Retrieve an existing box by ID
const sameBox = await Box.get(box.id)

// List all boxes in the account
const boxes = await Box.list()

// Destroy
await box.delete()
```

The `UPSTASH_BOX_API_KEY` environment variable is used for authentication. No REST endpoint paths (e.g., `POST /v1/boxes`) are surfaced in the public documentation — the docs index (`https://upstash.com/docs/llms.txt`) lists no Box-specific REST API pages, only SDK pages.

**Implication for Go usage**: To drive Box lifecycle from Go, you would need to reverse-engineer the HTTP calls the TS SDK makes (inspect network traffic or read the SDK source on npm), or use `exec` to shell out to a Node script, or wait for an official REST API to be documented.

---

## Question 2: Is there a Go SDK, or only TypeScript?

**Short answer**: Only a TypeScript/JavaScript SDK exists for Upstash Box. No Go SDK is documented or available.

The npm package is `@upstash/box`. Upstash does have Go SDKs for other products (QStash: `github.com/upstash/qstash-go`, Redis: `github.com/upstash/go-redis`, Vector: via REST), but no `upstash/box-go` repository exists.

The docs index lists only one SDK section:
- `https://upstash.com/docs/box/sdks/ts/getting-started`

There is no equivalent Go path. To consume Box from Go you would need to call the undocumented REST API directly (HTTP client with `UPSTASH_BOX_API_KEY` bearer token), which would require SDK source inspection to discover endpoint shapes.

---

## Question 3: What runtimes are supported? Can you run arbitrary Go binaries or custom Docker images, or is it Node-only?

**Short answer**: Node, Python, and Go are all listed as supported runtimes. Custom runtimes (described as "Dockerfile-like") are also advertised. Arbitrary Go binaries can be run via shell commands once a box is running. There is no mention of user-supplied Docker images.

From the official documentation (`how-it-works.md`):

> "Node, Python, Go or other runtimes" are available.

Key points:
- The `runtime` parameter to `Box.create()` accepts at minimum `"node"`, `"python"`, and `"go"`.
- The docs mention that "custom runtimes" can be defined "using something similar to Dockerfiles" — the exact mechanism is not fully elaborated in the current preview docs.
- Boxes run on Alpine Linux with 2 vCPU / 2 GB RAM / 10 GB disk. Extra Alpine packages can be installed via `sudo apk add`.
- **Go binaries**: Since you get full shell access and the Go runtime is a first-class option, you can compile and run Go binaries inside a box using `box.exec.command()`. There is no claim of user-supplied custom Docker images; the underlying container image is Upstash-controlled.

---

## Question 4: What HTTP endpoints does a running Box expose? Does it have a fixed URL/hostname?

**Short answer**: No inbound HTTP endpoints are exposed by default, and no fixed public URL/hostname is documented for a running box. Boxes have full outbound networking but there is no documented mechanism to expose a port or get an inbound URL.

Key networking facts from the docs:
- Boxes have outbound access to HTTP, HTTPS, DNS, WebSockets, and raw TCP.
- Infrastructure is on AWS with 22.5 Gbps bandwidth per host.
- No documentation mentions: a per-box hostname, an ingress URL, port forwarding, or any way to make a box reachable via HTTP from the outside.

Each box has a `box.id` field (used with `Box.get(box.id)` to reconnect), but this is a control-plane identifier, not an HTTP address.

**Implication**: Boxes appear to be compute-only outbound environments. To use one as an HTTP server you would need to start a process inside it and then either: tunnel through a third-party tool (e.g., ngrok via shell), or rely on the box making outbound calls back to your infrastructure (webhook/polling pattern). There is no native "expose port" primitive documented.

---

## Question 5: How do you execute code inside a Box? Is it arbitrary shell commands or structured agent APIs?

**Short answer**: Both. You can run fully arbitrary shell commands via `box.exec.command()` (pipes, redirects, chains all work), or run an AI agent against a natural-language prompt via `box.agent.run()` / `box.agent.stream()`.

### Shell execution (arbitrary commands)

```typescript
// Run any shell expression — pipes, redirects, chained commands all work
const result = await box.exec.command("cd /work && go build ./... && ./myapp")
// result.status: "completed" | "failed"
// result.result: stdout
// result.logs(): timestamped output with log levels

// Cancel a long-running command
const run = box.exec.command("sleep 300")
await run.cancel()
```

```typescript
// Run a code snippet with a specified language
const result = await box.exec.code({
  code: `package main\nimport "fmt"\nfunc main() { fmt.Println("hi") }`,
  lang: "go",
  timeout: 30000,
})
// result.output: string
// result.exit_code: number
```

There is also `box.exec.code()` which accepts a language tag and source string directly (evaluates inline code, not a shell command).

### Agent execution (AI-driven)

```typescript
// Fire-and-wait
const result = await box.agent.run({
  prompt: "Write a Go HTTP server in /app/main.go and run it",
  timeout: 120000,
  onToolUse: (tool) => console.log("agent used:", tool.name),
})

// Streaming
for await (const event of box.agent.stream({ prompt: "..." })) {
  process.stdout.write(event.text ?? "")
}
```

Supported agent models:
- `ClaudeCode.Opus_4_6`, `ClaudeCode.Sonnet_4_5`, etc.
- `Codex.GPT_5_3_Codex`
- OpenCode (third option)

Agents have access to the same filesystem, shell, and git primitives. The agent loop runs inside the box, not on the caller's machine.

---

## Summary Table

| Question | Answer |
|---|---|
| REST API for create/destroy | No public REST API documented; TypeScript SDK only (`Box.create()`, `box.delete()`) |
| Go SDK | No Go SDK exists; TypeScript/Node only |
| Supported runtimes | Node, Python, Go, and custom (Dockerfile-like); arbitrary binaries via shell; no user Docker images |
| Box HTTP endpoints / hostname | No inbound HTTP exposure; no per-box public URL; outbound-only networking |
| Code execution model | Arbitrary shell commands (`box.exec.command()`), code snippets (`box.exec.code()`), or AI agent prompts (`box.agent.run()`) |

---

## Notes on Developer Preview Status

All APIs and pricing are subject to change. Free tier: 5 CPU hours, 10 concurrent boxes. Paid: $0.10/active vCPU-hour, $0.10/GB-month storage. No published timeline for a stable REST API or Go SDK.
