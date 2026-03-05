package tools

import (
	"context"
	"net/http"
	"sort"
	"time"
)

func BuildCatalog(opts BuildOptions) ([]Tool, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if opts.ApprovalMode == "" {
		opts.ApprovalMode = ApprovalModeFullAuto
	}
	if opts.AskUserTimeout <= 0 {
		opts.AskUserTimeout = 5 * time.Minute
	}

	jobManager := NewJobManager(opts.WorkspaceRoot, opts.Now)
	todos := newTodoStore()

	tools := []Tool{
		askUserQuestionTool(opts.AskUserBroker, opts.AskUserTimeout),
		observationalMemoryTool(opts.WorkspaceRoot, opts.MemoryManager, opts.AgentRunner),
		readTool(opts.WorkspaceRoot),
		writeTool(opts.WorkspaceRoot),
		editTool(opts.WorkspaceRoot),
		bashTool(jobManager),
		jobOutputTool(jobManager),
		jobKillTool(jobManager),
		lsTool(opts.WorkspaceRoot),
		globTool(opts.WorkspaceRoot),
		grepTool(opts.WorkspaceRoot),
		applyPatchTool(opts.WorkspaceRoot),
		gitStatusTool(opts.WorkspaceRoot),
		gitDiffTool(opts.WorkspaceRoot),
		fetchTool(opts.HTTPClient),
		downloadTool(opts.WorkspaceRoot, opts.HTTPClient),
	}

	if opts.EnableTodos {
		tools = append(tools, todosTool(todos))
	}
	if opts.EnableLSP {
		tools = append(tools, lspDiagnosticsTool(opts.WorkspaceRoot), lspReferencesTool(opts.WorkspaceRoot), lspRestartTool(opts.WorkspaceRoot))
	}
	if opts.Sourcegraph.Endpoint != "" {
		tools = append(tools, sourcegraphTool(opts.HTTPClient, opts.Sourcegraph))
	}
	if opts.EnableMCP && opts.MCPRegistry != nil {
		tools = append(tools, listMCPResourcesTool(opts.MCPRegistry), readMCPResourceTool(opts.MCPRegistry))
		dynamic, err := dynamicMCPTools(context.Background(), opts.MCPRegistry)
		if err != nil {
			return nil, err
		}
		tools = append(tools, dynamic...)
	}
	if opts.EnableAgent && opts.AgentRunner != nil {
		tools = append(tools, agentTool(opts.AgentRunner))
		if opts.EnableWebOps && opts.WebFetcher != nil {
			tools = append(tools, agenticFetchTool(opts.WebFetcher, opts.AgentRunner), webSearchTool(opts.WebFetcher), webFetchTool(opts.WebFetcher))
		}
	}

	for i := range tools {
		tools[i].Handler = applyPolicy(tools[i].Definition, opts.ApprovalMode, opts.Policy, tools[i].Handler)
	}

	sort.SliceStable(tools, func(i, j int) bool {
		return tools[i].Definition.Name < tools[j].Definition.Name
	})
	return tools, nil
}
