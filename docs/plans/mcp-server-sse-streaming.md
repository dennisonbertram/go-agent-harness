# MCP Server: SSE Streaming for Run Results

## Status: PLANNED
## Related Issues: TBD
## Priority: Medium

---

## Problem

The current `get_run_status` tool on the MCP server is poll-only. A caller must repeatedly invoke `get_run_status` to determine when a run finishes. For runs that take 30–300 seconds, this produces unnecessary HTTP traffic and adds latency (up to one polling interval) between run completion and the caller being notified.

The MCP protocol version `2025-11-25` defines Streamable HTTP transport, which allows a server to push notifications to the client over a persistent SSE connection on `GET /mcp`. This enables the MCP server to actively push run events to interested clients instead of waiting for polls.

---

## Solution

Upgrade the MCP server (`internal/mcpserver/`) to:

1. Implement Streamable HTTP transport alongside the existing synchronous POST handler.
2. Add a `subscribe_run` tool that registers a client's interest in a specific run's events.
3. Fan out run events from `harnessd` (via internal polling or event hooks) to all registered SSE subscribers.
4. Send MCP notifications over the SSE stream as events arrive.

The existing `POST /mcp` handler and the `get_run_status` polling tool are preserved without modification.

---

## Protocol

MCP protocol version: `2025-11-25`

### Streamable HTTP Transport

| Method | Path | Purpose |
|---|---|---|
| `POST /mcp` | existing | Synchronous tool calls and initialize handshake |
| `GET /mcp` | new | SSE stream; server sends notifications to client |

The `GET /mcp` endpoint:
- Returns `Content-Type: text/event-stream`
- Returns `Cache-Control: no-cache`
- Sends SSE events as `data: <json>\n\n`
- Keeps connection alive until client disconnects or server shuts down
- Each SSE event is a JSON-RPC 2.0 notification (no `id` field)

---

## Architecture

### Components

```
internal/mcpserver/
  mcpserver.go         ← existing; add GET /mcp handler, wire Broker
  broker.go            ← new: fan-out hub for run event notifications
  subscriber.go        ← new: per-client SSE subscriber
  poller.go            ← new: polls harnessd for run state changes
  tools.go             ← new: subscribe_run handler
  sse_test.go          ← new
  broker_test.go       ← new
  poller_test.go       ← new
```

### Broker

The `Broker` is the central fan-out component. It maintains a map of `run_id → []chan Notification`. When a run event arrives, the broker dispatches it to all channels registered for that `run_id`.

```go
// broker.go

type Notification struct {
    Method string          `json:"method"`
    Params json.RawMessage `json:"params"`
}

type Broker struct {
    mu          sync.RWMutex
    subscribers map[string][]chan Notification // run_id → channels
}

func NewBroker() *Broker

// Subscribe registers a channel to receive notifications for runID.
// Returns a cancel func that removes the subscription and closes the channel.
func (b *Broker) Subscribe(runID string) (<-chan Notification, func())

// Publish sends n to all subscribers of runID.
// Non-blocking: if a subscriber channel is full, the notification is dropped for that subscriber.
func (b *Broker) Publish(runID string, n Notification)

// PublishAll sends n to all subscribers regardless of run_id.
// Used for global notifications like notifications/tools/list_changed.
func (b *Broker) PublishAll(n Notification)

// ActiveSubscriptions returns the count of active channels across all run_ids.
// Used in tests to verify cleanup.
func (b *Broker) ActiveSubscriptions() int
```

Subscriber channel buffer size: 64. If the buffer fills (slow client), notifications are dropped for that subscriber only. This prevents a slow client from blocking the broker.

### Subscriber

```go
// subscriber.go

type Subscriber struct {
    w       http.ResponseWriter
    flusher http.Flusher
    ch      <-chan Notification
    done    <-chan struct{} // closed when client disconnects
}

func NewSubscriber(w http.ResponseWriter, ch <-chan Notification) (*Subscriber, error)

// Stream writes SSE events to w until done is closed or context is cancelled.
func (s *Subscriber) Stream(ctx context.Context) error
```

Each notification is serialized as:
```
data: {"jsonrpc":"2.0","method":"...","params":{...}}\n\n
```

The SSE `event:` type field is NOT used; all events use the default unnamed event type. Clients filter by `method` inside the JSON payload.

### Poller

The Poller bridges harnessd's REST API to the Broker. It polls active runs and publishes events when state changes.

