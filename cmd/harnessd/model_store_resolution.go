package main

import (
	"context"
	"strings"

	"go-agent-harness/internal/modelstore"
	"go-agent-harness/internal/provider/catalog"
)

// storeDiscoverer exposes a provider's stored model list to the registry's
// model→provider resolution.
//
// Without this, a model that exists only in the model store cannot be run.
// The picker offers it (the models endpoint reads the store), but starting a
// run resolves the model through the registry, which only knows the bundled
// catalog — so the run fails with "not found in any provider" for a model the
// UI just offered. Registering the store as a discovery source closes that gap.
type storeDiscoverer struct {
	service  *modelstore.Service
	provider string
}

func (d storeDiscoverer) Models(ctx context.Context) ([]catalog.DiscoveredModel, error) {
	_, fetched := d.service.Snapshot()
	entry := fetched[d.provider]
	out := make([]catalog.DiscoveredModel, 0, len(entry.Models))
	for _, m := range entry.Models {
		out = append(out, catalog.DiscoveredModel{
			ID:            m.ID,
			Name:          m.DisplayName,
			ContextWindow: m.ContextWindow,
		})
	}
	return out, nil
}

// registerStoreModels makes every provider in the model store resolvable and
// runnable.
//
// Two things are needed. Resolution consults registered discoverers, but skips
// any whose provider is absent from the catalog — so a user-added provider also
// needs a catalog entry before its models can be found. Client creation reads
// the same entry for the base URL and protocol.
func registerStoreModels(
	svc *modelstore.Service, registry *catalog.ProviderRegistry, cat *catalog.Catalog,
) {
	if svc == nil || registry == nil || cat == nil {
		return
	}
	providers, _ := svc.Snapshot()
	for name, p := range providers {
		if _, known := cat.Providers[name]; !known {
			// A provider the user added by hand. Give the catalog just enough
			// to resolve and construct a client for it.
			cat.Providers[name] = catalog.ProviderEntry{
				DisplayName:         name,
				BaseURL:             p.BaseURL,
				Protocol:            p.Protocol,
				APIKeyOptional:      p.AuthKind == modelstore.AuthNone,
				TokenSourceRequired: p.AuthKind == modelstore.AuthSubscription,
				Models:              map[string]catalog.Model{},
			}
		}
		registry.SetDiscovery(name, storeDiscoverer{service: svc, provider: name})
	}
}

// applyStoreCredentials hands credentials the model store can resolve to the
// provider registry, which builds the clients that runs actually use.
//
// Without this a key saved through the settings UI reaches the Keychain and the
// store — the settings page even confirms it resolves — while the registry goes
// on reading only environment variables. The model is listed, the credential
// reads as present, and the run fails with "API key env ... is not set". The
// two halves have to be told about each other.
//
// Environment references are skipped: the registry already reads those itself,
// and copying them in would only duplicate what it can see.
//
// Returns how many providers were handed a credential. The key itself is never
// logged or returned.
func applyStoreCredentials(
	ctx context.Context, svc *modelstore.Service, registry *catalog.ProviderRegistry,
) int {
	if svc == nil || registry == nil {
		return 0
	}
	providers, _ := svc.Snapshot()
	applied := 0
	for name, p := range providers {
		if p.AuthKind != modelstore.AuthAPIKey {
			continue
		}
		ref := strings.TrimSpace(p.KeyRef)
		if ref == "" || strings.HasPrefix(ref, modelstore.SchemeEnv+":") {
			continue
		}
		key, err := modelstore.ResolveCredential(ctx, ref)
		if err != nil || key == "" {
			continue
		}
		registry.SetAPIKey(name, key)
		applied++
	}
	return applied
}
