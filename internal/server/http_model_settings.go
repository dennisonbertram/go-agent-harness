package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"go-agent-harness/internal/modelstore"
)

// modelSettingsService is the subset of modelstore.Service the routes need,
// kept as an interface so tests can drive the handlers without a real store.
type modelSettingsService interface {
	Snapshot() (map[string]modelstore.Provider, map[string]modelstore.Fetch)
	PutProvider(ctx context.Context, p modelstore.Provider, secret string) error
	DeleteProvider(ctx context.Context, name string) error
	FetchProvider(ctx context.Context, name string) (int, error)
	SetExposed(provider string, exposed map[string]bool) error
	SetCost(provider, model string, input, output float64) error
	// CredentialStatus resolves the credential rather than reporting that one
	// was configured — the two differ whenever a referenced env var is unset.
	CredentialStatus(ctx context.Context, name string) bool
}

// modelSettingsProvider is one row of the settings page.
type modelSettingsProvider struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Protocol  string `json:"protocol"`
	AuthKind  string `json:"auth_kind"`
	Builtin   bool   `json:"builtin"`
	HasKeyRef bool   `json:"has_credential"`
	// KeyRef is echoed so the UI can say where a key lives. It names a
	// location, never a secret.
	KeyRef       string               `json:"key_ref,omitempty"`
	ModelCount   int                  `json:"model_count"`
	ExposedCount int                  `json:"exposed_count"`
	FetchedAt    string               `json:"fetched_at,omitempty"`
	FetchError   string               `json:"fetch_error,omitempty"`
	Models       []modelSettingsEntry `json:"models,omitempty"`
}

type modelSettingsEntry struct {
	ID            string   `json:"id"`
	DisplayName   string   `json:"display_name,omitempty"`
	ContextWindow int      `json:"context_window,omitempty"`
	InputCost     *float64 `json:"input_per_1m_tokens_usd,omitempty"`
	OutputCost    *float64 `json:"output_per_1m_tokens_usd,omitempty"`
	CostSource    string   `json:"cost_source,omitempty"`
	Modalities    []string `json:"modalities,omitempty"`
	Exposed       bool     `json:"exposed"`
}

func (s *Server) registerModelSettingsRoutes(
	mux *http.ServeMux,
	auth func(http.Handler) http.Handler,
	read func(http.Handler) http.Handler,
	admin func(http.Handler) http.Handler,
) {
	mux.Handle("/v1/model-settings", auth(read(http.HandlerFunc(s.handleModelSettings))))
	// Mutations are admin-gated: they write credentials and change what every
	// client can run.
	mux.Handle("/v1/model-settings/providers", auth(admin(http.HandlerFunc(s.handleModelSettingsProviders))))
	mux.Handle("/v1/model-settings/providers/", auth(admin(http.HandlerFunc(s.handleModelSettingsProviderByName))))
}

func (s *Server) modelSettings() modelSettingsService {
	if s.modelSettingsSvc == nil {
		return nil
	}
	return s.modelSettingsSvc
}

func (s *Server) requireModelSettings(w http.ResponseWriter) modelSettingsService {
	svc := s.modelSettings()
	if svc == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented",
			"model settings are not configured on this daemon")
		return nil
	}
	return svc
}

// handleModelSettings lists every provider with its models, for the settings
// page. This is the one place that returns models the user has *not* exposed —
// the picker needs the filtered view, the settings page needs the full one.
func (s *Server) handleModelSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	svc := s.requireModelSettings(w)
	if svc == nil {
		return
	}

	providers, fetched := svc.Snapshot()
	rows := make([]modelSettingsProvider, 0, len(providers))
	for name, p := range providers {
		entry := fetched[name]
		row := modelSettingsProvider{
			Name:       name,
			BaseURL:    p.BaseURL,
			Protocol:   p.Protocol,
			AuthKind:   string(p.AuthKind),
			Builtin:    p.Builtin,
			HasKeyRef:  svc.CredentialStatus(r.Context(), name),
			KeyRef:     p.KeyRef,
			ModelCount: len(entry.Models),
			FetchError: entry.Error,
		}
		if !entry.At.IsZero() {
			row.FetchedAt = entry.At.UTC().Format("2006-01-02T15:04:05Z")
		}
		for _, m := range entry.Models {
			if m.Exposed {
				row.ExposedCount++
			}
			row.Models = append(row.Models, modelSettingsEntry{
				ID:            m.ID,
				DisplayName:   m.DisplayName,
				ContextWindow: m.ContextWindow,
				InputCost:     m.InputCost,
				OutputCost:    m.OutputCost,
				CostSource:    m.CostSource,
				Modalities:    m.Modalities,
				Exposed:       m.Exposed,
			})
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"providers": rows})
}

type putProviderRequest struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Protocol string `json:"protocol"`
	AuthKind string `json:"auth_kind"`
	KeyRef   string `json:"key_ref"`
	// APIKey is write-only: it is stored at KeyRef and never returned.
	APIKey string `json:"api_key"`
}

// handleModelSettingsProviders adds or updates a provider.
func (s *Server) handleModelSettingsProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	svc := s.requireModelSettings(w)
	if svc == nil {
		return
	}

	var req putProviderRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body is not valid JSON")
		return
	}

	provider := modelstore.Provider{
		Name:     strings.TrimSpace(req.Name),
		BaseURL:  strings.TrimSpace(req.BaseURL),
		Protocol: strings.TrimSpace(req.Protocol),
		AuthKind: modelstore.AuthKind(strings.TrimSpace(req.AuthKind)),
		KeyRef:   strings.TrimSpace(req.KeyRef),
	}
	if err := svc.PutProvider(r.Context(), provider, req.APIKey); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "provider": provider.Name})
}

type exposeRequest struct {
	Exposed map[string]bool `json:"exposed"`
}

type costRequest struct {
	Model  string  `json:"model"`
	Input  float64 `json:"input_per_1m_tokens_usd"`
	Output float64 `json:"output_per_1m_tokens_usd"`
}

// handleModelSettingsProviderByName routes the per-provider actions:
//
//	DELETE /v1/model-settings/providers/{name}
//	POST   /v1/model-settings/providers/{name}/fetch
//	POST   /v1/model-settings/providers/{name}/expose
//	POST   /v1/model-settings/providers/{name}/cost
func (s *Server) handleModelSettingsProviderByName(w http.ResponseWriter, r *http.Request) {
	svc := s.requireModelSettings(w)
	if svc == nil {
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/v1/model-settings/providers/")
	name, action, _ := strings.Cut(rest, "/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "provider name is required")
		return
	}

	switch {
	case action == "" && r.Method == http.MethodDelete:
		if err := svc.DeleteProvider(r.Context(), name); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case action == "fetch" && r.Method == http.MethodPost:
		count, err := svc.FetchProvider(r.Context(), name)
		if err != nil {
			// The provider is reachable-or-not; that is the user's problem to
			// fix, not a server fault, so this is a 400 with the real reason.
			writeError(w, http.StatusBadGateway, "fetch_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "model_count": count})

	case action == "expose" && r.Method == http.MethodPost:
		var req exposeRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body is not valid JSON")
			return
		}
		if err := svc.SetExposed(name, req.Exposed); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case action == "cost" && r.Method == http.MethodPost:
		var req costRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body is not valid JSON")
			return
		}
		if req.Input < 0 || req.Output < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "costs cannot be negative")
			return
		}
		if err := svc.SetCost(name, req.Model, req.Input, req.Output); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		writeError(w, http.StatusNotFound, "not_found", "unknown model-settings action")
	}
}
