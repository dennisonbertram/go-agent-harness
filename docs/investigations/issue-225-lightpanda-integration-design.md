# Issue #225: Lightpanda Integration Design — Architecture Perspective

**Date:** 2026-03-14
**Perspective:** Integration Architecture
**Issue:** Research — evaluate Lightpanda as headless browser backend for browser tool

---

## 1. Current Browser Tool Architecture

### State: No browser tool exists

A full audit of `internal/harness/tools/` and all registered tools confirms there is **no browser tool** of any kind in the harness. The existing web-related primitives are:

| Tool | Location | Nature |
|------|----------|--------|
| `fetch` | `internal/harness/tools/fetch.go` | Dumb HTTP GET via `net/http`, returns raw body |
| `download` | `internal/harness/tools/download.go` | HTTP GET + write to workspace file |
| `web_fetch` | `internal/harness/tools/deferred/web.go` | Delegates to `WebFetcher` interface |
| `web_search` | `internal/harness/tools/deferred/web.go` | Delegates to `WebFetcher` interface |
| `agentic_fetch` | `internal/harness/tools/deferred/agent.go` | `WebFetcher.Fetch()` + sub-agent analysis |

### The `WebFetcher` interface (defined, not implemented in tree)

```go
// internal/harness/tools/types.go
type WebFetcher interface {
    Search(ctx context.Context, query string, maxResults int) ([]map[string]any, error)
    Fetch(ctx context.Context, url string) (string, error)
}
```

This interface is injected into `BuildOptions.WebFetcher` and wired into `DefaultRegistryOptions`. There is **no concrete implementation** of `WebFetcher` in the main source tree — it is either provided externally or left nil (which causes the `web_fetch`/`web_search`/`agentic_fetch` tools to not be registered at all, since the registry builder checks `opts.WebFetcher != nil`).

### Tool registration flow

```
NewDefaultRegistryWithOptions(workspaceRoot, opts)
  └── if EnableWebOps && WebFetcher != nil:
        register web_fetch, web_search, agentic_fetch (all TierDeferred)
```

All three web tools are **TierDeferred** — they only become visible to the LLM after the agent uses `find_tool` to discover and activate them. This is the correct tier for a browser tool as well, given its heavyweight nature.

### What is missing

No existing tool provides:
- JavaScript execution in page context
- DOM traversal or CSS selector querying
- Cookie/session management
- Click/form interaction
- Screenshot capture
- Waiting for async content (SPA rendering)
- Multi-page session state

The current `fetch` tool retrieves static HTML only. Pages that require JS rendering, login sessions, or interactive navigation cannot be meaningfully consumed by an agent using only `fetch`.

---

## 2. Lightpanda Background (for Integration Context)

Lightpanda is a headless browser written in Zig, designed specifically for AI agent and automation workloads. Key facts relevant to integration:

- **Protocol**: Exposes a CDP (Chrome DevTools Protocol) endpoint over WebSocket. Any CDP client library can talk to it.
- **JavaScript**: Runs a JavaScript engine (Jint-compatible subset), handles most page JS without a full V8.
- **No rendering**: No GPU, no display server required — optimized for content extraction, not pixel-perfect rendering.
- **Resource profile**: Claims ~10x lower memory than Chrome, significantly faster cold start.
- **Limitations**: Does not handle WebGL, WebRTC, complex CSS animations, some advanced Web APIs. SPAs with aggressive lazy loading may partially fail.
- **Distribution**: Single binary, available as Docker image `ghcr.io/lightpanda-io/lightpanda`.
- **Go CDP client**: The `github.com/chromedp/chromedp` library can connect to any CDP endpoint, making it a natural Go client for Lightpanda.

---

## 3. Integration Design Options

### Option A: Direct Replacement — Replace current `fetch`/`web_fetch` backend with Lightpanda

**Concept:** Replace the `net/http` backend behind `fetch` and `web_fetch` with a Lightpanda connection that renders pages before returning content.

**How it would work:**
- The `fetch` tool handler opens a Lightpanda tab, navigates to the URL, waits for `DOMContentLoaded` (or a configurable condition), extracts `document.body.innerText` or full HTML, and closes the tab.
- `web_fetch` and `agentic_fetch` delegate to the same backend.

**Pros:**
- Transparent upgrade — no new tool names, existing tool descriptions are valid.
- Agents that already use `fetch` get JS-rendered content without learning new tools.
- Minimal new surface area to test.

