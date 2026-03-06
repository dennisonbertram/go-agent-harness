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
	"go-agent-harness/internal/provider/pricing"
)

type Config struct {
	APIKey          string
	BaseURL         string
	Model           string
	Client          *http.Client
	PricingResolver pricing.Resolver
}

type Client struct {
	apiKey          string
	baseURL         string
	model           string
	client          *http.Client
	pricingResolver pricing.Resolver
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
		apiKey:          config.APIKey,
		baseURL:         strings.TrimRight(baseURL, "/"),
		model:           model,
		client:          httpClient,
		pricingResolver: config.PricingResolver,
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
	usage, usageStatus := normalizeUsage(response.Usage)
	result.Usage = &usage
	result.UsageStatus = usageStatus
	cost, costStatus, totalCostUSD := c.computeCost(model, usage, usageStatus, response)
	result.Cost = &cost
	result.CostStatus = costStatus
	result.CostUSD = &totalCostUSD

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
	Usage   *usage   `json:"usage,omitempty"`
	CostUSD *float64 `json:"cost_usd,omitempty"`
}

type choice struct {
	Message chatCompletionMessage `json:"message"`
}

type chatCompletionMessage struct {
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls"`
}

type usage struct {
	PromptTokens            int                     `json:"prompt_tokens"`
	CompletionTokens        int                     `json:"completion_tokens"`
	TotalTokens             int                     `json:"total_tokens"`
	PromptTokensDetails     *promptTokensDetails    `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *completionTokensDetail `json:"completion_tokens_details,omitempty"`
	CostUSD                 *float64                `json:"cost_usd,omitempty"`
}

type promptTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
	AudioTokens  int `json:"audio_tokens"`
}

type completionTokensDetail struct {
	ReasoningTokens int `json:"reasoning_tokens"`
	AudioTokens     int `json:"audio_tokens"`
}

func normalizeUsage(in *usage) (harness.CompletionUsage, harness.UsageStatus) {
	if in == nil {
		return harness.CompletionUsage{}, harness.UsageStatusProviderUnreported
	}
	out := harness.CompletionUsage{
		PromptTokens:     in.PromptTokens,
		CompletionTokens: in.CompletionTokens,
		TotalTokens:      in.TotalTokens,
	}
	if out.TotalTokens == 0 && (out.PromptTokens > 0 || out.CompletionTokens > 0) {
		out.TotalTokens = out.PromptTokens + out.CompletionTokens
	}
	if in.PromptTokensDetails != nil {
		out.CachedPromptTokens = intPtr(in.PromptTokensDetails.CachedTokens)
		out.InputAudioTokens = intPtr(in.PromptTokensDetails.AudioTokens)
	}
	if in.CompletionTokensDetails != nil {
		out.ReasoningTokens = intPtr(in.CompletionTokensDetails.ReasoningTokens)
		out.OutputAudioTokens = intPtr(in.CompletionTokensDetails.AudioTokens)
	}
	return out, harness.UsageStatusProviderReported
}

func intPtr(v int) *int {
	n := v
	return &n
}

func (c *Client) computeCost(model string, usage harness.CompletionUsage, usageStatus harness.UsageStatus, response completionResponse) (harness.CompletionCost, harness.CostStatus, float64) {
	cost := harness.CompletionCost{
		Estimated: false,
	}
	if usageStatus == harness.UsageStatusProviderUnreported {
		return cost, harness.CostStatusProviderUnreported, 0
	}
	if explicit, ok := explicitCostUSD(response); ok {
		cost.TotalUSD = explicit
		return cost, harness.CostStatusAvailable, explicit
	}
	if c.pricingResolver == nil {
		return cost, harness.CostStatusUnpricedModel, 0
	}
	resolved, ok := c.pricingResolver.Resolve("openai", model)
	if !ok {
		return cost, harness.CostStatusUnpricedModel, 0
	}
	cost.PricingVersion = resolved.PricingVersion
	cachedPromptTokens := valueOrZero(usage.CachedPromptTokens)
	billablePromptTokens := usage.PromptTokens
	if resolved.Rates.CacheReadPer1MTokensUSD > 0 && cachedPromptTokens > 0 {
		if cachedPromptTokens > billablePromptTokens {
			cachedPromptTokens = billablePromptTokens
		}
		billablePromptTokens -= cachedPromptTokens
		cost.CacheReadUSD = tokensToUSD(cachedPromptTokens, resolved.Rates.CacheReadPer1MTokensUSD)
	}
	cost.InputUSD = tokensToUSD(billablePromptTokens, resolved.Rates.InputPer1MTokensUSD)
	cost.OutputUSD = tokensToUSD(usage.CompletionTokens, resolved.Rates.OutputPer1MTokensUSD)
	cost.CacheWriteUSD = 0
	cost.TotalUSD = cost.InputUSD + cost.OutputUSD + cost.CacheReadUSD + cost.CacheWriteUSD
	return cost, harness.CostStatusAvailable, cost.TotalUSD
}

func explicitCostUSD(response completionResponse) (float64, bool) {
	if response.CostUSD != nil {
		return *response.CostUSD, true
	}
	if response.Usage != nil && response.Usage.CostUSD != nil {
		return *response.Usage.CostUSD, true
	}
	return 0, false
}

func tokensToUSD(tokens int, per1M float64) float64 {
	if tokens <= 0 || per1M <= 0 {
		return 0
	}
	return (float64(tokens) / 1_000_000.0) * per1M
}

func valueOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
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
