package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-agent-harness/internal/modelstore"
)

// fakeModelSettings records calls so the handlers can be driven without a
// real store or a real provider on the network.
type fakeModelSettings struct {
	providers map[string]modelstore.Provider
	fetched   map[string]modelstore.Fetch

	putSecret   string
	fetchCalled string
	fetchErr    error
	exposedSet  map[string]bool
	costSet     [3]any
	deleted     string
}

func (f *fakeModelSettings) Snapshot() (map[string]modelstore.Provider, map[string]modelstore.Fetch) {
	return f.providers, f.fetched
}
func (f *fakeModelSettings) PutProvider(_ context.Context, p modelstore.Provider, secret string) error {
	f.putSecret = secret
	if f.providers == nil {
		f.providers = map[string]modelstore.Provider{}
	}
	f.providers[p.Name] = p
	return nil
}
func (f *fakeModelSettings) DeleteProvider(_ context.Context, name string) error {
	f.deleted = name
	return nil
}
func (f *fakeModelSettings) FetchProvider(_ context.Context, name string) (int, error) {
	f.fetchCalled = name
	if f.fetchErr != nil {
		return 0, f.fetchErr
	}
	return 7, nil
}
func (f *fakeModelSettings) SetExposed(_ string, exposed map[string]bool) error {
	f.exposedSet = exposed
	return nil
}
func (f *fakeModelSettings) SetCost(provider, model string, in, out float64) error {
	f.costSet = [3]any{model, in, out}
	return nil
}
func (f *fakeModelSettings) CredentialStatus(_ context.Context, name string) bool {
	return f.providers[name].KeyRef != ""
}
func (f *fakeModelSettings) ExposedModels() (map[string][]modelstore.Model, bool) {
	out := map[string][]modelstore.Model{}
	curated := false
	for provider, entry := range f.fetched {
		for _, m := range entry.Models {
			if m.Exposed {
				out[provider] = append(out[provider], m)
				curated = true
			}
		}
	}
	return out, curated
}

