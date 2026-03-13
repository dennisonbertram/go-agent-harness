package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/c-bata/go-prompt"
	"go-agent-harness/internal/provider/catalog"
	"golang.org/x/term"
)

// sessionStats tracks cumulative cost and token usage across runs.
type sessionStats struct {
	cost         float64
	inputTokens  int64
	outputTokens int64
}

func (s *sessionStats) update(payload map[string]interface{}) {
	if v, ok := payload["cumulative_cost_usd"]; ok {
		s.cost = toFloat(v)
	}
	// cumulative_usage is a nested object with prompt_tokens and completion_tokens.
	if usage, ok := payload["cumulative_usage"].(map[string]interface{}); ok {
		if v, ok := usage["prompt_tokens"]; ok {
			s.inputTokens = int64(toFloat(v))
		}
		if v, ok := usage["completion_tokens"]; ok {
			s.outputTokens = int64(toFloat(v))
		}
	}
}

// slashSuggestions defines all slash-command completions with descriptions.
var slashSuggestions = []prompt.Suggest{
	{Text: "/model", Description: "show current model"},
	{Text: "/model ", Description: "switch to model <name>"},
	{Text: "/models", Description: "list available models"},
	{Text: "/provider", Description: "show or set current provider"},
	{Text: "/provider ", Description: "set provider <name>"},
	{Text: "/details", Description: "toggle verbose tool output"},
	{Text: "/file ", Description: "attach file <path[:start-end]>"},
	{Text: "/settings", Description: "open settings menu"},
	{Text: "/clear", Description: "clear screen"},
	{Text: "/help", Description: "show help"},
	{Text: "/quit", Description: "exit the REPL"},
}

// completer returns go-prompt suggestions. It only activates when the input starts with '/'.
func completer(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()
	if !strings.HasPrefix(text, "/") {
		return nil
	}
	return prompt.FilterHasPrefix(slashSuggestions, text, true)
}

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
	currentProvider := ""
	display.PrintBanner(*url, currentModel)

	// Load model catalog (best-effort; nil if unavailable)
	var modelCatalog *catalog.Catalog
	if cat, err := catalog.LoadCatalog(*catalogPath); err == nil {
		modelCatalog = cat
	}

	// Session cost tracking
	stats := &sessionStats{}

	printSessionSummary := func() {
		if stats.cost > 0 || stats.inputTokens > 0 || stats.outputTokens > 0 {
			fmt.Println(display.color(colorDim, fmt.Sprintf(
				"Session cost: $%.5f | Tokens: %s in / %s out",
				stats.cost,
				formatTokens(stats.inputTokens),
				formatTokens(stats.outputTokens),
			)))
		}
	}

	// Save terminal state before go-prompt takes over. handleUserInput needs to
	// temporarily restore cooked mode to read mid-run answers via bufio.
	var termState *term.State
	if term.IsTerminal(int(os.Stdin.Fd())) {
		termState, _ = term.GetState(int(os.Stdin.Fd()))
	}

	// Load prompt history from disk (best-effort; errors are non-fatal).
	// Use historyDiskCap (1000) as the in-memory size so that all on-disk entries are
	// preserved across sessions: historyDefaultMax (100) would silently truncate the
	// disk file to the most recent 100 entries on the first Save of each session.
	hist := NewHistory(historyDiskCap)
	histPath := defaultHistoryPath()
	if err := hist.Load(histPath); err != nil {
		display.PrintError(fmt.Sprintf("history load: %v", err))
	}

	// saveHistory is a helper that persists history best-effort.
	saveHistory := func() {
		if err := hist.Save(histPath); err != nil {
			display.PrintError(fmt.Sprintf("history save: %v", err))
		}
	}

	// Graceful shutdown: save history before printing summary and exiting.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println()
		saveHistory()
		printSessionSummary()
		fmt.Println("Goodbye!")
		os.Exit(0)
	}()

	var conversationID string
	var pendingFileContent string // accumulated /file attachments for next prompt

	// livePrefix returns the prompt string shown by go-prompt.
	livePrefix := func() (string, bool) {
		if currentModel != "" {
			label := currentModel
			if currentProvider != "" {
				label = currentProvider + "/" + currentModel
			}
			return display.promptString(label), true
		}
		return display.promptString(""), true
	}

	// executor is called by go-prompt when the user presses Enter.
	executor := func(input string) {
		input = strings.TrimSpace(input)
		if input == "" {
			return
		}
		if input == "quit" || input == "exit" || input == "/quit" || input == "/exit" {
			saveHistory()
			printSessionSummary()
			fmt.Println("Goodbye!")
			os.Exit(0)
		}
		if handled, fileContent := handleCommand(input, &currentModel, &currentProvider, display, modelCatalog); handled {
			// Persist slash commands to disk history so they survive restarts.
			hist.Add(input)
			saveHistory()
			if fileContent != "" {
				pendingFileContent += fileContent
			}
			return
		}

		// Save non-command prompts to history.
		hist.Add(input)
		saveHistory()

		// Prepend any pending file content to the prompt
		userPrompt := input
		if pendingFileContent != "" {
			userPrompt = pendingFileContent + "\n" + input
			pendingFileContent = ""
		}

		runResp, err := client.CreateRun(userPrompt, currentModel, currentProvider, conversationID)
		if err != nil {
			display.PrintError(err.Error())
			return
		}

		// Track conversation for multi-turn
		if conversationID == "" {
			conversationID = runResp.RunID // first run's ID serves as conversation anchor
		}

		if err := streamRun(client, display, termState, runResp.RunID, userPrompt, stats); err != nil {
			display.PrintError(err.Error())
		}
	}

	p := prompt.New(
		executor,
		completer,
		prompt.OptionLivePrefix(livePrefix),
		prompt.OptionTitle("go-agent-harness"),
		prompt.OptionPrefix(""),
		prompt.OptionPrefixTextColor(prompt.Green),
		prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionCompletionOnDown(),
		prompt.OptionHistory(hist.Entries()),
	)
	p.Run()
}

