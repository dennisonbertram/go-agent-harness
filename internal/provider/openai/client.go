package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
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
	ProviderName    string // e.g. "openai", "deepseek" — used for pricing resolution
}

type Client struct {
	apiKey          string
	baseURL         string
	model           string
	client          *http.Client
	pricingResolver pricing.Resolver
	providerName    string
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
	providerName := config.ProviderName
	if providerName == "" {
		providerName = "openai"
	}
	return &Client{
		apiKey:          config.APIKey,
		baseURL:         strings.TrimRight(baseURL, "/"),
		model:           model,
		client:          httpClient,
		pricingResolver: config.PricingResolver,
		providerName:    providerName,
	}, nil
}

func (c *Client) Complete(ctx context.Context, req harness.CompletionRequest) (harness.CompletionResult, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	payload := completionRequest{
		Model:         model,
		Messages:      mapMessages(req.Messages),
		Tools:         mapTools(req.Tools),
		ToolChoice:    "auto",
		Stream:        req.Stream != nil,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	if !payload.Stream {
		payload.StreamOptions = nil
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

	if payload.Stream {
		if httpRes.StatusCode >= 300 {
			responseBody, readErr := io.ReadAll(httpRes.Body)
			if readErr != nil {
				return harness.CompletionResult{}, fmt.Errorf("read error response body: %w", readErr)
			}
			return harness.CompletionResult{}, fmt.Errorf("openai request failed (%d): %s", httpRes.StatusCode, strings.TrimSpace(string(responseBody)))
		}
		return c.decodeStreamingResponse(model, httpRes.Body, req.Stream)
	}

	responseBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return harness.CompletionResult{}, fmt.Errorf("read response body: %w", err)
	}

	if httpRes.StatusCode >= 300 {
		return harness.CompletionResult{}, fmt.Errorf("openai request failed (%d): %s", httpRes.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return c.decodeCompletionResponse(model, responseBody)
}

func (c *Client) decodeCompletionResponse(model string, responseBody []byte) (harness.CompletionResult, error) {
	var response completionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return harness.CompletionResult{}, fmt.Errorf("decode response: %w", err)
	}
	return c.resultFromCompletionResponse(model, response)
}

func (c *Client) decodeStreamingResponse(model string, body io.Reader, streamFn func(harness.CompletionDelta)) (harness.CompletionResult, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var lines []string
	state := streamedCompletionState{}
	receivedDone := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := processStreamBlock(strings.Join(lines, "\n"), &state, streamFn)
			if err != nil {
				return harness.CompletionResult{}, err
			}
			if done {
				receivedDone = true
				break
			}
			lines = lines[:0]
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return harness.CompletionResult{}, fmt.Errorf("read stream: %w", err)
	}
	if !receivedDone {
		done, err := processStreamBlock(strings.Join(lines, "\n"), &state, streamFn)
		if err != nil {
			return harness.CompletionResult{}, err
		}
		receivedDone = done
	}
	if !receivedDone {
		return harness.CompletionResult{}, fmt.Errorf("stream ended before [DONE]")
	}

	response := completionResponse{
		Choices: []choice{{
			Message: chatCompletionMessage{
				Content:   state.content.String(),
				ToolCalls: state.toolCalls(),
			},
		}},
		Usage: state.usage,
	}
	return c.resultFromCompletionResponse(model, response)
}

func (c *Client) resultFromCompletionResponse(model string, response completionResponse) (harness.CompletionResult, error) {
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
	Model         string         `json:"model"`
	Messages      []chatMessage  `json:"messages"`
	Tools         []toolSpec     `json:"tools,omitempty"`
	ToolChoice    string         `json:"tool_choice,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
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

type completionChunk struct {
	Choices []chunkChoice `json:"choices"`
	Usage   *usage        `json:"usage,omitempty"`
}

type choice struct {
	Message chatCompletionMessage `json:"message"`
}

type chunkChoice struct {
	Delta        chatCompletionMessageDelta `json:"delta"`
	FinishReason *string                    `json:"finish_reason,omitempty"`
}

type chatCompletionMessage struct {
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls"`
}

type chatCompletionMessageDelta struct {
	Content          string              `json:"content,omitempty"`
	ReasoningContent string              `json:"reasoning_content,omitempty"`
	ToolCalls        []chatToolCallDelta `json:"tool_calls,omitempty"`
}

type chatToolCallDelta struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function chatToolCallDeltaField `json:"function,omitempty"`
}

type chatToolCallDeltaField struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type streamedCompletionState struct {
	content   strings.Builder
	reasoning strings.Builder
	usage     *usage
	toolCall  []*streamedToolCall
}

type streamedToolCall struct {
	ID        string
	Type      string
	Name      string
	Arguments strings.Builder
}

func processStreamBlock(raw string, state *streamedCompletionState, streamFn func(harness.CompletionDelta)) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return false, nil
	}

	dataLines := make([]string, 0, 4)
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if len(dataLines) == 0 {
		return false, nil
	}

	data := strings.Join(dataLines, "\n")
	if data == "[DONE]" {
		return true, nil
	}

	var chunk completionChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return false, fmt.Errorf("decode stream chunk: %w", err)
	}
	if chunk.Usage != nil {
		state.usage = chunk.Usage
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" {
			state.content.WriteString(choice.Delta.Content)
			if streamFn != nil {
				streamFn(harness.CompletionDelta{Content: choice.Delta.Content})
			}
		}
		if choice.Delta.ReasoningContent != "" {
			state.reasoning.WriteString(choice.Delta.ReasoningContent)
			if streamFn != nil {
				streamFn(harness.CompletionDelta{Reasoning: choice.Delta.ReasoningContent})
			}
		}
		for _, delta := range choice.Delta.ToolCalls {
			if delta.Index < 0 {
				return false, fmt.Errorf("invalid stream tool call index %d", delta.Index)
			}
			state.ensureToolCall(delta.Index)
			call := state.toolCall[delta.Index]
			if delta.ID != "" {
				call.ID = delta.ID
			}
			if delta.Type != "" {
				call.Type = delta.Type
			}
			if delta.Function.Name != "" {
				call.Name = delta.Function.Name
			}
			if delta.Function.Arguments != "" {
				call.Arguments.WriteString(delta.Function.Arguments)
			}
			if streamFn != nil {
				streamFn(harness.CompletionDelta{
					ToolCall: harness.ToolCallDelta{
						Index:     delta.Index,
						ID:        delta.ID,
						Name:      delta.Function.Name,
						Arguments: delta.Function.Arguments,
					},
				})
			}
		}
	}
	return false, nil
}

func (s *streamedCompletionState) ensureToolCall(index int) {
	for len(s.toolCall) <= index {
		s.toolCall = append(s.toolCall, &streamedToolCall{})
	}
}

func (s *streamedCompletionState) toolCalls() []chatToolCall {
	if len(s.toolCall) == 0 {
		return nil
	}
	out := make([]chatToolCall, 0, len(s.toolCall))
	for index, call := range s.toolCall {
		if call == nil {
			continue
		}
		callType := call.Type
		if callType == "" {
			callType = "function"
		}
		id := call.ID
		if id == "" {
			id = "call_" + strconv.Itoa(index)
		}
		out = append(out, chatToolCall{
			ID:   id,
			Type: callType,
			Function: chatToolCallFunction{
				Name:      call.Name,
				Arguments: call.Arguments.String(),
			},
		})
	}
	return slices.Clip(out)
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
	resolved, ok := c.pricingResolver.Resolve(c.providerName, model)
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
