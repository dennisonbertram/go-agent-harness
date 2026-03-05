package observationalmemory

import (
	"context"
	"fmt"
	"strings"
)

type Reflector interface {
	Reflect(ctx context.Context, scope ScopeKey, cfg Config, observations []ObservationChunk, existingReflection string) (string, error)
}

type ModelReflector struct {
	Model Model
}

func (r ModelReflector) Reflect(ctx context.Context, scope ScopeKey, cfg Config, observations []ObservationChunk, existingReflection string) (string, error) {
	if r.Model == nil {
		return "", fmt.Errorf("reflector model is required")
	}
	messages := buildReflectionPrompt(scope, cfg, observations, existingReflection)
	out, err := r.Model.Complete(ctx, ModelRequest{Messages: messages})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
