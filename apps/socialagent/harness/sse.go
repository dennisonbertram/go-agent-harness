package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// sseEvent holds the parsed fields of one SSE event.
type sseEvent struct {
	id    string
	event string
	data  string
}

// eventPayload is the top-level JSON structure in each SSE data field.
type eventPayload struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// completedPayload is the payload for run.completed events.
type completedPayload struct {
	Output string `json:"output"`
}

// failedPayload is the payload for run.failed events.
type failedPayload struct {
	Error string `json:"error"`
}

// StreamEvents connects to GET /v1/runs/{runID}/events, reads the SSE stream,
// and blocks until a terminal event (run.completed, run.failed, run.cancelled)
// is received or the context is cancelled.
func (c *Client) StreamEvents(ctx context.Context, runID string) (*RunResult, error) {
	url := fmt.Sprintf("%s/v1/runs/%s/events", c.baseURL, runID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without a global timeout for the streaming connection —
	// the caller's context controls cancellation instead.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: unexpected status %d", url, resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var cur sseEvent

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "id:"):
			cur.id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "event:"):
			cur.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			cur.data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case strings.HasPrefix(line, "retry:"):
			// ignored
		case line == "":
			// Blank line signals end of event — process if we have an event type.
			if cur.event == "" {
				cur = sseEvent{}
				continue
			}

			result, done, err := handleSSEEvent(cur)
			cur = sseEvent{}

			if err != nil {
				return nil, err
			}
			if done {
				return result, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading SSE stream: %w", err)
	}

	// Stream ended without a terminal event.
	return nil, errors.New("SSE stream ended without terminal event")
}

// handleSSEEvent processes a complete SSE event and returns:
//   - (result, true, nil) when a terminal success event is reached
//   - (nil, false, err) when a terminal failure event is reached
//   - (nil, false, nil) for non-terminal events (continue reading)
func handleSSEEvent(ev sseEvent) (*RunResult, bool, error) {
	switch ev.event {
	case "run.completed":
		var envelope eventPayload
		if err := json.Unmarshal([]byte(ev.data), &envelope); err != nil {
			return nil, false, fmt.Errorf("parse run.completed envelope: %w", err)
		}
		var p completedPayload
		if len(envelope.Payload) > 0 {
			if err := json.Unmarshal(envelope.Payload, &p); err != nil {
				return nil, false, fmt.Errorf("parse run.completed payload: %w", err)
			}
		}
		return &RunResult{
			Output: p.Output,
			RunID:  envelope.RunID,
		}, true, nil

	case "run.failed":
		var envelope eventPayload
		if err := json.Unmarshal([]byte(ev.data), &envelope); err != nil {
			return nil, false, fmt.Errorf("parse run.failed envelope: %w", err)
		}
		var p failedPayload
		if len(envelope.Payload) > 0 {
			if err := json.Unmarshal(envelope.Payload, &p); err != nil {
				return nil, false, fmt.Errorf("parse run.failed payload: %w", err)
			}
		}
		msg := p.Error
		if msg == "" {
			msg = "run failed"
		}
		return nil, false, errors.New(msg)

	case "run.cancelled":
		return nil, false, errors.New("run cancelled")

	default:
		// Non-terminal event — keep reading.
		return nil, false, nil
	}
}