```go
// poller.go

type RunPoller struct {
    client   HarnessPoller
    broker   *Broker
    interval time.Duration
    mu       sync.Mutex
    watched  map[string]string // run_id → last known status
}

type HarnessPoller interface {
    GetRun(ctx context.Context, runID string) (RunStatus, error)
}

func NewRunPoller(client HarnessPoller, broker *Broker, interval time.Duration) *RunPoller

// Watch adds runID to the set of polled runs.
func (p *RunPoller) Watch(runID string)

// Unwatch removes runID. Called after terminal state is published.
func (p *RunPoller) Unwatch(runID string)

// Run starts the poll loop. Blocks until ctx is cancelled.
func (p *RunPoller) Run(ctx context.Context)
```

Poll interval: 2 seconds (matches `wait_for_run` in the stdio spec).

Terminal states: `completed`, `failed`. When a terminal state is observed, the poller publishes a final notification and calls `Unwatch`.

---

## Notifications

### run/event

Sent when a run changes status or emits a new message/step.

```json
{
  "jsonrpc": "2.0",
  "method": "run/event",
  "params": {
    "run_id": "abc123",
    "event_type": "status_changed",
    "status": "running",
    "step": 3
  }
}
```

`event_type` values: `status_changed`, `step_completed`, `tool_called`, `message_added`.

For this implementation, only `status_changed` is required. The others are reserved for future use when the harnessd SSE event stream is plumbed in.

### run/completed

Sent exactly once when a run reaches a terminal state.

```json
{
  "jsonrpc": "2.0",
  "method": "run/completed",
  "params": {
    "run_id": "abc123",
    "status": "completed",
    "cost_usd": 0.0042,
    "error": ""
  }
}
```

`status` is `completed` or `failed`. `error` is empty string when status is `completed`.

### notifications/tools/list_changed

Standard MCP notification. Sent when the tool list changes (currently only on server startup/restart). Reserved; not actively used in this implementation but the infrastructure is wired.

---

## subscribe_run Tool

### Tool Definition

**Name**: `subscribe_run`

**Description**: Subscribe to live events for a run. Returns a stream_id. Connect to GET /mcp with SSE to receive run/event and run/completed notifications for this run. Notifications include the run_id so multiple subscriptions can share one SSE connection.

**Input schema**:
```json
{
  "type": "object",
  "properties": {
    "run_id": { "type": "string", "description": "Run ID to subscribe to" }
  },
  "required": ["run_id"]
}
```

**Response** (on success):
```json
{"stream_id": "abc123", "run_id": "abc123", "sse_endpoint": "GET /mcp"}
```

`stream_id` equals `run_id` in this implementation (no separate fan-out key needed).

**Behavior**:
1. Validate `run_id` is non-empty.
2. GET `/v1/runs/{run_id}` to verify the run exists (return isError:true if 404).
3. If run is already in terminal state, return immediately with the final status (no subscription needed).
4. Call `poller.Watch(runID)` to add to poll set.
5. Return `{stream_id, run_id, sse_endpoint}`.

**Go signature**:
```go
func handleSubscribeRun(client HarnessPoller, poller *RunPoller) ToolHandler
```

---

## Server Changes

### GET /mcp Handler

Add to `mcpserver.Server.ServeHTTP`:

```go
case r.Method == http.MethodGet && r.URL.Path == "/mcp":
    s.handleSSE(w, r)
```

```go
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.WriteHeader(http.StatusOK)
    flusher.Flush()

    // Each SSE connection subscribes to ALL notifications via a global channel.
    // Clients filter by run_id in params.
    ch, cancel := s.broker.SubscribeAll()
    defer cancel()

    ctx := r.Context()
    for {
        select {
        case <-ctx.Done():
            return
        case n, ok := <-ch:
            if !ok {
                return
            }
            b, err := json.Marshal(JSONRPCNotification{
                JSONRPC: "2.0",
                Method:  n.Method,
                Params:  n.Params,
            })
            if err != nil {
                continue
            }
            fmt.Fprintf(w, "data: %s\n\n", b)
            flusher.Flush()
        }
    }
}
```

### Updated Broker: SubscribeAll

Add a global subscription channel to `Broker` (separate from per-run subscriptions):

```go
// SubscribeAll registers a channel that receives ALL notifications published to the broker.
// Returns the channel and a cancel func.
func (b *Broker) SubscribeAll() (<-chan Notification, func())
```

Internal: `broker.go` maintains a second subscriber list `globalSubscribers []chan Notification`. `Publish` and `PublishAll` both write to global subscribers as well as per-run subscribers.

### Server struct additions

```go
type Server struct {
    // existing fields unchanged
    mux     *http.ServeMux
    tools   map[string]ToolHandler

    // new fields
    broker  *Broker
    poller  *RunPoller
    client  *HarnessHTTPClient // existing or new field
}
```

`NewServer` must initialize `broker`, `poller`, and start the poller goroutine:

```go
func NewServer(addr string) *Server {
    b := NewBroker()
    client := NewHarnessHTTPClient(addr)
    p := NewRunPoller(client, b, 2*time.Second)
    s := &Server{broker: b, poller: p, client: client}
    // register tools including subscribe_run
    go p.Run(context.Background()) // TODO: wire to server shutdown context
    return s
}
```

The poller goroutine must be stopped on server shutdown. Add a `Shutdown(ctx context.Context) error` method to `Server` if not already present.

---

## Test Requirements

### Unit Tests

**T1** (`sse_test.go`): `GET /mcp` returns HTTP 200 with `Content-Type: text/event-stream`. Connection stays open (verified by goroutine reading from response body without EOF).

**T2** (`sse_test.go`): `subscribe_run` with valid run_id registers listener in broker. Broker's `ActiveSubscriptions()` increases by 1 after tool call.

**T3** (`broker_test.go`): Publishing a notification to `run_id` delivers it to a subscribed channel within 100ms. Non-subscribed channels receive nothing.

**T4** (`broker_test.go`): Publishing `run/completed` to `run_id` results in the notification appearing on the SSE stream (end-to-end through broker → SSE handler → response writer mock).

**T5** (`sse_test.go`): When the SSE client disconnects (close response body / cancel request context), the broker's `ActiveSubscriptions()` drops back to 0. No goroutine leak (verified with `goleak` or by checking goroutine count before/after).

**T6** (`broker_test.go`): Two concurrent SSE clients subscribed to the same run_id both receive a published notification.

**T7** (`broker_test.go`): A notification for `run_id: "A"` does not appear on a client only subscribed to `run_id: "B"`. (Both clients use `SubscribeAll`; each must filter by run_id in params — verify that the params.run_id is correct in the published notification.)

**T8** (`poller_test.go`): `RunPoller` with a mock `HarnessPoller` that returns `running` → `running` → `completed`. Verifies `run/event` (status_changed) is published, then `run/completed` is published, then `Unwatch` is called (map shrinks to 0).

**T9** (`poller_test.go`): `subscribe_run` tool called on a run already in `completed` state returns the final status immediately without adding to the watch set.

### Integration Test (`sse_test.go`)

**T10**: Full flow:
1. Start `httptest.Server` mock for harnessd (returns `running` on first poll, `completed` on second).
2. POST `/mcp` → `initialize`.
3. POST `/mcp` → `tools/call subscribe_run`.
4. Open `GET /mcp` SSE connection.
5. Assert `run/event` notification received.
6. Assert `run/completed` notification received.
7. Assert SSE connection still open after completion (server does not close it).

### Race Tests

**T11** (`sse_test.go`): 20 concurrent goroutines each open a `GET /mcp` connection. Simultaneously, 5 goroutines publish 10 notifications each via the broker. Run with `-race`. No data races.

**T12** (`sse_test.go`): One subscriber connects while a run transitions to `completed` simultaneously. Verify that the subscriber receives either the `run/event` + `run/completed` pair, or just `run/completed`, but never a deadlock and no goroutine leak.

---

## Regression Tests

**R1**: Existing `POST /mcp` synchronous tool call tests pass unchanged. The new `GET /mcp` handler must not interfere with the POST handler. Verified by running all existing `mcpserver` tests.

**R2**: `get_run_status` polling tool still returns correct data. No change to its handler code.

**R3**: `start_run` and `list_runs` tools unaffected. All existing tool tests pass.

**R4**: `go test ./internal/mcpserver/... -race` passes with 0 data races.

---

## Failure Modes and Edge Cases

| Scenario | Behavior |
|---|---|
| Subscriber's channel buffer full (slow client) | Notification dropped for that subscriber; other subscribers unaffected |
| harnessd returns error on poll | Broker not notified; poller retries on next interval; error logged to stderr |
| Run does not exist (404 on subscribe_run) | Return isError:true immediately; do not add to watch set |
| GET /mcp on non-flusher ResponseWriter (e.g., in test without hijacking) | Return 500 with message "streaming not supported" |
| Server shuts down with active SSE connections | Context cancellation drains connections; in-flight notifications dropped |
| Multiple subscribe_run calls for same run_id | Each call is idempotent at the broker level; poller.Watch is idempotent |

---

## Out of Scope

- Proxying the raw `harnessd` SSE event stream (that is a richer future integration)
- Per-event filtering (client receives all events for subscribed run_id)
- Authentication of SSE connections (separate issue)
- stdio transport SSE streaming (separate spec: mcp-server-stdio-transport.md has its own scope)
- Resumable SSE streams (Last-Event-ID support) — not required by current MCP clients