**Cons:**
- The `fetch` tool is used for API calls (JSON endpoints, GitHub APIs, etc.) where JS rendering is wasted overhead. Opening a browser tab for `https://api.github.com/repos/...` is a 10x slowdown for no benefit.
- No way for the agent to express "I need click/session/screenshot" — the tool API is just `{url} -> content`.
- A single backend instance is a bottleneck. The `fetch` tool is `ParallelSafe: true`; Lightpanda tab concurrency needs careful session management.
- Migration path is fragile: the current `fetch` tool handles arbitrary HTTP schemes and raw bodies (binary APIs, large downloads). A browser backend is wrong for those use cases.

**Assessment:** Unsuitable. The semantics of `fetch` are "raw HTTP GET." Conflating that with "browser render" degrades performance for the majority of use cases.

---

### Option B: Pluggable Backend — New `browser` tool family with a `BrowserBackend` interface (RECOMMENDED)

**Concept:** Introduce a `BrowserBackend` interface in `internal/harness/tools/` parallel to `WebFetcher`. The browser tool family is a separate set of deferred tools that an agent discovers and activates when it needs real browser interaction. Lightpanda is the default implementation; a `chromedp`-based Chromium implementation can be added later.

**Interface shape (Go sketch — not implementation):**

```go
// internal/harness/tools/types.go (additions)

// BrowserSession represents a single browser tab lifecycle.
// Implementations must be safe for sequential use by one goroutine.
// Sessions are not concurrent — each Navigate/Click/Eval is sequential within a tab.
type BrowserSession interface {
    // Navigate loads a URL and waits for the page to reach the given readiness state.
    Navigate(ctx context.Context, url string, wait WaitCondition) error

    // Content returns the current page's rendered text content (body.innerText).
    Content(ctx context.Context) (string, error)

    // HTML returns the full rendered HTML of the current page.
    HTML(ctx context.Context) (string, error)

    // QueryAll returns text content of all elements matching the CSS selector.
    QueryAll(ctx context.Context, selector string) ([]string, error)

    // EvalJS evaluates a JavaScript expression in the page context and returns
    // the JSON-marshaled result. For extraction only — not for mutation.
    EvalJS(ctx context.Context, expr string) (string, error)

    // Screenshot captures the viewport as PNG bytes.
    // Returns ErrScreenshotNotSupported if the backend cannot render pixels.
    Screenshot(ctx context.Context) ([]byte, error)

    // Click simulates a user click on the first element matching selector.
    Click(ctx context.Context, selector string) error

    // FillInput sets the value of a form input element.
    FillInput(ctx context.Context, selector, value string) error

    // Close releases the tab and all associated resources.
    Close(ctx context.Context) error
}

// WaitCondition controls how Navigate waits for page readiness.
type WaitCondition string

const (
    WaitDOMContentLoaded WaitCondition = "domcontentloaded" // default
    WaitNetworkIdle      WaitCondition = "networkidle"       // wait for all network requests to settle
    WaitLoad             WaitCondition = "load"              // window.load event
)

// BrowserBackend manages browser tab lifecycle.
// Implementations must be safe for concurrent NewSession calls.
type BrowserBackend interface {
    // NewSession creates a new isolated browser tab/context.
    NewSession(ctx context.Context) (BrowserSession, error)

    // Name returns the backend name for logging and error messages (e.g. "lightpanda", "chromium").
    Name() string

    // Close shuts down the backend, releasing all resources.
    Close() error
}

// ErrScreenshotNotSupported is returned by Screenshot on backends that do not support pixel rendering.
var ErrScreenshotNotSupported = errors.New("browser: screenshot not supported by this backend")

// ErrBrowserTimeout is returned when a browser operation exceeds its deadline.
var ErrBrowserTimeout = errors.New("browser: operation timed out")
```

**Tool API exposed to agent:**

The agent would see these deferred tools (activated via `find_tool`):

```
browser_navigate   — navigate to URL, return rendered text
browser_query      — CSS selector query on current page
browser_eval       — evaluate JS expression, return result
browser_click      — click element matching selector
browser_fill       — fill form input
browser_screenshot — capture page screenshot (may be unavailable)
```

Each tool call operates on a **per-run browser session**: when the agent first calls any `browser_*` tool, a session is opened and stored in the run context. Subsequent calls reuse it. The session is closed when the run completes (via a cleanup hook).

**Session lifecycle in run context:**

```go
// The browser session is stored in context, keyed by a private context key.
// The runner's tool execution infrastructure already supports this pattern
// via ContextKeyRunID and similar keys in types.go.
const ContextKeyBrowserSession contextKey = "browser_session"
```

