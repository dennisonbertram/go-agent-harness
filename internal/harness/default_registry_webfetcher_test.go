package harness

// GAP-2/GAP-3 wiring proof at the production registry layer
// (NewDefaultRegistryWithOptions, tools_default.go).
//
// Before this change, DefaultRegistryOptions had no WebFetcher field at all,
// so web_fetch/web_search/agentic_fetch could never be registered by the
// production HTTP runtime (cmd/harnessd) regardless of any guard — the
// WebFetcher-gated branch in tools_default.go was permanently dead code
// there. This test proves the newly-added field both (a) actually reaches
// the registered web_fetch tool, and (b) is guarded by construction: an
// unguarded, real-HTTP WebFetcher test double supplied via
// DefaultRegistryOptions.WebFetcher still cannot reach a loopback
// destination through web_fetch.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
)

// stubSkillVerifierForRegistryTest satisfies both SkillLister and
// SkillVerifier so the conditional registration of verify_skill can be
// exercised. It reports one skill so the skill tool is registered too.
type stubSkillVerifierForRegistryTest struct{}

func (stubSkillVerifierForRegistryTest) GetSkill(string) (htools.SkillInfo, bool) {
	return htools.SkillInfo{}, false
}

func (stubSkillVerifierForRegistryTest) ListSkills() []htools.SkillInfo {
	return []htools.SkillInfo{{Name: "demo", Description: "a demo skill"}}
}

func (stubSkillVerifierForRegistryTest) ResolveSkill(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (stubSkillVerifierForRegistryTest) GetSkillFilePath(string) (string, bool) {
	return "", false
}

func (stubSkillVerifierForRegistryTest) UpdateSkillVerification(context.Context, string, bool, time.Time, string) error {
	return nil
}

type fakeAgentRunnerForWebTest struct{}

func (fakeAgentRunnerForWebTest) RunPrompt(_ context.Context, prompt string) (string, error) {
	return "ran: " + prompt, nil
}

// realHTTPWebFetcherForRegistryTest performs a real, unguarded HTTP fetch —
// the shape any actual production WebFetcher implementation would take.
type realHTTPWebFetcherForRegistryTest struct {
	client *http.Client
}

func (f *realHTTPWebFetcherForRegistryTest) Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (f *realHTTPWebFetcherForRegistryTest) Search(_ context.Context, query string, maxResults int) ([]map[string]any, error) {
	return []map[string]any{{"query": query, "max": maxResults}}, nil
}

func TestNewDefaultRegistryWithOptions_WebFetchTool_RefusesLoopbackByDefault(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("internal secret"))
	}))
	defer srv.Close()

	workspace := t.TempDir()
	registry := NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		AgentRunner:  fakeAgentRunnerForWebTest{},
		WebFetcher:   &realHTTPWebFetcherForRegistryTest{client: &http.Client{Timeout: 5 * time.Second}},
	})

	args, _ := json.Marshal(map[string]any{"url": srv.URL})
	out, err := registry.Execute(context.Background(), "web_fetch", args)
	if err == nil && !strings.Contains(out, "\"error\"") {
		t.Fatalf("expected web_fetch (built by NewDefaultRegistryWithOptions) to refuse a loopback destination, but it succeeded: err=%v out=%s", err, out)
	}
}

// TestNewDefaultRegistryWithOptions_AgenticFetchTool_RefusesLoopbackByDefault
// proves agentic_fetch — which also calls WebFetcher.Fetch with an
// agent-supplied URL — is guarded the same way as web_fetch.
//
// Ported from the deleted duplicate tool catalog's test suite, which was the
// only place this case was covered. The catalog it exercised no longer exists;
// the guarantee still has to hold, so the test now runs against the surviving
// registry.
func TestNewDefaultRegistryWithOptions_AgenticFetchTool_RefusesLoopbackByDefault(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("internal secret"))
	}))
	defer srv.Close()

	registry := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		AgentRunner:  fakeAgentRunnerForWebTest{},
		WebFetcher:   &realHTTPWebFetcherForRegistryTest{client: &http.Client{Timeout: 5 * time.Second}},
	})

	args, _ := json.Marshal(map[string]any{"url": srv.URL, "prompt": "summarize"})
	out, err := registry.Execute(context.Background(), "agentic_fetch", args)
	if err == nil && !strings.Contains(out, "\"error\"") {
		t.Fatalf("expected agentic_fetch to refuse a loopback destination, but it succeeded: err=%v out=%s", err, out)
	}
}

// TestNewDefaultRegistryWithOptions_VerifySkillRegistration pins the
// conditional registration of verify_skill: present when a SkillVerifier is
// configured, absent when it is nil. Ported from the deleted duplicate
// catalog's test suite, which was the only place this was covered.
func TestNewDefaultRegistryWithOptions_VerifySkillRegistration(t *testing.T) {
	t.Parallel()

	hasVerifySkill := func(registry *Registry) bool {
		for _, def := range registry.Definitions() {
			if def.Name == "verify_skill" {
				return true
			}
		}
		return false
	}

	verifier := &stubSkillVerifierForRegistryTest{}
	withVerifier := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode:  ToolApprovalModeFullAuto,
		SkillLister:   verifier,
		SkillVerifier: verifier,
	})
	if !hasVerifySkill(withVerifier) {
		t.Error("expected verify_skill to be registered when a SkillVerifier is configured")
	}

	withoutVerifier := NewDefaultRegistryWithOptions(t.TempDir(), DefaultRegistryOptions{
		ApprovalMode: ToolApprovalModeFullAuto,
		SkillLister:  verifier,
	})
	if hasVerifySkill(withoutVerifier) {
		t.Error("verify_skill must not be registered when SkillVerifier is nil")
	}
}

// TestNewDefaultRegistryWithOptions_WebFetchTool_AllowlistPermitsExplicitHost
// proves the operator-configured NetworkAllowlist (GAP-3) actually reaches
// the WebFetcher-backed tools built by NewDefaultRegistryWithOptions, not
// just the download tool.
func TestNewDefaultRegistryWithOptions_WebFetchTool_AllowlistPermitsExplicitHost(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	host := strings.TrimPrefix(strings.TrimPrefix(srv.URL, "http://"), "https://")
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// keep host:port as-is; the allowlist below uses the bare host, so
		// strip the port for the hostname-match branch of the guard.
		host = host[:idx]
	}

	workspace := t.TempDir()
	registry := NewDefaultRegistryWithOptions(workspace, DefaultRegistryOptions{
		ApprovalMode:     ToolApprovalModeFullAuto,
		AgentRunner:      fakeAgentRunnerForWebTest{},
		WebFetcher:       &realHTTPWebFetcherForRegistryTest{client: &http.Client{Timeout: 5 * time.Second}},
		NetworkAllowlist: []string{host},
	})

	args, _ := json.Marshal(map[string]any{"url": srv.URL})
	out, err := registry.Execute(context.Background(), "web_fetch", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected allowlisted host to be reachable and return its content, got: %s", out)
	}
}
