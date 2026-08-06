package modelstore

import (
	"context"
	"os"
	"testing"
)

func TestLiveProviderFetchEnabled(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		liveOptIn  string
		credential string
		want       bool
	}{
		{name: "credential only", credential: "secret", want: false},
		{name: "flag only", liveOptIn: "1", want: false},
		{name: "explicit opt-in and credential", liveOptIn: "1", credential: "secret", want: true},
		{name: "unexpected flag value", liveOptIn: "true", credential: "secret", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := liveProviderFetchEnabled(tc.liveOptIn, tc.credential); got != tc.want {
				t.Fatalf("liveProviderFetchEnabled(%q, credential set=%t) = %t, want %t", tc.liveOptIn, tc.credential != "", got, tc.want)
			}
		})
	}
}

func liveProviderFetchEnabled(liveOptIn, credential string) bool {
	return liveOptIn == "1" && credential != ""
}

// Verifies the fetcher against the real provider APIs. Skipped unless an
// explicit opt-in and the matching credential are present, so ordinary tests
// stay offline and deterministic even on credentialed machines.
//
//	HARNESS_TEST_LIVE_PROVIDERS=1 OPENAI_API_KEY=... go test ./internal/modelstore/ -run '^TestLiveFetchAgainstRealProviders$/openai$' -v
func TestLiveFetchAgainstRealProviders(t *testing.T) {
	cases := []struct{ name, url, proto, key string }{
		{"openai", "https://api.openai.com/v1", ProtocolOpenAICompat, os.Getenv("OPENAI_API_KEY")},
		{"openrouter", "https://openrouter.ai/api/v1", ProtocolOpenAICompat, os.Getenv("OPENROUTER_API_KEY")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !liveProviderFetchEnabled(os.Getenv("HARNESS_TEST_LIVE_PROVIDERS"), tc.key) {
				t.Skipf("set HARNESS_TEST_LIVE_PROVIDERS=1 and credential for %s to run live provider smoke", tc.name)
			}
			models, err := NewFetcher().Fetch(context.Background(),
				Provider{Name: tc.name, BaseURL: tc.url, Protocol: tc.proto}, tc.key)
			if err != nil {
				t.Fatalf("live fetch: %v", err)
			}
			priced := 0
			for _, m := range models {
				if m.InputCost != nil {
					priced++
				}
			}
			t.Logf("%s: %d models, %d with pricing", tc.name, len(models), priced)
			if len(models) == 0 {
				t.Fatal("no models returned")
			}
			t.Logf("  first: %s ctx=%d", models[0].ID, models[0].ContextWindow)
		})
	}
}
