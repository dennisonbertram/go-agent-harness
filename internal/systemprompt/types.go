package systemprompt

import (
	"context"
	"time"
)

type Extensions struct {
	Behaviors []string
	Talents   []string
	Skills    []string
	Custom    string
}

type ResolveRequest struct {
	Model              string
	AgentIntent        string
	DefaultAgentIntent string
	PromptProfile      string
	TaskContext        string
	Extensions         Extensions
}

type EnvironmentInfo struct {
	OS         string
	Arch       string
	Hostname   string
	Username   string
	WorkingDir string
	Shell      string
	GoVersion  string
}

type RuntimeContextInput struct {
	RunStartedAt          time.Time
	Now                   time.Time
	Step                  int
	PromptTokensTotal     int
	CompletionTokensTotal int
	TotalTokens           int
	LastTurnTokens        int
	CostUSDTotal          float64
	LastTurnCostUSD       float64
	CostStatus            string
	EstimatedContextTokens int
	MessageCount           int
	Environment           EnvironmentInfo
}

type Warning struct {
	Code    string
	Message string
}

// SkillResolver resolves skill names into interpolated prompt content.
type SkillResolver interface {
	ResolveSkill(ctx context.Context, name, args, workspace string) (string, error)
}

type ResolvedPrompt struct {
	StaticPrompt         string
	ResolvedIntent       string
	ResolvedModelProfile string
	ModelFallback        bool
	Behaviors            []string
	Talents              []string
	Skills               []string
	Warnings             []Warning
}

type Engine interface {
	Resolve(req ResolveRequest) (ResolvedPrompt, error)
	RuntimeContext(in RuntimeContextInput) string
}
