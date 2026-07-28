package main

import (
	"context"
	"log/slog"
	"os"

	"go-agent-harness/internal/modelstore"
	"go-agent-harness/internal/provider/catalog"
	"go-agent-harness/internal/provider/codex"
	"go-agent-harness/internal/provider/kimi"
)

// buildModelSettings loads the persisted model store and seeds it from the
// bundled catalog so the settings page has something to show on first open.
//
// Seeding never overwrites what the user configured: it only fills gaps. A
// failure here is not fatal — the daemon still serves completions, the
// settings page just reports that it is unavailable.
func buildModelSettings(path string, cat *catalog.Catalog, logger *slog.Logger) *modelstore.Service {
	if path == "" {
		path = modelstore.DefaultPath()
	}
	svc, err := modelstore.NewService(path)
	if err != nil {
		if logger != nil {
			logger.Warn("model settings unavailable", "path", path, "error", err)
		}
		return nil
	}
	if cat != nil {
		seedFromCatalog(svc, cat)
	}
	wireSubscriptionTokens(svc)
	return svc
}

// seedFromCatalog registers the bundled providers and their curated models.
//
// The catalog is the only source of pricing for providers whose own API does
// not report any — OpenAI's model endpoint returns just an id — so seeding it
// is what keeps costs visible before anyone types one in by hand.
func seedFromCatalog(svc *modelstore.Service, cat *catalog.Catalog) {
	for name, entry := range cat.Providers {
		protocol := entry.Protocol
		if protocol == "" {
			protocol = modelstore.ProtocolOpenAICompat
		}

		auth := modelstore.AuthAPIKey
		keyRef := ""
		switch {
		case entry.TokenSourceRequired:
			auth = modelstore.AuthSubscription
		case entry.APIKeyEnv != "":
			keyRef = modelstore.EnvRef(entry.APIKeyEnv)
			// Prefer a key already in the keychain over the environment, so a
			// key saved through the settings page wins on the next start.
			if modelstore.KeychainAvailable() {
				if _, err := modelstore.ResolveCredential(context.Background(), modelstore.KeychainRef(name)); err == nil {
					keyRef = modelstore.KeychainRef(name)
				}
			}
		case entry.APIKeyOptional:
			auth = modelstore.AuthNone
		}

		svc.SeedProvider(modelstore.Provider{
			Name:     name,
			BaseURL:  entry.BaseURL,
			Protocol: protocol,
			AuthKind: auth,
			KeyRef:   keyRef,
			// Verified against the live endpoint: it answers with an empty
			// set even for an authenticated ChatGPT subscription.
			NoListing: name == "codex-subscription",
		})

		models := make([]modelstore.Model, 0, len(entry.Models))
		for id, m := range entry.Models {
			entryModel := modelstore.Model{
				ID:            id,
				DisplayName:   m.DisplayName,
				ContextWindow: m.ContextWindow,
				MaxOutput:     m.MaxOutputTokens,
				Modalities:    m.Modalities,
			}
			if m.Pricing != nil {
				in := m.Pricing.InputPer1MTokensUSD
				out := m.Pricing.OutputPer1MTokensUSD
				entryModel.InputCost = &in
				entryModel.OutputCost = &out
			}
			models = append(models, entryModel)
		}
		if len(models) > 0 {
			svc.SeedModels(name, models)
		}
	}
}

// modelSettingsPath honours an override so a test or a second daemon can use
// its own store instead of the shared one.
func modelSettingsPath() string {
	if p := os.Getenv("HARNESS_MODEL_STORE_PATH"); p != "" {
		return p
	}
	return modelstore.DefaultPath()
}

// wireSubscriptionTokens teaches the model store how to obtain a token for
// each subscription provider.
//
// These credentials are short-lived and refreshed through the vendor's own
// flow, so the store cannot just read a file — without this a subscription
// provider fetches unauthenticated and the provider answers 401.
func wireSubscriptionTokens(svc *modelstore.Service) {
	// Read the Kimi CLI's own credential rather than refreshing a copy: the
	// harness cannot refresh (its OAuth client is rejected) and a copy goes
	// stale as soon as the CLI rotates its refresh token.
	svc.SetTokenResolver("kimi-subscription", kimi.NewVendorTokenSource("").WithRefresher(kimi.NewCLIRefresher()).Token)
	store := codex.DefaultStore()
	if source, err := codex.NewTokenSource(store, codex.NewRefreshFunc(nil, "", nil)); err == nil {
		svc.SetTokenResolver("codex-subscription", source.Token)
	}
}
