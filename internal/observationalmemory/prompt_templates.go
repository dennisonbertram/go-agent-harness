package observationalmemory

import (
	"fmt"
	"strings"
)

type PromptMessage struct {
	Role    string
	Content string
}

func buildObservationPrompt(scope ScopeKey, cfg Config, unobserved []TranscriptMessage, existing []ObservationChunk, reflection string) []PromptMessage {
	var b strings.Builder
	b.WriteString("You are an observational memory processor for an autonomous coding agent.\n")
	b.WriteString("Extract concrete, durable observations that help future coding turns.\n")
	b.WriteString("Do not include fluff. Prefer facts, constraints, decisions, and durable context.\n")
	b.WriteString("For each observation, prefix it with IMPORTANCE:x.x on its own line, where x.x is a float from 0.0 to 1.0.\n")
	b.WriteString("Use 0.9-1.0 for critical constraints or hard user preferences (e.g. \"never auto-commit\").\n")
	b.WriteString("Use 0.5-0.8 for useful context (e.g. current module being worked on).\n")
	b.WriteString("Use 0.1-0.4 for low-value or transient details.\n")
	b.WriteString("Format each observation as:\nIMPORTANCE:x.x\n<observation text>\n\n")
	b.WriteString("Respond with plain text only.\n\n")
	b.WriteString("Scope:\n")
	b.WriteString(fmt.Sprintf("- tenant_id: %s\n- conversation_id: %s\n- agent_id: %s\n\n", scope.TenantID, scope.ConversationID, scope.AgentID))
	b.WriteString("Current memory summary:\n")
	if reflection != "" {
		b.WriteString("Reflection:\n")
		b.WriteString(reflection)
		b.WriteString("\n\n")
	}
	if len(existing) == 0 {
		b.WriteString("- (none)\n\n")
	} else {
		for _, obs := range existing {
			b.WriteString(fmt.Sprintf("- [%d] %s\n", obs.Seq, strings.TrimSpace(obs.Content)))
		}
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("Observe threshold tokens: %d\n", cfg.ObserveMinTokens))
	b.WriteString("New transcript segment:\n")
	for _, msg := range unobserved {
		role := msg.Role
		if role == "" {
			role = "unknown"
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[%d] %s: %s\n", msg.Index, role, content))
	}

	return []PromptMessage{
		{Role: "system", Content: "Produce a concise observational memory update."},
		{Role: "user", Content: b.String()},
	}
}

func buildReflectionPrompt(scope ScopeKey, cfg Config, observations []ObservationChunk, existingReflection string) []PromptMessage {
	var b strings.Builder
	b.WriteString("You are compressing observational memory for long-lived agent performance.\n")
	b.WriteString("Create a compact, structured reflection that preserves durable decisions, constraints, and preferences.\n")
	b.WriteString("Also identify any observations that supersede or contradict each other.\n\n")
	b.WriteString("Respond using EXACTLY this format (all three sections are required):\n\n")
	b.WriteString("SUMMARY:\n<compressed prose summary of all observations>\n\n")
	b.WriteString("SUPERSESSIONS:\n")
	b.WriteString("- [seq:N] replaces [seq:M]: <reason why N supersedes M>\n")
	b.WriteString("(Use this section when a newer observation replaces an older one. Leave empty if none.)\n\n")
	b.WriteString("CONTRADICTIONS:\n")
	b.WriteString("- [seq:N] vs [seq:M]: <description of the conflicting claims>\n")
	b.WriteString("(Use this section when two observations conflict and neither clearly wins. Leave empty if none.)\n\n")
	b.WriteString("Scope:\n")
	b.WriteString(fmt.Sprintf("- tenant_id: %s\n- conversation_id: %s\n- agent_id: %s\n\n", scope.TenantID, scope.ConversationID, scope.AgentID))
	b.WriteString(fmt.Sprintf("Reflection threshold tokens: %d\n\n", cfg.ReflectThresholdTokens))
	if existingReflection != "" {
		b.WriteString("Existing reflection:\n")
		b.WriteString(existingReflection)
		b.WriteString("\n\n")
	}
	b.WriteString("Observation chunks (use [seq:N] notation to reference them):\n")
	for _, obs := range observations {
		b.WriteString(fmt.Sprintf("- [seq:%d] %s\n", obs.Seq, strings.TrimSpace(obs.Content)))
	}
	return []PromptMessage{
		{Role: "system", Content: "Produce a structured compressed reflection with SUMMARY, SUPERSESSIONS, and CONTRADICTIONS sections."},
		{Role: "user", Content: b.String()},
	}
}
