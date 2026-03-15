# Lightpanda Headless Browser: Technical Evaluation
## Issue #225 — Research Report

**Date:** 2026-03-14
**Status:** Research Complete
**Scope:** Evaluate Lightpanda as headless browser backend for a new browser tool in go-agent-harness

---

## Executive Summary

Five key findings:

1. **No browser tool exists today.** The harness has no CDP, screenshot, or JS-rendering capability. The current `WebFetcher` interface only does plain HTTP GET + optional search. A "browser tool" would be net-new surface area.
2. **Lightpanda is technically viable as a CDP backend.** It exposes a standard CDP WebSocket server on port 9222, compatible with `chromedp` and `go-rod`. The Go integration path is clear and requires no new Zig/non-Go code in the harness.
3. **Lightpanda is beta-quality and not suitable as the sole backend.** Roughly 5% of sites crash, React/Vue/Angular SPAs routinely fail, screenshots and PDFs are explicitly unsupported, and Windows support is absent.
4. **AGPL-3.0 license is a hard constraint.** If the harness is ever offered as a SaaS or networked service, Lightpanda's AGPL-3.0 license triggers source-disclosure obligations for the entire application. This must be cleared with legal before production adoption.
5. **Recommended path: implement a `BrowserFetcher` interface backed by `chromedp`/Chromium first; add Lightpanda as an optional sidecar for high-throughput scraping scenarios.** Lightpanda's performance advantage (9x memory, 11x speed) is real but only materialises for workloads involving hundreds of concurrent pages — not typical for an agent harness running single-user interactive sessions.

---

## 1. Current State Analysis

### What the harness provides today

The harness exposes three web-facing tools through the `WebFetcher` interface:

| Tool | Implementation | JS execution? | Visual rendering? |
|------|---------------|---------------|------------------|
| `fetch` | Plain `net/http` GET, 128KB limit | No | No |
| `web_fetch` | `WebFetcher.Fetch()` (injected) | Depends on implementation | Depends |
| `web_search` | `WebFetcher.Search()` (injected) | No | No |
| `agentic_fetch` | `WebFetcher.Fetch()` + sub-agent analysis | Depends | No |

The `WebFetcher` interface is defined in `internal/harness/tools/types.go`:

```go
type WebFetcher interface {
    Search(ctx context.Context, query string, maxResults int) ([]map[string]any, error)
    Fetch(ctx context.Context, url string) (string, error)
}
```

**Critical gap:** `Fetch` returns raw HTML text. For JS-heavy SPAs, this returns the pre-hydration skeleton, not the rendered page. The harness has no mechanism to wait for JavaScript to execute, interact with DOM elements, handle cookies/sessions, fill forms, or capture screenshots.

### What a "browser tool" would need to add

A meaningful browser tool for agent use would need at minimum:
- JavaScript execution with DOM access (rendered page content)
- Navigation (back/forward, redirect following)
- Optionally: form interaction, click, type
- Optionally: screenshot capture
- Session persistence across tool calls (cookies, localStorage)

---

## 2. Lightpanda Technical Profile

### What it is

Lightpanda is a headless browser written from scratch in Zig (v0.15.2). It is not Chromium-based. It implements its own HTML parser, DOM engine, and JavaScript execution layer using V8. It is purpose-built for automation and data extraction — it never loads CSS, images, fonts, or calculates layout.

- **GitHub:** https://github.com/lightpanda-io/browser
- **License:** AGPL-3.0 (strict copyleft for network-exposed services)
- **Status:** Beta (as of 2026-03)
- **Written in:** Zig (no Go library; controlled via CDP)
- **Version:** v0.2.5 (nightly releases)

### Protocol exposed

Lightpanda exposes a **Chrome DevTools Protocol (CDP) server** over WebSocket on port 9222 by default:

```bash
# Start Lightpanda CDP server
lightpanda serve --host 127.0.0.1 --port 9222

# Or via Docker
docker run -d --name lightpanda -p 9222:9222 lightpanda/browser:nightly
```

The CDP endpoint format: `ws://127.0.0.1:9222`

This is the same protocol Chrome uses, so any CDP-capable Go library can connect to it without modification.

### Performance benchmarks (vendor claims)

Tested with Puppeteer requesting 100 pages from a local website on AWS EC2 m5.large:

