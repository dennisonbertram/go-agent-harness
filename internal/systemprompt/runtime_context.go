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

	var b strings.Builder
	b.WriteString("<runtime_context>\n")
	b.WriteString(fmt.Sprintf("run_started_at_utc: %s\n", started.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("current_time_utc: %s\n", now.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("elapsed_seconds: %d\n", elapsed))
	b.WriteString(fmt.Sprintf("step: %d\n", step))
	b.WriteString("cost_status: unavailable_phase1\n")
	b.WriteString("</runtime_context>")
	return b.String()
}
