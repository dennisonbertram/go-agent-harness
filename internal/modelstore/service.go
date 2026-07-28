package modelstore

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Service is the read/write API the HTTP layer uses. It owns loading, saving,
// credential resolution, and fetching so no handler has to sequence those.
type Service struct {
	path    string
	fetcher *Fetcher

	mu             sync.Mutex
	store          *Store
	tokenResolvers map[string]TokenResolver
}

// NewService loads the store from disk, or starts empty when absent.
func NewService(path string) (*Service, error) {
	store, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Service{path: path, store: store, fetcher: NewFetcher()}, nil
}

// SeedProvider registers a provider the bundled catalog knows about, without
// disturbing one the user has already configured. Seeding runs at startup so
// the settings page is populated on first open rather than blank.
func (s *Service) SeedProvider(p Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.store.Providers[p.Name]; exists {
		return
	}
	p.Builtin = true
	if p.Protocol == "" {
		p.Protocol = ProtocolOpenAICompat
	}
	if p.AuthKind == "" {
		p.AuthKind = AuthAPIKey
	}
	s.store.Providers[p.Name] = p
}

// SeedModels records catalog-supplied models for a provider that has never
// been fetched. It is how curated pricing reaches a provider whose own API
// does not report any — without it, an unfetched provider would show every
// model at "cost unknown".
func (s *Service) SeedModels(provider string, models []Model) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.store.Fetched[provider]; ok && len(existing.Models) > 0 {
		return
	}
	for i := range models {
		if models[i].InputCost != nil && models[i].CostSource == "" {
			models[i].CostSource = CostFromCatalog
		}
	}
	s.store.Fetched[provider] = Fetch{At: time.Time{}, Models: models}
}

// Snapshot returns a copy of the current state for rendering.
func (s *Service) Snapshot() (map[string]Provider, map[string]Fetch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	providers := make(map[string]Provider, len(s.store.Providers))
	for k, v := range s.store.Providers {
		providers[k] = v
	}
	fetched := make(map[string]Fetch, len(s.store.Fetched))
	for k, v := range s.store.Fetched {
		models := make([]Model, len(v.Models))
		copy(models, v.Models)
		fetched[k] = Fetch{At: v.At, Models: models, Error: v.Error}
	}
	return providers, fetched
}

// PutProvider adds or updates a provider. A non-empty secret is written to the
// location the provider's KeyRef names; the secret never reaches the store
// file itself.
func (s *Service) PutProvider(ctx context.Context, p Provider, secret string) error {
	// Validate before touching the credential store: writing the secret first
	// meant a rejected configuration had already replaced a working key.
	if err := ValidateProvider(&p); err != nil {
		return err
	}
	if secret != "" {
		if p.KeyRef == "" {
			// Default to the keychain where it exists, since the alternative
			// is a plaintext file.
			if KeychainAvailable() {
				p.KeyRef = KeychainRef(p.Name)
			} else {
				return fmt.Errorf(
					"no credential location for %q: set an env var reference, or run on macOS to use the keychain", p.Name)
			}
		}
		if err := StoreCredential(ctx, p.KeyRef, secret); err != nil {
			return err
		}
	}

	s.mu.Lock()
	if err := s.store.PutProvider(p); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return s.save()
}

// DeleteProvider removes a user-added provider and its stored credential.
func (s *Service) DeleteProvider(ctx context.Context, name string) error {
	s.mu.Lock()
	p, ok := s.store.Providers[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("provider %q is not configured", name)
	}
	if err := s.store.DeleteProvider(name); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()

	// Best effort: a leftover keychain entry is untidy, not broken, and must
	// not fail the delete the user already saw succeed.
	if p.KeyRef != "" {
		_ = DeleteCredential(ctx, p.KeyRef)
	}
	return s.save()
}

