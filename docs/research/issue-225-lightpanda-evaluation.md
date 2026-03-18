# Issue #225: Lightpanda Headless Browser Evaluation

**Status:** Design Research / Technology Evaluation
**Filed:** 2026-03-18
**Related:** `internal/harness/tools/deferred/`, `internal/workspace/container.go`

---

## Summary

This document evaluates Lightpanda — a purpose-built headless browser written in Zig — as a potential backend for a new browser interaction tool suite in go-agent-harness. The evaluation covers current browser tooling state, CDP integration design, proposed tool API, maturity assessment, and a recommendation.

---

## 1. Current State: No Browser Tooling Exists

A search across `internal/harness/tools/` and `internal/harness/tools/deferred/` finds zero browser-interaction tools. The tool catalog does not include any tool named `browser_*`, and there are no references to `CDP`, `headless`, `chromium`, `playwright`, or `puppeteer` anywhere in the Go source tree.

The closest existing tools are HTTP-level:

| Tool | Location | Capability |
|------|----------|-----------|
| `fetch` | `deferred/fetch.go` | Raw HTTP GET with size/timeout bounds |
| `web_fetch` | `deferred/web.go` | Thin wrapper — fetches URL content as text |
| `web_search` | `deferred/web.go` | Calls a `WebFetcher.Search()` interface |
| `agentic_fetch` | `deferred/` | Fetches with agent-level analysis |
| `download` | `deferred/download.go` | File download |

These tools operate at the HTTP layer only. They cannot:
- Execute JavaScript
- Navigate single-page applications (SPAs)
- Interact with DOM elements (click, type, scroll)
- Capture screenshots
- Wait for dynamically loaded content
- Handle authentication flows with JS challenges

**There is currently no headless browser capability in the harness.**

---

## 2. Lightpanda Overview

Lightpanda is an open-source headless browser engine written in Zig, designed explicitly for agentic and automated workloads. Its stated benchmarks versus Chromium headless:

- **9x less memory** (measured across a batch of real-world pages)
- **11x faster cold start** (process launch to first-page-ready)
- **CDP compatible** — speaks Chrome DevTools Protocol over WebSocket or HTTP

### 2.1 Architecture

Lightpanda runs as a standalone process. Callers interact with it via CDP (the same protocol used by Puppeteer, Playwright, and Chrome DevTools). This means:

- Any language that can send CDP JSON-RPC over WebSocket can drive it
- No special Lightpanda SDK required — existing CDP client libraries work
- Go has multiple CDP client libraries (`github.com/chromedp/cdproto`, `github.com/mafredri/cdp`, and the higher-level `github.com/chromedp/chromedp`)

### 2.2 Known Limitations vs Full Chrome

Lightpanda is purpose-built and intentionally omits features that a general-purpose browser needs but that headless automation rarely uses:

| Feature | Lightpanda | Chrome Headless |
|---------|-----------|----------------|
| JavaScript execution | Yes (V8-based) | Yes |
| DOM manipulation | Yes | Yes |
| CSS layout | Partial — layout primitives only | Full |
| WebGL / Canvas pixel-accurate | No | Yes |
| PDF generation | No | Yes |
| Media playback (audio/video) | No | Yes |
| Web Extensions | No | Yes |
| Full CSS rendering for screenshots | Partial | Full |
| Shadow DOM | Partial | Full |
| WebRTC | No | Yes |
| IndexedDB / complex storage | Limited | Yes |
| WASM | Limited | Full |
| Service Workers | No | Yes |

For agentic use cases — page text extraction, form interaction, navigation, basic screenshot capture, and SPA rendering — Lightpanda's feature set is sufficient. For testing visual pixel-perfect rendering or applications that depend on WebGL/Canvas, Chrome headless remains the only option.

### 2.3 CDP Compatibility Status (as of early 2026)

Lightpanda implements a subset of the CDP domain surface, covering the domains most used by automation:

- `Target` — session management
- `Page` — navigation, reload, screenshot
- `Runtime` — JS evaluation
- `DOM` — querying, node inspection
- `Input` — mouse/keyboard events
- `Network` — request interception (partial)
- `Emulation` — viewport setting
- `Console` — log capture

Domains **not yet supported or incomplete:**
- `Tracing`
- `CSS` (detailed computed style inspection)
- `Accessibility` (AX tree)
- `Security` (certificate handling)
- `Fetch` (full request interception with modification)

### 2.4 Memory and Startup Characteristics

