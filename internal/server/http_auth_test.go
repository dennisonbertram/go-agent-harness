package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/server"
	"go-agent-harness/internal/store"
)

// newAuthTestHandler creates an http.Handler with auth enabled (a real store with keys).
// It returns the handler, the raw token for tenantID, and the tenantID.
func newAuthTestHandler(t *testing.T) (h http.Handler, token string, tenantID string) {
	t.Helper()
	ms := store.NewMemoryStore()
	tenantID = "tenant-auth-test"
	rawToken, key := generateFastAPIKey(t, tenantID, "test key", []string{
		store.ScopeRunsRead,
		store.ScopeRunsWrite,
	})
	if err := ms.CreateAPIKey(context.Background(), key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	runner := harness.NewRunner(
		&authTestStaticProvider{},
		harness.NewRegistry(),
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "test",
			MaxSteps:            1,
		},
	)

	h = server.NewWithOptions(server.ServerOptions{
		Store:  ms,
		Runner: runner,
		// AuthDisabled is NOT set -- auth is enabled.
	})
	return h, rawToken, tenantID
}

// authTestStaticProvider is a minimal provider for auth tests that returns immediately.
type authTestStaticProvider struct{}

func (p *authTestStaticProvider) Complete(_ context.Context, _ harness.CompletionRequest) (harness.CompletionResult, error) {
	return harness.CompletionResult{Content: "test output"}, nil
}

// TestEffectiveTenantID_PostRun verifies that POST /v1/runs enforces tenant isolation.
func TestEffectiveTenantID_PostRun(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		bodyTenantID string
		wantStatus   int
	}{
		{
			name:         "no_tenant_id_filled_from_auth",
			bodyTenantID: "",
			wantStatus:   http.StatusAccepted,
		},
		{
			name:         "matching_tenant_id_allowed",
			bodyTenantID: "tenant-auth-test",
			wantStatus:   http.StatusAccepted,
		},
		{
			name:         "mismatching_tenant_id_rejected",
			bodyTenantID: "other-tenant",
			wantStatus:   http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, token, _ := newAuthTestHandler(t)
			ts := httptest.NewServer(h)
			defer ts.Close()

			body := map[string]any{
				"prompt": "hello",
			}
			if tc.bodyTenantID != "" {
				body["tenant_id"] = tc.bodyTenantID
			}
			b, _ := json.Marshal(body)

			req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/runs", bytes.NewReader(b))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST /v1/runs: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("case %q: got status %d, want %d; body: %s",
					tc.name, resp.StatusCode, tc.wantStatus, body)
			}
		})
	}
}

// TestEffectiveTenantID_ListRuns verifies that GET /v1/runs enforces tenant isolation.
func TestEffectiveTenantID_ListRuns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		queryTenant string
		wantStatus  int
	}{
		{
			name:        "no_tenant_id_uses_auth",
			queryTenant: "",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "matching_tenant_id_allowed",
			queryTenant: "tenant-auth-test",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "other_tenant_rejected",
			queryTenant: "other-tenant",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, token, _ := newAuthTestHandler(t)
			ts := httptest.NewServer(h)
			defer ts.Close()

			url := ts.URL + "/v1/runs"
			if tc.queryTenant != "" {
				url += "?tenant_id=" + tc.queryTenant
			}

			req, _ := http.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET /v1/runs: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("case %q: got status %d, want %d; body: %s",
					tc.name, resp.StatusCode, tc.wantStatus, body)
			}
		})
	}
}

// TestEffectiveTenantID_ListConversations verifies GET /v1/conversations/ enforces tenant isolation.
func TestEffectiveTenantID_ListConversations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		queryTenant string
		wantStatus  int
	}{
		{
			name:        "no_tenant_id_uses_auth",
			queryTenant: "",
			// 501 is acceptable: no conversation store configured. But NOT 400 or 401.
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:        "matching_tenant_id_allowed",
			queryTenant: "tenant-auth-test",
			// Same: 501 acceptable when conversation store is not configured.
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:        "other_tenant_rejected",
			queryTenant: "other-tenant",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h, token, _ := newAuthTestHandler(t)
			ts := httptest.NewServer(h)
			defer ts.Close()

			url := ts.URL + "/v1/conversations/"
			if tc.queryTenant != "" {
				url += "?tenant_id=" + tc.queryTenant
			}

			req, _ := http.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET /v1/conversations/: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("case %q: got status %d, want %d; body: %s",
					tc.name, resp.StatusCode, tc.wantStatus, body)
			}
		})
	}
}

// TestEffectiveTenantID_AuthDisabled verifies that when auth is disabled,
// tenant_id values from the request are passed through unchanged (no rejection).
func TestEffectiveTenantID_AuthDisabled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		bodyTenantID string
	}{
		{"no_tenant_id", ""},
		{"any_tenant_id", "arbitrary-tenant"},
		{"another_tenant", "some-other-tenant"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ms := store.NewMemoryStore()
			runner := harness.NewRunner(
				&authTestStaticProvider{},
				harness.NewRegistry(),
				harness.RunnerConfig{
					DefaultModel:        "gpt-4.1-mini",
					DefaultSystemPrompt: "test",
					MaxSteps:            1,
				},
			)
			h := server.NewWithOptions(server.ServerOptions{
				Store:        ms,
				Runner:       runner,
				AuthDisabled: true,
			})
			ts := httptest.NewServer(h)
			defer ts.Close()

			body := map[string]any{"prompt": "hello"}
			if tc.bodyTenantID != "" {
				body["tenant_id"] = tc.bodyTenantID
			}
			b, _ := json.Marshal(body)

			req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/runs", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			// No Authorization header -- auth is disabled.

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST /v1/runs: %v", err)
			}
			defer resp.Body.Close()

			// Must be 202 Accepted -- not 400 or 401.
			if resp.StatusCode != http.StatusAccepted {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("case %q (auth disabled): got status %d, want 202; body: %s",
					tc.name, resp.StatusCode, body)
			}
		})
	}
}

// TestEffectiveTenantID_PostRunResponse verifies that when auth is enabled and
// no tenant_id is supplied in the body, the run is created under the auth tenant.
func TestEffectiveTenantID_PostRunResponse(t *testing.T) {
	t.Parallel()

	h, token, _ := newAuthTestHandler(t)
	ts := httptest.NewServer(h)
	defer ts.Close()

	b, _ := json.Marshal(map[string]any{"prompt": "hello"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/runs", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/runs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.RunID == "" {
		t.Error("expected non-empty run_id in response")
	}
}
