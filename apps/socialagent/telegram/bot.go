package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://api.telegram.org"

// Bot is a Telegram bot client.
type Bot struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewBot creates a Bot using the Telegram Bot API base URL.
func NewBot(token string) *Bot {
	return &Bot{
		token:      token,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{},
	}
}

// NewBotWithBaseURL creates a Bot with a custom base URL, useful for testing.
func NewBotWithBaseURL(token, baseURL string) *Bot {
	return &Bot{
		token:      token,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// apiResponse is the generic Telegram API response envelope.
type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// ParseUpdate reads the incoming webhook HTTP request and parses it into an
// Update. It returns an error if the body is not valid JSON, the message is
// missing, or the message has no text.
func (b *Bot) ParseUpdate(r *http.Request) (*Update, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("telegram: read request body: %w", err)
	}

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		return nil, fmt.Errorf("telegram: parse update JSON: %w", err)
	}

	if update.Message == nil {
		return nil, fmt.Errorf("telegram: update contains no message")
	}
	if strings.TrimSpace(update.Message.Text) == "" {
		return nil, fmt.Errorf("telegram: message has no text (only text messages are supported)")
	}

	return &update, nil
}

// SendMessage sends a text message to the given chat via the Telegram Bot API.
func (b *Bot) SendMessage(ctx context.Context, chatID int64, text string) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	return b.post(ctx, "sendMessage", payload)
}

// SetWebhook registers the given URL as the bot's incoming webhook.
// secretToken is sent with every update in the X-Telegram-Bot-Api-Secret-Token
// header so the server can authenticate requests as originating from Telegram.
func (b *Bot) SetWebhook(ctx context.Context, webhookURL string, secretToken string) error {
	payload := map[string]interface{}{
		"url":          webhookURL,
		"secret_token": secretToken,
	}
	return b.post(ctx, "setWebhook", payload)
}

// DisplayName returns a human-readable name for the user. It returns "Unknown"
// if the user is nil or has no name set.
func (b *Bot) DisplayName(u *User) string {
	if u == nil {
		return "Unknown"
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		return "Unknown"
	}
	return name
}

// post marshals payload as JSON and POSTs it to the given Telegram Bot API method.
func (b *Bot) post(ctx context.Context, method string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/%s", b.baseURL, b.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: HTTP request to %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("telegram: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: %s returned HTTP %d: %s", method, resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("telegram: parse response JSON: %w", err)
	}

	if !apiResp.OK {
		return fmt.Errorf("telegram: %s failed: %s", method, apiResp.Description)
	}

	return nil
}
