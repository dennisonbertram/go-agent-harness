package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

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
		return
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			send(ctx, ch, SSEErrorMsg{Err: err})
		}
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var event string
	var dataParts []string

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
					send(ctx, ch, SSEDropMsg{})
				}
				if _, ok := msg.(SSEDoneMsg); ok {
					return
				}
			}
			event, dataParts = "", nil
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		send(ctx, ch, SSEErrorMsg{Err: err})
	}
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
