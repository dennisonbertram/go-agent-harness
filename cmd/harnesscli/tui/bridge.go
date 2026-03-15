package tui

import (
	"context"
	"net/http"
)

// StartSSEBridge opens an SSE connection and returns a channel of messages
// and a cancel function. The bridge converts server-sent events into
// BubbleTea-compatible messages for the TUI model.
// Full implementation in a later ticket.
func StartSSEBridge(ctx context.Context, client *http.Client, baseURL, runID string) (<-chan interface{}, func()) {
	ch := make(chan interface{})
	// Use ctx directly; no derived context needed until real implementation.
	// Calling cancel is a no-op that satisfies the cancel-func return contract.
	cancel := func() {}
	_ = ctx
	_ = client
	_ = baseURL
	_ = runID
	close(ch)
	return ch, cancel
}