// handleCommand processes slash commands.
// Returns (true, "") if the input was a handled command.
// Returns (true, content) if /file was used and content should be prepended to next prompt.
// Returns (false, "") if the input is not a command.
func handleCommand(input string, currentModel *string, currentProvider *string, display *Display, modelCatalog *catalog.Catalog) (bool, string) {
	if !strings.HasPrefix(input, "/") {
		return false, ""
	}
	parts := strings.Fields(input)
	switch parts[0] {
	case "/model":
		if len(parts) == 1 {
			display.PrintModelInfo(*currentModel)
		} else {
			*currentModel = parts[1]
			*currentProvider = "" // clear provider override when manually setting model
			display.PrintModelSwitched(parts[1])
		}
		return true, ""
	case "/models":
		if modelCatalog == nil {
			display.PrintModelsList(modelCatalog) // prints "catalog not available"
			return true, ""
		}
		if chosen, prov := selectModel(modelCatalog, display.NoColor); chosen != "" {
			*currentModel = chosen
			*currentProvider = prov
			display.PrintModelSwitched(chosen)
		}
		return true, ""
	case "/provider":
		if len(parts) == 1 {
			if *currentProvider != "" {
				fmt.Println(display.color(colorCyan, "Provider: "+*currentProvider))
			} else {
				fmt.Println(display.color(colorDim, "Provider: (auto-detected from model)"))
			}
		} else {
			*currentProvider = parts[1]
			fmt.Println(display.color(colorCyan, "Provider: "+parts[1]))
		}
		return true, ""
	case "/details":
		verbose := display.ToggleVerbose()
		if verbose {
			fmt.Println(display.color(colorCyan, "Tool output: verbose"))
		} else {
			fmt.Println(display.color(colorCyan, "Tool output: compact"))
		}
		return true, ""
	case "/file":
		if len(parts) < 2 {
			display.PrintError("usage: /file <path[:start-end]>")
			return true, ""
		}
		content, err := loadFileArg(parts[1])
		if err != nil {
			display.PrintError(fmt.Sprintf("file: %v", err))
			return true, ""
		}
		fmt.Println(display.color(colorDim, fmt.Sprintf("[attached: %s (%d chars)]", parts[1], len(content))))
		return true, content
	case "/settings":
		showSettings(display)
		return true, ""
	case "/clear":
		fmt.Print("\033[2J\033[H") // clear screen and move cursor to top-left
		return true, ""
	case "/help":
		display.PrintHelp()
		return true, ""
	}
	display.PrintError(fmt.Sprintf("unknown command: %s (try /help)", parts[0]))
	return true, ""
}

