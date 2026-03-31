// Package summarizer generates privacy-safe user profile summaries by calling
// the harness LLM API and persisting the result via a ProfileStore.
package summarizer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go-agent-harness/apps/socialagent/harness"
)

const maxMessages = 50

const summarizationSystemPrompt = `You are a profile summarizer. Analyze the conversation and extract a concise user profile.

Return ONLY valid JSON with this exact structure:
{
  "summary": "2-3 sentence description of the user based on the conversation",
  "interests": ["keyword1", "keyword2", "keyword3"],
  "looking_for": "brief phrase describing what the user is looking for"
}

Rules:
- Strip all PII: phone numbers, email addresses, physical addresses, full names
- Keep the summary to 2-3 sentences maximum
- Interests should be single keywords or short phrases
- looking_for should be a brief phrase (e.g., "hiking partners", "dating", "professional networking")
- Return only the JSON object, no extra text or markdown`

// HarnessClient is the interface for making LLM calls via the harness API.
type HarnessClient interface {
	SendAndWait(ctx context.Context, req harness.RunRequest) (*harness.RunResult, error)
}

// ProfileStore is the interface for saving user profiles.
type ProfileStore interface {
	UpsertProfile(ctx context.Context, userID, summary string, interests []string, lookingFor string) error
}

// Message represents a single message in a conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ProfileSummary is the JSON structure returned by the LLM.
type ProfileSummary struct {
	Summary    string   `json:"summary"`
	Interests  []string `json:"interests"`
	LookingFor string   `json:"looking_for"`
}

// Summarizer generates and stores profile summaries for users.
type Summarizer struct {
	harness        HarnessClient
	store          ProfileStore
	harnessBaseURL string
}

// New creates a new Summarizer.
func New(harnessClient HarnessClient, store ProfileStore, harnessBaseURL string) *Summarizer {
	return &Summarizer{
		harness:        harnessClient,
		store:          store,
		harnessBaseURL: harnessBaseURL,
	}
}

// UpdateProfile generates a new summary for a user based on their conversation
// history and persists it via the ProfileStore.
func (s *Summarizer) UpdateProfile(ctx context.Context, userID, conversationID, displayName string) error {
	messages, err := s.fetchMessages(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("fetch messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	prompt := buildSummarizationPrompt(displayName, messages)

	result, err := s.harness.SendAndWait(ctx, harness.RunRequest{
		Prompt:         prompt,
		ConversationID: "summary-" + userID,
		SystemPrompt:   summarizationSystemPrompt,
	})
	if err != nil {
		return fmt.Errorf("harness summarize: %w", err)
	}

	var summary ProfileSummary
	if err := json.Unmarshal([]byte(result.Output), &summary); err != nil {
		// If JSON parsing fails, use the raw output as the summary.
		summary = ProfileSummary{Summary: result.Output}
	}

	return s.store.UpsertProfile(ctx, userID, summary.Summary, summary.Interests, summary.LookingFor)
}

// fetchMessages retrieves conversation messages from the harness API.
// GET {harnessBaseURL}/v1/conversations/{conversationID}/messages
func (s *Summarizer) fetchMessages(ctx context.Context, conversationID string) ([]Message, error) {
	url := fmt.Sprintf("%s/v1/conversations/%s/messages", s.harnessBaseURL, conversationID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("harness returned %d", resp.StatusCode)
	}

	var result struct {
		Messages []Message `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// buildSummarizationPrompt formats the last maxMessages messages into a prompt
// for the LLM to analyze and produce a JSON profile.
func buildSummarizationPrompt(displayName string, messages []Message) string {
	// Take only the last maxMessages messages.
	if len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Analyze the following conversation with user %q and produce a profile summary.\n\n", displayName))
	sb.WriteString("Conversation:\n")

	for _, m := range messages {
		role := m.Role
		if role == "user" {
			role = "User"
		} else {
			role = "Assistant"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, m.Content))
	}

	sb.WriteString("\nReturn a JSON profile summary as specified.")
	return sb.String()
}
