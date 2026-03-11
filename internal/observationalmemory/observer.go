package observationalmemory

import (
	"context"
	"fmt"
	"strconv"
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

// ParseObservationChunks parses the observer LLM output into one or more
// ObservationChunk values. Each chunk may be prefixed with "IMPORTANCE:x.x"
// on its own line. If no IMPORTANCE prefix is present the entire output is
// treated as a single unscored chunk (Importance==0.0).
func ParseObservationChunks(raw string) []ObservationChunk {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Check whether any IMPORTANCE: prefix exists at all.
	if !strings.Contains(raw, "IMPORTANCE:") {
		return []ObservationChunk{{Content: raw}}
	}

	var chunks []ObservationChunk
	// Split on blank lines to get candidate blocks.
	blocks := splitBlocks(raw)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		importance, content := parseImportancePrefix(block)
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		chunks = append(chunks, ObservationChunk{
			Importance: importance,
			Content:    content,
		})
	}
	if len(chunks) == 0 {
		return []ObservationChunk{{Content: raw}}
	}
	return chunks
}

// splitBlocks splits text into blocks separated by one or more blank lines.
func splitBlocks(text string) []string {
	var blocks []string
	var current strings.Builder
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if current.Len() > 0 {
				blocks = append(blocks, current.String())
				current.Reset()
			}
		} else {
			if current.Len() > 0 {
				current.WriteByte('\n')
			}
			current.WriteString(line)
		}
	}
	if current.Len() > 0 {
		blocks = append(blocks, current.String())
	}
	return blocks
}

// parseImportancePrefix extracts the IMPORTANCE:x.x prefix from the first
// line of a block, returning the parsed importance value and the remaining
// content. If no prefix is present, importance is 0.0 and content is the
// full block.
func parseImportancePrefix(block string) (float64, string) {
	firstNewline := strings.IndexByte(block, '\n')
	var firstLine, rest string
	if firstNewline == -1 {
		firstLine = block
		rest = ""
	} else {
		firstLine = block[:firstNewline]
		rest = block[firstNewline+1:]
	}
	firstLine = strings.TrimSpace(firstLine)
	if strings.HasPrefix(firstLine, "IMPORTANCE:") {
		valStr := strings.TrimPrefix(firstLine, "IMPORTANCE:")
		valStr = strings.TrimSpace(valStr)
		if v, err := strconv.ParseFloat(valStr, 64); err == nil {
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			return v, rest
		}
	}
	// No valid prefix — return full block with zero importance.
	return 0.0, block
}
