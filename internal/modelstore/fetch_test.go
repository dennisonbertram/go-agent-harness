package modelstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fetchFrom(t *testing.T, p Provider, credential string, handler http.HandlerFunc) ([]Model, error) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	p.BaseURL = server.URL
	return NewFetcher().Fetch(context.Background(), p, credential)
}

// The exact shape OpenAI returns: ids only, no pricing, no context window.
func TestFetchOpenAICompatShape(t *testing.T) {
	var gotAuth string
	models, err := fetchFrom(t,
		Provider{Name: "openai", Protocol: ProtocolOpenAICompat}, "sk-test",
		func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			if !strings.HasSuffix(r.URL.Path, "/models") {
				t.Errorf("requested %q, want a /models path", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":[
				{"id":"gpt-5.6-sol","object":"model","created":1,"owned_by":"openai"},
				{"id":"gpt-4.1-mini","object":"model","created":2,"owned_by":"openai"}]}`))
		})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization header = %q", gotAuth)
	}
	if len(models) != 2 || models[0].ID != "gpt-5.6-sol" {
		t.Fatalf("unexpected models: %+v", models)
	}
	// Costs must stay unknown rather than defaulting to zero, which would
	// render as free.
	if models[0].InputCost != nil {
		t.Fatalf("cost should be unknown, got %v", *models[0].InputCost)
	}
}

// Anthropic authenticates differently and reports context and output caps.
func TestFetchAnthropicShapeAndHeaders(t *testing.T) {
	var key, version string
	models, err := fetchFrom(t,
		Provider{Name: "anthropic", Protocol: ProtocolAnthropic}, "sk-ant",
		func(w http.ResponseWriter, r *http.Request) {
			key = r.Header.Get("x-api-key")
			version = r.Header.Get("anthropic-version")
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-opus-5",
				"display_name":"Claude Opus 5","max_input_tokens":1000000,"max_tokens":128000}]}`))
		})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if key != "sk-ant" {
		t.Fatalf("x-api-key = %q — Anthropic does not use bearer auth", key)
	}
	if version == "" {
		t.Fatal("anthropic-version header is required and was not sent")
	}
	m := models[0]
	if m.DisplayName != "Claude Opus 5" || m.ContextWindow != 1000000 || m.MaxOutput != 128000 {
		t.Fatalf("fields not mapped: %+v", m)
	}
}

// OpenRouter is the one provider in scope that reports pricing, as per-token
// strings that have to become dollars per million tokens.
func TestFetchOpenRouterPricingConversion(t *testing.T) {
	models, err := fetchFrom(t,
		Provider{Name: "openrouter", Protocol: ProtocolOpenAICompat}, "",
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"id":"anthropic/claude-opus-5","name":"Claude Opus 5",
				"context_length":1000000,
				"pricing":{"prompt":"0.000005","completion":"0.000025"},
				"architecture":{"input_modalities":["text","image"]}}]}`))
		})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	m := models[0]
	if m.InputCost == nil || *m.InputCost != 5 {
		t.Fatalf("input cost = %v, want 5 per million", m.InputCost)
	}
	if m.OutputCost == nil || *m.OutputCost != 25 {
		t.Fatalf("output cost = %v, want 25 per million", m.OutputCost)
	}
	if m.CostSource != CostFromProvider {
		t.Fatalf("cost source = %q", m.CostSource)
	}
	if len(m.Modalities) != 2 {
		t.Fatalf("modalities not captured: %+v", m.Modalities)
	}
}

// A zero price and an absent price are different facts; only a real zero from
// the provider should read as free.
func TestMalformedPricingIsUnknownNotFree(t *testing.T) {
	models, err := fetchFrom(t,
		Provider{Protocol: ProtocolOpenAICompat}, "",
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"id":"m","pricing":{"prompt":"","completion":"x"}}]}`))
		})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if models[0].InputCost != nil {
		t.Fatalf("malformed pricing became a real cost: %v", *models[0].InputCost)
	}
}

// A provider with no credential configured (Ollama, LM Studio) must not get an
// empty Authorization header, which some servers reject outright.
func TestNoCredentialSendsNoAuthHeader(t *testing.T) {
	var present bool
	_, err := fetchFrom(t, Provider{Protocol: ProtocolOpenAICompat}, "",
		func(w http.ResponseWriter, r *http.Request) {
			_, present = r.Header["Authorization"]
			_, _ = w.Write([]byte(`{"data":[{"id":"llama"}]}`))
		})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if present {
		t.Fatal("an Authorization header was sent for a credential-less provider")
	}
}

func TestHTTPErrorIsReportedWithStatus(t *testing.T) {
	_, err := fetchFrom(t, Provider{Protocol: ProtocolOpenAICompat}, "bad",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
		})
	if err == nil {
		t.Fatal("a 401 should surface as an error")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("error should carry status and reason: %v", err)
	}
}

// A gateway answering with an HTML error page must not dump the page into the
// UI — the same defect that was fixed for provider completion errors.
func TestHTMLErrorPageIsSummarized(t *testing.T) {
	_, err := fetchFrom(t, Provider{Protocol: ProtocolOpenAICompat}, "",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("<html><head><style>" + strings.Repeat("x", 5000) + "</style></head></html>"))
		})
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(err.Error()) > 300 {
		t.Fatalf("error is %d chars — the page was not summarized", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "HTML") {
		t.Fatalf("error should name the cause: %v", err)
	}
}

// An empty list means something is wrong with the endpoint or credential; it
// must not silently wipe the provider's models.
func TestEmptyListIsAnError(t *testing.T) {
	_, err := fetchFrom(t, Provider{Protocol: ProtocolOpenAICompat}, "",
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"data":[]}`))
		})
	if err == nil {
		t.Fatal("an empty model list should be reported, not accepted")
	}
}

func TestBaseURLTrailingSlashIsHandled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "//") {
			t.Errorf("doubled slash in path: %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"m"}]}`))
	}))
	defer server.Close()

	_, err := NewFetcher().Fetch(context.Background(),
		Provider{Protocol: ProtocolOpenAICompat, BaseURL: server.URL + "/"}, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
}
