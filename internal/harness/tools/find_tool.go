package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go-agent-harness/internal/harness/tools/descriptions"
)

// findToolArgs are the arguments for the find_tool meta-tool.
type findToolArgs struct {
	Query string `json:"query"`
}

// buildFindToolDescription dynamically constructs the find_tool description
// by appending a one-line-per-tool catalog of deferred tools to the base description.
func buildFindToolDescription(deferredDefs []Definition) string {
	base := descriptions.Load("find_tool")
	if len(deferredDefs) == 0 {
		return base
	}

	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\nAvailable tools (use select:<name> to activate):\n")
	for _, def := range deferredDefs {
		summary := firstSentence(def.Description, 100)
		b.WriteString(fmt.Sprintf("- %s: %s\n", def.Name, summary))
	}
	return b.String()
}

// firstSentence extracts the first sentence from s, or truncates at maxLen.
// A sentence ends at the first period followed by a space or end-of-string.
func firstSentence(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Find first period followed by space or end of string.
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			// Period at end of string or followed by a space.
			if i+1 >= len(s) || s[i+1] == ' ' {
				sentence := s[:i+1]
				if len(sentence) > maxLen {
					return sentence[:maxLen-3] + "..."
				}
				return sentence
			}
		}
	}

	// No sentence-ending period found; truncate if needed.
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// FindToolTool creates the find_tool meta-tool that searches deferred tools
// and activates matches. It is a core-tier tool (always visible).
func FindToolTool(searcher ToolSearcher, deferredDefs []Definition, tracker ActivationTrackerInterface) Tool {
	def := Definition{
		Name:        "find_tool",
		Description: buildFindToolDescription(deferredDefs),
		Tier:        TierCore,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search keywords or 'select:<tool_name>' to activate a specific tool",
				},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		},
	}

	handler := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args findToolArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("invalid find_tool arguments: %w", err)
		}

		query := strings.TrimSpace(args.Query)
		if query == "" {
			return MarshalToolResult(map[string]any{
				"error": "query is required",
			})
		}

		// Extract runID from context
		runID := RunIDFromContext(ctx)
		if runID == "" {
			return "", fmt.Errorf("find_tool requires a run context")
		}

		// Direct select mode: "select:<tool_name>"
		if strings.HasPrefix(query, "select:") {
			toolName := strings.TrimSpace(strings.TrimPrefix(query, "select:"))
			return handleDirectSelect(toolName, deferredDefs, tracker, runID)
		}

		// Keyword search mode
		results := searcher.Search(query, deferredDefs)
		if len(results) == 0 {
			return MarshalToolResult(map[string]any{
				"message": "No matching tools found. Try different keywords.",
				"query":   query,
			})
		}

		// Auto-activate all matched tools
		var activated []string
		for _, r := range results {
			tracker.Activate(runID, r.Name)
			activated = append(activated, r.Name)
		}

		return MarshalToolResult(map[string]any{
			"message":   fmt.Sprintf("Found and activated %d tool(s). They are now available for use.", len(activated)),
			"activated": activated,
			"results":   results,
		})
	}

	return Tool{Definition: def, Handler: handler}
}

// handleDirectSelect activates a specific tool by name.
func handleDirectSelect(toolName string, deferredDefs []Definition, tracker ActivationTrackerInterface, runID string) (string, error) {
	if toolName == "" {
		return MarshalToolResult(map[string]any{
			"error": "tool name is required after 'select:'",
		})
	}

	// Find the tool in deferred definitions
	for _, def := range deferredDefs {
		if def.Name == toolName {
			tracker.Activate(runID, toolName)
			return MarshalToolResult(map[string]any{
				"message":   fmt.Sprintf("Tool '%s' activated and now available for use.", toolName),
				"activated": []string{toolName},
			})
		}
	}

	return MarshalToolResult(map[string]any{
		"error": fmt.Sprintf("Tool '%s' not found in available deferred tools.", toolName),
		"hint":  "Use a search query instead of select: to find tools by keyword.",
	})
}