A `browserSessionManager` (held by `DefaultRegistryOptions`) would:
- Create a session on first `browser_*` call for a given run ID.
- Cache it for the run duration.
- Register a cleanup callback (via the existing `CallbackManager` or a dedicated cleanup hook) to `session.Close()` when the run ends.

**Configuration in `DefaultRegistryOptions`:**

```go
type DefaultRegistryOptions struct {
    // ... existing fields ...

    // BrowserBackend enables the browser_* tool family.
    // If nil, browser tools are not registered.
    BrowserBackend htools.BrowserBackend

    // EnableBrowser controls whether browser tools are registered.
    // Requires BrowserBackend to be non-nil.
    EnableBrowser bool
}
```

**Wiring in `NewDefaultRegistryWithOptions`:**

```go
if opts.EnableBrowser && opts.BrowserBackend != nil {
    deferredTools = append(deferredTools,
        deferred.BrowserNavigateTool(opts.BrowserBackend, sessionMgr),
        deferred.BrowserQueryTool(opts.BrowserBackend, sessionMgr),
        deferred.BrowserEvalTool(opts.BrowserBackend, sessionMgr),
        deferred.BrowserClickTool(opts.BrowserBackend, sessionMgr),
        deferred.BrowserFillTool(opts.BrowserBackend, sessionMgr),
        deferred.BrowserScreenshotTool(opts.BrowserBackend, sessionMgr),
    )
}
```

**Lightpanda implementation shape:**

```go
// internal/harness/tools/browser/lightpanda.go

// LightpandaBackend connects to a running Lightpanda process via CDP WebSocket.
type LightpandaBackend struct {
    wsURL     string          // e.g. "ws://localhost:9222"
    allocCtx  context.Context // chromedp allocator context for Lightpanda endpoint
    allocCancel context.CancelFunc
}

// NewLightpandaBackend connects to a Lightpanda instance at wsURL.
// wsURL is the CDP WebSocket endpoint exposed by lightpanda (e.g. "ws://localhost:9222").
func NewLightpandaBackend(ctx context.Context, wsURL string) (*LightpandaBackend, error)

func (b *LightpandaBackend) NewSession(ctx context.Context) (htools.BrowserSession, error)
func (b *LightpandaBackend) Name() string { return "lightpanda" }
func (b *LightpandaBackend) Close() error
```

The `chromedp` library's `chromedp.NewRemoteAllocator` accepts a CDP WebSocket URL and can connect to any CDP-compatible endpoint, including Lightpanda.

**Pros:**
- Clean separation: `fetch`/`web_fetch` for HTTP, `browser_*` for rendered/interactive pages.
- Swappable: Chromium can be plugged in as a `BrowserBackend` without any tool-layer changes.
- Deferred tier: agent only gets browser tools when it asks for them, keeping context window clean.
- Per-run session: stateful multi-step interactions (login → navigate → extract) work naturally.
- Fits existing extension patterns: matches how `CronClient`, `CallbackManager`, `WebFetcher` are all injected via `DefaultRegistryOptions`.

**Cons:**
- More surface area: 6 new tool definitions, descriptions, and handlers.
- Session management adds statefulness that must be properly cleaned up on run completion, run failure, and timeout.
- Screenshot returns binary PNG bytes — the tool result system currently returns strings; this needs a base64-encoded output format.

**Migration path:**
1. Define `BrowserBackend` + `BrowserSession` interfaces.
2. Implement `LightpandaBackend` using `chromedp` remote allocator.
3. Implement `ChromiumBackend` using standard `chromedp` allocator.
4. Wire into `DefaultRegistryOptions` behind a feature flag.
5. Add `HARNESS_BROWSER_BACKEND=lightpanda` / `chromium` env var to `cmd/harnessd`.

---

### Option C: Sidecar Container — Run Lightpanda as a sidecar in the workspace Docker container

**Concept:** When provisioning a `ContainerWorkspace`, also start Lightpanda in the same Docker network. The harnessd instance inside the container connects to `ws://lightpanda:9222` as its `BrowserBackend`.

**How it would work in `container.go`:**

The `ContainerWorkspace.Provision()` call (in `internal/workspace/container.go`) currently starts only the harnessd container. To add Lightpanda as a sidecar:

