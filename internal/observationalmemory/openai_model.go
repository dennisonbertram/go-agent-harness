package observationalmemory

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

// OpenAIConfig configures an observational-memory model client that speaks the
// OpenAI-compatible chat completions API.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

// OpenAIModel implements Model against /v1/chat/completions.
type OpenAIModel struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewOpenAIModel(config OpenAIConfig) (*OpenAIModel, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("openai api key is required")
	}
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = "gpt-5-nano"
	}
	httpClient := config.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenAIModel{
		apiKey:  config.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  httpClient,
	}, nil
}

func (m *OpenAIModel) Complete(ctx context.Context, req ModelRequest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("openai model is nil")
	}
	payload := omCompletionRequest{
		Model:    m.model,
		Messages: mapPromptMessages(req.Messages),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpRes, err := m.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer httpRes.Body.Close()

	responseBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	if httpRes.StatusCode >= 300 {
		return "", fmt.Errorf("openai request failed (%d): %s", httpRes.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var response omCompletionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openai response had no choices")
	}
	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

type omCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []omChatMessage `json:"messages"`
}

type omChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type omCompletionResponse struct {
	Choices []omChoice `json:"choices"`
}

type omChoice struct {
	Message omChoiceMessage `json:"message"`
}

type omChoiceMessage struct {
	Content string `json:"content"`
}

func mapPromptMessages(messages []PromptMessage) []omChatMessage {
	out := make([]omChatMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, omChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return out
}
