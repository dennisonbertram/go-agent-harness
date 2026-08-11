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
	var wg sync.WaitGroup

	// Reading happens on its own goroutine so the loop below can select on
	// cancellation. scanner.Scan blocks inside a read, so a context check placed
	// in the loop body can only fire after a line has already arrived — which
	// left the process ignoring SIGINT whenever it was idle (issue #1321).
	//
	// The reader stays parked in Read until stdin closes. That is deliberate: on
	// the cancellation path the process is on its way out, and there is no way to
	// interrupt a blocking read on stdin portably.
	lines := make(chan string)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(t.in)
		// Allow large payloads (up to 4MB per line).
		const maxTokenSize = 4 * 1024 * 1024
		scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		var line string
		select {
		case <-ctx.Done():
			// Let accepted work finish and respond before returning.
			wg.Wait()
			return ctx.Err()
		case l, ok := <-lines:
			if !ok {
				// EOF: the host closed stdin. This is the common shutdown path.
				wg.Wait()
				return nil
			}
			line = l
		}

		if strings.TrimSpace(line) == "" {
			continue
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
}

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
