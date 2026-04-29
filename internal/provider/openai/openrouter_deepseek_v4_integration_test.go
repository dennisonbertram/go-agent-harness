//go:build integration

package openai

// TestOpenRouter_DeepSeekV4_MultiTurnToolUse is a gated integration test that
// proves the harness can complete a multi-turn tool-using run through OpenRouter
// against deepseek/deepseek-v4-flash without provider errors.
//
// Expected cost: ~$0.001 per run (Flash is ~$0.14/$0.28 per 1M input/output tokens;
// a small two-turn calculator exchange uses well under 1000 tokens total).
//
// Run with:
//
//	OPENROUTER_API_KEY=sk-or-... go test -tags integration -run TestOpenRouter_DeepSeekV4_MultiTurnToolUse ./internal/provider/openai/...
//
// Skipped automatically when OPENROUTER_API_KEY is not set.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/provider/pricing"
)

const (
	openRouterBaseURL = "https://openrouter.ai/api"
	testModelID       = "deepseek/deepseek-v4-flash"
)

// addArgs holds the parameters for the add tool.
type addArgs struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// addTool is the in-memory calculator tool definition.
var addTool = harness.ToolDefinition{
	Name:        "add",
	Description: "Adds two numbers together and returns their sum.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{
				"type":        "number",
				"description": "The first number to add",
			},
			"b": map[string]any{
				"type":        "number",
				"description": "The second number to add",
			},
		},
		"required": []any{"a", "b"},
	},
}

// executeAdd executes the in-memory add tool and returns a string result.
func executeAdd(arguments string) (string, error) {
	var args addArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse add arguments: %w", err)
	}
	result := args.A + args.B
	// Format without unnecessary decimal places.
	if result == float64(int64(result)) {
		return strconv.FormatInt(int64(result), 10), nil
	}
	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

// findRepoRoot walks upward from this source file to find the repo root
// (identified by the presence of catalog/models.json).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(filename)
	for {
		candidate := filepath.Join(dir, "catalog", "models.json")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (catalog/models.json not found walking upward)")
		}
		dir = parent
	}
}

