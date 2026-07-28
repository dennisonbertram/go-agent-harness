package deferred_test

// Execution-path tests that existed only in the deleted duplicate tool package.
//
// The LSP case runs the real tool against a fake `gopls` on PATH — the only
// test in the tree that exercises those tools' exec path rather than just
// their definitions.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/harness/tools/deferred"
)

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

	opts := tools.BuildOptions{WorkspaceRoot: workspace}
	diag := deferred.LspDiagnosticsTool(opts)
	out, err := diag.Handler(context.Background(), json.RawMessage(`{"file_path":"a.go"}`))
	if err != nil {
		t.Fatalf("lsp_diagnostics success branch failed: %v", err)
	}
	if !strings.Contains(out, "diagnostics") {
		t.Fatalf("expected diagnostics output, got %s", out)
	}

	refs := deferred.LspReferencesTool(opts)
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

func TestSourcegraphTool_MissingEndpointErrors(t *testing.T) {

	tool := deferred.SourcegraphTool(tools.BuildOptions{HTTPClient: http.DefaultClient})
	if _, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"x"}`)); err == nil {
		t.Fatalf("expected missing endpoint error")
	}
}
