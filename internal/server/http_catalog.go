package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"go-agent-harness/internal/harness"
	"go-agent-harness/internal/modelstore"
	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/provider/codex"
	"go-agent-harness/internal/provider/kimi"
)

// ModelResponse is the JSON shape for a single model in the /v1/models response.
type ModelResponse struct {
	ID       string   `json:"id"`
	Provider string   `json:"provider"`
	Aliases  []string `json:"aliases"`
	// Costs are pointers so an unknown rate is absent from the JSON rather
	// than serialised as 0. A plain float64 turned "we do not know what this
	// costs" into "$0 in · $0 out", which reads as free — and several
	// providers whose rate we genuinely do not have charge real money.
	InputCostPerMTok  *float64 `json:"input_cost_per_mtok,omitempty"`
	OutputCostPerMTok *float64 `json:"output_cost_per_mtok,omitempty"`
	// Modalities lists the model's input modalities (e.g. "text", "image")
	// from the catalog. Clients use it to pre-flight modality-gated input
	// such as image paste (epic #818). Omitted when unknown.
	Modalities []string `json:"modalities,omitempty"`
}

// ProviderResponse is the JSON shape for a single provider in the /v1/providers response.
type ProviderResponse struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	APIKeyEnv  string `json:"api_key_env"`
	AuthType   string `json:"auth_type,omitempty"`
	BaseURL    string `json:"base_url"`
	ModelCount int    `json:"model_count"`
	// Health distinguishes "a credential exists" from "the credential works":
	// unconfigured | ok | failed | unverified. A subscription token that
	// expired overnight is still Configured and still cannot complete a run.
	Health string `json:"health"`
	// HealthError carries why a failed provider failed, so a client can say
	// what to do about it instead of only hiding the models.
	HealthError string `json:"health_error,omitempty"`
}

func (s *Server) registerCatalogRoutes(
	mux *http.ServeMux,
	auth func(http.Handler) http.Handler,
	read func(http.Handler) http.Handler,
	write func(http.Handler) http.Handler,
	admin func(http.Handler) http.Handler,
) {
	mux.Handle("/v1/models", auth(read(http.HandlerFunc(s.handleModels))))
	mux.Handle("/v1/providers", auth(read(http.HandlerFunc(s.handleProviders))))
	mux.Handle("/v1/providers/", auth(admin(http.HandlerFunc(s.handleProviderByName))))
	mux.Handle("/v1/summarize", auth(write(http.HandlerFunc(s.handleSummarize))))
}

