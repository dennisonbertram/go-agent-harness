package catalog

import (
	"context"
	"sync"
	"time"
)

// ProviderHealthState reports whether a provider's credentials actually work,
// which is a different question from whether any credential is present.
// "configured" only says a key or a token file exists; a subscription token
// that expired overnight is still configured and still cannot complete a run.
type ProviderHealthState string

const (
	// HealthUnconfigured — no credential at all.
	HealthUnconfigured ProviderHealthState = "unconfigured"
	// HealthOK — a credential was obtained just now, or a real request
	// against this provider recently succeeded.
	HealthOK ProviderHealthState = "ok"
	// HealthFailed — obtaining a credential failed, or a real request was
	// rejected as unauthorised.
	HealthFailed ProviderHealthState = "failed"
	// HealthUnverified — a credential is present but has never been exercised.
	// An API key cannot be validated without spending a request, so this is
	// the honest answer rather than an optimistic "ok".
	HealthUnverified ProviderHealthState = "unverified"
)

// ProviderHealth is one provider's credential verdict.
type ProviderHealth struct {
	State ProviderHealthState
	Error string
}

type healthEntry struct {
	health    ProviderHealth
	checkedAt time.Time
}

// healthTTL bounds how often a token source is asked for a credential. A
// listing request must not trigger a refresh storm, and a token that just
// failed will not start working within a few seconds.
const healthTTL = 60 * time.Second

var (
	healthMu    sync.Mutex
	healthCache = map[string]healthEntry{}
	// healthNow is swappable so tests can age the cache without sleeping.
	healthNow = time.Now
)

// CheckProviderHealth reports whether the named provider's credentials work.
//
// For a provider backed by a token source (the subscription providers) this
// asks for a credential, which is the same call a run makes — an expired
// refresh token fails here exactly as it would mid-run. For an API-key
// provider there is no way to tell without spending a real request, so the
// result stays "unverified" until a run reports back via ReportProviderAuth.
func (r *ProviderRegistry) CheckProviderHealth(ctx context.Context, name string) ProviderHealth {
	if r == nil {
		return ProviderHealth{State: HealthUnconfigured}
	}
	if !r.IsConfigured(name) {
		return ProviderHealth{State: HealthUnconfigured}
	}

	healthMu.Lock()
	entry, ok := healthCache[name]
	if ok && healthNow().Sub(entry.checkedAt) < healthTTL {
		healthMu.Unlock()
		return entry.health
	}
	healthMu.Unlock()

	r.mu.RLock()
	source := r.tokenSources[name]
	r.mu.RUnlock()

	health := ProviderHealth{State: HealthUnverified}
	if source != nil {
		// Bound the probe: a hung refresh must not stall a listing request.
		probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if _, err := source.Token(probeCtx); err != nil {
			health = ProviderHealth{State: HealthFailed, Error: err.Error()}
		} else {
			health = ProviderHealth{State: HealthOK}
		}
	}

	healthMu.Lock()
	healthCache[name] = healthEntry{health: health, checkedAt: healthNow()}
	healthMu.Unlock()
	return health
}

// ReportProviderAuth records what a real request observed, so an API-key
// provider that cannot be probed still becomes known-good or known-bad once it
// has actually been used. Call it with authOK=false only for a credential
// rejection (401/403), never for a rate limit or a server error — those say
// nothing about whether the key is valid.
func ReportProviderAuth(name string, authOK bool, message string) {
	if name == "" {
		return
	}
	health := ProviderHealth{State: HealthOK}
	if !authOK {
		health = ProviderHealth{State: HealthFailed, Error: message}
	}
	healthMu.Lock()
	healthCache[name] = healthEntry{health: health, checkedAt: healthNow()}
	healthMu.Unlock()
}

// ResetProviderHealth clears the cache. Used by tests and after a credential
// import, where the old verdict is stale by definition.
func ResetProviderHealth() {
	healthMu.Lock()
	healthCache = map[string]healthEntry{}
	healthMu.Unlock()
}
