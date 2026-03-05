package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go-agent-harness/internal/harness"
	openai "go-agent-harness/internal/provider/openai"
	"go-agent-harness/internal/server"
)

type providerFactory func(config openai.Config) (harness.Provider, error)

var (
	runMain            = run
	exitFunc           = os.Exit
	runWithSignalsFunc = runWithSignals
)

func main() {
	if err := runMain(); err != nil {
		log.Printf("fatal: %v", err)
		exitFunc(1)
	}
}

func run() error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	return runWithSignalsFunc(sig, os.Getenv, func(config openai.Config) (harness.Provider, error) {
		return openai.NewClient(config)
	})
}

func runWithSignals(sig <-chan os.Signal, getenv func(string) string, newProvider providerFactory) error {
	if sig == nil {
		return fmt.Errorf("signal channel is required")
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if newProvider == nil {
		newProvider = func(config openai.Config) (harness.Provider, error) {
			return openai.NewClient(config)
		}
	}

	apiKey := getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required")
	}

	workspace := getenvOrDefault("HARNESS_WORKSPACE", ".")
	model := getenvOrDefault("HARNESS_MODEL", "gpt-4.1-mini")
	addr := getenvOrDefault("HARNESS_ADDR", ":8080")
	systemPrompt := getenvOrDefault("HARNESS_SYSTEM_PROMPT", "You are a practical coding assistant. Prefer using tools for file inspection and tests when needed.")
	maxSteps := getenvIntOrDefault("HARNESS_MAX_STEPS", 8)
	askUserTimeoutSeconds := getenvIntOrDefault("HARNESS_ASK_USER_TIMEOUT_SECONDS", 300)
	approvalMode := getenvToolApprovalModeOrDefault("HARNESS_TOOL_APPROVAL_MODE", harness.ToolApprovalModeFullAuto)

	provider, err := newProvider(openai.Config{
		APIKey:  apiKey,
		BaseURL: getenv("OPENAI_BASE_URL"),
		Model:   model,
	})
	if err != nil {
		return fmt.Errorf("create openai provider: %w", err)
	}

	askUserBroker := harness.NewInMemoryAskUserQuestionBroker(time.Now)
	tools := harness.NewDefaultRegistryWithOptions(workspace, harness.DefaultRegistryOptions{
		ApprovalMode:   approvalMode,
		Policy:         nil,
		AskUserBroker:  askUserBroker,
		AskUserTimeout: time.Duration(askUserTimeoutSeconds) * time.Second,
	})
	runner := harness.NewRunner(provider, tools, harness.RunnerConfig{
		DefaultModel:        model,
		DefaultSystemPrompt: systemPrompt,
		MaxSteps:            maxSteps,
		AskUserTimeout:      time.Duration(askUserTimeoutSeconds) * time.Second,
		AskUserBroker:       askUserBroker,
		ToolApprovalMode:    approvalMode,
	})

	handler := server.New(runner)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		log.Printf("harness server listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-sig:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	select {
	case err := <-serverErr:
		return err
	case <-serverDone:
	}
	return nil
}

func getenvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func getenvToolApprovalModeOrDefault(key string, fallback harness.ToolApprovalMode) harness.ToolApprovalMode {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch harness.ToolApprovalMode(value) {
	case harness.ToolApprovalModeFullAuto, harness.ToolApprovalModePermissions:
		return harness.ToolApprovalMode(value)
	default:
		return fallback
	}
}
