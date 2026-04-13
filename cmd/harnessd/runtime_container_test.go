package main

import (
	"testing"
	"time"

	"go-agent-harness/internal/harness"
)

func TestBuildMCPStdioRuntimeCreatesCatalogAndServer(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	runtime, err := buildMCPStdioRuntime(workspace)
	if err != nil {
		t.Fatalf("buildMCPStdioRuntime: %v", err)
	}
	if runtime.workspace != workspace {
		t.Fatalf("workspace: got %q", runtime.workspace)
	}
	if len(runtime.catalog) == 0 {
		t.Fatal("expected tool catalog")
	}
	if runtime.server == nil {
		t.Fatal("expected stdio server")
	}
	if got, want := runtime.server.ToolCount(), len(runtime.catalog); got != want {
		t.Fatalf("ToolCount: got %d want %d", got, want)
	}
}

func TestBuildHTTPRuntimeAssemblesRunnerSubagentsAndHTTPServer(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	askUserBroker := harness.NewInMemoryAskUserQuestionBroker(time.Now)
	msgSummarizer := &lazySummarizer{}
	callbackStarter := &callbackRunStarter{}
	baseRegistryOptions := harness.DefaultRegistryOptions{
		ApprovalMode:      harness.ToolApprovalModeFullAuto,
		AskUserBroker:     askUserBroker,
		AskUserTimeout:    time.Minute,
		MessageSummarizer: msgSummarizer,
	}
	tools := harness.NewDefaultRegistryWithOptions(workspace, baseRegistryOptions)

	runtime, err := buildHTTPRuntime(httpRuntimeOptions{
		addr:                "127.0.0.1:0",
		workspace:           workspace,
		provider:            &noopProvider{},
		tools:               tools,
		runnerCfg:           harness.RunnerConfig{DefaultModel: "gpt-4.1-mini"},
		skillLister:         nil,
		baseRegistryOptions: baseRegistryOptions,
		cronClient:          nil,
		modelCatalog:        nil,
		providerRegistry:    nil,
		runStore:            nil,
		triggers:            buildTriggerRuntime(nil, nil),
		callbackStarter:     callbackStarter,
		msgSummarizer:       msgSummarizer,
		skillManager:        nil,
		subagentBaseRef:     "HEAD",
		subagentConfigTOML:  "",
	})
	if err != nil {
		t.Fatalf("buildHTTPRuntime: %v", err)
	}
	if runtime.runner == nil {
		t.Fatal("expected runner")
	}
	if runtime.subagentManager == nil {
		t.Fatal("expected subagent manager")
	}
	if runtime.handler == nil {
		t.Fatal("expected http handler")
	}
	if runtime.httpServer == nil {
		t.Fatal("expected http server")
	}
	if runtime.httpServer.Addr != "127.0.0.1:0" {
		t.Fatalf("httpServer.Addr: got %q", runtime.httpServer.Addr)
	}
	if runtime.httpServer.Handler == nil {
		t.Fatal("expected http server handler")
	}
	callbackStarter.mu.Lock()
	boundRunner := callbackStarter.runner
	callbackStarter.mu.Unlock()
	if boundRunner != runtime.runner {
		t.Fatal("expected callback starter to bind the built runner")
	}
	msgSummarizer.mu.Lock()
	innerSummarizer := msgSummarizer.summarizer
	msgSummarizer.mu.Unlock()
	if innerSummarizer == nil {
		t.Fatal("expected lazy summarizer to bind to the built runner")
	}

}
