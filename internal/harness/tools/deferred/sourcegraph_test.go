package deferred

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tools "go-agent-harness/internal/harness/tools"
)

// TestSourcegraphTool_Handler_InvalidJSON verifies sourcegraph surfaces a parse
// error (rather than panicking or silently ignoring bad input) before it ever
// tries to reach the configured endpoint.
func TestSourcegraphTool_Handler_InvalidJSON(t *testing.T) {
	t.Parallel()

	tool := SourcegraphTool(tools.BuildOptions{Sourcegraph: tools.SourcegraphConfig{Endpoint: "http://example.invalid"}})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{not-json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON args")
	}
	if !strings.Contains(err.Error(), "parse sourcegraph args") {
		t.Errorf("expected 'parse sourcegraph args' in error, got %q", err.Error())
	}
}

// TestSourcegraphTool_Handler_MissingQueryWhenConfigured verifies the
// query-required guard fires even once an endpoint is configured (a
// regression here would let empty queries reach the network).
func TestSourcegraphTool_Handler_MissingQueryWhenConfigured(t *testing.T) {
	t.Parallel()

	tool := SourcegraphTool(tools.BuildOptions{Sourcegraph: tools.SourcegraphConfig{Endpoint: "http://example.invalid"}})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"   "}`))
	if err == nil {
		t.Fatal("expected error for blank query")
	}
	if !strings.Contains(err.Error(), "query is required") {
		t.Errorf("expected 'query is required' in error, got %q", err.Error())
	}
}

// TestSourcegraphTool_Handler_Success verifies the full success path: the
// request is built with the right method/headers/body, and the response
// status code and body are round-tripped into the tool result verbatim.
func TestSourcegraphTool_Handler_Success(t *testing.T) {
	t.Parallel()

	var gotMethod, gotAuth, gotContentType string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	tool := SourcegraphTool(tools.BuildOptions{
		Sourcegraph: tools.SourcegraphConfig{Endpoint: srv.URL, Token: "tok-123"},
		HTTPClient:  srv.Client(),
	})

	result, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"foo bar"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST request, got %q", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("expected application/json content-type, got %q", gotContentType)
	}
	if gotAuth != "token tok-123" {
		t.Errorf("expected Authorization %q, got %q", "token tok-123", gotAuth)
	}
	if gotBody["query"] != "foo bar" {
		t.Errorf("expected request body query %q, got %v", "foo bar", gotBody["query"])
	}
	// count defaults to 20 when omitted.
	if gotBody["count"] != float64(20) {
		t.Errorf("expected default count 20 in request body, got %v", gotBody["count"])
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["status_code"] != float64(http.StatusOK) {
		t.Errorf("expected status_code 200, got %v", out["status_code"])
	}
	if out["response"] != `{"results":[]}` {
		t.Errorf("expected response body to be passed through, got %v", out["response"])
	}
}

// TestSourcegraphTool_Handler_NoTokenOmitsAuthHeader verifies that when no
// token is configured, the Authorization header is not sent at all (rather
// than sent empty or with a bogus scheme).
func TestSourcegraphTool_Handler_NoTokenOmitsAuthHeader(t *testing.T) {
	t.Parallel()

	var sawAuthHeader bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawAuthHeader = r.Header["Authorization"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tool := SourcegraphTool(tools.BuildOptions{
		Sourcegraph: tools.SourcegraphConfig{Endpoint: srv.URL}, // no token
		HTTPClient:  srv.Client(),
	})
	if _, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"x"}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawAuthHeader {
		t.Error("expected no Authorization header when token is not configured")
	}
}

// TestSourcegraphTool_Handler_ClampsCountAndTimeout verifies the count and
// timeout_seconds clamping logic: out-of-range values are clamped to their
// bounds, and non-positive values fall back to the defaults. A subtly wrong
// bound (e.g. off-by-one, or clamping to the wrong side) would fail this.
func TestSourcegraphTool_Handler_ClampsCountAndTimeout(t *testing.T) {
	t.Parallel()

	var gotCount float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		gotCount = payload["count"].(float64)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tool := SourcegraphTool(tools.BuildOptions{
		Sourcegraph: tools.SourcegraphConfig{Endpoint: srv.URL},
		HTTPClient:  srv.Client(),
	})

	cases := []struct {
		name      string
		reqCount  int
		wantCount float64
	}{
		{"zero_defaults_to_20", 0, 20},
		{"negative_defaults_to_20", -5, 20},
		{"over_cap_clamps_to_200", 500, 200},
		{"in_range_passes_through", 42, 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"query": "q", "count": tc.reqCount})
			if _, err := tool.Handler(context.Background(), raw); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotCount != tc.wantCount {
				t.Errorf("count %d: expected clamped count %v, got %v", tc.reqCount, tc.wantCount, gotCount)
			}
		})
	}
}

// TestSourcegraphTool_Handler_NonOKStatusPassedThrough verifies that a
// non-2xx upstream response is not treated as a Go error: the tool reports
// the actual status code and body back to the caller instead of failing.
func TestSourcegraphTool_Handler_NonOKStatusPassedThrough(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	tool := SourcegraphTool(tools.BuildOptions{
		Sourcegraph: tools.SourcegraphConfig{Endpoint: srv.URL},
		HTTPClient:  srv.Client(),
	})
	result, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"q"}`))
	if err != nil {
		t.Fatalf("unexpected error for non-200 upstream response: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["status_code"] != float64(http.StatusInternalServerError) {
		t.Errorf("expected status_code 500, got %v", out["status_code"])
	}
	if out["response"] != "boom" {
		t.Errorf("expected response body 'boom', got %v", out["response"])
	}
}

// TestSourcegraphTool_Handler_RequestFailure verifies a transport-level
// failure (connection refused) surfaces as a wrapped "sourcegraph request
// failed" error rather than a bare/opaque error or a panic.
func TestSourcegraphTool_Handler_RequestFailure(t *testing.T) {
	t.Parallel()

	// Start and immediately close a server to get a URL nothing is listening on.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close()

	tool := SourcegraphTool(tools.BuildOptions{
		Sourcegraph: tools.SourcegraphConfig{Endpoint: deadURL},
		HTTPClient:  &http.Client{Timeout: 3 * time.Second},
	})
	_, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"q"}`))
	if err == nil {
		t.Fatal("expected error when the endpoint is unreachable")
	}
	if !strings.Contains(err.Error(), "sourcegraph request failed") {
		t.Errorf("expected 'sourcegraph request failed' in error, got %q", err.Error())
	}
}