// loadFileArg reads file content for a /file argument.
// Supports optional line range: "path:start-end" (1-based, inclusive).
func loadFileArg(arg string) (string, error) {
	path := arg
	var startLine, endLine int

	// Check for line range suffix: path:10-30
	if idx := strings.LastIndex(arg, ":"); idx > 0 {
		rangePart := arg[idx+1:]
		if dash := strings.Index(rangePart, "-"); dash >= 0 {
			s, err1 := strconv.Atoi(rangePart[:dash])
			e, err2 := strconv.Atoi(rangePart[dash+1:])
			if err1 == nil && err2 == nil && s > 0 && e >= s {
				path = arg[:idx]
				startLine, endLine = s, e
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)
	if startLine > 0 {
		lines := strings.Split(content, "\n")
		if endLine > len(lines) {
			endLine = len(lines)
		}
		content = strings.Join(lines[startLine-1:endLine], "\n")
	}

	// Determine language hint from extension for fenced code block
	ext := ""
	if dot := strings.LastIndex(path, "."); dot >= 0 {
		ext = path[dot+1:]
	}
	return fmt.Sprintf("```%s\n// %s\n%s\n```\n", ext, path, content), nil
}

// envOrDefault returns the value of the named environment variable, or defaultVal if unset/empty.
func envOrDefault(name, defaultVal string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return defaultVal
}

// formatTokens formats a token count with commas for readability.
func formatTokens(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%s,%03d", formatTokens(n/1000), n%1000)
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

func streamRun(client *Client, display *Display, termState *term.State, runID, prompt string, stats *sessionStats) error {
	display.PrintRunStarted(runID, prompt)

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
			if stats != nil {
				stats.update(ev.Payload)
			}

		case "run.waiting_for_user":
			display.PrintWaitingForInput()
			if err := handleUserInput(client, display, termState, runID); err != nil {
				display.PrintError(fmt.Sprintf("input handling: %v", err))
			}

		case "run.completed":
			display.FlushAssistantMessage()
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

// handleUserInput reads answers to mid-run questions from the user.
// It temporarily restores the terminal to cooked mode (if in raw mode from go-prompt)
// so that bufio.Reader can read lines normally, then re-enables raw mode before returning.
func handleUserInput(client *Client, display *Display, termState *term.State, runID string) error {
	pending, err := client.GetPendingInput(runID)
	if err != nil {
		return err
	}

	// Restore cooked mode so bufio.Reader works correctly.
	stdinFd := int(os.Stdin.Fd())
	if termState != nil {
		_ = term.Restore(stdinFd, termState)
		defer func() {
			// Re-enter raw mode after we're done reading so go-prompt keeps working.
			_, _ = term.MakeRaw(stdinFd)
		}()
	}

	reader := bufio.NewReader(os.Stdin)
	answers := make(map[string]string)
	for _, q := range pending.Questions {
		display.PrintQuestion(q)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("input interrupted")
		}
		answer := resolveAnswer(strings.TrimRight(line, "\r\n"), q)
		answers[q.QuestionText] = answer
	}

	return client.SubmitInput(runID, answers)
}
