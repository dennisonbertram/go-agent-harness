package main

import (
	"fmt"
	"os"
	"strings"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

type Display struct {
	NoColor bool
}

func NewDisplay(noColor bool) *Display {
	// Also respect NO_COLOR env var
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}
	return &Display{NoColor: noColor}
}

func (d *Display) color(code, text string) string {
	if d.NoColor {
		return text
	}
	return code + text + colorReset
}

func (d *Display) PrintDelta(content string) {
	fmt.Print(content)
}

func (d *Display) PrintThinkingDelta(content string) {
	fmt.Print(d.color(colorDim, content))
}

func (d *Display) PrintToolStart(toolName string) {
	fmt.Println(d.color(colorCyan, fmt.Sprintf("[tool: %s]", toolName)))
}

func (d *Display) PrintToolComplete(toolName string, result map[string]interface{}) {
	summary := ""
	if r, ok := result["result"]; ok {
		s := fmt.Sprintf("%v", r)
		if len(s) > 120 {
			s = s[:120] + "..."
		}
		summary = s
	}
	if summary != "" {
		fmt.Println(d.color(colorDim, fmt.Sprintf("  → %s", summary)))
	}
}

func (d *Display) PrintThinking() {
	fmt.Println(d.color(colorDim, "[thinking...]"))
}

func (d *Display) PrintRunStarted(runID string) {
	fmt.Println(d.color(colorDim, fmt.Sprintf("Run started: %s", runID)))
}

func (d *Display) PrintRunCompleted() {
	fmt.Println() // ensure newline after streaming
	fmt.Println(d.color(colorGreen, "--- run completed ---"))
}

func (d *Display) PrintRunFailed(errMsg string) {
	fmt.Println()
	fmt.Println(d.color(colorYellow, fmt.Sprintf("--- run failed: %s ---", errMsg)))
}

func (d *Display) PrintWaitingForInput() {
	fmt.Println()
	fmt.Println(d.color(colorYellow, "[agent is asking a question]"))
}

func (d *Display) PrintQuestion(q Question) {
	fmt.Println(d.color(colorBold, q.QuestionText))
	if len(q.Options) > 0 {
		for i, opt := range q.Options {
			desc := ""
			if opt.Description != "" {
				desc = d.color(colorDim, " — "+opt.Description)
			}
			fmt.Printf("  %d) %s%s\n", i+1, opt.Label, desc)
		}
		if q.MultiSelect {
			fmt.Print(d.color(colorDim, "(comma-separated numbers, or type a custom answer) "))
		} else {
			fmt.Print(d.color(colorDim, "(enter number or type a custom answer) "))
		}
	} else {
		fmt.Print("> ")
	}
}

func (d *Display) PrintUsage(payload map[string]interface{}) {
	cost, ok := payload["cumulative_cost_usd"]
	if !ok {
		return
	}
	fmt.Println(d.color(colorDim, fmt.Sprintf("[cost: $%.4f]", toFloat(cost))))
}

func (d *Display) PrintPrompt(model string) {
	if model != "" {
		fmt.Print(d.color(colorGreen, fmt.Sprintf("harness(%s)> ", model)))
	} else {
		fmt.Print(d.color(colorGreen, "harness> "))
	}
}

func (d *Display) PrintBanner(url, model string) {
	fmt.Println(d.color(colorBold, "Demo CLI for go-agent-harness"))
	fmt.Println(d.color(colorDim, fmt.Sprintf("Connected to %s", url)))
	if model != "" {
		fmt.Println(d.color(colorDim, fmt.Sprintf("Model: %s", model)))
	} else {
		fmt.Println(d.color(colorDim, "Model: (server default)"))
	}
	fmt.Println(d.color(colorDim, "Type 'quit', 'exit', or '/help' for commands. Ctrl-C to interrupt."))
	fmt.Println()
}

func (d *Display) PrintModelInfo(model string) {
	if model == "" {
		fmt.Println(d.color(colorCyan, "Model: (server default)"))
	} else {
		fmt.Println(d.color(colorCyan, fmt.Sprintf("Model: %s", model)))
	}
}

func (d *Display) PrintModelSwitched(model string) {
	fmt.Println(d.color(colorGreen, fmt.Sprintf("Switched to model: %s", model)))
}

func (d *Display) PrintHelp() {
	fmt.Println(d.color(colorBold, "Commands:"))
	fmt.Printf("  %s  show current model\n", d.color(colorCyan, "/model"))
	fmt.Printf("  %s  switch to a different model\n", d.color(colorCyan, "/model <name>"))
	fmt.Printf("  %s  show this help\n", d.color(colorCyan, "/help"))
	fmt.Printf("  %s  exit the REPL\n", d.color(colorDim, "quit / exit"))
}

func (d *Display) PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
}

// resolveAnswer converts user input to the appropriate answer string for a question
func resolveAnswer(input string, q Question) string {
	input = strings.TrimSpace(input)
	if len(q.Options) == 0 {
		return input
	}

	if q.MultiSelect {
		parts := strings.Split(input, ",")
		var labels []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			idx := 0
			if _, err := fmt.Sscanf(p, "%d", &idx); err == nil && idx >= 1 && idx <= len(q.Options) {
				labels = append(labels, q.Options[idx-1].Label)
			} else {
				// Treat as custom answer
				return input
			}
		}
		return strings.Join(labels, ",")
	}

	// Single select
	idx := 0
	if _, err := fmt.Sscanf(input, "%d", &idx); err == nil && idx >= 1 && idx <= len(q.Options) {
		return q.Options[idx-1].Label
	}
	return input
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}
