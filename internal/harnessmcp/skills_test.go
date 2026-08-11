package harnessmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestListSkillsTool — a delegate can invoke a skill by name, and the caller
// writing its prompt had no way to see which exist (issue #1324).
func TestListSkillsTool(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"skills":[
		  {"name":"deploy","description":"Ship a release","source":"global","context":"conversation","file_path":"/home/u/.go-harness/skills/deploy/SKILL.md"}
		]}`))
	}))
	defer srv.Close()

	res, err := newListSkillsHandler(NewHarnessClient(srv.URL))(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("list_skills: %v", err)
	}
	if gotPath != "/v1/skills" {
		t.Errorf("hit %q, want /v1/skills", gotPath)
	}

	var out struct {
		Skills []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Source      string `json:"source"`
			FilePath    string `json:"file_path"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Skills) != 1 || out.Skills[0].Name != "deploy" {
		t.Fatalf("unexpected skills: %+v", out.Skills)
	}
	// A filesystem path is of no use to a remote caller and leaks the daemon
	// host's layout, so it is projected away.
	if out.Skills[0].FilePath != "" {
		t.Errorf("file_path must not be exposed, got %q", out.Skills[0].FilePath)
	}
	if out.Skills[0].Description == "" {
		t.Error("description is what makes a skill selectable; it must survive")
	}
}

// TestListSkillsAdvertisedAndDispatchable — advertised and callable.
func TestListSkillsAdvertisedAndDispatchable(t *testing.T) {
	advertised := false
	for _, tool := range toolDefs() {
		if tool.Name == "list_skills" {
			advertised = true
		}
	}
	if !advertised {
		t.Error("list_skills is not advertised")
	}
	d := NewDispatcher(NewHarnessClient("http://127.0.0.1:1"), RealClock{})
	if _, ok := d.tools["list_skills"]; !ok {
		t.Error("list_skills has no handler")
	}
}

// TestEveryToolNameIsUnique — two definitions of one name would make the
// advertised schema and the dispatched handler disagree, the exact class of bug
// that made the two MCP surfaces drift.
func TestEveryToolNameIsUnique(t *testing.T) {
	seen := map[string]int{}
	for _, tool := range toolDefs() {
		seen[tool.Name]++
	}
	for name, n := range seen {
		if n > 1 {
			t.Errorf("tool %q is defined %d times", name, n)
		}
	}
	for _, tool := range toolDefs() {
		if strings.TrimSpace(tool.Description) == "" {
			t.Errorf("tool %q has no description; a caller cannot choose it", tool.Name)
		}
	}
}