| Metric | Chrome | Lightpanda | Ratio |
|--------|--------|------------|-------|
| Execution time | 25.2s | 2.3s | 11x faster |
| Peak memory | 207MB | 24MB | 9x less |
| Concurrent instances (8GB host) | ~15 | ~140 | ~9x more |

**Caveat:** These are vendor-supplied benchmarks on synthetic workloads. Real-world gains on modern SPAs will be substantially lower due to Lightpanda's incomplete Web API coverage causing fallbacks or failures.

### JavaScript and Web API support

Lightpanda uses V8 for JavaScript execution but builds its own Web API layer:

- **What works:** HTTP/HTTPS loading, HTML5 parsing, basic DOM manipulation, XHR, Fetch API, cookies, `document.querySelector`, `innerHTML`, `textContent`
- **What does not work:** Screenshots, PDFs, WebRTC, CSS layout coordinates, `history.pushState()` (intermittent), full `location` object, WebSockets (limited), Canvas, WebAssembly (unconfirmed), browser extensions
- **Framework compatibility:** Static/server-rendered HTML works well; React/Vue/Angular SPAs frequently fail; login flows often break

The project states honestly: "There are hundreds of Web APIs. Developing a browser is a huge task."

### Platform support

| Platform | Binary | Docker |
|----------|--------|--------|
| Linux x86_64 | Yes (nightly) | Yes |
| Linux aarch64 | Yes (nightly) | Yes |
| macOS aarch64 (Apple Silicon) | Yes (nightly) | Via Docker |
| macOS x86_64 (Intel) | Build from source or Docker | Via Docker |
| Windows | No (planned 2026, WSL2 workaround) | Via Docker |

**Deployment note:** Docker is the recommended path for consistent operation. Image: `lightpanda/browser:nightly` (amd64 + arm64).

### Maturity and stability

