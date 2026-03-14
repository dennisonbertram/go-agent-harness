package training

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// truncationThreshold is the token count above which middle messages are dropped.
const truncationThreshold = 180000

// jsonlEntry is the on-disk format of a rollout JSONL record.
type jsonlEntry struct {
	Ts   time.Time      `json:"ts"`
	Seq  uint64         `json:"seq"`
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// ExportFromJSONL reads a rollout JSONL file and produces a TraceBundle.
func ExportFromJSONL(path string) (*TraceBundle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open rollout file: %w", err)
	}
	defer f.Close()

	var entries []jsonlEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e jsonlEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan rollout file: %w", err)
	}

	// Sort by sequence number for deterministic processing.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Seq < entries[j].Seq
	})

	bundle := &TraceBundle{
		Outcome: "unknown",
	}

	// Track tool calls by call_id for matching calls to results
	type pendingCall struct {
		name    string
		args    map[string]any
		stepIdx int
	}
	pendingCalls := make(map[string]*pendingCall)

	// Track seen tool call signatures for retry detection
	type callSig struct {
		name string
		args string
	}
	seenCalls := make(map[callSig]int) // signature -> count

	for _, e := range entries {
		switch e.Type {
		case "run.started":
			if v, ok := e.Data["run_id"].(string); ok {
				bundle.RunID = v
			}
			if v, ok := e.Data["prompt"].(string); ok {
				bundle.Messages = append(bundle.Messages, Message{
					Role:    "user",
					Content: v,
				})
			}
			if v, ok := e.Data["system_prompt"].(string); ok {
				bundle.SystemPrompt = v
			}
			if v, ok := e.Data["task_id"].(string); ok {
				bundle.TaskID = v
			}

		case "tool.call":
			name, _ := e.Data["name"].(string)
			callID, _ := e.Data["call_id"].(string)
			step := intFromData(e.Data, "step")
			var args map[string]any
			if a, ok := e.Data["args"].(map[string]any); ok {
				args = a
			}
			pendingCalls[callID] = &pendingCall{name: name, args: args, stepIdx: step}

		case "tool.result":
			callID, _ := e.Data["call_id"].(string)
			name, _ := e.Data["name"].(string)
			output, _ := e.Data["output"].(string)
			success, _ := e.Data["success"].(bool)
			step := intFromData(e.Data, "step")

			tc := ToolCallTrace{
				Name:    name,
				Output:  output,
				Success: success,
				StepIdx: step,
			}

			// Get args from pending call
			if pc, ok := pendingCalls[callID]; ok {
				tc.Args = pc.args
				tc.StepIdx = pc.stepIdx
				delete(pendingCalls, callID)
			}

			// Check for retry
			argsJSON, _ := json.Marshal(tc.Args)
			sig := callSig{name: tc.Name, args: string(argsJSON)}
			seenCalls[sig]++
			if seenCalls[sig] > 1 {
				tc.Retried = true
			}

			bundle.ToolCalls = append(bundle.ToolCalls, tc)

			// Add as message
			bundle.Messages = append(bundle.Messages, Message{
				Role:       "tool",
				Content:    output,
				ToolName:   name,
				ToolCallID: callID,
			})

		case "llm.completion.finished":
			if usage, ok := e.Data["usage"].(map[string]any); ok {
				if total, ok := usage["total_tokens"].(float64); ok {
					bundle.TokenCount = int(total)
				}
			}
			if cost, ok := e.Data["cost_usd"].(float64); ok {
				bundle.CostUSD += cost
			}
			if content, ok := e.Data["content"].(string); ok && content != "" {
				role := "assistant"
				if r, ok := e.Data["role"].(string); ok {
					role = r
				}
				bundle.Messages = append(bundle.Messages, Message{
					Role:    role,
					Content: content,
				})
			}

		case "context.window.snapshot":
			snap := ContextSnapshot{
				StepIdx:     intFromData(e.Data, "step"),
				TotalTokens: intFromData(e.Data, "max_context_tokens"),
				UsedTokens:  intFromData(e.Data, "estimated_total_tokens"),
			}
			if r, ok := e.Data["usage_ratio"].(float64); ok {
				snap.Ratio = r
			}
			bundle.ContextSnapshots = append(bundle.ContextSnapshots, snap)
			if snap.Ratio > bundle.MaxContextRatio {
				bundle.MaxContextRatio = snap.Ratio
			}

		case "anti_pattern.detected":
			ap := AntiPatternAlert{
				StepIdx: intFromData(e.Data, "step"),
			}
			if v, ok := e.Data["type"].(string); ok {
				ap.Type = v
			}
			if v, ok := e.Data["tool_name"].(string); ok {
				ap.Message = fmt.Sprintf("%s: %s", ap.Type, v)
			}
			bundle.AntiPatterns = append(bundle.AntiPatterns, ap)

		case "run.completed":
			bundle.Outcome = "pass"
			if v := intFromData(e.Data, "steps"); v > 0 {
				bundle.Steps = v
			}

		case "run.failed":
			bundle.Outcome = "fail"
			if v := intFromData(e.Data, "steps"); v > 0 {
				bundle.Steps = v
			}
		}
	}

	// Compute derived metrics.
	computeFirstTryRate(bundle)
	computeEfficiencyScore(bundle)
	applyTruncation(bundle)

	return bundle, nil
}

// computeFirstTryRate calculates the fraction of non-retried tool calls.
func computeFirstTryRate(b *TraceBundle) {
	if len(b.ToolCalls) == 0 {
		b.FirstTryRate = 0
		return
	}
	nonRetried := 0
	for _, tc := range b.ToolCalls {
		if !tc.Retried {
			nonRetried++
		}
	}
	b.FirstTryRate = float64(nonRetried) / float64(len(b.ToolCalls))
}

// computeEfficiencyScore = 1.0 / (steps * cost) normalized to [0,1].
func computeEfficiencyScore(b *TraceBundle) {
	steps := b.Steps
	if steps <= 0 {
		steps = 1
	}
	cost := b.CostUSD
	if cost <= 0 {
		cost = 0.001
	}
	raw := 1.0 / (float64(steps) * cost)
	// Normalize: cap at 1.0
	if raw > 1.0 {
		raw = 1.0
	}
	b.EfficiencyScore = raw
}

// applyTruncation drops middle messages if token count exceeds threshold.
func applyTruncation(b *TraceBundle) {
	if b.TokenCount <= truncationThreshold {
		return
	}
	b.Truncated = true
	b.TruncationStrategy = "middle_drop"

	msgCount := len(b.Messages)
	if msgCount <= 5 {
		return // too few messages to truncate
	}

	// Keep first 20% and last 30%
	keepFirst := int(float64(msgCount) * 0.20)
	keepLast := int(float64(msgCount) * 0.30)
	if keepFirst < 1 {
		keepFirst = 1
	}
	if keepLast < 1 {
		keepLast = 1
	}
	if keepFirst+keepLast >= msgCount {
		return
	}

	truncated := make([]Message, 0, keepFirst+keepLast+1)
	truncated = append(truncated, b.Messages[:keepFirst]...)
	truncated = append(truncated, Message{
		Role:    "system",
		Content: fmt.Sprintf("[%d messages truncated]", msgCount-keepFirst-keepLast),
	})
	truncated = append(truncated, b.Messages[msgCount-keepLast:]...)

	b.TruncatedTokens = b.TokenCount // original count before truncation
	b.Messages = truncated
}

// intFromData extracts an int from a map[string]any, handling float64 JSON decoding.
func intFromData(data map[string]any, key string) int {
	v, ok := data[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