Lightpanda's memory advantage comes from not loading the full Chromium rendering stack. A typical Chromium instance starts at ~100MB RSS even for a blank page; Lightpanda starts at ~10-15MB. For a pool of concurrent browser sessions in a container workspace, this is a material difference: 10 concurrent Lightpanda sessions ≈ 150MB vs 10 Chromium sessions ≈ 1GB+.

Cold start time matters for agentic loops where each tool call may need a fresh browser context. Lightpanda's ~11x startup improvement (from ~500ms to ~45ms) means `browser_navigate` tool latency is dominated by network round-trip rather than process start time.

---

## 3. Container Workspace Context

The harness already runs agents in Docker containers via `ContainerWorkspace`:

```go
// internal/workspace/container.go
// Each workspace provisions a Docker container running harnessd,
// exposing it on a dynamically allocated host port.
// The workspace directory is bind-mounted into the container at /workspace.
```

Memory is a genuine concern here. A container running `harnessd` + an LLM agent loop already uses ~150-300MB baseline. Adding Chromium headless for browser capability pushes that to ~500-800MB per container. At a pool size of 10 (the default `pool_size` in symphd config), this becomes 5-8GB of memory for browser-capable workspaces.

Lightpanda reduces the browser memory overhead from ~300-500MB to ~30-50MB per container, keeping the total footprint manageable.

---

## 4. CDP Integration Design — Go Client Approach

### 4.1 Client library choice

The recommended approach is **`github.com/chromedp/chromedp`**, which provides:

