package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"go-agent-harness/internal/provider/catalog"
)

func main() {
	url := flag.String("url", "http://localhost:8080", "Harness server URL")
	model := flag.String("model", "", "Model to use (default: server default)")
	noColor := flag.Bool("no-color", false, "Disable colored output")
	catalogPath := flag.String("catalog", envOrDefault("HARNESS_MODEL_CATALOG_PATH", "catalog/models.json"), "Path to model catalog JSON")
	flag.Parse()

	client := NewClient(*url)
	display := NewDisplay(*noColor)

	// Health check
	if err := client.HealthCheck(); err != nil {
		display.PrintError(fmt.Sprintf("Cannot connect to %s: %v", *url, err))
		os.Exit(1)
	}

	currentModel := *model
	display.PrintBanner(*url, currentModel)

	// Load model catalog (best-effort; nil if unavailable)
	var modelCatalog *catalog.Catalog
	if cat, err := catalog.LoadCatalog(*catalogPath); err == nil {
		modelCatalog = cat
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nGoodbye!")
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)
	var conversationID string

	for {
		display.PrintPrompt(currentModel)
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "quit" || input == "exit" || input == "/quit" || input == "/exit" {
			fmt.Println("Goodbye!")
			break
		}
		if handleCommand(input, &currentModel, display, modelCatalog) {
			continue
		}

		runResp, err := client.CreateRun(input, currentModel, conversationID)
		if err != nil {
			display.PrintError(err.Error())
			continue
		}

		// Track conversation for multi-turn
		if conversationID == "" {
			conversationID = runResp.RunID // first run's ID serves as conversation anchor
		}

		if err := streamRun(client, display, scanner, runResp.RunID); err != nil {
			display.PrintError(err.Error())
		}
	}
}

// handleCommand processes slash commands. Returns true if the input was a command.
func handleCommand(input string, currentModel *string, display *Display, modelCatalog *catalog.Catalog) bool {
	if !strings.HasPrefix(input, "/") {
		return false
	}
	parts := strings.Fields(input)
	switch parts[0] {
	case "/model":
		if len(parts) == 1 {
			display.PrintModelInfo(*currentModel)
		} else {
			*currentModel = parts[1]
			display.PrintModelSwitched(parts[1])
		}
		return true
	case "/models":
		display.PrintModelsList(modelCatalog)
		return true
	case "/help":
		display.PrintHelp()
		return true
	}
	display.PrintError(fmt.Sprintf("unknown command: %s (try /help)", parts[0]))
	return true
}

// envOrDefault returns the value of the named environment variable, or defaultVal if unset/empty.
func envOrDefault(name, defaultVal string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return defaultVal
}

// providerOrder returns provider keys from the catalog sorted alphabetically.
func providerOrder(providers map[string]catalog.ProviderEntry) []string {
	keys := make([]string, 0, len(providers))
	for k := range providers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// modelOrder returns model keys from a provider sorted alphabetically.
func modelOrder(models map[string]catalog.Model) []string {
	keys := make([]string, 0, len(models))
	for k := range models {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func streamRun(client *Client, display *Display, scanner *bufio.Scanner, runID string) error {
	display.PrintRunStarted(runID)

	return client.StreamEvents(runID, func(ev Event) error {
		switch ev.Type {
		case "assistant.message.delta":
			if content, ok := ev.Payload["content"].(string); ok {
				display.PrintDelta(content)
			}

		case "assistant.thinking.delta":
			if content, ok := ev.Payload["content"].(string); ok {
				display.PrintThinkingDelta(content)
			}

		case "tool.call.started":
			if name, ok := ev.Payload["tool"].(string); ok {
				display.PrintToolStart(name)
			}

		case "tool.call.completed":
			if name, ok := ev.Payload["tool"].(string); ok {
				display.PrintToolComplete(name, ev.Payload)
			}

		case "llm.turn.requested":
			display.PrintThinking()

		case "usage.delta":
			display.PrintUsage(ev.Payload)

		case "run.waiting_for_user":
			display.PrintWaitingForInput()
			if err := handleUserInput(client, display, scanner, runID); err != nil {
				display.PrintError(fmt.Sprintf("input handling: %v", err))
			}

		case "run.completed":
			display.PrintRunCompleted()

		case "run.failed":
			errMsg := "unknown error"
			if e, ok := ev.Payload["error"].(string); ok {
				errMsg = e
			}
			display.PrintRunFailed(errMsg)
		}

		return nil
	})
}

func handleUserInput(client *Client, display *Display, scanner *bufio.Scanner, runID string) error {
	pending, err := client.GetPendingInput(runID)
	if err != nil {
		return err
	}

	answers := make(map[string]string)
	for _, q := range pending.Questions {
		display.PrintQuestion(q)
		if !scanner.Scan() {
			return fmt.Errorf("input interrupted")
		}
		answer := resolveAnswer(scanner.Text(), q)
		answers[q.QuestionText] = answer
	}

	return client.SubmitInput(runID, answers)
}
