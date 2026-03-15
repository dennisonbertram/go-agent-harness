package conclusionwatcher

import (
	"context"
	"fmt"

	"go-agent-harness/internal/harness"
)

// InjectValidationPrompt appends ValidationPrompt to the LLM response content.
// Returns a mutated PostMessageHookResult with HookActionContinue and the
// modified response. Always allocates a new CompletionResult to avoid aliasing.
func InjectValidationPrompt(
	result harness.PostMessageHookResult,
	response *harness.CompletionResult,
	prompt string,
	detection DetectionResult,
) harness.PostMessageHookResult {
	mutated := *response // copy value
	mutated.Content = response.Content + prompt
	result.Action = harness.HookActionContinue
	result.MutatedResponse = &mutated
	return result
}

// PauseForUser returns a PostMessageHookResult with HookActionBlock, using
// the detection evidence as the block reason. The step is halted and the
// reason is surfaced to the runner (and ultimately to the user via SSE).
func PauseForUser(detection DetectionResult) harness.PostMessageHookResult {
	return harness.PostMessageHookResult{
		Action: harness.HookActionBlock,
		Reason: fmt.Sprintf("[conclusion-watcher] pattern=%s evidence=%s step=%d run=%s",
			detection.Pattern, detection.Evidence, detection.Step, detection.RunID),
	}
}

// RequestCritique calls the CritiqueProvider with the response content,
// then injects the critique using InjectValidationPrompt.
// Returns an error if the provider call fails; callers should fall back to
// InjectValidationPrompt on error.
func RequestCritique(
	ctx context.Context,
	result harness.PostMessageHookResult,
	response *harness.CompletionResult,
	detection DetectionResult,
	provider CritiqueProvider,
) (harness.PostMessageHookResult, error) {
	critique, err := provider.Critique(ctx, response.Content)
	if err != nil {
		return result, err
	}
	injected := InjectValidationPrompt(result, response, "\n\n[CRITIQUE] "+critique, detection)
	return injected, nil
}