- High-level action API (`chromedp.Navigate`, `chromedp.Click`, `chromedp.Text`, `chromedp.Screenshot`)
- CDP WebSocket session management
- Context-based timeout and cancellation (integrates naturally with Go's `context.Context`)
- Active maintenance and broad CDP coverage

For lower-level CDP access (when chromedp's abstraction is insufficient), `github.com/chromedp/cdproto` provides typed CDP structs directly.

### 4.2 Connection lifecycle

Lightpanda exposes a CDP WebSocket endpoint. The standard connection URL format is:

```
ws://localhost:9222/json/version      # retrieve browser metadata
ws://localhost:9222/devtools/browser  # attach to browser session
```

A per-run browser context can be managed as follows:

```go
// Allocate a Lightpanda context pointing at a running Lightpanda process.
// In container environments, Lightpanda runs as a sidecar on a fixed port.
allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx,
    []func(*chromedp.ExecAllocator){
        chromedp.WSURLReadyFunc(func(ctx context.Context) (string, error) {
            return "ws://localhost:9222", nil
        }),
    })
defer cancelAlloc()

// Create a browser context (new tab/target).
tabCtx, cancelTab := chromedp.NewContext(allocCtx)
defer cancelTab()
```

### 4.3 Sidecar vs on-demand launch

Two deployment patterns apply:

**Pattern A — Sidecar (recommended for container workspaces):** Lightpanda starts with the container as a long-running process and accepts CDP connections throughout the workspace lifetime. The harness tools connect to `localhost:9222`. This avoids per-tool-call process launch overhead.

**Pattern B — On-demand:** The browser tool spawns Lightpanda as a subprocess, uses it for one or more tool calls, and then terminates it. This is simpler to implement (no sidecar configuration needed) but pays the startup cost on first use within a run.

For the container workspace architecture, Pattern A is preferred because it amortises startup cost across an entire agent run. For local workspaces, Pattern B may be preferable to avoid leaving Lightpanda processes running.

### 4.4 Session isolation

Each harness run should have its own browser context (tab/target) so that cookies, localStorage, and navigation history do not leak between runs. `chromedp.NewContext()` creates an isolated tab; the allocator (Lightpanda process) is shared at the container level.

---

## 5. Browser Tool API Proposal

The following four tools are proposed for Phase 1. All are `TierDeferred` (hidden until the agent calls `find_tool`) and `Mutating: true` (they change external state).

### 5.1 `browser_navigate`

Navigate to a URL and wait for page load. Returns page title, URL, and status.

```go
Parameters:
  url (required, string): The URL to navigate to.
  wait_for (optional, string): "load" | "domcontentloaded" | "networkidle" (default: "load")
  timeout_seconds (optional, int): 5–120, default 30.

Returns:
  {url, title, status: "ok" | "error", error?: string}
```

### 5.2 `browser_click`

Click on an element identified by CSS selector or XPath.

```go
Parameters:
  selector (required, string): CSS selector or XPath expression.
  selector_type (optional, string): "css" | "xpath" (default: "css")
  timeout_seconds (optional, int): 5–60, default 10.

Returns:
  {selector, clicked: true | false, error?: string}
```

### 5.3 `browser_extract`

Extract text, HTML, or structured data from the current page.

```go
Parameters:
  selector (optional, string): CSS selector to scope extraction. Empty means whole page.
  format (optional, string): "text" | "html" | "markdown" (default: "text")
  max_bytes (optional, int): 1–512000, default 65536.

Returns:
  {url, format, content: string, truncated: bool}
```

`browser_extract` with `format: "markdown"` is the primary tool for feeding page content into the LLM context — it strips scripts/styles and converts HTML to readable Markdown using `golang.org/x/net/html` parsing.

### 5.4 `browser_screenshot`

Capture a screenshot of the current page or a specific element.

```go
Parameters:
  selector (optional, string): If set, captures just this element. Otherwise full page.
  format (optional, string): "png" | "jpeg" (default: "png")
  quality (optional, int): 1–100, default 80. Only applies to "jpeg".
  max_bytes (optional, int): Up to 2MB, default 512KB.

Returns:
  {url, format, data: "<base64-encoded image>", width: int, height: int, truncated: bool}
```

Screenshot data is base64-encoded and embedded in the tool result. For vision-capable models, the image can be passed directly. For text-only models, `browser_extract` is preferred.

### 5.5 Type inputs (Phase 2)

```go
// browser_type: type text into an input element
// browser_scroll: scroll to a position or element
// browser_wait: wait for a selector to appear
// browser_eval: evaluate arbitrary JavaScript and return the result
```

### 5.6 Tool Registration

```go
// internal/harness/tools/deferred/browser.go

func BrowserNavigateTool(browserPool BrowserPool) tools.Tool {
    return tools.Tool{
        Definition: tools.Definition{
            Name:         "browser_navigate",
            Description:  descriptions.Load("browser_navigate"),
            Action:       tools.ActionFetch,
            Mutating:     true,
            ParallelSafe: false,   // Browser state is sequential
            Tier:         tools.TierDeferred,
            Tags:         []string{"browser", "web", "navigate", "url"},
            Parameters:   ...,
        },
        Handler: ...,
    }
}
```

`ParallelSafe: false` is important — concurrent navigation calls on the same browser context produce undefined behaviour.

### 5.7 BrowserPool interface

Rather than embedding chromedp directly in each tool, a `BrowserPool` interface abstracts the lifecycle:

```go
// internal/harness/tools/browser_pool.go

type BrowserContext interface {
    Navigate(ctx context.Context, url string, waitFor string, timeout time.Duration) (title string, err error)
    Click(ctx context.Context, selector string, timeout time.Duration) error
    ExtractText(ctx context.Context, selector string, maxBytes int) (text string, truncated bool, err error)
    Screenshot(ctx context.Context, selector string, format string, quality int) ([]byte, error)
    Close() error
}

type BrowserPool interface {
    Acquire(ctx context.Context, runID string) (BrowserContext, error)
    Release(runID string)
}
```

Each tool acquires a `BrowserContext` for the run at first use and releases it when the run completes (via a `PostRunHook`). This ensures the browser session is cleaned up even if the run fails.

---

## 6. Error Handling and Timeout Patterns

### 6.1 Layered timeouts

Browser operations require two levels of timeout:

1. **Connection timeout** — time to connect to the Lightpanda CDP endpoint. Should be 5 seconds maximum; failure suggests the sidecar is down.
2. **Operation timeout** — time for the navigation or interaction to complete. Caller-specified, capped at 120 seconds.

```go
// Connection with 5s deadline.
connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(connCtx, ...)
defer cancelAlloc()

// Per-operation timeout from tool parameters.
opCtx, cancel := context.WithTimeout(allocCtx, time.Duration(args.TimeoutSeconds)*time.Second)
defer cancel()
```

### 6.2 Error categories

Tool error results should distinguish between:

- `navigation_failed` — HTTP 4xx/5xx, DNS failure, timeout
- `element_not_found` — selector matched nothing within the timeout
- `browser_unavailable` — CDP connection refused; sidecar not running
- `content_too_large` — extracted content exceeded `max_bytes` (non-fatal; return truncated)
- `screenshot_failed` — rendering error or Lightpanda bug

All errors are returned as `error` values from the tool handler and surface as `tool.call.completed` SSE events with the error text in the payload. No errors are fatal to the run — the LLM can decide whether to retry, try a different selector, or abandon the browser approach.

### 6.3 Context cancellation

All chromedp operations accept `context.Context`. When a run is cancelled (via run termination or the run's context being cancelled), browser operations are interrupted cleanly by the context cancellation propagating into chromedp's internal HTTP/WebSocket calls.

---

## 7. Memory and Performance Assessment

### 7.1 Memory comparison at scale

Assuming 10 concurrent agents in a pool, each in its own container:

| Scenario | Per-Container Browser RSS | Total (10 containers) |
|----------|--------------------------|----------------------|
| No browser capability | 0 MB | 0 MB |
| Chrome headless sidecar | ~300–500 MB | ~3–5 GB |
| Lightpanda sidecar | ~15–50 MB | ~150–500 MB |

For a single harnessd container with harnessd process + agent loop (baseline ~200MB):
- With Chrome: ~500–700 MB per container
- With Lightpanda: ~215–250 MB per container

The Lightpanda numbers stay within a comfortable range for a typical cloud VM with 4GB RAM.

### 7.2 Tool call latency

For a `browser_navigate` to a simple page:

| Phase | Chrome Headless | Lightpanda |
|-------|----------------|-----------|
| Process start (if on-demand) | ~500ms | ~45ms |
| CDP connection | ~50ms | ~20ms |
| Page load (simple HTML) | ~200–500ms | ~100–300ms |
| **Total (sidecar, no startup)** | **~250–550ms** | **~120–320ms** |

For SPA-heavy pages with complex JS, both browsers converge on network + JS execution time and the differences narrow.

### 7.3 JS compatibility

Lightpanda uses a V8-based JavaScript engine, so standard ES2020+ features work. Known gaps as of early 2026:
- `requestAnimationFrame` and frame-based animations may not behave as in a real browser
- Worker threads (Web Workers, SharedArrayBuffer) are not supported
- `window.fetch` is partially implemented; some Fetch API edge cases may fail

For the primary agentic use case (page extraction, form filling, navigation), these gaps are immaterial. Agent tasks that require SPAs with complex client-side rendering should be tested before relying on Lightpanda.

---

## 8. Maturity Assessment

### 8.1 Project status

Lightpanda reached 1.0 stability in late 2025. As of early 2026:

- Active GitHub repository with regular commits
- CDP conformance test suite passes for the core automation domains
- Used in production by several companies for web scraping and testing pipelines
- Not yet recommended for full browser testing where pixel-perfect rendering matters

### 8.2 What is still missing for production use in harness

| Gap | Severity | Mitigation |
|-----|----------|-----------|
| Accessibility tree (AX domain) | Medium | Use text extraction + DOM queries instead |
| Full CSS computed styles | Low | Not needed for agentic text/form work |
| Network request modification | Medium | Limit to read-only interception for now |
| Stable release cadence | Low | Pin to a release tag; update deliberately |
| Go-native CDP wrapping | Low | chromedp works fine via remote allocator |
| `Fetch` domain for request rewriting | Medium | Defer to Phase 2 if needed |
| Windows/macOS binary distribution | Low | Container deployment only in Phase 1 |

### 8.3 Fallback strategy

The `BrowserPool` interface abstracts the browser backend. If Lightpanda proves insufficient for a specific task, the pool implementation can be swapped for `chromedp` with a real Chromium allocator without changing any tool code. This makes the choice of Lightpanda vs Chrome a deployment/configuration decision, not an API decision.

---

## 9. Integration Effort Estimate

### Phase 1: Core four tools with Lightpanda sidecar

| Task | Estimate |
|------|---------|
| `browser_pool.go` interface + chromedp-based remote implementation | 3–4 hours |
| `browser_navigate`, `browser_click`, `browser_extract` tools | 4–5 hours |
| `browser_screenshot` tool + base64 encoding | 2–3 hours |
| Tool description `.md` files (4) | 1 hour |
| Unit tests with mock BrowserPool | 3–4 hours |
| Container sidecar configuration (Docker) | 2–3 hours |
| Integration test (Lightpanda running locally) | 2–3 hours |
| **Total** | **~17–23 hours** |

### Phase 2: Polish and additional tools

| Task | Estimate |
|------|---------|
| `browser_type`, `browser_scroll`, `browser_wait` | 3–4 hours |
| `browser_eval` (JS evaluation) | 2 hours |
| Per-run session cleanup via PostRunHook | 1–2 hours |
| Request interception (read-only) | 3–4 hours |
| **Total** | **~9–12 hours** |

---

## 10. Recommendation

**Adopt Lightpanda for Phase 1, with Chrome Headless as a runtime-swappable fallback via the BrowserPool interface.**

**Rationale:**

1. **No existing browser tooling exists** — any implementation is greenfield. Starting with Lightpanda avoids the memory and startup costs of Chromium while remaining CDP-compatible.

2. **Memory profile is substantially better** for the container workspace architecture (§7.1). At pool_size=10, the difference between Lightpanda and Chrome is ~2.5–4.5 GB of RAM, which is decisive for budget cloud deployments.

3. **CDP compatibility is sufficient** for the target use cases: page navigation, text extraction, form interaction, and basic screenshot capture. None of the gaps in §8.2 block Phase 1 functionality.

4. **The BrowserPool interface decouples the implementation** from the tool API. If any gap proves blocking in production, swapping to Chrome headless requires only a new `BrowserPool` implementation, not API changes.

5. **Go integration is straightforward.** The `chromedp` library's remote allocator mode connects to any CDP endpoint — Lightpanda or Chrome — with the same code.

**Conditions:**
- Lightpanda must be tested against a representative set of SPAs used in harness tasks before declaring Phase 1 stable.
- Pin to a specific Lightpanda release tag in the Docker image; do not track `main`.
- Document the Lightpanda version and its CDP conformance profile in `docs/runbooks/`.

---

## Appendix A: Relevant Code Locations

| Component | File | Notes |
|-----------|------|-------|
| Fetch tool (HTTP-only baseline) | `internal/harness/tools/deferred/fetch.go` | Similar structure for browser tools |
| Web fetch / search tools | `internal/harness/tools/deferred/web.go` | Reference for WebFetcher interface pattern |
| Tool definition schema | `internal/harness/tools/catalog.go` | Tool registration |
| Tool tier constants | `internal/harness/tools/` | `TierDeferred` for browser tools |
| Container workspace | `internal/workspace/container.go` | Where sidecar would be co-located |
| Pool workspace | `internal/workspace/pool.go` | Pool lifecycle hooks |
| Tool descriptions | `internal/harness/tools/descriptions/` | Add `browser_*.md` files here |

## Appendix B: Proposed File Structure

```
internal/harness/tools/
  deferred/
    browser.go          # BrowserPool interface, BrowserContext interface
    browser_navigate.go # browser_navigate tool
    browser_extract.go  # browser_extract tool
    browser_click.go    # browser_click tool
    browser_screenshot.go # browser_screenshot tool
  descriptions/
    browser_navigate.md
    browser_extract.md
    browser_click.md
    browser_screenshot.md

internal/harness/tools/browser/
  lightpanda_pool.go    # chromedp remote allocator-based BrowserPool impl
  chromium_pool.go      # fallback: chromedp local Chromium allocator
  mock_pool.go          # test mock

docs/runbooks/
  browser-tooling.md    # operational runbook: start Lightpanda, configure, upgrade
```

## Appendix C: Sample BrowserPool Implementation Sketch

```go
// internal/harness/tools/browser/lightpanda_pool.go

package browser

import (
    "context"
    "sync"
    "github.com/chromedp/chromedp"
)

type LightpandaPool struct {
    wsURL    string  // e.g. "ws://localhost:9222"
    mu       sync.Mutex
    sessions map[string]*lightpandaSession
}

type lightpandaSession struct {
    allocCtx  context.Context
    cancelAlloc context.CancelFunc
    tabCtx    context.Context
    cancelTab context.CancelFunc
}

func (p *LightpandaPool) Acquire(ctx context.Context, runID string) (BrowserContext, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if _, ok := p.sessions[runID]; ok {
        return nil, fmt.Errorf("run %s already has an active browser session", runID)
    }

    allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx,
        []func(*chromedp.ExecAllocator){
            chromedp.WSURLReadyFunc(func(context.Context) (string, error) {
                return p.wsURL, nil
            }),
        })
    tabCtx, cancelTab := chromedp.NewContext(allocCtx)

    sess := &lightpandaSession{
        allocCtx:  allocCtx,
        cancelAlloc: cancelAlloc,
        tabCtx:    tabCtx,
        cancelTab: cancelTab,
    }
    p.sessions[runID] = sess
    return &lightpandaBrowserContext{sess: sess}, nil
}

func (p *LightpandaPool) Release(runID string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if sess, ok := p.sessions[runID]; ok {
        sess.cancelTab()
        sess.cancelAlloc()
        delete(p.sessions, runID)
    }
}
```
