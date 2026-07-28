package catalog

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubTokenSource struct {
	token string
	err   error
	calls int
}

func (s *stubTokenSource) Token(context.Context) (string, error) {
	s.calls++
	return s.token, s.err
}

func testRegistry(t *testing.T) *ProviderRegistry {
	t.Helper()
	ResetProviderHealth()
	catalog, err := LoadCatalog("../../../catalog/models.json")
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return NewProviderRegistryWithEnv(catalog, func(string) string { return "" })
}

// The case that started this: a Kimi subscription token that expired overnight
// is still "configured" and still cannot complete a run. Health has to tell
// those apart or the picker keeps offering a model that always fails.
func TestExpiredTokenReportsFailedNotConfigured(t *testing.T) {
	reg := testRegistry(t)
	source := &stubTokenSource{err: errors.New("Kimi token refresh returned HTTP 401")}
	reg.SetTokenSource("kimi-subscription", source)

	if !reg.IsConfigured("kimi-subscription") {
		t.Fatal("a provider with a token source should count as configured")
	}
	health := reg.CheckProviderHealth(context.Background(), "kimi-subscription")
	if health.State != HealthFailed {
		t.Fatalf("health = %q, want %q", health.State, HealthFailed)
	}
	if health.Error == "" {
		t.Fatal("a failed provider must say why")
	}
}

func TestWorkingTokenReportsOK(t *testing.T) {
	reg := testRegistry(t)
	reg.SetTokenSource("codex-subscription", &stubTokenSource{token: "tok"})

	if got := reg.CheckProviderHealth(context.Background(), "codex-subscription").State; got != HealthOK {
		t.Fatalf("health = %q, want %q", got, HealthOK)
	}
}

func TestUnconfiguredProviderIsNotProbed(t *testing.T) {
	reg := testRegistry(t)
	if got := reg.CheckProviderHealth(context.Background(), "deepseek").State; got != HealthUnconfigured {
		t.Fatalf("health = %q, want %q", got, HealthUnconfigured)
	}
}

// An API key cannot be validated without spending a request, so claiming "ok"
// would be a guess. It stays unverified until a real run reports back.
func TestAPIKeyProviderIsUnverifiedUntilUsed(t *testing.T) {
	ResetProviderHealth()
	catalog, err := LoadCatalog("../../../catalog/models.json")
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	reg := NewProviderRegistryWithEnv(catalog, func(name string) string {
		if name == "OPENAI_API_KEY" {
			return "sk-test"
		}
		return ""
	})

	if got := reg.CheckProviderHealth(context.Background(), "openai").State; got != HealthUnverified {
		t.Fatalf("health = %q, want %q", got, HealthUnverified)
	}

	ReportProviderAuth("openai", false, "openai request failed (401): invalid key")
	if got := reg.CheckProviderHealth(context.Background(), "openai").State; got != HealthFailed {
		t.Fatalf("after a 401, health = %q, want %q", got, HealthFailed)
	}

	ReportProviderAuth("openai", true, "")
	if got := reg.CheckProviderHealth(context.Background(), "openai").State; got != HealthOK {
		t.Fatalf("after a success, health = %q, want %q", got, HealthOK)
	}
}

// Listing providers must not trigger a refresh per request: a picker that
// opens twice would otherwise hammer the vendor's token endpoint.
func TestProbeIsCachedWithinTTL(t *testing.T) {
	reg := testRegistry(t)
	source := &stubTokenSource{token: "tok"}
	reg.SetTokenSource("codex-subscription", source)

	for i := 0; i < 5; i++ {
		reg.CheckProviderHealth(context.Background(), "codex-subscription")
	}
	if source.calls != 1 {
		t.Fatalf("token source called %d times, want 1 within the TTL", source.calls)
	}

	// Age the cache past the TTL and it should probe again.
	original := healthNow
	healthNow = func() time.Time { return original().Add(2 * healthTTL) }
	defer func() { healthNow = original }()

	reg.CheckProviderHealth(context.Background(), "codex-subscription")
	if source.calls != 2 {
		t.Fatalf("token source called %d times, want 2 after the TTL expired", source.calls)
	}
}