1. Create a Docker user-defined network per workspace.
2. Start `ghcr.io/lightpanda-io/lightpanda:latest` on that network with container name `lightpanda-<workspaceID>`.
3. Start the harnessd container on the same network with env `HARNESS_BROWSER_WS_URL=ws://lightpanda-<workspaceID>:9222`.
4. harnessd reads this env var at startup and creates a `LightpandaBackend` pointed at that URL.
5. Both containers are destroyed when `ContainerWorkspace.Destroy()` is called.

**Go communication:** The harnessd Go code communicates with Lightpanda over the CDP WebSocket URL — no changes to the IPC mechanism are needed beyond what Option B already requires.

**Container orchestration implications:**

The existing `ContainerWorkspace` already manages Docker containers via the Docker SDK. Adding a sidecar requires:
- `docker network create` before container creation.
- Two `ContainerCreate` + `ContainerStart` calls (ordering matters: Lightpanda first, then harnessd to avoid connection failures on startup).
- Two `ContainerStop` + `ContainerRemove` calls in `Destroy()`.
- A readiness probe for Lightpanda: poll `ws://lightpanda:9222` or `http://lightpanda:9222/json/version` before marking the workspace ready.

**Pros:**
- Clean isolation: browser state is fully in the container, no browser process leaking into the host.
- Works well with the existing workspace abstraction — `LocalWorkspace` would use a locally-running Lightpanda process; `ContainerWorkspace` would use the sidecar.
- Failure is contained: Lightpanda crash doesn't take down harnessd.

**Cons:**
- Significantly increases container provisioning complexity and startup time.
- Adds a second container per workspace to the pool — doubles Docker resource overhead.
- Requires Docker network management, which is not currently in the workspace code.
- The pool (`internal/workspace/pool.go`) would need to account for dual-container workspaces.
- `LocalWorkspace` and `WorktreeWorkspace` would need a different Lightpanda provision path (e.g., launch a local binary).
- Testing the workspace provisioning with a second container is significantly harder.

**Assessment:** Option C is the right deployment topology for production container environments, but it is an **operational concern**, not an architectural one. It should be implemented as a variant of Option B's `BrowserBackend` injection — where `LightpandaBackend` is initialized with either a local URL or a sidecar URL, both paths using the same CDP WebSocket protocol.

---

## 4. Recommended Approach

**Implement Option B (Pluggable Backend) using CDP over WebSocket.**

This is the correct choice for this codebase because:

1. **Fits the existing extension pattern exactly.** Every external capability (`WebFetcher`, `CronClient`, `MCPRegistry`, `CallbackManager`) is injected as an interface into `DefaultRegistryOptions`. A `BrowserBackend` is just one more such injectable. No runner changes are needed.

2. **Lightpanda becomes the default; Chromium is an upgrade path.** The interface is defined once. Lightpanda provides the lightweight default. Chromium can be swapped in for pages that need full V8, WebGL, etc.

3. **Deferred tier is correct.** Browser tools are expensive and stateful. They should not appear in every agent's context window — only when the agent explicitly reaches for them via `find_tool`.

4. **Per-run session management is the right granularity.** The run is already the unit of agent execution. Session state (cookies, localStorage, navigation history) is naturally run-scoped. When the run ends, the session closes.

5. **No breakage to existing tools.** `fetch`, `web_fetch`, `web_search` are unchanged. An agent gets browser capability in addition to, not instead of, these.

---

## 5. Go Interface Sketches (Non-Exhaustive)

These are interface shapes for design discussion — not implementation.

### Core interfaces

```go
// Package: internal/harness/tools

// BrowserBackend manages the lifecycle of browser sessions.
// Implementations (LightpandaBackend, ChromiumBackend) are injected
// via DefaultRegistryOptions.BrowserBackend.
type BrowserBackend interface {
    NewSession(ctx context.Context) (BrowserSession, error)
    Name() string
    Close() error
}

// BrowserSession is an isolated browser tab.
// Tool handlers obtain a session via BrowserSessionManager.Get(runID).
type BrowserSession interface {
    Navigate(ctx context.Context, url string, wait WaitCondition) error
    Content(ctx context.Context) (string, error)
    HTML(ctx context.Context) (string, error)
    QueryAll(ctx context.Context, selector string) ([]string, error)
    EvalJS(ctx context.Context, expr string) (string, error)
    Screenshot(ctx context.Context) ([]byte, error)
    Click(ctx context.Context, selector string) error
    FillInput(ctx context.Context, selector, value string) error
    Close(ctx context.Context) error
}

type WaitCondition string

const (
    WaitDOMContentLoaded WaitCondition = "domcontentloaded"
    WaitNetworkIdle      WaitCondition = "networkidle"
    WaitLoad             WaitCondition = "load"
)

// BrowserSessionManager maps run IDs to open sessions.
// Sessions are created on first use and closed when the run ends.
type BrowserSessionManager interface {
    Get(ctx context.Context, runID string) (BrowserSession, error)
    Close(runID string) error
}
```

