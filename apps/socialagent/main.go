// socialagent is a Telegram-facing social agent that delegates work to harnessd
// over HTTP. It never imports internal/ packages; all harness interaction is
// done via the public harnessd REST API.
package main

import (
	"log"
	"net/http"
	"os"

	"go-agent-harness/apps/socialagent/config"
	"go-agent-harness/apps/socialagent/db"
	"go-agent-harness/apps/socialagent/gateway"
	"go-agent-harness/apps/socialagent/harness"
	"go-agent-harness/apps/socialagent/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("socialagent: configuration error: %v", err)
		os.Exit(1)
	}

	// Log config at startup, redacting sensitive values.
	log.Printf("socialagent: starting up")
	log.Printf("  harness_url  = %s", cfg.HarnessURL)
	log.Printf("  listen_addr  = %s", cfg.ListenAddr)
	log.Printf("  database_url = %s", redact(cfg.DatabaseURL))
	log.Printf("  bot_token    = %s", redact(cfg.TelegramBotToken))
	log.Printf("  system_prompt= [%d chars]", len(cfg.SystemPrompt))

	store, err := db.NewStore(cfg.DatabaseURL)
	if err != nil {
		log.Printf("socialagent: db: %v", err)
		os.Exit(1)
	}
	defer store.Close()

	bot := telegram.NewBot(cfg.TelegramBotToken)
	harnessClient := harness.NewClient(cfg.HarnessURL)
	gw := gateway.NewGateway(bot, store, harnessClient, cfg.SystemPrompt)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook/telegram", gw.HandleWebhook)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	log.Printf("socialagent listening on %s", cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}

// redact replaces all but the first 4 characters of a string with asterisks,
// so tokens and URLs with credentials are not fully exposed in logs.
func redact(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "****"
}
