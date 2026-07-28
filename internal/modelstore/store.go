// Package modelstore persists the user's provider configuration and the model
// lists fetched from those providers.
//
// It exists because a static catalog goes stale the moment a provider ships a
// model, and because a picker listing every model a provider has ever served is
// unusable. The store lets the user add a provider, pull that provider's real
// current model list, and choose which of those models the UI offers.
package modelstore

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AuthKind is how a provider's credential is supplied.
type AuthKind string

const (
	// AuthAPIKey reads a key from an environment variable or the registry.
	AuthAPIKey AuthKind = "api_key"
	// AuthSubscription uses a vendor CLI login imported by the harness.
	AuthSubscription AuthKind = "subscription"
	// AuthNone is a local server that needs no credential.
	AuthNone AuthKind = "none"
)

// Provider is one endpoint the harness can send completions to.
type Provider struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	// Protocol selects the wire format: "openai_compat" or "anthropic".
	Protocol string   `json:"protocol"`
	AuthKind AuthKind `json:"auth_kind"`
	// KeyRef points at where the credential lives; the credential itself is
	// never written here. This file is configuration, not a secret store, so
	// it stays readable and diffable without leaking keys.
	//
	//	env:OPENAI_API_KEY              read from the environment
	//	keychain:<service>/<account>    macOS Keychain
	//	file:<path>                     a 0600 file on disk
	KeyRef string `json:"key_ref,omitempty"`
	// NoListing marks a provider whose endpoint does not publish a model list.
	// The ChatGPT Codex backend answers its /models route with an empty set
	// even when authenticated, so a fetch there is not a failure to report —
	// it simply has nothing to return, and the catalog stays authoritative.
	NoListing bool `json:"no_listing,omitempty"`
	// Builtin marks a provider seeded from the static catalog. Builtin
	// providers can be reconfigured but not deleted, so a bad edit can always
	// be undone by removing the override.
	Builtin bool `json:"builtin,omitempty"`
}

// Model is one model as last fetched from its provider.
//
// Costs are pointers because "unknown" and "free" are different facts and must
// not render the same. Most providers' model endpoints return neither pricing
// nor context window — OpenAI's returns only id/object/created/owned_by — so a
// fetch usually cannot fill these in and must not erase what is already known.
type Model struct {
	ID            string   `json:"id"`
	DisplayName   string   `json:"display_name,omitempty"`
	ContextWindow int      `json:"context_window,omitempty"`
	MaxOutput     int      `json:"max_output_tokens,omitempty"`
	InputCost     *float64 `json:"input_per_1m_tokens_usd,omitempty"`
	OutputCost    *float64 `json:"output_per_1m_tokens_usd,omitempty"`
	Modalities    []string `json:"modalities,omitempty"`
	// Exposed controls whether the model is offered in the UI. Defaulting to
	// false is the whole point: a provider with 341 models should contribute
	// nothing to the picker until the user picks from it.
	Exposed bool `json:"exposed"`
	// CostSource records where the pricing came from, so a figure the user
	// typed is never silently overwritten by a fetch that knows less.
	CostSource string `json:"cost_source,omitempty"`
}

// Cost provenance values.
const (
	CostFromProvider = "provider" // the provider's own endpoint returned it
	CostFromCatalog  = "catalog"  // seeded from the bundled static catalog
	CostFromUser     = "user"     // typed in by hand
)

// Fetch is one provider's model list as of a point in time.
type Fetch struct {
	At     time.Time `json:"at"`
	Models []Model   `json:"models"`
	// Error records why the last fetch failed, so the UI can show a stale
	// list and say why it is stale rather than showing nothing.
	Error string `json:"error,omitempty"`
}

// Store is the whole persisted document.
type Store struct {
	Providers map[string]Provider `json:"providers"`
	Fetched   map[string]Fetch    `json:"fetched"`

	mu sync.Mutex
}

