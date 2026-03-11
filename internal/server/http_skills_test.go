package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/harness/tools"
)

// mockSkillManager is a simple in-memory mock for SkillManager.
type mockSkillManager struct {
	mu     sync.RWMutex
	skills map[string]tools.SkillInfo
}

func newMockSkillManager(initial ...tools.SkillInfo) *mockSkillManager {
	m := &mockSkillManager{
		skills: make(map[string]tools.SkillInfo),
	}
	for _, s := range initial {
		m.skills[s.Name] = s
	}
	return m
}

func (m *mockSkillManager) GetSkill(name string) (tools.SkillInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[name]
	return s, ok
}

func (m *mockSkillManager) ListSkills() []tools.SkillInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]tools.SkillInfo, 0, len(m.skills))
	for _, s := range m.skills {
		list = append(list, s)
	}
	return list
}

func (m *mockSkillManager) ResolveSkill(_ context.Context, _, _, _ string) (string, error) {
	return "", fmt.Errorf("not implemented in mock")
}

func (m *mockSkillManager) GetSkillFilePath(name string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[name]
	if !ok {
		return "", false
	}
	fp := s.FilePath
	if fp == "" {
		fp = "/skills/" + name + "/SKILL.md"
	}
	return fp, true
}

func (m *mockSkillManager) UpdateSkillVerification(_ context.Context, name string, verified bool, verifiedAt time.Time, verifiedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found", name)
	}
	s.Verified = verified
	s.VerifiedAt = verifiedAt.UTC().Format(time.RFC3339)
	s.VerifiedBy = verifiedBy
	m.skills[name] = s
	return nil
}

// testRunnerForSkills builds a minimal runner for skill HTTP handler tests.
func testRunnerForSkills(t *testing.T) *harness.Runner {
	t.Helper()
	registry := harness.NewRegistry()
	return harness.NewRunner(
		&staticProvider{result: harness.CompletionResult{Content: "done"}},
		registry,
		harness.RunnerConfig{
			DefaultModel:        "gpt-4.1-mini",
			DefaultSystemPrompt: "You are helpful.",
			MaxSteps:            1,
		},
	)
}

// skillsTestServer builds a test server with a mock skill manager.
func skillsTestServer(t *testing.T, manager SkillManager) *httptest.Server {
	t.Helper()
	runner := testRunnerForSkills(t)
	s := NewWithSkills(runner, nil, manager)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// sampleSkills returns a small set of sample SkillInfo objects for testing.
func sampleSkills() []tools.SkillInfo {
	return []tools.SkillInfo{
		{
			Name:        "code-review",
			Description: "Reviews code for quality issues",
			Source:      "local",
			Verified:    false,
		},
		{
			Name:        "deploy",
			Description: "Deploys an application",
			Source:      "global",
			Verified:    true,
			VerifiedAt:  "2026-01-01T00:00:00Z",
			VerifiedBy:  "admin",
		},
	}
}

// TestSkillsList_Returns200WithList verifies GET /v1/skills returns a list.
func TestSkillsList_Returns200WithList(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	res, err := http.Get(ts.URL + "/v1/skills")
	if err != nil {
		t.Fatalf("GET /v1/skills: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var resp struct {
		Skills []tools.SkillInfo `json:"skills"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(resp.Skills))
	}
}

// TestSkillsList_MethodNotAllowed verifies POST /v1/skills returns 405.
func TestSkillsList_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	res, err := http.Post(ts.URL+"/v1/skills", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /v1/skills: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}

// TestSkillGetByName_Returns200 verifies GET /v1/skills/{name} returns the skill.
func TestSkillGetByName_Returns200(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	res, err := http.Get(ts.URL + "/v1/skills/code-review")
	if err != nil {
		t.Fatalf("GET /v1/skills/code-review: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var skill tools.SkillInfo
	if err := json.NewDecoder(res.Body).Decode(&skill); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if skill.Name != "code-review" {
		t.Errorf("expected name code-review, got %q", skill.Name)
	}
}

// TestSkillGetByName_Returns404ForUnknown verifies 404 for an unknown skill name.
func TestSkillGetByName_Returns404ForUnknown(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	res, err := http.Get(ts.URL + "/v1/skills/does-not-exist")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", res.StatusCode)
	}
}

// TestSkillVerify_Returns200AndSetsVerified verifies POST /v1/skills/{name}/verify.
func TestSkillVerify_Returns200AndSetsVerified(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	// code-review starts unverified.
	body := `{"verified_by":"ci-system"}`
	res, err := http.Post(ts.URL+"/v1/skills/code-review/verify", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST verify: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(b))
	}

	var skill tools.SkillInfo
	if err := json.NewDecoder(res.Body).Decode(&skill); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !skill.Verified {
		t.Error("expected verified=true after verification")
	}
	if skill.VerifiedBy != "ci-system" {
		t.Errorf("expected verified_by ci-system, got %q", skill.VerifiedBy)
	}
}

// TestSkillVerify_Returns404ForUnknown verifies 404 for unknown skill.
func TestSkillVerify_Returns404ForUnknown(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	res, err := http.Post(ts.URL+"/v1/skills/ghost/verify", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", res.StatusCode)
	}
}

// TestSkillVerify_DefaultsVerifiedByToApi verifies verified_by defaults to "api" when omitted.
func TestSkillVerify_DefaultsVerifiedByToApi(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	// No body provided — verified_by should default to "api".
	res, err := http.Post(ts.URL+"/v1/skills/code-review/verify", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(b))
	}

	var skill tools.SkillInfo
	if err := json.NewDecoder(res.Body).Decode(&skill); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if skill.VerifiedBy != "api" {
		t.Errorf("expected verified_by api, got %q", skill.VerifiedBy)
	}
}

// TestSkillEndpoints_Return501WhenNotConfigured verifies all skills endpoints return 501 when skills is nil.
func TestSkillEndpoints_Return501WhenNotConfigured(t *testing.T) {
	t.Parallel()

	runner := testRunnerForSkills(t)
	// Use NewWithSkills with nil skills manager — all skills endpoints should 501.
	s := NewWithSkills(runner, nil, nil)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/v1/skills", ""},
		{http.MethodGet, "/v1/skills/code-review", ""},
		{http.MethodPost, "/v1/skills/code-review/verify", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			// NOTE: subtests must NOT be parallel here: parent's t.Cleanup
			// runs after all subtests complete, keeping the server alive.
			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = bytes.NewBufferString(tc.body)
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, bodyReader)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusNotImplemented {
				body, _ := io.ReadAll(res.Body)
				t.Errorf("expected 501, got %d: %s", res.StatusCode, string(body))
			}
		})
	}
}

// TestSkillVerify_MethodNotAllowed verifies non-POST on verify returns 405.
func TestSkillVerify_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager(sampleSkills()...)
	ts := skillsTestServer(t, manager)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/skills/code-review/verify", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET verify: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}

// TestSkillsList_EmptyListWhenNoSkills verifies empty list is returned, not null.
func TestSkillsList_EmptyListWhenNoSkills(t *testing.T) {
	t.Parallel()

	manager := newMockSkillManager() // empty
	ts := skillsTestServer(t, manager)

	res, err := http.Get(ts.URL + "/v1/skills")
	if err != nil {
		t.Fatalf("GET /v1/skills: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}

	var resp struct {
		Skills []tools.SkillInfo `json:"skills"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(resp.Skills))
	}
}
