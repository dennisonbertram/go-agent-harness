package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type errPolicy struct{}

func (e errPolicy) Allow(_ context.Context, _ PolicyInput) (PolicyDecision, error) {
	return PolicyDecision{}, errors.New("policy boom")
}

func TestPolicyBranchesAndValidationErrors(t *testing.T) {
	workspace := t.TempDir()
	list, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace, ApprovalMode: ApprovalModePermissions, Policy: nil})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	write := findToolByName(t, list, "write")
	out, err := write.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","content":"x"}`))
	if err != nil {
		t.Fatalf("expected structured deny output, got err: %v", err)
	}
	if !strings.Contains(out, "permission_denied") {
		t.Fatalf("expected permission_denied output, got %s", out)
	}

	list2, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace, ApprovalMode: ApprovalModePermissions, Policy: errPolicy{}})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	write = findToolByName(t, list2, "write")
	out, err = write.Handler(context.Background(), json.RawMessage(`{"path":"a.txt","content":"x"}`))
	if err != nil {
		t.Fatalf("expected structured policy_error output, got err: %v", err)
	}
	if !strings.Contains(out, "permission_error") {
		t.Fatalf("expected permission_error output, got %s", out)
	}

	fullAuto, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace, ApprovalMode: ApprovalModeFullAuto})
	if err != nil {
		t.Fatalf("BuildCatalog full auto: %v", err)
	}
	fetch := findToolByName(t, fullAuto, "fetch")
	if _, err := fetch.Handler(context.Background(), json.RawMessage(`{"url":"ftp://x"}`)); err == nil {
		t.Fatalf("expected invalid fetch scheme error")
	}
	download := findToolByName(t, fullAuto, "download")
	if _, err := download.Handler(context.Background(), json.RawMessage(`{"url":"http://x"}`)); err == nil {
		t.Fatalf("expected missing file_path error")
	}
}

func TestApplyPatchFindReplaceBranches(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "p.txt"), []byte("a\na\n"), 0o644); err != nil {
		t.Fatalf("write p.txt: %v", err)
	}
	list, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	patch := findToolByName(t, list, "apply_patch")

	if _, err := patch.Handler(context.Background(), json.RawMessage(`{"path":"p.txt"}`)); err == nil {
		t.Fatalf("expected missing find error")
	}
	if _, err := patch.Handler(context.Background(), json.RawMessage(`{"path":"p.txt","find":"missing","replace":"x"}`)); err == nil {
		t.Fatalf("expected not present error")
	}

	out, err := patch.Handler(context.Background(), json.RawMessage(`{"path":"p.txt","find":"a","replace":"A","replace_all":true}`))
	if err != nil {
		t.Fatalf("replace_all patch failed: %v", err)
	}
	if !strings.Contains(out, `"replacements":2`) {
		t.Fatalf("expected 2 replacements, got %s", out)
	}
}

func TestWriteMissingExpectedVersionBranch(t *testing.T) {
	workspace := t.TempDir()
	list, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	write := findToolByName(t, list, "write")
	out, err := write.Handler(context.Background(), json.RawMessage(`{"path":"missing.txt","content":"x","expected_version":"abc"}`))
	if err != nil {
		t.Fatalf("expected stale_write output, got err: %v", err)
	}
	if !strings.Contains(out, `"stale_write"`) {
		t.Fatalf("expected stale_write output, got %s", out)
	}
}

