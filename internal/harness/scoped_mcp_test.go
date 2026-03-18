package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	htools "go-agent-harness/internal/harness/tools"
	"go-agent-harness/internal/mcp"
)

// mockGlobalMCPRegistry is a test MCPRegistry representing the global registry.
type mockGlobalMCPRegistry struct {
	tools     map[string][]htools.MCPToolDefinition
	resources map[string][]htools.MCPResource
	callLog   []string
	mu        sync.Mutex
}

func newMockGlobalMCPRegistry() *mockGlobalMCPRegistry {
	return &mockGlobalMCPRegistry{
		tools:     make(map[string][]htools.MCPToolDefinition),
		resources: make(map[string][]htools.MCPResource),
	}
}

func (m *mockGlobalMCPRegistry) ListTools(_ context.Context) (map[string][]htools.MCPToolDefinition, error) {
	return m.tools, nil
}

func (m *mockGlobalMCPRegistry) CallTool(_ context.Context, server, tool string, args json.RawMessage) (string, error) {
	m.mu.Lock()
	m.callLog = append(m.callLog, fmt.Sprintf("global:%s/%s", server, tool))
	m.mu.Unlock()
	return fmt.Sprintf("global-result:%s/%s", server, tool), nil
}

func (m *mockGlobalMCPRegistry) ListResources(_ context.Context, server string) ([]htools.MCPResource, error) {
	return m.resources[server], nil
}

func (m *mockGlobalMCPRegistry) ReadResource(_ context.Context, server, uri string) (string, error) {
	return fmt.Sprintf("global-resource:%s/%s", server, uri), nil
}

func TestScopedMCPRegistry_ListTools_UnionOfGlobalAndPerRun(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	global.tools["global-server"] = []htools.MCPToolDefinition{
		{Name: "gtool", Description: "global tool"},
	}

	// Create a per-run ClientManager with a test connection.
	cm := mcp.NewClientManager()
	perRunServerName := "perrun-server"
	err := cm.AddServerWithConn(perRunServerName, func() (mcp.Conn, error) {
		return newFakeMCPConn(perRunServerName, []mcp.ToolDef{
			{Name: "ptool", Description: "per-run tool"},
		}), nil
	})
	if err != nil {
		t.Fatalf("AddServerWithConn: %v", err)
	}

	scoped := NewScopedMCPRegistry(global, cm, []string{perRunServerName})
	defer scoped.Close()

	tools, err := scoped.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	// Should have both servers.
	if len(tools) != 2 {
		t.Errorf("expected 2 servers, got %d: %v", len(tools), tools)
	}
	if _, ok := tools["global-server"]; !ok {
		t.Error("missing global-server in tools map")
	}
	if _, ok := tools[perRunServerName]; !ok {
		t.Errorf("missing %s in tools map", perRunServerName)
	}
}

func TestScopedMCPRegistry_ListTools_PerRunServerShadowsGlobal(t *testing.T) {
	sharedName := "shared-server"

	global := newMockGlobalMCPRegistry()
	global.tools[sharedName] = []htools.MCPToolDefinition{
		{Name: "old-tool", Description: "global version"},
	}

	cm := mcp.NewClientManager()
	err := cm.AddServerWithConn(sharedName, func() (mcp.Conn, error) {
		return newFakeMCPConn(sharedName, []mcp.ToolDef{
			{Name: "new-tool", Description: "per-run version"},
		}), nil
	})
	if err != nil {
		t.Fatalf("AddServerWithConn: %v", err)
	}

	scoped := NewScopedMCPRegistry(global, cm, []string{sharedName})
	defer scoped.Close()

	tools, err := scoped.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	defs, ok := tools[sharedName]
	if !ok {
		t.Fatalf("missing %s in tools map", sharedName)
	}
	if len(defs) != 1 || defs[0].Name != "new-tool" {
		t.Errorf("expected per-run tool to shadow global; got %+v", defs)
	}
}