- **Beta status** — self-described "work in progress"
- Community-reported crash rate: ~5% of pages
- Playwright scripts can break when Lightpanda adds new Web APIs (due to Playwright's feature-detection path selection)
- Known segfault when Playwright calls `connect_over_cdp` and queries browser version
- 1,300+ open GitHub issues as of late 2025
- Active development with pre-seed funding raised mid-2025

### License risk

Lightpanda is AGPL-3.0. This license requires that:
> If you modify AGPL-licensed code and make it available to users over a network (including through a SaaS application), you must make your source code available to those users.

For the go-agent-harness:
- **Running Lightpanda as a sidecar process** (not embedding its source) via Docker does not create direct AGPL obligation on the harness itself — the harness only connects over CDP WebSocket.
- **However:** Google's AGPL policy prohibits use in any Google product. Many enterprise legal teams treat AGPL-licensed backends as off-limits for any commercially deployed service.
- **Mitigation:** Lightpanda offers commercial licensing. Contact required.

### Go client library situation

Lightpanda provides **no Go client library**. Interaction happens exclusively through CDP. Two Go CDP libraries can connect to Lightpanda:

**chromedp** (`github.com/chromedp/chromedp`):
```go
allocCtx, cancel := chromedp.NewRemoteAllocator(
    context.Background(),
    "ws://127.0.0.1:9222",
    chromedp.NoModifyURL,
)
defer cancel()
ctx, cancel := chromedp.NewContext(allocCtx)
defer cancel()
```
Known past issue: `Target.createTarget` missing `browserContextId` field — resolved April 2025 (issue #283).

**go-rod** (`github.com/go-rod/rod`):
```go
browser := rod.New().ControlURL("ws://127.0.0.1:9222").MustConnect()
```
go-rod supports custom WebSocket URLs for remote browser connections. No known compatibility issues with Lightpanda documented.

### MCP server status

Lightpanda has a **native MCP server** baked into the binary (`lightpanda mcp`) as of late 2025. An official Go-based MCP server (`gomcp`) was also built but **archived on 2026-03-13** with the note: "the browser embeds an MCP server natively." The native MCP server exposes `goto`, `evaluate`, `LP.getSemanticTree`, `LP.getInteractiveElements`, and `LP.getStructuredData`.

---

## 3. Integration Path Analysis

### Option A: Wrap chromedp connecting to Lightpanda sidecar

This is the lowest-friction path. The harness starts Lightpanda as a sidecar Docker container (or subprocess on Linux), then creates a new `BrowserFetcher` implementation backed by `chromedp.RemoteAllocator`.

**Required changes:**
1. Define a `BrowserFetcher` interface in `internal/harness/tools/types.go` (separate from `WebFetcher`, since it needs more capabilities)
2. Implement `LightpandaBrowserFetcher` in a new `internal/provider/browser/lightpanda/` package
3. Add `browser_fetch`, `browser_navigate`, `browser_click` tools (or a unified `browser` tool) in `internal/harness/tools/deferred/browser.go`
4. Add `BuildOptions.BrowserFetcher` field
5. Add sidecar lifecycle management (start/stop/healthcheck Lightpanda process or container)

**What the tool would return:**
- Rendered HTML after JavaScript execution
- Optionally: DOM text content (stripped of tags)
- No screenshots (Lightpanda limitation)
- No PDF generation (Lightpanda limitation)

**Fallback story:** When a page fails in Lightpanda (crash or unsupported API), the tool must either return an error or fall back to plain `web_fetch`. A `BrowserFetcher` interface allows swapping backends at construction time.

### Option B: Use the MCP approach

Lightpanda's native MCP server (`lightpanda mcp`) exposes browser actions as MCP tools. The harness already has MCP integration via `connect_mcp`. An agent could be configured to connect to Lightpanda's MCP server as an MCP endpoint.

**Advantage:** Zero new Go code in the harness tools layer.
**Disadvantage:** Requires the user to separately manage Lightpanda process, creates indirection (agent → harness MCP → Lightpanda MCP → browser), and the native MCP tools are still early/limited.

### Option C: Implement against chromedp/Chromium first, add Lightpanda later

Design a `BrowserFetcher` interface and implement it backed by a real Chromium/Chrome instance via `chromedp`. This provides full Web API coverage and screenshot support from day one. Add Lightpanda support as an alternative implementation for high-throughput scraping use cases where AGPL risk is acceptable.

**This is the recommended path** (see Section 6).

---

## 4. Comparative Analysis

| Dimension | `fetch` (current) | chromedp + Chromium | go-rod + Chromium | Lightpanda + chromedp | Playwright MCP | Browserless.io |
|-----------|-------------------|--------------------|--------------------|----------------------|----------------|---------------|
| JS execution | No | Yes (full) | Yes (full) | Partial (Web API gaps) | Yes (full) | Yes (full) |
| Screenshots | No | Yes | Yes | **No** | Yes | Yes |
| PDF generation | No | Yes | Yes | **No** | Yes | Yes |
| Go library? | Yes (net/http) | Yes (chromedp) | Yes (go-rod) | No (CDP only) | No (subprocess) | No (HTTP API) |
| Memory per instance | Minimal | 207MB+ | 207MB+ | 24MB | 200MB+ | Cloud (no local) |
| Startup time | Instant | 2-5s | 2-5s | Instant | 2-5s | Network latency |
| License | BSD/Go stdlib | BSD | MIT | **AGPL-3.0** | Apache 2.0 | Commercial |
| Production ready? | Yes | Yes | Yes | **Beta (no)** | Yes | Yes |
| SPA compatibility | Poor | Excellent | Excellent | Poor-moderate | Excellent | Excellent |
| Docker image size | N/A | 1.3GB+ | 1.3GB+ | ~50MB | ~1.5GB+ | Cloud |
| Windows support | Yes | Yes | Yes | **No (2026 planned)** | Yes | Yes |
| Cost | Free | Free | Free | Free/AGPL or Commercial | Free | $50+/mo |

### Go CDP library comparison

| Feature | chromedp | go-rod |
|---------|----------|--------|
| Stars | 11k+ | 6.8k |
| Remote CDP support | Yes (RemoteAllocator) | Yes (ControlURL) |
| API style | Context-based actions | Method chaining |
| Concurrency | Single event loop (deadlock risk) | Thread-safe |
| Lightpanda compat | Yes (post-April 2025 fix) | Yes |
| Recommendation | Good for simple automation | Better for high concurrency |

---

## 5. Risk Matrix

| Risk | Severity | Likelihood | Mitigation |
|------|----------|------------|------------|
| AGPL license blocks SaaS deployment | **Critical** | High if networked | Legal review; use commercial license or swap backend |
| Lightpanda crash on target page (~5%) | High | Medium | Error-return with fallback to plain `web_fetch` |
| SPA/framework pages fail (React/Vue/Angular) | High | High for modern sites | Feature-gate as "experimental"; document limitations |
| No screenshot capability | Medium | Certain | Treat as out-of-scope for Lightpanda backend |
| Windows host incompatibility | Medium | Medium (dev machines) | Docker-only deployment on Windows |
| API breaking changes (beta churn) | Medium | High | Pin to specific release tag, not `nightly` |
| Playwright script breakage on Lightpanda API additions | Low | Medium | Prefer chromedp/go-rod over Playwright for harness |
| Telemetry (enabled by default) | Low | Certain | Set `LIGHTPANDA_DISABLE_TELEMETRY=true` |
| Intel Mac no native binary | Low | Low | Docker covers this |

---

## 6. Recommended Next Steps

### Immediate: Design the `BrowserFetcher` interface (no implementation yet)

Define an interface that can be satisfied by both a Chromium backend and a Lightpanda backend:

```go
// BrowserFetcher provides JavaScript-rendered page fetching capabilities.
// Unlike WebFetcher, this executes JS and waits for page hydration.
type BrowserFetcher interface {
    // Navigate fetches a URL after JS execution. Returns rendered HTML.
    Navigate(ctx context.Context, url string) (string, error)
    // Evaluate runs JavaScript in the context of the loaded page.
    Evaluate(ctx context.Context, url, script string) (string, error)
}
```

### Short-term: Implement with chromedp + Chromium

Add `internal/provider/browser/chromium/` implementing `BrowserFetcher` via `chromedp.RemoteAllocator` connecting to a locally-managed Chrome/Chromium process. This gives:
- Full Web API coverage
- Screenshot support
- No AGPL complications
- Production-grade reliability

Wire it into `BuildOptions` as `BrowserFetcher` and register a `browser_navigate` deferred tool.

### Medium-term: Add Lightpanda as an alternative backend

Once the interface is stable, add `internal/provider/browser/lightpanda/` implementing the same `BrowserFetcher` interface, connecting to a Lightpanda sidecar. This backend is appropriate for:
- High-volume scraping workloads (agents processing hundreds of pages)
- Environments where Chrome container overhead is prohibitive
- Workloads known to work on simple/server-rendered sites

**Gate it behind a configuration flag** (`HARNESS_BROWSER_BACKEND=lightpanda`) and document the limitations clearly.

### Do not do

- Do not embed Lightpanda's source code in the harness (triggers AGPL).
- Do not use Lightpanda as the primary/only browser backend — its beta status and Web API gaps make it unsuitable as the sole implementation.
- Do not use Playwright MCP as the integration path — it adds a Node.js process management dependency and excessive indirection.
- Do not connect to Lightpanda Cloud (SaaS endpoint) without reviewing commercial terms and AGPL compliance for the harness's deployment model.

---

## 7. Sources

- Lightpanda GitHub repository: https://github.com/lightpanda-io/browser
- Lightpanda official website: https://lightpanda.io/
- Lightpanda documentation (CDP): https://lightpanda.io/docs/cloud-offer/tools/cdp
- Lightpanda "What is a True Headless Browser": https://lightpanda.io/blog/posts/what-is-a-true-headless-browser
- Lightpanda CDP vs Playwright vs Puppeteer: https://lightpanda.io/blog/posts/cdp-vs-playwright-vs-puppeteer-is-this-the-wrong-question
- Lightpanda gomcp (archived): https://github.com/lightpanda-io/gomcp
- Lightpanda + chromedp Issue #283 (resolved): https://github.com/lightpanda-io/browser/issues/283
- "How to use Lightpanda in 2026": https://roundproxies.com/blog/lightpanda/
- "How to Fix Common Lightpanda Issues": https://roundproxies.com/blog/lightpanda-errors/
- chromedp Go package: https://pkg.go.dev/github.com/chromedp/chromedp
- chromedp GitHub: https://github.com/chromedp/chromedp
- go-rod GitHub: https://github.com/go-rod/rod
- go-rod Go package: https://pkg.go.dev/github.com/go-rod/rod
- Lightpanda byteiota overview: https://byteiota.com/lightpanda-11x-faster-headless-browser-for-ai-automation/
- Agentic browser landscape 2026: https://nohackspod.com/blog/agentic-browser-landscape-2026
- Browserless.io pricing: https://www.browserless.io/pricing
- AGPL license analysis (Open Core Ventures): https://www.opencoreventures.com/blog/agpl-license-is-a-non-starter-for-most-companies
- Lightpanda installation docs: https://lightpanda.io/docs/open-source/installation
- Lightpanda GitHub issues: https://github.com/lightpanda-io/browser/issues