// FetchProvider pulls the provider's live model list and persists it.
// A failure is recorded against the provider and returned, leaving the
// previous list in place.
func (s *Service) FetchProvider(ctx context.Context, name string) (int, error) {
	s.mu.Lock()
	p, ok := s.store.Providers[name]
	s.mu.Unlock()
	if !ok {
		return 0, fmt.Errorf("provider %q is not configured", name)
	}

	if p.NoListing {
		return 0, fmt.Errorf(
			"provider %q does not publish a model list; its models come from the "+
				"bundled catalog and can be edited here", name)
	}

	var credential string
	var err error
	if p.AuthKind == AuthSubscription {
		// A subscription token comes from the vendor's refresh flow, not from
		// a file or an environment variable.
		resolve := s.tokenResolver(name)
		if resolve == nil {
			err = fmt.Errorf(
				"provider %q is a subscription provider with no token source wired; "+
					"log in with its vendor CLI and restart the daemon", name)
			s.recordError(name, err)
			return 0, err
		}
		credential, err = resolve(ctx)
		if err != nil {
			err = fmt.Errorf("obtain %s subscription token: %w", name, err)
			s.recordError(name, err)
			return 0, err
		}
	} else if p.AuthKind != AuthNone {
		credential, err = ResolveCredential(ctx, p.KeyRef)
	}
	// AuthNone deliberately resolves nothing. It used to resolve KeyRef anyway,
	// so switching a provider to "no credential needed" — especially alongside a
	// new base URL — still sent the old secret to the new endpoint.
	if err != nil {
		s.recordError(name, err)
		return 0, err
	}
	if credential == "" && p.AuthKind == AuthAPIKey {
		err := fmt.Errorf("provider %q has no credential; add a key before fetching", name)
		s.recordError(name, err)
		return 0, err
	}

	models, err := s.fetcher.Fetch(ctx, p, credential)
	if err != nil {
		s.recordError(name, err)
		return 0, err
	}

	s.mu.Lock()
	s.store.RecordFetch(name, models, time.Now())
	s.mu.Unlock()
	return len(models), s.save()
}

func (s *Service) recordError(provider string, err error) {
	s.mu.Lock()
	s.store.RecordFetchError(provider, err.Error(), time.Now())
	s.mu.Unlock()
	_ = s.save()
}

// SetExposed records which models the UI should offer.
func (s *Service) SetExposed(provider string, exposed map[string]bool) error {
	s.mu.Lock()
	err := s.store.SetExposed(provider, exposed)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.save()
}

// SetCost records a hand-entered price.
func (s *Service) SetCost(provider, model string, input, output float64) error {
	s.mu.Lock()
	err := s.store.SetCost(provider, model, input, output)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.save()
}

// ExposedModels returns the user's selection, and whether a selection exists
// at all. Callers must not treat "no selection" as "expose nothing" — that
// would empty the picker on a fresh install with no explanation.
func (s *Service) ExposedModels() (map[string][]Model, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.ExposedModels(), s.store.HasExposedSelection()
}

func (s *Service) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.Save(s.path)
}

// TokenResolver supplies a bearer credential for a subscription provider.
//
// Subscription logins are not static secrets: the token is short-lived and the
// vendor's refresh flow has to run to produce a usable one. That machinery
// already exists per provider, so the service takes a function rather than
// reaching for the credential itself — which also keeps this package free of
// provider imports.
type TokenResolver func(context.Context) (string, error)

// SetTokenResolver registers how to obtain a subscription provider's token.
func (s *Service) SetTokenResolver(provider string, resolve TokenResolver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokenResolvers == nil {
		s.tokenResolvers = map[string]TokenResolver{}
	}
	s.tokenResolvers[provider] = resolve
}

func (s *Service) tokenResolver(provider string) TokenResolver {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenResolvers[provider]
}

// CredentialStatus reports whether a usable credential can actually be
// obtained for a provider right now.
//
// This resolves the reference rather than checking that one was configured.
// A provider seeded with "env:CEREBRAS_API_KEY" has a reference whether or not
// that variable is set, so testing the reference reports "configured" for a
// provider that cannot authenticate — which is what made the settings page
// disagree with the fetch it had just refused.
func (s *Service) CredentialStatus(ctx context.Context, name string) bool {
	s.mu.Lock()
	p, ok := s.store.Providers[name]
	s.mu.Unlock()
	if !ok {
		return false
	}
	switch p.AuthKind {
	case AuthNone:
		return true
	case AuthSubscription:
		resolve := s.tokenResolver(name)
		if resolve == nil {
			return false
		}
		token, err := resolve(ctx)
		return err == nil && token != ""
	default:
		credential, err := ResolveCredential(ctx, p.KeyRef)
		return err == nil && credential != ""
	}
}