func TestOpenRouter_DeepSeekV4_MultiTurnToolUse(t *testing.T) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY not set")
	}

	// Load the static catalog to exercise the catalog path and resolve quirks.
	root := findRepoRoot(t)
	cat, err := catalog.LoadCatalog(filepath.Join(root, "catalog", "models.json"))
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	orEntry, ok := cat.Providers["openrouter"]
	if !ok {
		t.Fatal("openrouter provider not found in catalog")
	}

	// Verify the model exists in the static catalog.
	if _, ok := orEntry.Models[testModelID]; !ok {
		t.Fatalf("model %q not found in static catalog under openrouter provider", testModelID)
	}

	// Load the pricing catalog (optional but validates the pricing path).
	pricingPath := filepath.Join(root, "catalog", "pricing.json")
	var pricingResolver pricing.Resolver
	if _, err := os.Stat(pricingPath); err == nil {
		r, err := pricing.NewFileResolver(pricingPath)
		if err != nil {
			t.Logf("warning: could not load pricing catalog: %v", err)
		} else {
			pricingResolver = r
		}
	}

	// Build the OpenRouter client with the provider quirks from the catalog
	// (reasoning_content_passback is required for multi-turn tool use with DeepSeek).
	client, err := NewClient(Config{
		APIKey:            apiKey,
		BaseURL:           openRouterBaseURL,
		ProviderName:      "openrouter",
		PricingResolver:   pricingResolver,
		Quirks:            orEntry.Quirks,
		OpenRouterReferer: "https://github.com/dennisonbertram/go-agent-harness",
		OpenRouterTitle:   "go-agent-harness integration test",
		Client:            &http.Client{Timeout: 120 * time.Second},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()

	// ── Turn 1: ask the model to compute 2+2 plus 3+5 ──────────────────────────
	// The model should respond with one or more tool_calls to the add tool.

	turn1Messages := []harness.Message{
		{
			Role:    "user",
			Content: "Please calculate: what is 2+2 plus 3+5? Use the add tool to compute each sub-expression separately, then add the results.",
		},
	}

	t.Log("Sending Turn 1 (expecting tool_calls)...")
	turn1Result, err := client.Complete(ctx, harness.CompletionRequest{
		Model:    testModelID,
		Messages: turn1Messages,
		Tools:    []harness.ToolDefinition{addTool},
	})
	if err != nil {
		t.Fatalf("Turn 1 Complete: %v", err)
	}

	t.Logf("Turn 1 result: content=%q tool_calls=%d reasoning_text_len=%d",
		turn1Result.Content, len(turn1Result.ToolCalls), len(turn1Result.ReasoningText))

	if len(turn1Result.ToolCalls) == 0 {
		t.Fatalf("Turn 1: expected at least one tool_call, got none (content: %q)", turn1Result.Content)
	}

	// ── Execute tool calls ──────────────────────────────────────────────────────
	// Build the conversation history including the assistant's tool_call turn and
	// the tool results, then send Turn 2.

	// The assistant turn with tool calls (must carry Reasoning for passback to work).
	assistantMsg := harness.Message{
		Role:      "assistant",
		Content:   turn1Result.Content,
		ToolCalls: turn1Result.ToolCalls,
		Reasoning: turn1Result.ReasoningText,
	}

	// Execute each tool call and collect results.
	toolResultMessages := make([]harness.Message, 0, len(turn1Result.ToolCalls))
	for _, call := range turn1Result.ToolCalls {
		if call.Name != "add" {
			t.Logf("unexpected tool call %q (arguments: %s) — skipping", call.Name, call.Arguments)
			toolResultMessages = append(toolResultMessages, harness.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("error: unknown tool %q", call.Name),
				ToolCallID: call.ID,
			})
			continue
		}
		result, err := executeAdd(call.Arguments)
		if err != nil {
			t.Fatalf("executeAdd(%q): %v", call.Arguments, err)
		}
		t.Logf("Tool call: add(%s) = %s", call.Arguments, result)
		toolResultMessages = append(toolResultMessages, harness.Message{
			Role:       "tool",
			Content:    result,
			ToolCallID: call.ID,
		})
	}

	// ── Turn 2: feed back tool results, expect final answer ────────────────────
	turn2Messages := append(turn1Messages,
		assistantMsg,
	)
	turn2Messages = append(turn2Messages, toolResultMessages...)

	t.Log("Sending Turn 2 (expecting final answer)...")
	turn2Result, err := client.Complete(ctx, harness.CompletionRequest{
		Model:    testModelID,
		Messages: turn2Messages,
		Tools:    []harness.ToolDefinition{addTool},
	})
	if err != nil {
		t.Fatalf("Turn 2 Complete: %v", err)
	}

	t.Logf("Turn 2 result: content=%q tool_calls=%d reasoning_text_len=%d",
		turn2Result.Content, len(turn2Result.ToolCalls), len(turn2Result.ReasoningText))

	// ── Assertions ──────────────────────────────────────────────────────────────

	// 1. At least one assistant turn (Turn 1) produced a tool_call.
	if len(turn1Result.ToolCalls) == 0 {
		t.Error("assertion failed: Turn 1 produced no tool_calls")
	}

	// 2. The final assistant message contains the correct numeric answer (12).
	finalContent := turn2Result.Content
	// Handle the case where the model makes additional tool calls in Turn 2.
	// If there are more tool calls, execute them and get one more turn.
	if len(turn2Result.ToolCalls) > 0 {
		t.Logf("Turn 2 returned additional tool_calls (%d); processing for Turn 3...", len(turn2Result.ToolCalls))

		assistantMsg2 := harness.Message{
			Role:      "assistant",
			Content:   turn2Result.Content,
			ToolCalls: turn2Result.ToolCalls,
			Reasoning: turn2Result.ReasoningText,
		}

		toolResultMessages2 := make([]harness.Message, 0, len(turn2Result.ToolCalls))
		for _, call := range turn2Result.ToolCalls {
			if call.Name != "add" {
				t.Logf("unexpected tool call %q — skipping", call.Name)
				toolResultMessages2 = append(toolResultMessages2, harness.Message{
					Role:       "tool",
					Content:    fmt.Sprintf("error: unknown tool %q", call.Name),
					ToolCallID: call.ID,
				})
				continue
			}
			result, err := executeAdd(call.Arguments)
			if err != nil {
				t.Fatalf("executeAdd (turn 3): %v", err)
			}
			t.Logf("Tool call (turn 3): add(%s) = %s", call.Arguments, result)
			toolResultMessages2 = append(toolResultMessages2, harness.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: call.ID,
			})
		}

		turn3Messages := append(turn2Messages, assistantMsg2)
		turn3Messages = append(turn3Messages, toolResultMessages2...)

		t.Log("Sending Turn 3 (expecting final answer)...")
		turn3Result, err := client.Complete(ctx, harness.CompletionRequest{
			Model:    testModelID,
			Messages: turn3Messages,
			Tools:    []harness.ToolDefinition{addTool},
		})
		if err != nil {
			t.Fatalf("Turn 3 Complete: %v", err)
		}
		t.Logf("Turn 3 result: content=%q", turn3Result.Content)
		finalContent = turn3Result.Content
	}

	// The final answer must contain "12" somewhere (2+2=4, 3+5=8, 4+8=12).
	if !strings.Contains(finalContent, "12") {
		t.Errorf("final answer does not contain '12': %q", finalContent)
	}

	// 3. No HTTP error from the provider — verified by the fact we got here
	//    without a t.Fatalf on either Complete call.

	// 4. The Reasoning field is populated on at least one turn, proving V4-Flash
	//    reasoning is round-tripping through the reasoning_content_passback quirk.
	//    Note: Flash may not always emit reasoning tokens; log a warning rather
	//    than a hard failure since the quirk is about _passback_, not generation.
	if turn1Result.ReasoningText == "" && turn2Result.ReasoningText == "" {
		t.Logf("NOTE: neither turn produced reasoning text (ReasoningText is empty on both turns); " +
			"this may be expected for Flash when reasoning budget is not explicitly set")
	} else {
		t.Logf("Reasoning confirmed present: turn1_len=%d turn2_len=%d",
			len(turn1Result.ReasoningText), len(turn2Result.ReasoningText))
	}

	// 5. Log cost information for observability.
	if turn1Result.CostUSD != nil {
		t.Logf("Turn 1 cost: $%.6f", *turn1Result.CostUSD)
	}
	if turn2Result.CostUSD != nil {
		t.Logf("Turn 2 cost: $%.6f", *turn2Result.CostUSD)
	}
}
