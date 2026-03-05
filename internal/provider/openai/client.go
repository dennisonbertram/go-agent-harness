package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-agent-harness/internal/harness"
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

type Client struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewClient(config Config) (*Client, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("openai api key is required")
	}
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	model := config.Model
	if model == "" {
		model = "gpt-4.1-mini"
	}
	httpClient := config.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	return &Client{
		apiKey:  config.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  httpClient,
	}, nil
}

func (c *Client) Complete(ctx context.Context, req harness.CompletionRequest) (harness.CompletionResult, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	payload := completionRequest{
		Model:      model,
		Messages:   mapMessages(req.Messages),
		Tools:      mapTools(req.Tools),
		ToolChoice: "auto",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return harness.CompletionResult{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return harness.CompletionResult{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpRes, err := c.client.Do(httpReq)
	if err != nil {
		return harness.CompletionResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer httpRes.Body.Close()

	responseBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return harness.CompletionResult{}, fmt.Errorf("read response body: %w", err)
	}

	if httpRes.StatusCode >= 300 {
		return harness.CompletionResult{}, fmt.Errorf("openai request failed (%d): %s", httpRes.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var response completionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return harness.CompletionResult{}, fmt.Errorf("decode response: %w", err)
	}
	if len(response.Choices) == 0 {
		return harness.CompletionResult{}, fmt.Errorf("openai response had no choices")
	}

	choice := response.Choices[0]
	result := harness.CompletionResult{
		Content: strings.TrimSpace(choice.Message.Content),
	}
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]harness.ToolCall, 0, len(choice.Message.ToolCalls))
		for _, call := range choice.Message.ToolCalls {
			result.ToolCalls = append(result.ToolCalls, harness.ToolCall{
				ID:        call.ID,
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			})
		}
	}
	return result, nil
}

type completionRequest struct {
	Model      string        `json:"model"`
	Messages   []chatMessage `json:"messages"`
	Tools      []toolSpec    `json:"tools,omitempty"`
	ToolChoice string        `json:"tool_choice,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type toolSpec struct {
	Type     string          `json:"type"`
	Function toolSpecDetails `json:"function"`
}

type toolSpecDetails struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type chatToolCall struct {
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type"`
	Function chatToolCallFunction `json:"function"`
}

type chatToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type completionResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message chatCompletionMessage `json:"message"`
}

type chatCompletionMessage struct {
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls"`
}

func mapMessages(messages []harness.Message) []chatMessage {
	mapped := make([]chatMessage, 0, len(messages))
	for _, msg := range messages {
		chatMsg := chatMessage{
			Role:       msg.Role,
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
		}
		if msg.Content != "" {
			chatMsg.Content = msg.Content
		}
		if len(msg.ToolCalls) > 0 {
			chatMsg.ToolCalls = make([]chatToolCall, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				chatMsg.ToolCalls = append(chatMsg.ToolCalls, chatToolCall{
					ID:   call.ID,
					Type: "function",
					Function: chatToolCallFunction{
						Name:      call.Name,
						Arguments: call.Arguments,
					},
				})
			}
		}
		mapped = append(mapped, chatMsg)
	}
	return mapped
}

func mapTools(definitions []harness.ToolDefinition) []toolSpec {
	if len(definitions) == 0 {
		return nil
	}
	mapped := make([]toolSpec, 0, len(definitions))
	for _, def := range definitions {
		mapped = append(mapped, toolSpec{
			Type: "function",
			Function: toolSpecDetails{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return mapped
}
