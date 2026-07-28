package modelstore

import (
	"context"
	"os"
	"testing"
)

// Verifies the fetcher against the real provider APIs. Skipped unless a
// credential is present, so CI stays offline and deterministic.
//
//	OPENAI_API_KEY=... go test ./internal/modelstore/ -run TestLive -v
func TestLiveFetchAgainstRealProviders(t *testing.T) {
	cases := []struct{ name, url, proto, key string }{
		{"openai", "https://api.openai.com/v1", ProtocolOpenAICompat, os.Getenv("OPENAI_API_KEY")},
		{"openrouter", "https://openrouter.ai/api/v1", ProtocolOpenAICompat, os.Getenv("OPENROUTER_API_KEY")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.key == "" {
				t.Skipf("no credential for %s", tc.name)
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