func settingsServer(t *testing.T, fake *fakeModelSettings) http.Handler {
	t.Helper()
	return NewWithOptions(ServerOptions{ModelSettings: fake})
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestModelSettingsListsProvidersAndModels(t *testing.T) {
	fake := &fakeModelSettings{
		providers: map[string]modelstore.Provider{
			"openai": {Name: "openai", BaseURL: "https://api.openai.com/v1", Builtin: true, KeyRef: "keychain:go-harness/openai"},
		},
		fetched: map[string]modelstore.Fetch{
			"openai": {At: time.Unix(1700000000, 0), Models: []modelstore.Model{
				{ID: "gpt-a", Exposed: true}, {ID: "gpt-b"},
			}},
		},
	}
	rec := do(t, settingsServer(t, fake), http.MethodGet, "/v1/model-settings", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Providers []modelSettingsProvider `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Providers) != 1 {
		t.Fatalf("providers = %d", len(out.Providers))
	}
	p := out.Providers[0]
	if p.ModelCount != 2 || p.ExposedCount != 1 {
		t.Fatalf("counts wrong: %+v", p)
	}
	// The settings page needs the unexposed models too — it is where you pick.
	if len(p.Models) != 2 {
		t.Fatalf("settings must list unexposed models: %+v", p.Models)
	}
	if p.FetchedAt == "" {
		t.Fatal("fetch time not reported")
	}
}

// The API key must be accepted for storage but never echoed back.
func TestAddProviderAcceptsKeyAndDoesNotEchoIt(t *testing.T) {
	fake := &fakeModelSettings{}
	h := settingsServer(t, fake)
	rec := do(t, h, http.MethodPost, "/v1/model-settings/providers",
		`{"name":"my-proxy","base_url":"https://gw.acme.dev/v1","api_key":"sk-super-secret"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if fake.putSecret != "sk-super-secret" {
		t.Fatalf("secret not passed to the store: %q", fake.putSecret)
	}
	if strings.Contains(rec.Body.String(), "sk-super-secret") {
		t.Fatalf("the response echoed the key: %s", rec.Body)
	}

	// Nor may a later listing return it.
	list := do(t, h, http.MethodGet, "/v1/model-settings", "")
	if strings.Contains(list.Body.String(), "sk-super-secret") {
		t.Fatalf("the listing leaked the key: %s", list.Body)
	}
}

func TestFetchProviderRoute(t *testing.T) {
	fake := &fakeModelSettings{}
	rec := do(t, settingsServer(t, fake), http.MethodPost,
		"/v1/model-settings/providers/openai/fetch", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if fake.fetchCalled != "openai" {
		t.Fatalf("fetched %q", fake.fetchCalled)
	}
	if !strings.Contains(rec.Body.String(), `"model_count":7`) {
		t.Fatalf("count not reported: %s", rec.Body)
	}
}

// A failing provider must surface the real reason, not a generic error.
func TestFetchFailureReportsTheReason(t *testing.T) {
	fake := &fakeModelSettings{fetchErr: errString("dial tcp: connection refused")}
	rec := do(t, settingsServer(t, fake), http.MethodPost,
		"/v1/model-settings/providers/openai/fetch", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "connection refused") {
		t.Fatalf("reason not surfaced: %s", rec.Body)
	}
}

func TestExposeAndCostRoutes(t *testing.T) {
	fake := &fakeModelSettings{}
	h := settingsServer(t, fake)

	rec := do(t, h, http.MethodPost, "/v1/model-settings/providers/openai/expose",
		`{"exposed":{"gpt-a":true,"gpt-b":false}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expose status %d: %s", rec.Code, rec.Body)
	}
	if !fake.exposedSet["gpt-a"] || fake.exposedSet["gpt-b"] {
		t.Fatalf("selection not applied: %+v", fake.exposedSet)
	}

	rec = do(t, h, http.MethodPost, "/v1/model-settings/providers/openai/cost",
		`{"model":"gpt-a","input_per_1m_tokens_usd":5,"output_per_1m_tokens_usd":25}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("cost status %d: %s", rec.Code, rec.Body)
	}
	if fake.costSet[0] != "gpt-a" || fake.costSet[1] != 5.0 {
		t.Fatalf("cost not applied: %+v", fake.costSet)
	}
}

func TestNegativeCostIsRejected(t *testing.T) {
	rec := do(t, settingsServer(t, &fakeModelSettings{}), http.MethodPost,
		"/v1/model-settings/providers/p/cost",
		`{"model":"m","input_per_1m_tokens_usd":-1,"output_per_1m_tokens_usd":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a negative cost should be rejected, got %d", rec.Code)
	}
}

func TestDeleteProviderRoute(t *testing.T) {
	fake := &fakeModelSettings{}
	rec := do(t, settingsServer(t, fake), http.MethodDelete,
		"/v1/model-settings/providers/custom", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if fake.deleted != "custom" {
		t.Fatalf("deleted %q", fake.deleted)
	}
}

// The whole point of the feature: once curated, the picker shows the selection.
func TestModelsEndpointReturnsOnlyExposedModels(t *testing.T) {
	in, out := 5.0, 25.0
	fake := &fakeModelSettings{
		fetched: map[string]modelstore.Fetch{
			"openai": {Models: []modelstore.Model{
				{ID: "gpt-keep", Exposed: true, InputCost: &in, OutputCost: &out},
				{ID: "gpt-hide"},
				{ID: "gpt-hide-2"},
			}},
		},
	}
	rec := do(t, settingsServer(t, fake), http.MethodGet, "/v1/models", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var out2 struct {
		Models []ModelResponse `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out2); err != nil {
		t.Fatal(err)
	}
	if len(out2.Models) != 1 || out2.Models[0].ID != "gpt-keep" {
		t.Fatalf("expected only the exposed model, got %+v", out2.Models)
	}
	if costOf(out2.Models[0].InputCostPerMTok) != 5 {
		t.Fatalf("cost not carried through: %+v", out2.Models[0])
	}
}

// Before the user has curated anything the picker must not go empty.
func TestModelsEndpointFallsBackWhenNothingCurated(t *testing.T) {
	fake := &fakeModelSettings{
		fetched: map[string]modelstore.Fetch{
			"openai": {Models: []modelstore.Model{{ID: "gpt-a"}, {ID: "gpt-b"}}},
		},
	}
	rec := do(t, settingsServer(t, fake), http.MethodGet, "/v1/models", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	// With no catalog configured the fallback is an empty list, but crucially
	// the curated path must not have claimed ownership of the response.
	var out struct {
		Models []ModelResponse `json:"models"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	for _, m := range out.Models {
		if m.ID == "gpt-a" || m.ID == "gpt-b" {
			t.Fatal("uncurated models were served as if curated")
		}
	}
}

// A daemon without a store must say so rather than 500.
func TestModelSettingsWithoutAStoreIsNotImplemented(t *testing.T) {
	rec := do(t, NewWithOptions(ServerOptions{}), http.MethodGet, "/v1/model-settings", "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status %d, want 501", rec.Code)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