func TestJobManagerCleanupAndResolveDirBranches(t *testing.T) {
	workspace := t.TempDir()
	mgr := NewJobManager(workspace, time.Now)
	mgr.maxJobs = 0
	if _, err := mgr.runBackground("echo hi", 1, "."); err == nil {
		t.Fatalf("expected max job limit error")
	}

	mgr2 := NewJobManager(workspace, func() time.Time { return time.Unix(1000, 0) })
	mgr2.ttl = 0
	mgr2.maxJobs = 2
	_, err := mgr2.runBackground("echo hi", 1, ".")
	if err != nil {
		t.Fatalf("runBackground: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	mgr2.cleanupExpired()
	if _, err := resolveWorkingDir(workspace, "nested"); err != nil {
		// nested may not exist, but path resolution should still be inside workspace.
		if !strings.Contains(err.Error(), "escapes") && !strings.Contains(err.Error(), "absolute") {
			t.Fatalf("unexpected resolveWorkingDir error: %v", err)
		}
	}
}

func TestLSPSuccessAndErrorBranchesWithFakeGopls(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	binDir := t.TempDir()
	script := filepath.Join(binDir, "gopls")
	scriptContent := "#!/bin/bash\nif [ \"$1\" = \"workspace_symbol\" ]; then echo refs; exit 0; fi\nif [ \"$1\" = \"check\" ]; then echo diagnostics; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("write fake gopls: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	list, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace, EnableLSP: true})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	diag := findToolByName(t, list, "lsp_diagnostics")
	out, err := diag.Handler(context.Background(), json.RawMessage(`{"file_path":"a.go"}`))
	if err != nil {
		t.Fatalf("lsp_diagnostics success branch failed: %v", err)
	}
	if !strings.Contains(out, "diagnostics") {
		t.Fatalf("expected diagnostics output, got %s", out)
	}

	refs := findToolByName(t, list, "lsp_references")
	out, err = refs.Handler(context.Background(), json.RawMessage(`{"symbol":"Main","path":"a.go"}`))
	if err != nil {
		t.Fatalf("lsp_references success branch failed: %v", err)
	}
	if !strings.Contains(out, "refs") {
		t.Fatalf("expected refs output, got %s", out)
	}

	// Force command failure path still returning JSON output.
	if err := os.WriteFile(script, []byte("#!/bin/bash\nexit 2\n"), 0o755); err != nil {
		t.Fatalf("overwrite fake gopls: %v", err)
	}
	out, err = refs.Handler(context.Background(), json.RawMessage(`{"symbol":"Main"}`))
	if err != nil {
		t.Fatalf("expected lsp_references failure branch as JSON output: %v", err)
	}
	if !strings.Contains(out, "\"exit_code\":1") {
		t.Fatalf("expected exit_code 1 output, got %s", out)
	}
}

func TestSourcegraphAndMCPAndAgentErrorBranches(t *testing.T) {
	workspace := t.TempDir()

	tool := sourcegraphTool(http.DefaultClient, SourcegraphConfig{})
	if _, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"x"}`)); err == nil {
		t.Fatalf("expected missing endpoint error")
	}

	mcp := &fakeMCP{}
	list, err := BuildCatalog(BuildOptions{WorkspaceRoot: workspace, EnableMCP: true, MCPRegistry: mcp, EnableAgent: true, AgentRunner: &fakeRunner{}, EnableWebOps: true, WebFetcher: &fakeWeb{}})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	mcpList := findToolByName(t, list, "list_mcp_resources")
	if _, err := mcpList.Handler(context.Background(), json.RawMessage(`{"mcp_name":"bad"}`)); err == nil {
		t.Fatalf("expected mcp list error branch")
	}

	mcpRead := findToolByName(t, list, "read_mcp_resource")
	if _, err := mcpRead.Handler(context.Background(), json.RawMessage(`{"mcp_name":"x","uri":"missing"}`)); err == nil {
		t.Fatalf("expected mcp read error branch")
	}

	agent := findToolByName(t, list, "agent")
	if _, err := agent.Handler(context.Background(), json.RawMessage(`{"prompt":"please fail"}`)); err == nil {
		t.Fatalf("expected agent runner error branch")
	}

	agentic := findToolByName(t, list, "agentic_fetch")
	if _, err := agentic.Handler(context.Background(), json.RawMessage(`{"prompt":"ok","url":"https://fail.example"}`)); err == nil {
		t.Fatalf("expected agentic fetch web error branch")
	}

	search := findToolByName(t, list, "web_search")
	if _, err := search.Handler(context.Background(), json.RawMessage(`{"query":"fail"}`)); err == nil {
		t.Fatalf("expected web search error branch")
	}

	fetch := findToolByName(t, list, "web_fetch")
	if _, err := fetch.Handler(context.Background(), json.RawMessage(`{"url":"https://fail.example"}`)); err == nil {
		t.Fatalf("expected web fetch error branch")
	}
}