// DefaultPath is where the store lives, alongside the credential files.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "models.json"
	}
	return filepath.Join(home, ".harness", "models.json")
}

// New returns an empty store.
func New() *Store {
	return &Store{
		Providers: map[string]Provider{},
		Fetched:   map[string]Fetch{},
	}
}

// Load reads the store, returning an empty one when the file does not exist.
// A missing file is the normal first-run state, not an error.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read model store: %w", err)
	}
	store := New()
	if err := json.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("parse model store %s: %w", path, err)
	}
	if store.Providers == nil {
		store.Providers = map[string]Provider{}
	}
	if store.Fetched == nil {
		store.Fetched = map[string]Fetch{}
	}
	return store, nil
}

// Save writes the store atomically. A half-written config file would strand
// the user with no providers and no obvious way back.
func (s *Store) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create model store directory: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode model store: %w", err)
	}
	data = append(data, '\n')

	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return fmt.Errorf("write model store: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return fmt.Errorf("commit model store: %w", err)
	}
	return nil
}

// ValidateProvider checks and normalises a provider without storing it.
//
// It is separate from PutProvider so a caller can reject bad input *before*
// writing a secret. Writing first meant a rejected update still replaced the
// live credential, and the API reported failure while the old configuration
// quietly started using the new key.
func ValidateProvider(p *Provider) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("provider name is required")
	}
	if strings.TrimSpace(p.BaseURL) == "" {
		return fmt.Errorf("provider %q: base URL is required", p.Name)
	}
	if p.Protocol == "" {
		p.Protocol = ProtocolOpenAICompat
	}
	if p.AuthKind == "" {
		p.AuthKind = AuthAPIKey
	}
	// An unrecognised value must not fall through to a default. A misspelled
	// protocol silently authenticated the OpenAI way, and a misspelled auth
	// kind skipped the missing-key check and fetched unauthenticated — both
	// surfacing as a confusing provider error rather than a config mistake.
	switch p.Protocol {
	case ProtocolOpenAICompat, ProtocolAnthropic:
	default:
		return fmt.Errorf("provider %q: unknown protocol %q (want %q or %q)",
			p.Name, p.Protocol, ProtocolOpenAICompat, ProtocolAnthropic)
	}
	switch p.AuthKind {
	case AuthAPIKey, AuthSubscription, AuthNone:
	default:
		return fmt.Errorf("provider %q: unknown auth kind %q (want %q, %q or %q)",
			p.Name, p.AuthKind, AuthAPIKey, AuthSubscription, AuthNone)
	}
	return nil
}

// PutProvider adds or replaces a provider.
func (s *Store) PutProvider(p Provider) error {
	if err := ValidateProvider(&p); err != nil {
		return err
	}
	name := p.Name
	s.mu.Lock()
	defer s.mu.Unlock()
	// Preserve builtin-ness: a user editing a catalog provider is overriding
	// it, not converting it into a deletable custom one.
	if existing, ok := s.Providers[name]; ok && existing.Builtin {
		p.Builtin = true
	}
	s.Providers[name] = p
	return nil
}

// DeleteProvider removes a user-added provider and its fetched models.
// Builtin providers cannot be deleted — they come from the bundled catalog and
// would reappear on restart, which would read as the delete silently failing.
func (s *Store) DeleteProvider(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.Providers[name]
	if !ok {
		return fmt.Errorf("provider %q is not configured", name)
	}
	if p.Builtin {
		return fmt.Errorf("provider %q is built in and cannot be removed; clear its credentials instead", name)
	}
	delete(s.Providers, name)
	delete(s.Fetched, name)
	return nil
}

