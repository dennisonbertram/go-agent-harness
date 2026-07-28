package modelstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Wire protocols a provider can speak.
const (
	ProtocolOpenAICompat = "openai_compat"
	ProtocolAnthropic    = "anthropic"
)

// Fetcher pulls a provider's current model list from that provider's own API.
//
// Each provider is the authority on which models it serves, so the list always
// comes from the provider rather than from an aggregator or a bundled file.
// Pricing is a separate matter: most model endpoints do not report it (OpenAI's
// returns only id/object/created/owned_by), so a fetch fills in costs
// opportunistically and the store keeps whatever it already knew.
type Fetcher struct {
	HTTP *http.Client
}

// NewFetcher returns a Fetcher with a bounded timeout. A settings page waiting
// on a wedged provider should fail rather than hang.
func NewFetcher() *Fetcher {
	return &Fetcher{HTTP: newFetchClient()}
}

// newFetchClient refuses redirects.
//
// Go strips Authorization when a redirect changes host, but it has no such rule
// for x-api-key, so an Anthropic-protocol provider answering /models with a
// redirect would hand the key to whatever host it named. A model listing has no
// legitimate need to redirect across hosts, so not following one at all is both
// safer and clearer than trying to decide which headers to keep.
func newFetchClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf(
				"refusing to follow a redirect to %s: a credential would be sent to it",
				req.URL.Host)
		},
	}
}

// Fetch retrieves the provider's models. The credential is resolved by the
// caller so this stays testable without touching a keychain or the environment.
func (f *Fetcher) Fetch(ctx context.Context, p Provider, credential string) ([]Model, error) {
	client := f.HTTP
	if client == nil {
		client = newFetchClient()
	}

	endpoint := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}

	switch p.Protocol {
	case ProtocolAnthropic:
		// Anthropic authenticates with x-api-key and requires a version header.
		if credential != "" {
			req.Header.Set("x-api-key", credential)
		}
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reach %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read models response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned %d: %s",
			endpoint, resp.StatusCode, summarizeBody(string(body)))
	}

	models, err := parseModels(body)
	if err != nil {
		return nil, fmt.Errorf("parse models from %s: %w", endpoint, err)
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("%s returned no models", endpoint)
	}
	return models, nil
}

// summarizeBody keeps an error readable when a provider answers with an HTML
// error page rather than JSON.
func summarizeBody(body string) string {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "<") {
		return "non-JSON response (HTML error page)"
	}
	if len(body) > 200 {
		return body[:200] + "…"
	}
	return body
}

// modelsEnvelope covers the shapes every provider in scope returns. The field
// set is a union rather than one struct per provider because the OpenAI-compat
// listing is near-universal and the extras are simply absent when unsupported.
type modelsEnvelope struct {
	Data []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`         // OpenRouter
		DisplayName string `json:"display_name"` // Anthropic

		// Context window, spelled differently per provider.
		ContextLength  int `json:"context_length"`   // OpenRouter
		MaxInputTokens int `json:"max_input_tokens"` // Anthropic
		MaxTokens      int `json:"max_tokens"`       // Anthropic (output cap)
		TopProvider    *struct {
			ContextLength       int `json:"context_length"`
			MaxCompletionTokens int `json:"max_completion_tokens"`
		} `json:"top_provider"` // OpenRouter

		// Pricing, when the provider reports it at all. OpenRouter quotes
		// per-token strings; everything else omits the field.
		Pricing *struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
		} `json:"pricing"`

		Architecture *struct {
			InputModalities []string `json:"input_modalities"`
		} `json:"architecture"` // OpenRouter
	} `json:"data"`
}

func parseModels(body []byte) ([]Model, error) {
	var env modelsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}

	out := make([]Model, 0, len(env.Data))
	for _, entry := range env.Data {
		if strings.TrimSpace(entry.ID) == "" {
			continue
		}
		m := Model{ID: entry.ID}

		if entry.DisplayName != "" {
			m.DisplayName = entry.DisplayName
		} else if entry.Name != "" {
			m.DisplayName = entry.Name
		}

		switch {
		case entry.ContextLength > 0:
			m.ContextWindow = entry.ContextLength
		case entry.MaxInputTokens > 0:
			m.ContextWindow = entry.MaxInputTokens
		case entry.TopProvider != nil && entry.TopProvider.ContextLength > 0:
			m.ContextWindow = entry.TopProvider.ContextLength
		}

		if entry.MaxTokens > 0 {
			m.MaxOutput = entry.MaxTokens
		} else if entry.TopProvider != nil && entry.TopProvider.MaxCompletionTokens > 0 {
			m.MaxOutput = entry.TopProvider.MaxCompletionTokens
		}

		// Per-token strings become per-million-token dollars, the unit the
		// rest of the harness prices in.
		if entry.Pricing != nil {
			if in, ok := perMillion(entry.Pricing.Prompt); ok {
				out2, ok2 := perMillion(entry.Pricing.Completion)
				if ok2 {
					m.InputCost = &in
					m.OutputCost = &out2
					m.CostSource = CostFromProvider
				}
			}
		}

		if entry.Architecture != nil && len(entry.Architecture.InputModalities) > 0 {
			m.Modalities = entry.Architecture.InputModalities
		}

		out = append(out, m)
	}
	return out, nil
}

// perMillion converts a per-token price string to dollars per million tokens.
// A malformed or absent value yields false rather than a zero price, because a
// zero would render as "free" and mislead a cost estimate.
func perMillion(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	// NaN and ±Inf parse cleanly and slip past "v < 0". They cannot be encoded
	// as JSON, so one such price would make every later save of the store fail
	// — the whole file, not just that model.
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, false
	}
	return v * 1_000_000, true
}
