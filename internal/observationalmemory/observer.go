package observationalmemory

import (
	"context"
	"fmt"
	"strings"
)

type ModelRequest struct {
	Messages []PromptMessage
}

type Model interface {
	Complete(ctx context.Context, req ModelRequest) (string, error)
}

type Observer interface {
	Observe(ctx context.Context, scope ScopeKey, cfg Config, unobserved []TranscriptMessage, existing []ObservationChunk, reflection string) (string, error)
}

type ModelObserver struct {
	Model Model
}

func (o ModelObserver) Observe(ctx context.Context, scope ScopeKey, cfg Config, unobserved []TranscriptMessage, existing []ObservationChunk, reflection string) (string, error) {
	if o.Model == nil {
		return "", fmt.Errorf("observer model is required")
	}
	messages := buildObservationPrompt(scope, cfg, unobserved, existing, reflection)
	out, err := o.Model.Complete(ctx, ModelRequest{Messages: messages})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
