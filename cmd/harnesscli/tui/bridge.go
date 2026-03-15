package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// sseChanCap is the channel buffer depth for SSE messages.
// The bridge uses non-blocking sends: if the TUI update loop falls behind
// (e.g. heavy rendering), excess events emit SSEDropMsg rather than
// stalling the HTTP scanner. 256 slots covers burst-heavy tool-call streams
// without consuming significant memory (~8KB at 32 bytes/msg).
const sseChanCap = 256

// StartSSEBridge connects to the SSE endpoint at url and delivers decoded
// tea.Msg values on the returned channel. Call stop() to disconnect early.
// The channel is closed when the stream ends or ctx is cancelled.
func StartSSEBridge(ctx context.Context, url string) (<-chan tea.Msg, func()) {
	ch := make(chan tea.Msg, sseChanCap)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer cancel()
		defer close(ch)
		runBridge(ctx, url, ch)
	}()

	return ch, cancel
}

func runBridge(ctx context.Context, url string, ch chan<- tea.Msg) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		send(ctx, ch, SSEErrorMsg{Err: err})
		send(ctx, ch, SSEDoneMsg{EventType: "bridge.closed"})
		return
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			send(ctx, ch, SSEErrorMsg{Err: err})
			send(ctx, ch, SSEDoneMsg{EventType: "bridge.closed"})
		}
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var event string
	var dataParts []string
	var consecutiveDrops int

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			// Per SSE spec: multiple data: lines are concatenated with "\n".
			dataParts = append(dataParts, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		case line == "":
			if len(dataParts) > 0 {
				data := strings.Join(dataParts, "\n")
				msg := decodeSSE(event, data)
				if !trySend(ch, msg) {
					consecutiveDrops++
					// Channel is full; send SSEDropMsg non-blocking too so
					// we do not stall the scanner goroutine under sustained
					// backpressure. If the drop notification also cannot fit
					// it is silently discarded — the TUI is already lagging.
					trySend(ch, SSEDropMsg{})
					if consecutiveDrops >= 10 {
						send(ctx, ch, SSEErrorMsg{Err: fmt.Errorf("SSE bridge: too many dropped messages, stream may be corrupt")})
						consecutiveDrops = 0
					}
				} else {
					consecutiveDrops = 0
				}
				if _, ok := msg.(SSEDoneMsg); ok {
					return
				}
			}
			event, dataParts = "", nil
		}
	}
	// Flush any partial event buffered before EOF / connection drop.
	// Per SSE spec the stream should end with a blank line, but servers
	// may close the connection abruptly; deliver whatever data was pending.
	if len(dataParts) > 0 && ctx.Err() == nil {
		data := strings.Join(dataParts, "\n")
		msg := decodeSSE(event, data)
		trySend(ch, msg) // best-effort; drop on backpressure at EOF
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		send(ctx, ch, SSEErrorMsg{Err: err})
	}
	// Signal stream end (covers normal EOF; run.completed/run.failed paths
	// return above after sending their own SSEDoneMsg from decodeSSE).
	send(ctx, ch, SSEDoneMsg{})
}

type sseEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func decodeSSE(event, data string) tea.Msg {
	var env sseEnvelope
	if err := json.Unmarshal([]byte(data), &env); err != nil {
		return SSEErrorMsg{Err: err}
	}
	if env.Type == "run.completed" || env.Type == "run.failed" {
		return SSEDoneMsg{EventType: env.Type}
	}
	// Unknown event types are forwarded as SSEEventMsg so that consumers
	// can inspect EventType and Raw. No silent discard.
	return SSEEventMsg{EventType: env.Type, Raw: env.Payload}
}

func send(ctx context.Context, ch chan<- tea.Msg, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
}

func trySend(ch chan<- tea.Msg, msg tea.Msg) bool {
	select {
	case ch <- msg:
		return true
	default:
		return false
	}
}
