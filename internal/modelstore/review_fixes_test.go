package modelstore

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A provider set to "no credential needed" must not send the key its old
// reference still points at — switching auth kind and base URL together used to
// deliver the previous secret to the new endpoint.
func TestAuthNoneSendsNoCredential(t *testing.T) {
	t.Setenv("REVIEW_TEST_KEY", "sk-must-not-be-sent")

	var gotAuth, gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"local-model"}]}`))
	}))
	defer srv.Close()

	svc := newTestService(t)
	if err := svc.PutProvider(context.Background(), Provider{
		Name:     "local",
		BaseURL:  srv.URL,
		Protocol: ProtocolOpenAICompat,
		AuthKind: AuthNone,
		KeyRef:   EnvRef("REVIEW_TEST_KEY"),
	}, ""); err != nil {
		t.Fatalf("put provider: %v", err)
	}
	if _, err := svc.FetchProvider(context.Background(), "local"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gotAuth != "" || gotAPIKey != "" {
		t.Fatalf("credential sent to a no-auth provider: Authorization=%q x-api-key=%q",
			gotAuth, gotAPIKey)
	}
}

// Go does not strip x-api-key across a redirect, so following one would hand an
// Anthropic key to whatever host the provider named.
func TestFetchRefusesRedirect(t *testing.T) {
	var reachedElsewhere bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reachedElsewhere = true
		_, _ = w.Write([]byte(`{"data":[{"id":"x"}]}`))
	}))
	defer elsewhere.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/models", http.StatusFound)
	}))
	defer redirector.Close()

	f := NewFetcher()
	_, err := f.Fetch(context.Background(), Provider{
		BaseURL: redirector.URL, Protocol: ProtocolAnthropic,
	}, "sk-secret")
	if err == nil {
		t.Fatal("expected the redirect to be refused")
	}
	if reachedElsewhere {
		t.Fatal("the credential was sent to the redirect target")
	}
}

// A rejected provider update must not have already replaced the live secret.
func TestPutProviderValidatesBeforeWritingSecret(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key")
	if err := os.WriteFile(keyFile, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := newTestService(t)
	// Missing base URL: the provider is invalid.
	err := svc.PutProvider(context.Background(), Provider{
		Name:   "broken",
		KeyRef: "file:" + keyFile,
	}, "replacement")
	if err == nil {
		t.Fatal("expected the invalid provider to be rejected")
	}
	got, readErr := os.ReadFile(keyFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "original" {
		t.Fatalf("the rejected update replaced the credential: %q", got)
	}
}

// An unrecognised protocol or auth kind is a configuration mistake and must be
// reported as one, not defaulted into a confusing provider error.
func TestUnknownProtocolAndAuthKindAreRejected(t *testing.T) {
	for _, p := range []Provider{
		{Name: "a", BaseURL: "https://x/v1", Protocol: "anthoropic"},
		{Name: "b", BaseURL: "https://x/v1", AuthKind: AuthKind("api-key")},
	} {
		if err := ValidateProvider(&p); err == nil {
			t.Fatalf("provider %+v was accepted", p)
		}
	}
}

// A price that cannot be encoded as JSON would make every later save of the
// whole store fail, so it must never reach the store.
func TestNonFiniteCostsAreRejected(t *testing.T) {
	if _, ok := perMillion("NaN"); ok {
		t.Error(`perMillion("NaN") was accepted`)
	}
	if _, ok := perMillion("+Inf"); ok {
		t.Error(`perMillion("+Inf") was accepted`)
	}

	s := New()
	s.RecordFetch("p", []Model{{ID: "m"}}, time.Now())
	if err := s.SetCost("p", "m", math.NaN(), 1); err == nil {
		t.Error("SetCost accepted NaN")
	}
	if err := s.SetCost("p", "m", 1, math.Inf(1)); err == nil {
		t.Error("SetCost accepted +Inf")
	}
	if err := s.Save(filepath.Join(t.TempDir(), "models.json")); err != nil {
		t.Fatalf("store became unsavable: %v", err)
	}
}

// A fetch that reports only ids must not erase what the catalog already knew.
func TestRecordFetchKeepsDisplayNameAndModalities(t *testing.T) {
	s := New()
	s.RecordFetch("p", []Model{{
		ID: "m", DisplayName: "Nice Name", Modalities: []string{"text", "image"},
	}}, time.Now())
	// Second fetch returns a bare id, the way OpenAI's listing does.
	s.RecordFetch("p", []Model{{ID: "m"}}, time.Now())

	got := s.Fetched["p"].Models[0]
	if got.DisplayName != "Nice Name" {
		t.Errorf("display name erased: %q", got.DisplayName)
	}
	if len(got.Modalities) != 2 {
		t.Errorf("modalities erased: %v", got.Modalities)
	}
}

// The kept model list is still the one from the last success, so a failure must
// not restamp it as freshly fetched.
func TestRecordFetchErrorKeepsFetchTime(t *testing.T) {
	s := New()
	fetchedAt := time.Now().Add(-48 * time.Hour)
	s.RecordFetch("p", []Model{{ID: "m"}}, fetchedAt)
	s.RecordFetchError("p", "provider unreachable", time.Now())

	if !s.Fetched["p"].At.Equal(fetchedAt) {
		t.Fatalf("failure restamped the list as fetched at %s, want %s",
			s.Fetched["p"].At, fetchedAt)
	}
	if s.Fetched["p"].Error == "" {
		t.Fatal("the failure was not recorded")
	}
}

// Updating a file credential must land at 0600 even when the old file was more
// permissive, and must not leave the target empty if the write fails.
func TestFileCredentialIsRewrittenAtRestrictiveMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := StoreCredential(context.Background(), "file:"+path, "new"); err != nil {
		t.Fatalf("store credential: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("credential file mode is %o, want 600", mode)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("credential is %q, want %q", got, "new")
	}
}