// RecordFetch stores a freshly fetched model list, carrying forward the two
// things a fetch cannot know: which models the user exposed, and any pricing
// the provider does not report.
func (s *Store) RecordFetch(provider string, fetched []Model, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prior := map[string]Model{}
	for _, m := range s.Fetched[provider].Models {
		prior[m.ID] = m
	}

	merged := make([]Model, 0, len(fetched))
	for _, m := range fetched {
		if old, ok := prior[m.ID]; ok {
			m.Exposed = old.Exposed
			// Never let a fetch that reports no pricing erase a known cost.
			// A user-entered cost outranks anything the provider says.
			if old.CostSource == CostFromUser || m.InputCost == nil {
				if old.InputCost != nil {
					m.InputCost = old.InputCost
					m.OutputCost = old.OutputCost
					m.CostSource = old.CostSource
				}
			}
			if m.ContextWindow == 0 {
				m.ContextWindow = old.ContextWindow
			}
			if m.MaxOutput == 0 {
				m.MaxOutput = old.MaxOutput
			}
			// Same rule as pricing: an endpoint that reports less than the
			// catalog knew must not erase what it omits. OpenAI's listing
			// returns bare ids, so without this the first fetch stripped every
			// display name and modality the catalog had supplied.
			if m.DisplayName == "" {
				m.DisplayName = old.DisplayName
			}
			if len(m.Modalities) == 0 {
				m.Modalities = old.Modalities
			}
		}
		merged = append(merged, m)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ID < merged[j].ID })
	s.Fetched[provider] = Fetch{At: at, Models: merged}
}

// RecordFetchError marks a failed refresh without discarding the last good
// list — a provider being briefly unreachable should not empty the picker.
func (s *Store) RecordFetchError(provider string, message string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.Fetched[provider]
	// At describes the retained model list, which is still the one from the
	// last success. Stamping it with the failure's time made a week-old list
	// read as fetched just now — the opposite of what the field is for.
	entry.Error = message
	s.Fetched[provider] = entry
}

// SetExposed marks which of a provider's models the UI should offer.
func (s *Store) SetExposed(provider string, exposed map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.Fetched[provider]
	if !ok {
		return fmt.Errorf("provider %q has no fetched models; fetch first", provider)
	}
	for i := range entry.Models {
		if want, ok := exposed[entry.Models[i].ID]; ok {
			entry.Models[i].Exposed = want
		}
	}
	s.Fetched[provider] = entry
	return nil
}

// SetCost records a hand-entered price, which later fetches will not overwrite.
func (s *Store) SetCost(provider, modelID string, input, output float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A non-encodable price would make every subsequent save of the whole store
	// fail, so it is rejected here rather than at the first write.
	for _, v := range []float64{input, output} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("provider %q: %q is not a usable price", provider, modelID)
		}
	}
	entry, ok := s.Fetched[provider]
	if !ok {
		return fmt.Errorf("provider %q has no fetched models", provider)
	}
	for i := range entry.Models {
		if entry.Models[i].ID != modelID {
			continue
		}
		entry.Models[i].InputCost = &input
		entry.Models[i].OutputCost = &output
		entry.Models[i].CostSource = CostFromUser
		s.Fetched[provider] = entry
		return nil
	}
	return fmt.Errorf("provider %q has no model %q", provider, modelID)
}

// ExposedModels returns every model the user chose to offer, across providers.
func (s *Store) ExposedModels() map[string][]Model {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string][]Model{}
	for provider, entry := range s.Fetched {
		for _, m := range entry.Models {
			if m.Exposed {
				out[provider] = append(out[provider], m)
			}
		}
	}
	return out
}

// HasExposedSelection reports whether the user has exposed anything at all.
//
// Callers use this to decide whether the store governs the model list. Until
// the user makes a first selection an empty store must not be read as "expose
// nothing" — that would leave a fresh install with an empty picker and no
// indication why.
func (s *Store) HasExposedSelection() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.Fetched {
		for _, m := range entry.Models {
			if m.Exposed {
				return true
			}
		}
	}
	return false
}

// ProviderNames lists configured providers in a stable order.
func (s *Store) ProviderNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.Providers))
	for name := range s.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
