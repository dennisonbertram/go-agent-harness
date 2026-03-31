// socialagent is a Telegram-facing social agent that delegates work to harnessd
// over HTTP. It never imports internal/ packages; all harness interaction is
// done via the public harnessd REST API.
package main

import (
	"log"
	"os"

	"go-agent-harness/apps/socialagent/config"
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

	// TODO: wire up Telegram gateway, harness HTTP client, and HTTP server.
	_ = cfg
}

// redact replaces all but the first 4 characters of a string with asterisks,
// so tokens and URLs with credentials are not fully exposed in logs.
func redact(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "****"
}
