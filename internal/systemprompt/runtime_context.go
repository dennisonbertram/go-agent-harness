package systemprompt

import (
	"fmt"
	"strings"
	"time"
)

func (e *FileEngine) RuntimeContext(in RuntimeContextInput) string {
	return BuildRuntimeContext(in)
}

func BuildRuntimeContext(in RuntimeContextInput) string {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	started := in.RunStartedAt.UTC()
	if started.IsZero() {
		started = now
	}
	elapsed := int(now.Sub(started).Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	step := in.Step
	if step <= 0 {
		step = 1
	}
	costStatus := strings.TrimSpace(in.CostStatus)
	if costStatus == "" {
		costStatus = "pending"
	}

	var b strings.Builder
	b.WriteString("<runtime_context>\n")
	b.WriteString(fmt.Sprintf("run_started_at_utc: %s\n", started.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("current_time_utc: %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("elapsed_seconds: %d\n", elapsed))
	b.WriteString(fmt.Sprintf("step: %d\n", step))
	b.WriteString(fmt.Sprintf("prompt_tokens_total: %d\n", in.PromptTokensTotal))
	b.WriteString(fmt.Sprintf("completion_tokens_total: %d\n", in.CompletionTokensTotal))
	b.WriteString(fmt.Sprintf("total_tokens: %d\n", in.TotalTokens))
	b.WriteString(fmt.Sprintf("last_turn_tokens: %d\n", in.LastTurnTokens))
	b.WriteString(fmt.Sprintf("cost_usd_total: %.6f\n", in.CostUSDTotal))
	b.WriteString(fmt.Sprintf("last_turn_cost_usd: %.6f\n", in.LastTurnCostUSD))
	b.WriteString(fmt.Sprintf("cost_status: %s\n", costStatus))
	b.WriteString(fmt.Sprintf("estimated_context_tokens: %d\n", in.EstimatedContextTokens))
	b.WriteString(fmt.Sprintf("message_count: %d\n", in.MessageCount))

	env := in.Environment
	var envLines []string
	if env.OS != "" {
		envLines = append(envLines, fmt.Sprintf("os: %s", env.OS))
	}
	if env.Arch != "" {
		envLines = append(envLines, fmt.Sprintf("arch: %s", env.Arch))
	}
	if env.Hostname != "" {
		envLines = append(envLines, fmt.Sprintf("hostname: %s", env.Hostname))
	}
	if env.Username != "" {
		envLines = append(envLines, fmt.Sprintf("user: %s", env.Username))
	}
	if env.WorkingDir != "" {
		envLines = append(envLines, fmt.Sprintf("working_dir: %s", env.WorkingDir))
	}
	if env.Shell != "" {
		envLines = append(envLines, fmt.Sprintf("shell: %s", env.Shell))
	}
	if env.GoVersion != "" {
		envLines = append(envLines, fmt.Sprintf("go_version: %s", env.GoVersion))
	}
	if len(envLines) > 0 {
		b.WriteString("<environment>\n")
		for _, line := range envLines {
			b.WriteString(line + "\n")
		}
		b.WriteString("</environment>\n")
	}

	b.WriteString("</runtime_context>")
	return b.String()
}
