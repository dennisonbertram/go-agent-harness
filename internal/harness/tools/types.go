package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	om "go-agent-harness/internal/observationalmemory"
	"go-agent-harness/internal/provider/catalog"
)

type Action string

const (
	ActionRead     Action = "read"
	ActionWrite    Action = "write"
	ActionList     Action = "list"
	ActionExecute  Action = "execute"
	ActionFetch    Action = "fetch"
	ActionDownload Action = "download"
)

type ApprovalMode string

const (
	ApprovalModeFullAuto    ApprovalMode = "full_auto"
	ApprovalModePermissions ApprovalMode = "permissions"
)

type PolicyInput struct {
	ToolName  string          `json:"tool_name"`
	Action    Action          `json:"action"`
	Path      string          `json:"path,omitempty"`
	Arguments json.RawMessage `json:"arguments"`
	Mutating  bool            `json:"mutating"`
}

type PolicyDecision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

type Policy interface {
	Allow(ctx context.Context, in PolicyInput) (PolicyDecision, error)
}

type Definition struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Parameters   map[string]any `json:"parameters"`
	Action       Action         `json:"-"`
	Mutating     bool           `json:"-"`
	ParallelSafe bool           `json:"-"`
}

type Handler func(ctx context.Context, args json.RawMessage) (string, error)

type Tool struct {
	Definition Definition
	Handler    Handler
}

type BuildOptions struct {
	WorkspaceRoot  string
	ApprovalMode   ApprovalMode
	Policy         Policy
	HTTPClient     *http.Client
	Now            func() time.Time
	AskUserBroker  AskUserQuestionBroker
	AskUserTimeout time.Duration
	MemoryManager  om.Manager

	MCPRegistry  MCPRegistry
	AgentRunner  AgentRunner
	WebFetcher   WebFetcher
	Sourcegraph  SourcegraphConfig
	EnableTodos  bool
	EnableLSP    bool
	EnableMCP    bool
	EnableAgent  bool
	EnableWebOps bool
	EnableSkills bool
	SkillLister  SkillLister
	ModelCatalog *catalog.Catalog
}

type SourcegraphConfig struct {
	Endpoint string
	Token    string
}

type MCPResource struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

type MCPToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type MCPRegistry interface {
	ListResources(ctx context.Context, server string) ([]MCPResource, error)
	ReadResource(ctx context.Context, server, uri string) (string, error)
	ListTools(ctx context.Context) (map[string][]MCPToolDefinition, error)
	CallTool(ctx context.Context, server, tool string, args json.RawMessage) (string, error)
}

type AgentRunner interface {
	RunPrompt(ctx context.Context, prompt string) (string, error)
}

// SkillInfo holds read-only skill metadata for the tool layer.
type SkillInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	ArgumentHint string   `json:"argument_hint,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	Source       string   `json:"source"`
}

// SkillLister provides skill lookup and listing for the skill tool.
type SkillLister interface {
	GetSkill(name string) (SkillInfo, bool)
	ListSkills() []SkillInfo
	ResolveSkill(name, args, workspace string) (string, error)
}

type WebFetcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]map[string]any, error)
	Fetch(ctx context.Context, url string) (string, error)
}

type AskUserQuestionRequest struct {
	RunID     string
	CallID    string
	Questions []AskUserQuestion
	Timeout   time.Duration
}

type AskUserQuestionPending struct {
	RunID      string            `json:"run_id"`
	CallID     string            `json:"call_id"`
	Tool       string            `json:"tool"`
	Questions  []AskUserQuestion `json:"questions"`
	DeadlineAt time.Time         `json:"deadline_at"`
}

type AskUserQuestionBroker interface {
	Ask(ctx context.Context, req AskUserQuestionRequest) (answers map[string]string, answeredAt time.Time, err error)
	Pending(runID string) (AskUserQuestionPending, bool)
	Submit(runID string, answers map[string]string) error
}

type contextKey string

const ContextKeyRunID contextKey = "run_id"
const ContextKeyToolCallID contextKey = "tool_call_id"
const ContextKeyRunMetadata contextKey = "run_metadata"
const ContextKeyTranscriptReader contextKey = "transcript_reader"

type RunMetadata struct {
	RunID          string
	TenantID       string
	ConversationID string
	AgentID        string
}

type TranscriptMessage struct {
	Index      int64  `json:"index"`
	Role       string `json:"role"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Content    string `json:"content,omitempty"`
}

type TranscriptSnapshot struct {
	RunID          string              `json:"run_id"`
	TenantID       string              `json:"tenant_id"`
	ConversationID string              `json:"conversation_id"`
	AgentID        string              `json:"agent_id"`
	Messages       []TranscriptMessage `json:"messages"`
	GeneratedAt    time.Time           `json:"generated_at"`
}

type TranscriptReader interface {
	Snapshot(limit int, includeTools bool) TranscriptSnapshot
}

func RunIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if meta, ok := ctx.Value(ContextKeyRunMetadata).(RunMetadata); ok {
		return meta.RunID
	}
	v, _ := ctx.Value(ContextKeyRunID).(string)
	return v
}

func ToolCallIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ContextKeyToolCallID).(string)
	return v
}

func RunMetadataFromContext(ctx context.Context) (RunMetadata, bool) {
	if ctx == nil {
		return RunMetadata{}, false
	}
	v, ok := ctx.Value(ContextKeyRunMetadata).(RunMetadata)
	return v, ok
}

func TranscriptReaderFromContext(ctx context.Context) (TranscriptReader, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(ContextKeyTranscriptReader).(TranscriptReader)
	return v, ok
}
