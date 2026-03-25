package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OpenRouterModel is the minimal live metadata needed from OpenRouter's
// public model listing endpoint.
type OpenRouterModel struct {
	ID            string
	Name          string
	ContextWindow int
}

// OpenRouterModelDiscoverer exposes cached live model lookup for OpenRouter.
type OpenRouterModelDiscoverer interface {
	Models(ctx context.Context) ([]OpenRouterModel, error)
}

// OpenRouterDiscoveryOptions configures OpenRouter live model discovery.
type OpenRouterDiscoveryOptions struct {
	Endpoint string
	Client   *http.Client
	TTL      time.Duration
	Now      func() time.Time
}

// OpenRouterDiscovery fetches and caches live model data from OpenRouter.
type OpenRouterDiscovery struct {
	endpoint string
	client   *http.Client
	ttl      time.Duration
	now      func() time.Time

	mu        sync.Mutex
	cached    []OpenRouterModel
	expiresAt time.Time
}

// NewOpenRouterDiscovery creates an OpenRouter discovery client with in-memory TTL caching.
func NewOpenRouterDiscovery(opts OpenRouterDiscoveryOptions) *OpenRouterDiscovery {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint == "" {
		endpoint = "https://openrouter.ai/api/v1/models"
	}
	return &OpenRouterDiscovery{
		endpoint: endpoint,
		client:   client,
		ttl:      ttl,
		now:      now,
	}
}

// Models returns cached live OpenRouter models when fresh, refreshes stale
// cache entries, and returns stale cached data if a refresh attempt fails.
func (d *OpenRouterDiscovery) Models(ctx context.Context) ([]OpenRouterModel, error) {
	if d == nil {
		return nil, fmt.Errorf("openrouter discovery is nil")
	}

	d.mu.Lock()
	if len(d.cached) > 0 && d.now().Before(d.expiresAt) {
		models := cloneOpenRouterModels(d.cached)
		d.mu.Unlock()
		return models, nil
	}
	cached := cloneOpenRouterModels(d.cached)
	d.mu.Unlock()

	models, err := d.fetch(ctx)
	if err != nil {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, err
	}

	d.mu.Lock()
	d.cached = cloneOpenRouterModels(models)
	d.expiresAt = d.now().Add(d.ttl)
	fresh := cloneOpenRouterModels(d.cached)
	d.mu.Unlock()
	return fresh, nil
}

func (d *OpenRouterDiscovery) fetch(ctx context.Context) ([]OpenRouterModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create openrouter request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch openrouter models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch openrouter models: status %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int    `json:"context_length"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode openrouter models: %w", err)
	}

	models := make([]OpenRouterModel, 0, len(payload.Data))
	for _, entry := range payload.Data {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		models = append(models, OpenRouterModel{
			ID:            id,
			Name:          strings.TrimSpace(entry.Name),
			ContextWindow: entry.ContextLength,
		})
	}
	return models, nil
}

func cloneOpenRouterModels(in []OpenRouterModel) []OpenRouterModel {
	if len(in) == 0 {
		return nil
	}
	out := make([]OpenRouterModel, len(in))
	copy(out, in)
	return out
}