// handleProviders handles GET /v1/providers.
// Returns provider availability based on whether their API key env vars are set.
func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	if s.catalog == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []ProviderResponse{}})
		return
	}

	providerNames := make([]string, 0, len(s.catalog.Providers))
	for name := range s.catalog.Providers {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)

	providers := make([]ProviderResponse, 0, len(providerNames))
	for _, name := range providerNames {
		entry := s.catalog.Providers[name]
		configured := false
		if s.providerRegistry != nil {
			configured = s.providerRegistry.IsConfigured(name)
		} else {
			configured = os.Getenv(entry.APIKeyEnv) != ""
		}
		health := catalog.ProviderHealth{State: catalog.HealthUnconfigured}
		if configured && s.providerRegistry != nil {
			health = s.providerRegistry.CheckProviderHealth(r.Context(), name)
		} else if configured {
			health = catalog.ProviderHealth{State: catalog.HealthUnverified}
		}
		providers = append(providers, ProviderResponse{
			Name:        name,
			Configured:  configured,
			Health:      string(health.State),
			HealthError: health.Error,
			APIKeyEnv:   entry.APIKeyEnv,
			AuthType: func() string {
				if entry.TokenSourceRequired {
					return "subscription"
				}
				return "api_key"
			}(),
			BaseURL:    entry.BaseURL,
			ModelCount: len(entry.Models),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// handleProviderByName handles provider credential mutations. Subscription
// imports are trigger-only: the daemon reads vendor credentials from its own
// host and no credential is accepted over HTTP.
func (s *Server) handleProviderByName(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/providers/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	name := parts[0]
	if name == "" {
		http.NotFound(w, r)
		return
	}

	if s.providerRegistry == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "provider registry is not configured")
		return
	}

	switch parts[1] {
	case "key":
		if r.Method != http.MethodPut {
			writeMethodNotAllowed(w, http.MethodPut)
			return
		}
		var body struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain a non-empty \"key\" field")
			return
		}
		s.providerRegistry.SetAPIKey(name, body.Key)
		w.WriteHeader(http.StatusNoContent)
	case "import-subscription":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if name != "codex-subscription" && name != "kimi-subscription" {
			http.NotFound(w, r)
			return
		}
		if err := s.importSubscriptionProvider(name); err != nil {
			writeError(w, http.StatusBadRequest, "subscription_import_failed", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// importSubscriptionProvider reuses the same local import and token-source
// construction used by harnessd bootstrap. It intentionally accepts no caller
// supplied credential material: vendor files are read only on the daemon host.
func (s *Server) importSubscriptionProvider(name string) error {
	switch name {
	case "codex-subscription":
		store := codex.DefaultStore()
		if _, err := store.Import(codex.DefaultVendorAuthPath()); err != nil {
			return err
		}
		source, err := codex.NewTokenSource(store, codex.NewRefreshFunc(nil, "", nil))
		if err != nil {
			return err
		}
		s.providerRegistry.SetTokenSource(name, source)
		return nil
	case "kimi-subscription":
		storePath := kimi.DefaultStorePath()
		if err := kimi.Import(kimi.VendorCredentialPath(), storePath); err != nil {
			return err
		}
		source, err := kimi.NewTokenSource(storePath, "", nil)
		if err != nil {
			return err
		}
		s.providerRegistry.SetTokenSource(name, source)
		return nil
	default:
		return fmt.Errorf("subscription import is not supported for provider %s", name)
	}
}

// handleSummarize handles POST /v1/summarize.
// Accepts a list of messages and returns an LLM-generated summary.
func (s *Server) handleSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	summarizer := s.runner.GetSummarizer()
	if summarizer == nil {
		writeError(w, http.StatusServiceUnavailable, "summarizer_not_configured", "summarizer not configured")
		return
	}

	var req struct {
		Messages []harness.Message `json:"messages"`
		System   string            `json:"system"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "messages is required and must not be empty")
		return
	}

	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, map[string]any{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	summary, err := summarizer.SummarizeMessages(r.Context(), msgs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "summarize_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

// handleModels handles GET /v1/models.
// Returns the list of available models from the catalog.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	// Once the user has curated a selection in the model settings page, that
	// selection governs the picker. Before they ever curate one, fall through
	// to the catalog: an empty store means "not configured yet", not "expose
	// nothing", and the difference is an empty picker with no explanation.
	if svc, ok := s.modelSettingsSvc.(interface {
		ExposedModels() (map[string][]modelstore.Model, bool)
	}); ok && svc != nil {
		if exposed, curated := svc.ExposedModels(); curated {
			var chosen []ModelResponse
			for provider, list := range exposed {
				for _, m := range list {
					entry := ModelResponse{ID: m.ID, Provider: provider, Modalities: m.Modalities}
					// Only a rate we actually have is reported; unknown stays absent.
					if m.InputCost != nil && m.OutputCost != nil {
						entry.InputCostPerMTok = m.InputCost
						entry.OutputCostPerMTok = m.OutputCost
					} else if in, out, ok := s.catalogRate(provider, m.ID); ok {
						// The store holds no rate because most providers' model
						// endpoints do not report one. The curated catalog does,
						// so fall back to it rather than showing nothing for a
						// model whose price is perfectly well known.
						entry.InputCostPerMTok, entry.OutputCostPerMTok = in, out
					}
					chosen = append(chosen, entry)
				}
			}
			sort.Slice(chosen, func(i, j int) bool {
				if chosen[i].Provider != chosen[j].Provider {
					return chosen[i].Provider < chosen[j].Provider
				}
				return chosen[i].ID < chosen[j].ID
			})
			writeJSON(w, http.StatusOK, map[string]any{"models": chosen})
			return
		}
	}

	if s.catalog == nil && s.providerRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]any{"models": []ModelResponse{}})
		return
	}

	var models []ModelResponse
	if s.providerRegistry != nil {
		results := s.providerRegistry.ListModelsContext(r.Context())
		for _, result := range results {
			aliases := s.providerRegistry.ModelAliasesContext(r.Context(), result.Provider)[result.ModelID]
			if aliases == nil {
				aliases = []string{}
			}
			var inputCost, outputCost *float64
			if result.Model.Pricing != nil {
				in := result.Model.Pricing.InputPer1MTokensUSD
				out := result.Model.Pricing.OutputPer1MTokensUSD
				inputCost, outputCost = &in, &out
			}
			models = append(models, ModelResponse{
				ID:                result.ModelID,
				Provider:          result.Provider,
				Aliases:           aliases,
				InputCostPerMTok:  inputCost,
				OutputCostPerMTok: outputCost,
				Modalities:        result.Model.Modalities,
			})
		}
	} else {
		type providerAliases map[string][]string
		aliasMap := make(map[string]providerAliases)
		for providerName, providerEntry := range s.catalog.Providers {
			pa := make(providerAliases)
			for alias, target := range providerEntry.Aliases {
				pa[target] = append(pa[target], alias)
			}
			aliasMap[providerName] = pa
		}

		providerNames := make([]string, 0, len(s.catalog.Providers))
		for name := range s.catalog.Providers {
			providerNames = append(providerNames, name)
		}
		sort.Strings(providerNames)

		for _, providerName := range providerNames {
			providerEntry := s.catalog.Providers[providerName]
			pa := aliasMap[providerName]

			modelIDs := make([]string, 0, len(providerEntry.Models))
			for id := range providerEntry.Models {
				modelIDs = append(modelIDs, id)
			}
			sort.Strings(modelIDs)

			for _, modelID := range modelIDs {
				model := providerEntry.Models[modelID]
				aliases := pa[modelID]
				if aliases == nil {
					aliases = []string{}
				}
				sort.Strings(aliases)

				var inputCost, outputCost *float64
				if model.Pricing != nil {
					in := model.Pricing.InputPer1MTokensUSD
					out := model.Pricing.OutputPer1MTokensUSD
					inputCost, outputCost = &in, &out
				}

				models = append(models, ModelResponse{
					ID:                modelID,
					Provider:          providerName,
					Aliases:           aliases,
					InputCostPerMTok:  inputCost,
					OutputCostPerMTok: outputCost,
					Modalities:        model.Modalities,
				})
			}
		}
	}

	if models == nil {
		models = []ModelResponse{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// catalogRate looks up a curated rate for a model the store could not price.
func (s *Server) catalogRate(provider, modelID string) (*float64, *float64, bool) {
	if s == nil || s.catalog == nil {
		return nil, nil, false
	}
	entry, ok := s.catalog.Providers[provider]
	if !ok {
		return nil, nil, false
	}
	model, ok := entry.Models[modelID]
	if !ok || model.Pricing == nil {
		return nil, nil, false
	}
	in := model.Pricing.InputPer1MTokensUSD
	out := model.Pricing.OutputPer1MTokensUSD
	return &in, &out, true
}
