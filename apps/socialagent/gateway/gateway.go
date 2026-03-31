// Package gateway wires together the Telegram bot, Postgres user store, and
// Harness HTTP client into a single HTTP handler.
package gateway

import (
	"context"
	"log"
	"net/http"
	"sync"

	"go-agent-harness/apps/socialagent/db"
	"go-agent-harness/apps/socialagent/harness"
	"go-agent-harness/apps/socialagent/telegram"
)

// UserStore is the subset of db.Store used by the gateway.
type UserStore interface {
	GetOrCreateUser(ctx context.Context, telegramID int64, displayName string) (*db.User, error)
}

// HarnessRunner is the subset of harness.Client used by the gateway.
type HarnessRunner interface {
	SendAndWait(ctx context.Context, req harness.RunRequest) (*harness.RunResult, error)
}

// MessageSender is the subset of telegram.Bot used by the gateway.
type MessageSender interface {
	ParseUpdate(r *http.Request) (*telegram.Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	DisplayName(u *telegram.User) string
}

// Gateway ties together the Telegram bot, user store, and harness runner.
// It serializes requests per-user so that a single conversation_id is never
// used by two concurrent harness runs.
type Gateway struct {
	bot          MessageSender
	store        UserStore
	harness      HarnessRunner
	systemPrompt string
	mu           sync.Map // map[int64]*sync.Mutex
}

// NewGateway creates a Gateway.  bot, store, and harnessClient must be non-nil.
func NewGateway(bot MessageSender, store UserStore, harnessClient HarnessRunner, systemPrompt string) *Gateway {
	return &Gateway{
		bot:          bot,
		store:        store,
		harness:      harnessClient,
		systemPrompt: systemPrompt,
	}
}

// HandleWebhook is the HTTP handler for POST /webhook/telegram.
// It always returns 200 OK — returning any other status causes Telegram to
// retry the same update indefinitely.
func (g *Gateway) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Parse the incoming Telegram update.
	update, err := g.bot.ParseUpdate(r)
	if err != nil {
		// Not a text message or malformed JSON — acknowledge silently.
		log.Printf("gateway: parse update: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Guard against a missing From field (e.g. channel posts).
	if update.Message.From == nil {
		log.Printf("gateway: update has no From user")
		w.WriteHeader(http.StatusOK)
		return
	}

	telegramID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := update.Message.Text
	displayName := g.bot.DisplayName(update.Message.From)

	// 2. Acquire per-user mutex to prevent concurrent runs on the same conversation.
	mu := g.userMutex(telegramID)
	mu.Lock()
	defer mu.Unlock()

	// 3. Look up (or create) the internal user record.
	user, err := g.store.GetOrCreateUser(ctx, telegramID, displayName)
	if err != nil {
		log.Printf("gateway: GetOrCreateUser(%d): %v", telegramID, err)
		g.sendError(ctx, chatID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 4. Delegate to the harness.
	result, err := g.harness.SendAndWait(ctx, harness.RunRequest{
		Prompt:         text,
		ConversationID: user.ConversationID,
		SystemPrompt:   g.systemPrompt,
		TenantID:       user.ID,
	})
	if err != nil {
		log.Printf("gateway: SendAndWait (user=%d): %v", telegramID, err)
		g.sendError(ctx, chatID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 5. Send the agent's output back to the user.
	if err := g.bot.SendMessage(ctx, chatID, result.Output); err != nil {
		log.Printf("gateway: SendMessage (chat=%d): %v", chatID, err)
	}

	w.WriteHeader(http.StatusOK)
}

// userMutex returns the per-user mutex for telegramID, creating it if needed.
func (g *Gateway) userMutex(telegramID int64) *sync.Mutex {
	v, _ := g.mu.LoadOrStore(telegramID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// sendError sends the standard error message to chatID, logging any failure.
func (g *Gateway) sendError(ctx context.Context, chatID int64) {
	if err := g.bot.SendMessage(ctx, chatID, "Sorry, something went wrong. Please try again."); err != nil {
		log.Printf("gateway: sendError (chat=%d): %v", chatID, err)
	}
}