### Lightpanda backend shape

```go
// Package: internal/harness/tools/browser

// LightpandaBackend connects to a Lightpanda CDP endpoint.
// wsURL is the CDP WebSocket URL (e.g. "ws://localhost:9222").
type LightpandaBackend struct {
    wsURL string
}

func NewLightpandaBackend(wsURL string) *LightpandaBackend

func (b *LightpandaBackend) NewSession(ctx context.Context) (htools.BrowserSession, error) {
    // Uses chromedp.NewRemoteAllocator(ctx, b.wsURL) to connect,
    // then chromedp.NewContext() to open a tab.
    // Returns a lightpandaSession that wraps a chromedp.Context.
}

func (b *LightpandaBackend) Name() string { return "lightpanda" }
func (b *LightpandaBackend) Close() error { return nil } // stateless; Lightpanda process is external
```

### Tool handler shape (browser_navigate)

```go
// Package: internal/harness/tools/deferred

func BrowserNavigateTool(backend htools.BrowserBackend, mgr htools.BrowserSessionManager) htools.Tool {
    def := htools.Definition{
        Name:         "browser_navigate",
        Description:  descriptions.Load("browser_navigate"),
        Action:       htools.ActionFetch,
        Mutating:     false,
        ParallelSafe: false, // sessions are per-run and sequential
        Tier:         htools.TierDeferred,
        Tags:         []string{"browser", "navigate", "web", "render", "javascript"},
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "url":  map[string]any{"type": "string"},
                "wait": map[string]any{"type": "string", "enum": []string{"domcontentloaded", "networkidle", "load"}},
            },
            "required": []string{"url"},
        },
    }
    handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
        args := struct {
            URL  string `json:"url"`
            Wait string `json:"wait"`
        }{Wait: "domcontentloaded"}
        if err := json.Unmarshal(raw, &args); err != nil {
            return "", fmt.Errorf("parse browser_navigate args: %w", err)
        }
        runID := htools.RunIDFromContext(ctx)
        sess, err := mgr.Get(ctx, runID)
        if err != nil {
            return "", fmt.Errorf("browser session: %w", err)
        }
        if err := sess.Navigate(ctx, args.URL, htools.WaitCondition(args.Wait)); err != nil {
            return "", fmt.Errorf("browser navigate: %w", err)
        }
        content, err := sess.Content(ctx)
        if err != nil {
            return "", fmt.Errorf("browser content: %w", err)
        }
        return htools.MarshalToolResult(map[string]any{
            "url":     args.URL,
            "content": content,
        })
    }
    return htools.Tool{Definition: def, Handler: handler}
}
```

---

## 6. Failure Modes and Fallback Design

### Lightpanda-specific failure modes

| Failure | Signal | Recommended handling |
|---------|--------|----------------------|
| CDP connect timeout | `err` from `NewSession` | Return `{"error": "browser_unavailable", "reason": "..."}` to agent; agent can fall back to `fetch` |
| Page load timeout | `context.DeadlineExceeded` during `Navigate` | Return `{"error": "browser_timeout", "url": "..."}` with partial content if available |
| JS execution error | chromedp eval error | Return `{"error": "eval_failed", "detail": "..."}` |
| Screenshot not supported | `ErrScreenshotNotSupported` | Tool returns `{"error": "screenshot_not_supported", "backend": "lightpanda"}` |
| Page requires WebGL/complex SPA | JS engine failures, blank content | Return what content is available with a `{"partial": true}` flag; agent decides whether to escalate |

### Automatic fallback to Chromium

Automatic transparent fallback is NOT recommended for two reasons:

1. **Hidden cost:** Chromium is significantly heavier than Lightpanda. Silently escalating to Chromium changes the cost/latency profile of tool calls without the agent being aware. This breaks the agent's model of tool cost.

2. **Agent control is better:** The correct design is to signal the failure clearly in the tool result (via an error code like `"js_unsupported"` or `"partial_render"`) and let the agent decide whether to retry, request a different approach, or accept partial results. The agent can include logic like: "if browser_navigate returns partial:true, try agentic_fetch or escalate."

If a multi-backend fallback is desired, it should be an explicit capability of the `BrowserBackend` implementation:

```go
// FallbackBackend tries primary, falls back to secondary if primary returns ErrUnsupported.
type FallbackBackend struct {
    Primary   BrowserBackend // e.g. LightpandaBackend
    Secondary BrowserBackend // e.g. ChromiumBackend
}
```

This makes the fallback explicit, configurable, and observable (it can log which backend was used in the tool result).

---

## 7. Testing Strategy

### Unit tests (no browser required)

The `BrowserBackend` interface enables full mock injection:

```go
// testutil/browser.go (or inline in _test.go)
type mockBrowserBackend struct {
    sessions []*mockBrowserSession
    err      error
}

func (m *mockBrowserBackend) NewSession(ctx context.Context) (htools.BrowserSession, error) {
    if m.err != nil { return nil, m.err }
    sess := &mockBrowserSession{content: "mock page content"}
    m.sessions = append(m.sessions, sess)
    return sess, nil
}

type mockBrowserSession struct {
    content    string
    navigated  []string
    evalResult string
    closed     bool
}
// ... implement all BrowserSession methods on mockBrowserSession
```

Unit tests for each tool handler verify:
- Correct tool name, parameters, description loading
- Args parsing (missing required, invalid enum values, extra fields)
- Session reuse across multiple calls in same run
- Session creation error propagates correctly
- Close is called on session when manager.Close(runID) is called
- Screenshot returns base64-encoded result (or correct ErrScreenshotNotSupported)

### Integration tests (real Lightpanda or Chromium)

```go
// internal/harness/tools/browser/lightpanda_integration_test.go
//go:build integration

func TestLightpandaBackend_Navigate(t *testing.T) {
    // Requires Lightpanda running at LIGHTPANDA_WS_URL env var
    wsURL := os.Getenv("LIGHTPANDA_WS_URL")
    if wsURL == "" {
        t.Skip("LIGHTPANDA_WS_URL not set")
    }
    backend := browser.NewLightpandaBackend(wsURL)
    // ... test navigate, content, query, eval
}
```

Integration tests are gated behind the `integration` build tag, consistent with `container_integration_test.go` and `vm_integration_test.go` in the workspace package.

### Testing with multiple backends

Since `BrowserBackend` is an interface, the same test suite can be run against both Lightpanda and Chromium via table-driven tests:

```go
func testBrowserBackend(t *testing.T, backend htools.BrowserBackend) {
    // ... shared test cases
}

func TestLightpandaIntegration(t *testing.T) { testBrowserBackend(t, lightpandaBackend) }
func TestChromiumIntegration(t *testing.T)   { testBrowserBackend(t, chromiumBackend) }
```

This provides a backend compatibility matrix and catches behavioral differences between Lightpanda and Chromium for the same page.

---

## 8. Implementation Sequence

If this research confirms Lightpanda is viable, the implementation order would be:

1. **Define interfaces** — `BrowserBackend`, `BrowserSession`, `WaitCondition`, `BrowserSessionManager` in `internal/harness/tools/types.go`.
2. **Implement `BrowserSessionManager`** — a thread-safe map of `runID -> BrowserSession`, with `Close(runID)` called by a run-completion hook.
3. **Implement `LightpandaBackend`** — in `internal/harness/tools/browser/lightpanda.go`, using `chromedp` remote allocator.
4. **Implement tool handlers** — `browser_navigate`, `browser_query`, `browser_eval`, `browser_click`, `browser_fill`, `browser_screenshot` in `internal/harness/tools/deferred/browser.go`.
5. **Write `.md` descriptions** — one per tool in `internal/harness/tools/descriptions/`.
6. **Wire into `DefaultRegistryOptions`** — add `BrowserBackend` field, register tools if non-nil.
7. **Wire into `cmd/harnessd`** — read `HARNESS_BROWSER_BACKEND` and `HARNESS_BROWSER_WS_URL` env vars, create backend, inject into server config.
8. **Unit tests** — mock-based tests for all tool handlers.
9. **Integration tests** — `//go:build integration` tests against real Lightpanda process.

---

## Summary

**Recommended approach:** Option B — a `BrowserBackend` interface injected via `DefaultRegistryOptions`, with `LightpandaBackend` as the first concrete implementation using `chromedp` as the CDP client. Browser tools are registered as deferred-tier tools (`browser_navigate`, `browser_query`, `browser_eval`, `browser_click`, `browser_fill`, `browser_screenshot`), discovered and activated by agents via `find_tool`. Session state is scoped to the run lifetime. Chromium can be added as a second backend without any tool-layer changes.