func TestScopedMCPRegistry_CallTool_RoutesToPerRun(t *testing.T) {
	global := newMockGlobalMCPRegistry()

	perRunName := "per-run-srv"
	cm := mcp.NewClientManager()
	err := cm.AddServerWithConn(perRunName, func() (mcp.Conn, error) {
		return newFakeMCPConn(perRunName, nil), nil
	})
	if err != nil {
		t.Fatalf("AddServerWithConn: %v", err)
	}

	scoped := NewScopedMCPRegistry(global, cm, []string{perRunName})
	defer scoped.Close()

	result, err := scoped.CallTool(context.Background(), perRunName, "any-tool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	// Should have gone to per-run, not global.
	if result == "" {
		t.Error("expected non-empty result from per-run server")
	}
	global.mu.Lock()
	if len(global.callLog) > 0 {
		t.Errorf("expected no calls to global; got %v", global.callLog)
	}
	global.mu.Unlock()
}

func TestScopedMCPRegistry_CallTool_RoutesToGlobal(t *testing.T) {
	global := newMockGlobalMCPRegistry()

	// Per-run has no servers.
	cm := mcp.NewClientManager()
	scoped := NewScopedMCPRegistry(global, cm, nil)
	defer scoped.Close()

	result, err := scoped.CallTool(context.Background(), "global-server", "gtool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result != "global-result:global-server/gtool" {
		t.Errorf("unexpected result %q", result)
	}
}

func TestScopedMCPRegistry_CallTool_UnknownServer_ReturnsError(t *testing.T) {
	cm := mcp.NewClientManager()
	scoped := NewScopedMCPRegistry(nil, cm, nil)
	defer scoped.Close()

	_, err := scoped.CallTool(context.Background(), "nonexistent", "tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestScopedMCPRegistry_Close_TeardownPerRunOnly(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	global.tools["global-server"] = []htools.MCPToolDefinition{
		{Name: "gtool", Description: "global"},
	}

	cm := mcp.NewClientManager()
	perRunName := "temp-server"
	err := cm.AddServerWithConn(perRunName, func() (mcp.Conn, error) {
		return newFakeMCPConn(perRunName, nil), nil
	})
	if err != nil {
		t.Fatalf("AddServerWithConn: %v", err)
	}

	scoped := NewScopedMCPRegistry(global, cm, []string{perRunName})

	// Close should succeed.
	if err := scoped.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After close, ListTools should return error.
	_, err = scoped.ListTools(context.Background())
	if err == nil {
		t.Error("expected error after close")
	}

	// Global registry should still work.
	tools, err := global.ListTools(context.Background())
	if err != nil {
		t.Fatalf("global ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Errorf("expected global tools intact; got %d", len(tools))
	}
}

func TestScopedMCPRegistry_Close_Idempotent(t *testing.T) {
	cm := mcp.NewClientManager()
	scoped := NewScopedMCPRegistry(nil, cm, nil)

	// Close twice should not panic or return error.
	if err := scoped.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := scoped.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestScopedMCPRegistry_EmptyPerRun_DelegatesToGlobal(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	global.tools["my-server"] = []htools.MCPToolDefinition{
		{Name: "tool1", Description: "t1"},
	}

	cm := mcp.NewClientManager()
	scoped := NewScopedMCPRegistry(global, cm, nil)
	defer scoped.Close()

	tools, err := scoped.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Errorf("expected 1 server from global; got %d", len(tools))
	}
	if _, ok := tools["my-server"]; !ok {
		t.Error("missing my-server from global")
	}
}

func TestScopedMCPRegistry_ListResources_DelegatesToGlobalOrPerRunNil(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	global.resources["global-server"] = []htools.MCPResource{{
		URI:  "resource://global/doc",
		Name: "doc",
	}}

	cm := mcp.NewClientManager()
	scoped := NewScopedMCPRegistry(global, cm, []string{"per-run"})
	defer scoped.Close()

	globalResources, err := scoped.ListResources(context.Background(), "global-server")
	if err != nil {
		t.Fatalf("ListResources global: %v", err)
	}
	if len(globalResources) != 1 || globalResources[0].URI != "resource://global/doc" {
		t.Fatalf("unexpected global resources: %+v", globalResources)
	}

	perRunResources, err := scoped.ListResources(context.Background(), "per-run")
	if err != nil {
		t.Fatalf("ListResources per-run: %v", err)
	}
	if perRunResources != nil {
		t.Fatalf("expected nil resources for per-run server, got %+v", perRunResources)
	}
}

func TestScopedMCPRegistry_ReadResource_DelegatesToGlobalOrErrorsForPerRun(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	cm := mcp.NewClientManager()
	scoped := NewScopedMCPRegistry(global, cm, []string{"per-run"})
	defer scoped.Close()

	content, err := scoped.ReadResource(context.Background(), "global-server", "resource://global/doc")
	if err != nil {
		t.Fatalf("ReadResource global: %v", err)
	}
	if content != "global-resource:global-server/resource://global/doc" {
		t.Fatalf("unexpected global resource content: %q", content)
	}

	_, err = scoped.ReadResource(context.Background(), "per-run", "resource://per-run/doc")
	if err == nil {
		t.Fatal("expected per-run resource read to fail")
	}
	if !strings.Contains(err.Error(), "does not support resources") {
		t.Fatalf("unexpected per-run error: %v", err)
	}
}

// --- Validation tests ---

func TestValidateMCPServerConfigs_Valid(t *testing.T) {
	configs := []MCPServerConfig{
		{Name: "stdio-srv", Command: "my-cmd", Args: []string{"--flag"}},
		{Name: "http-srv", URL: "http://localhost:3000/mcp"},
	}
	if err := validateMCPServerConfigs(configs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMCPServerConfigs_EmptyName(t *testing.T) {
	configs := []MCPServerConfig{
		{Name: "", Command: "cmd"},
	}
	err := validateMCPServerConfigs(configs)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestValidateMCPServerConfigs_NoCommandNoURL(t *testing.T) {
	configs := []MCPServerConfig{
		{Name: "srv"},
	}
	err := validateMCPServerConfigs(configs)
	if err == nil {
		t.Fatal("expected error for no command or url")
	}
}

func TestValidateMCPServerConfigs_BothCommandAndURL(t *testing.T) {
	configs := []MCPServerConfig{
		{Name: "srv", Command: "cmd", URL: "http://localhost:3000"},
	}
	err := validateMCPServerConfigs(configs)
	if err == nil {
		t.Fatal("expected error for both command and url")
	}
}

func TestValidateMCPServerConfigs_InvalidScheme(t *testing.T) {
	configs := []MCPServerConfig{
		{Name: "srv", URL: "ftp://example.com"},
	}
	err := validateMCPServerConfigs(configs)
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestValidateMCPServerConfigs_DuplicateNames(t *testing.T) {
	configs := []MCPServerConfig{
		{Name: "srv", Command: "cmd1"},
		{Name: "srv", Command: "cmd2"},
	}
	err := validateMCPServerConfigs(configs)
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
}

func TestValidateMCPServerConfigs_Empty(t *testing.T) {
	if err := validateMCPServerConfigs(nil); err != nil {
		t.Fatalf("unexpected error for empty: %v", err)
	}
}

func TestStartRun_MCPServers_InvalidConfig_NoCommand_NoURL(t *testing.T) {
	provider := &fakeProvider{}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{})

	_, err := runner.StartRun(RunRequest{
		Prompt: "hello",
		MCPServers: []MCPServerConfig{
			{Name: "bad-server"},
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid MCP config")
	}
}

func TestStartRun_MCPServers_BothCommandAndURL(t *testing.T) {
	provider := &fakeProvider{}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{})

	_, err := runner.StartRun(RunRequest{
		Prompt: "hello",
		MCPServers: []MCPServerConfig{
			{Name: "bad", Command: "cmd", URL: "http://localhost"},
		},
	})
	if err == nil {
		t.Fatal("expected error for both command and url")
	}
}

func TestStartRun_MCPServers_Empty_NoAllocation(t *testing.T) {
	provider := &fakeProvider{}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{})

	run, err := runner.StartRun(RunRequest{
		Prompt: "hello",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.ID == "" {
		t.Error("expected non-empty run ID")
	}
}

func TestRunRequest_NoMCPServers_BackwardCompat(t *testing.T) {
	// Verify that a RunRequest without MCPServers works the same as before.
	provider := &fakeProvider{}
	runner := NewRunner(provider, NewRegistry(), RunnerConfig{})

	req := RunRequest{Prompt: "test backward compat"}
	run, err := runner.StartRun(req)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.Status != RunStatusQueued {
		t.Errorf("expected queued status; got %s", run.Status)
	}
}

func TestBuildPerRunMCPRegistry_CollisionWithGlobal(t *testing.T) {
	// T3: run-level server colliding with global → still errors
	global := newMockGlobalMCPRegistry()
	runServers := []MCPServerConfig{
		{Name: "my-server", Command: "cmd"},
	}
	_, err := buildPerRunMCPRegistry(global, []string{"my-server"}, nil, runServers)
	if err == nil {
		t.Fatal("expected error for collision with global server name")
	}
}

func TestBuildPerRunMCPRegistry_NoCollision(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	runServers := []MCPServerConfig{
		{Name: "new-server", URL: "http://localhost:3000"},
	}
	scoped, err := buildPerRunMCPRegistry(global, []string{"existing-global"}, nil, runServers)
	if err != nil {
		t.Fatalf("buildPerRunMCPRegistry: %v", err)
	}
	defer scoped.Close()
}

// T1: profile server with same name as global → no error, profile version wins in ListTools.
func TestBuildPerRunMCPRegistry_ProfileServerShadowsGlobal(t *testing.T) {
	sharedName := "shared-server"
	global := newMockGlobalMCPRegistry()
	global.tools[sharedName] = []htools.MCPToolDefinition{
		{Name: "global-tool", Description: "from global"},
	}

	// Profile server with same name as global — should succeed (shadow semantics).
	profileServers := []MCPServerConfig{
		{Name: sharedName, URL: "http://localhost:3001"},
	}
	scoped, err := buildPerRunMCPRegistry(global, []string{sharedName}, profileServers, nil)
	if err != nil {
		t.Fatalf("expected no error for profile server shadowing global, got: %v", err)
	}
	defer scoped.Close()
}

// T2: profile server with unique name → added alongside globals.
func TestBuildPerRunMCPRegistry_ProfileServerUniqueName(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	global.tools["global-server"] = []htools.MCPToolDefinition{
		{Name: "g-tool", Description: "global"},
	}

	profileServers := []MCPServerConfig{
		{Name: "profile-only", URL: "http://localhost:3002"},
	}
	scoped, err := buildPerRunMCPRegistry(global, []string{"global-server"}, profileServers, nil)
	if err != nil {
		t.Fatalf("buildPerRunMCPRegistry: %v", err)
	}
	defer scoped.Close()

	// Verify that "profile-only" is registered in the per-run ClientManager.
	// We check the per-run server list rather than calling ListTools, which
	// would attempt a real network connection to the server.
	servers := scoped.perRun.ListServers()
	found := false
	for _, s := range servers {
		if s == "profile-only" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected profile-only in per-run servers; got %v", servers)
	}
	// "global-server" stays in the global registry — verify via isPerRun.
	if scoped.isPerRun("global-server") {
		t.Error("global-server should not be in per-run set")
	}
}

// T4: profile server AND run-level server with different names → both present.
func TestBuildPerRunMCPRegistry_ProfileAndRunBothPresent(t *testing.T) {
	global := newMockGlobalMCPRegistry()

	profileServers := []MCPServerConfig{
		{Name: "profile-srv", URL: "http://localhost:3003"},
	}
	runServers := []MCPServerConfig{
		{Name: "run-srv", URL: "http://localhost:3004"},
	}
	scoped, err := buildPerRunMCPRegistry(global, nil, profileServers, runServers)
	if err != nil {
		t.Fatalf("buildPerRunMCPRegistry: %v", err)
	}
	defer scoped.Close()

	// Both servers should be registered in the scoped registry.
	servers := scoped.perRun.ListServers()
	serverSet := make(map[string]struct{}, len(servers))
	for _, s := range servers {
		serverSet[s] = struct{}{}
	}
	if _, ok := serverSet["profile-srv"]; !ok {
		t.Error("expected profile-srv in per-run servers")
	}
	if _, ok := serverSet["run-srv"]; !ok {
		t.Error("expected run-srv in per-run servers")
	}
}

// T5: empty profile and empty run lists → delegates to global only.
func TestBuildPerRunMCPRegistry_EmptyBothLists_DelegatesToGlobal(t *testing.T) {
	global := newMockGlobalMCPRegistry()
	global.tools["global-only"] = []htools.MCPToolDefinition{
		{Name: "g-tool", Description: "global"},
	}

	scoped, err := buildPerRunMCPRegistry(global, []string{"global-only"}, nil, nil)
	if err != nil {
		t.Fatalf("buildPerRunMCPRegistry: %v", err)
	}
	defer scoped.Close()

	tools, err := scoped.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if _, ok := tools["global-only"]; !ok {
		t.Error("expected global-only in tools (global delegation)")
	}
	// No per-run servers registered.
	if len(scoped.perRun.ListServers()) != 0 {
		t.Errorf("expected no per-run servers, got %v", scoped.perRun.ListServers())
	}
}

// TestScopedMCPRegistry_ConcurrentAccessAndClose spawns goroutines that call
// ListTools, CallTool, ListResources, and ReadResource concurrently while
// another goroutine calls Close. The test must not panic or data-race (run
// with -race). Errors are acceptable because a closed registry returns errors.
func TestScopedMCPRegistry_ConcurrentAccessAndClose(t *testing.T) {
	// Do not call t.Parallel() here: with -race the goroutine storm is
	// sufficient and we want deterministic scheduling visibility.

	global := newMockGlobalMCPRegistry()
	global.tools["global-srv"] = []htools.MCPToolDefinition{
		{Name: "gtool", Description: "global"},
	}

	cm := mcp.NewClientManager()
	perRunName := "per-run-concurrent"
	err := cm.AddServerWithConn(perRunName, func() (mcp.Conn, error) {
		return newFakeMCPConn(perRunName, []mcp.ToolDef{
			{Name: "ctool", Description: "concurrent tool"},
		}), nil
	})
	if err != nil {
		t.Fatalf("AddServerWithConn: %v", err)
	}

	scoped := NewScopedMCPRegistry(global, cm, []string{perRunName})

	const goroutines = 8
	done := make(chan struct{})

	// Worker goroutines: continuously hammer the registry.
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer func() {
				// Recover from any panic so the test can report it cleanly.
				if r := recover(); r != nil {
					panic(fmt.Sprintf("goroutine %d panicked: %v", i, r))
				}
			}()
			for {
				select {
				case <-done:
					return
				default:
				}
				// Ignore errors — a closed registry is expected to return errors.
				ctx := context.Background()
				_, _ = scoped.ListTools(ctx)
				_, _ = scoped.CallTool(ctx, perRunName, "ctool", json.RawMessage(`{}`))
				_, _ = scoped.CallTool(ctx, "global-srv", "gtool", json.RawMessage(`{}`))
				_, _ = scoped.ListResources(ctx, perRunName)
				_, _ = scoped.ReadResource(ctx, perRunName, "file:///x")
			}
		}(i)
	}

	// Let the workers run briefly before closing.
	// We use a channel signal rather than time.Sleep to avoid fragility.
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		// Tiny busy-spin to let workers start; then close.
		for i := 0; i < 10000; i++ {
			// spin
		}
		_ = scoped.Close()
	}()

	<-closeDone
	close(done) // signal workers to stop

	// After close, all operations should return errors.
	_, err = scoped.ListTools(context.Background())
	if err == nil {
		t.Error("expected error after close from ListTools")
	}
	_, err = scoped.CallTool(context.Background(), perRunName, "ctool", json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error after close from CallTool")
	}
}

// TestStartRun_MCPServers_CollisionWithGlobalServerNames verifies that when
// a Runner is configured with GlobalMCPServerNames containing "my-global-server",
// a StartRun call with MCPServers: [{Name: "my-global-server", ...}] causes
// the run to reach RunStatusFailed (collision detected in the goroutine).
func TestStartRun_MCPServers_CollisionWithGlobalServerNames(t *testing.T) {
	t.Parallel()

	runner := NewRunner(&fakeProvider{}, NewRegistry(), RunnerConfig{
		GlobalMCPServerNames: []string{"my-global-server"},
		// GlobalMCPRegistry can be nil — only the names matter for collision detection.
	})

	run, err := runner.StartRun(RunRequest{
		Prompt: "test collision",
		MCPServers: []MCPServerConfig{
			{Name: "my-global-server", Command: "echo"},
		},
	})
	if err != nil {
		// StartRun itself does not check collision (happens in goroutine).
		// If it errors here the test still passes — the format validation
		// rejected it, which is also a correct early-rejection.
		// However, the current design does NOT error here; collision is async.
		// We leave this path as a safety net.
		t.Logf("StartRun returned synchronous error (acceptable): %v", err)
		return
	}

	// Wait for the run to reach a terminal state.
	deadline := time.Now().Add(4 * time.Second)
	var finalStatus RunStatus
	for {
		state, ok := runner.GetRun(run.ID)
		if !ok {
			t.Fatalf("run %q disappeared", run.ID)
		}
		if state.Status == RunStatusFailed || state.Status == RunStatusCompleted {
			finalStatus = state.Status
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for terminal status; last status: %s", state.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if finalStatus != RunStatusFailed {
		t.Errorf("expected RunStatusFailed due to collision, got %s", finalStatus)
	}
}

// --- Test helpers ---

// fakeProvider implements Provider for test setup.
type fakeProvider struct{}

func (f *fakeProvider) Complete(_ context.Context, _ CompletionRequest) (CompletionResult, error) {
	return CompletionResult{Content: "done"}, nil
}

// fakeMCPConn implements mcp.Conn for in-process testing.
type fakeMCPConn struct {
	name  string
	tools []mcp.ToolDef
	id    int64
}

func newFakeMCPConn(name string, tools []mcp.ToolDef) *fakeMCPConn {
	return &fakeMCPConn{name: name, tools: tools}
}

func (f *fakeMCPConn) Initialize(_ context.Context) error { return nil }

func (f *fakeMCPConn) ListTools(_ context.Context) ([]mcp.ToolDef, error) {
	return f.tools, nil
}

func (f *fakeMCPConn) CallTool(_ context.Context, name string, args json.RawMessage) (string, error) {
	return fmt.Sprintf(`{"content":[{"type":"text","text":"result:%s"}]}`, name), nil
}

func (f *fakeMCPConn) NextID() int64 {
	f.id++
	return f.id
}

func (f *fakeMCPConn) Close() error { return nil }
