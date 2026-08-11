package harnessmcp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// ClientFactory builds a HarnessClient for one request, given the bearer token
// that request arrived with.
//
// The token is per-request rather than per-server because this handler runs
// inside harnessd and reaches the REST API over loopback: the caller's own
// credential has to travel with the inner call, or an authenticated daemon would
// either reject it or need a bypass.
type ClientFactory func(bearerToken string) *HarnessClient

// NewHTTPHandler serves the MCP JSON-RPC protocol over HTTP using the same tool
// definitions and handlers as the stdio server.
//
// Before this, harnessd exposed a second, independent delegation API at /mcp with
// the same tool names and different schemas — its start_run took a prompt and
// nothing else, so a caller could not even choose a model. Two implementations of
// one protocol drift, and a fix or a restriction added to one silently misses the
// other (issue #1317). There is now one dispatcher behind two transports.
func NewHTTPHandler(newClient ClientFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			writeRPC(w, errorResponse(nil, -32700, "Parse error"))
			return
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			writeRPC(w, errorResponse(nil, -32700, "Parse error"))
			return
		}
		if req.Method == "" {
			writeRPC(w, errorResponse(req.ID, -32600, "Invalid Request"))
			return
		}

		d := NewDispatcher(newClient(bearerToken(r)), RealClock{})
		resp, shouldRespond := d.Dispatch(r.Context(), req)
		if !shouldRespond {
			// A notification: accepted, nothing to return.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeRPC(w, resp)
	})
}

// bearerToken extracts the caller's token, or "" when the daemon has auth
// disabled and the caller sent none.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return ""
}

// writeRPC marshals a JSON-RPC response.
func writeRPC(w http.ResponseWriter, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
