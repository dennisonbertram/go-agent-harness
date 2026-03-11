package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// sourcegraphConfig holds connection details for the Sourcegraph proxy endpoint.
// It is defined locally to avoid an import cycle with the tools package.
type sourcegraphConfig struct {
	Endpoint string
	Token    string
}

// handleSearchCode handles POST /v1/search/code.
// It proxies the request to a configured Sourcegraph instance.
func (s *Server) handleSearchCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	if strings.TrimSpace(s.sourcegraph.Endpoint) == "" {
		writeError(w, http.StatusNotImplemented, "not_configured", "sourcegraph not configured")
		return
	}

	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "query is required")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 200 {
		req.Limit = 200
	}

	// Build the upstream request payload.
	payload := map[string]any{
		"query": req.Query,
		"count": req.Limit,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("marshal request: %s", err))
		return
	}

	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.sourcegraph.Endpoint, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("build request: %s", err))
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(s.sourcegraph.Token) != "" {
		upstream.Header.Set("Authorization", "token "+s.sourcegraph.Token)
	}

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(upstream)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", fmt.Sprintf("sourcegraph request failed: %s", err))
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", fmt.Sprintf("read response: %s", err))
		return
	}

	// Attempt to parse and reformat the upstream response into our schema.
	// If parsing fails, wrap the raw response.
	var parsed struct {
		Results []struct {
			Repository string `json:"repository"`
			File       string `json:"file"`
			Line       int    `json:"line"`
			Content    string `json:"content"`
		} `json:"results"`
	}

	if err := json.Unmarshal(respBody, &parsed); err == nil && parsed.Results != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"count":   len(parsed.Results),
			"results": parsed.Results,
		})
		return
	}

	// Upstream returned something we can't parse — surface as raw response.
	writeJSON(w, http.StatusOK, map[string]any{
		"count":    0,
		"results":  []any{},
		"raw":      string(respBody),
		"upstream": resp.StatusCode,
	})
}
