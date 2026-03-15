package conclusionwatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EvaluatorResult is the structured response from the LLM evaluator.
type EvaluatorResult struct {
	HasUnjustifiedConclusion bool          `json:"has_unjustified_conclusion"`
	Patterns                 []PatternType `json:"patterns"`
	Evidence                 string        `json:"evidence"`
	Explanation              string        `json:"explanation"`
}

// Evaluator calls an LLM to detect conclusion-jumping in a single step.
type Evaluator interface {
	Evaluate(ctx context.Context, llmText string, toolHistory []string, proposedTools []string) (*EvaluatorResult, error)
}

// OpenAIEvaluator uses gpt-4o-mini via raw net/http (no SDK).
type OpenAIEvaluator struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

// NewOpenAIEvaluator creates an OpenAIEvaluator with sensible defaults.
func NewOpenAIEvaluator(apiKey string) *OpenAIEvaluator {
	return &OpenAIEvaluator{
		APIKey:  apiKey,
		Model:   "gpt-4o-mini",
		BaseURL: "https://api.openai.com/v1",
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// evaluatorSystemPrompt is the system message sent to the LLM.
const evaluatorSystemPrompt = `You are an expert at identifying when an AI assistant makes unjustified conclusions.
An unjustified conclusion is any assertion about code, architecture, file contents, or task
status that is NOT supported by prior tool usage shown in the conversation history.

Pattern types:
- hedge_assertion: uses "must be", "clearly", "obviously", "definitely", "I assume", "probably is" in declarative context
- unverified_file_claim: asserts what a file contains or how code works without having read it
- premature_completion: claims task is done/fixed/complete without test/verification tool being called
- skipped_diagnostic: proposes mutating action (write, edit, bash) without prior diagnostic (read, grep, ls)
- architecture_assumption: makes claims about design/architecture/intended flow without prior code exploration

Respond ONLY with valid JSON (no markdown):
{"has_unjustified_conclusion": bool, "patterns": [...], "evidence": "exact quote", "explanation": "brief why"}`

// Evaluate calls the OpenAI API to detect conclusion-jumping in the given LLM text.
func (e *OpenAIEvaluator) Evaluate(ctx context.Context, llmText string, toolHistory []string, proposedTools []string) (*EvaluatorResult, error) {
	// Build user message.
	var sb strings.Builder
	sb.WriteString("TOOL HISTORY (last steps):\n")
	if len(toolHistory) > 0 {
		sb.WriteString(strings.Join(toolHistory, "\n"))
	} else {
		sb.WriteString("(none)")
	}
	sb.WriteString("\n\nAI MESSAGE:\n")
	sb.WriteString(llmText)
	sb.WriteString("\n\nPROPOSED TOOL CALLS:\n")
	if len(proposedTools) > 0 {
		sb.WriteString(strings.Join(proposedTools, "\n"))
	} else {
		sb.WriteString("none")
	}

	// Build request body.
	reqBody := map[string]any{
		"model": e.Model,
		"messages": []map[string]string{
			{"role": "system", "content": evaluatorSystemPrompt},
			{"role": "user", "content": sb.String()},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("evaluator: marshal request: %w", err)
	}

	// Apply 10s timeout unless ctx deadline is sooner.
	evalCtx := ctx
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); !ok || deadline.Before(d) {
		var cancel context.CancelFunc
		evalCtx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	baseURL := e.BaseURL
	// Normalize: strip trailing slash and /v1 suffix so we can always append /v1/...
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")

	url := baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(evalCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("evaluator: create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+e.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("evaluator: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("evaluator: read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("evaluator: API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	// Decode the OpenAI chat completions response envelope.
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		return nil, fmt.Errorf("evaluator: decode envelope: %w", err)
	}
	if len(envelope.Choices) == 0 {
		return nil, fmt.Errorf("evaluator: no choices in response")
	}

	content := envelope.Choices[0].Message.Content

	// Parse the JSON returned by the LLM.
	var result EvaluatorResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("evaluator: LLM returned non-JSON content: %w (content: %q)", err, content)
	}

	return &result, nil
}
