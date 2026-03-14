package harnessmcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// StdioTransport reads newline-delimited JSON-RPC messages from in and writes
// responses to out. Each message is dispatched in its own goroutine, with
// concurrent writes to out serialized by mu.
type StdioTransport struct {
	in         io.Reader
	out        io.Writer
	mu         sync.Mutex // guards writes to out
	dispatcher *Dispatcher
}

// NewStdioTransport creates a StdioTransport using the given reader, writer and dispatcher.
func NewStdioTransport(in io.Reader, out io.Writer, d *Dispatcher) *StdioTransport {
	return &StdioTransport{
		in:         in,
		out:        out,
		dispatcher: d,
	}
}

// Run reads JSON-RPC messages from in until EOF or ctx cancellation.
// Each message is dispatched in its own goroutine.
func (t *StdioTransport) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(t.in)
	// Allow large payloads (up to 4MB per line).
	const maxTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)

	var wg sync.WaitGroup

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check context before dispatching.
		select {
		case <-ctx.Done():
			break
		default:
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Malformed JSON — write a parse error response.
			// We don't have an ID because we couldn't parse the message.
			resp := errorResponse(nil, -32700, "Parse error")
			_ = t.writeResponse(resp)
			continue
		}

		// Validate: method is required.
		if req.Method == "" {
			resp := errorResponse(req.ID, -32600, "Invalid Request")
			_ = t.writeResponse(resp)
			continue
		}

		wg.Add(1)
		go func(r Request) {
			defer wg.Done()
			resp, shouldRespond := t.dispatcher.Dispatch(ctx, r)
			if shouldRespond {
				_ = t.writeResponse(resp)
			}
		}(req)
	}

	// Wait for all in-flight dispatches to complete.
	wg.Wait()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdio transport: scanner: %w", err)
	}
	return nil
}

// writeResponse serializes resp as JSON + newline, serialized by mu.
func (t *StdioTransport) writeResponse(resp Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("stdio transport: marshal response: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	_, err = fmt.Fprintf(t.out, "%s\n", data)
	return err
}
