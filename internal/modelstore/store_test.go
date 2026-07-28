package modelstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cost(v float64) *float64 { return &v }

func TestLoadMissingFileIsEmptyNotError(t *testing.T) {
	store, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("first run should not error: %v", err)
	}
	if len(store.Providers) != 0 || len(store.Fetched) != 0 {
		t.Fatal("a missing file should load as an empty store")
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	store := New()
	if err := store.PutProvider(Provider{
		Name: "my-proxy", BaseURL: "https://gw.example/v1", KeyRef: EnvRef("MY_PROXY_KEY"),
	}); err != nil {
		t.Fatalf("put: %v", err)
	}
	store.RecordFetch("my-proxy", []Model{{ID: "a", InputCost: cost(1)}}, time.Unix(1000, 0))
	if err := store.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The file must be owner-only: it names credential locations.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permissions are %o, want 600", perm)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := again.Providers["my-proxy"].BaseURL; got != "https://gw.example/v1" {
		t.Fatalf("base URL did not round-trip: %q", got)
	}
	// Defaults are filled in on write, not left blank for the reader to guess.
	if got := again.Providers["my-proxy"].Protocol; got != "openai_compat" {
		t.Fatalf("protocol = %q, want openai_compat", got)
	}
}

// The store must never contain a secret — only a pointer to one.
func TestSavedFileContainsNoSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	store := New()
	_ = store.PutProvider(Provider{
		Name: "p", BaseURL: "https://x/v1", KeyRef: KeychainRef("p"),
	})
	if err := store.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "sk-") {
		t.Fatal("the store appears to contain a raw key")
	}
	if !strings.Contains(string(data), "keychain:go-harness/p") {
		t.Fatalf("expected a keychain reference, got:\n%s", data)
	}
}

// A refetch must not undo the user's selections — that is the whole feature.
func TestRefetchPreservesExposedSelection(t *testing.T) {
	store := New()
	store.RecordFetch("openai", []Model{{ID: "gpt-a"}, {ID: "gpt-b"}}, time.Unix(1, 0))
	if err := store.SetExposed("openai", map[string]bool{"gpt-b": true}); err != nil {
		t.Fatalf("set exposed: %v", err)
	}

	// Provider ships a new model; the old ones come back unchanged.
	store.RecordFetch("openai", []Model{{ID: "gpt-a"}, {ID: "gpt-b"}, {ID: "gpt-c"}}, time.Unix(2, 0))

	exposed := store.ExposedModels()["openai"]
	if len(exposed) != 1 || exposed[0].ID != "gpt-b" {
		t.Fatalf("selection lost across refetch: %+v", exposed)
	}
}

// OpenAI's /v1/models returns no pricing. A refetch from it must not wipe a
// cost the catalog or the user supplied.
func TestRefetchDoesNotErasePricingTheProviderOmits(t *testing.T) {
	store := New()
	store.RecordFetch("openai", []Model{
		{ID: "gpt-a", InputCost: cost(5), OutputCost: cost(25), CostSource: CostFromCatalog, ContextWindow: 400000},
	}, time.Unix(1, 0))

	// A bare fetch: id only, exactly what OpenAI returns.
	store.RecordFetch("openai", []Model{{ID: "gpt-a"}}, time.Unix(2, 0))

	got := store.Fetched["openai"].Models[0]
	if got.InputCost == nil || *got.InputCost != 5 {
		t.Fatalf("input cost was erased: %+v", got.InputCost)
	}
	if got.ContextWindow != 400000 {
		t.Fatalf("context window was erased: %d", got.ContextWindow)
	}
}

// A price the user typed outranks whatever the provider later reports.
func TestUserEnteredCostSurvivesAProviderReportedOne(t *testing.T) {
	store := New()
	store.RecordFetch("p", []Model{{ID: "m"}}, time.Unix(1, 0))
	if err := store.SetCost("p", "m", 2, 8); err != nil {
		t.Fatalf("set cost: %v", err)
	}
	store.RecordFetch("p", []Model{
		{ID: "m", InputCost: cost(99), OutputCost: cost(99), CostSource: CostFromProvider},
	}, time.Unix(2, 0))

	got := store.Fetched["p"].Models[0]
	if got.InputCost == nil || *got.InputCost != 2 {
		t.Fatalf("user cost was overwritten: %+v", got.InputCost)
	}
	if got.CostSource != CostFromUser {
		t.Fatalf("cost source = %q, want %q", got.CostSource, CostFromUser)
	}
}

// A provider being briefly unreachable must not empty the picker.
func TestFetchErrorKeepsTheLastGoodList(t *testing.T) {
	store := New()
	store.RecordFetch("p", []Model{{ID: "m", Exposed: true}}, time.Unix(1, 0))
	store.RecordFetchError("p", "dial tcp: connection refused", time.Unix(2, 0))

	entry := store.Fetched["p"]
	if len(entry.Models) != 1 {
		t.Fatalf("models were discarded on a failed fetch: %+v", entry.Models)
	}
	if entry.Error == "" {
		t.Fatal("the failure reason was not recorded")
	}
}

// An empty store must not be mistaken for "the user chose to expose nothing".
func TestHasExposedSelectionDistinguishesEmptyFromDeliberate(t *testing.T) {
	store := New()
	if store.HasExposedSelection() {
		t.Fatal("a fresh store must not claim a selection exists")
	}
	store.RecordFetch("p", []Model{{ID: "m"}}, time.Unix(1, 0))
	if store.HasExposedSelection() {
		t.Fatal("fetched-but-unselected must not count as a selection")
	}
	_ = store.SetExposed("p", map[string]bool{"m": true})
	if !store.HasExposedSelection() {
		t.Fatal("an explicit selection was not detected")
	}
}

func TestBuiltinProviderCannotBeDeleted(t *testing.T) {
	store := New()
	_ = store.PutProvider(Provider{Name: "openai", BaseURL: "https://api.openai.com/v1", Builtin: true})
	err := store.DeleteProvider("openai")
	if err == nil {
		t.Fatal("deleting a builtin provider should fail — it would reappear on restart")
	}
	if !strings.Contains(err.Error(), "built in") {
		t.Fatalf("error should explain why: %v", err)
	}
}

// Editing a catalog provider is an override, not a conversion into a
// deletable custom one.
func TestEditingABuiltinKeepsItBuiltin(t *testing.T) {
	store := New()
	_ = store.PutProvider(Provider{Name: "openai", BaseURL: "https://api.openai.com/v1", Builtin: true})
	_ = store.PutProvider(Provider{Name: "openai", BaseURL: "https://proxy.internal/v1"})
	if !store.Providers["openai"].Builtin {
		t.Fatal("builtin flag was lost on edit")
	}
}

func TestProviderRequiresNameAndURL(t *testing.T) {
	store := New()
	if err := store.PutProvider(Provider{BaseURL: "https://x"}); err == nil {
		t.Fatal("a provider without a name should be rejected")
	}
	if err := store.PutProvider(Provider{Name: "x"}); err == nil {
		t.Fatal("a provider without a base URL should be rejected")
	}
}

// MARK: credential references

func TestResolveEnvAndFileReferences(t *testing.T) {
	t.Setenv("MODELSTORE_TEST_KEY", "sk-from-env")
	got, err := ResolveCredential(context.Background(), EnvRef("MODELSTORE_TEST_KEY"))
	if err != nil || got != "sk-from-env" {
		t.Fatalf("env ref = %q, %v", got, err)
	}

	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("sk-from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveCredential(context.Background(), SchemeFile+":"+path)
	if err != nil || got != "sk-from-file" {
		t.Fatalf("file ref = %q, %v", got, err)
	}
}

// A provider needing no credential is a valid configuration (Ollama, LM Studio).
func TestEmptyReferenceIsNotAnError(t *testing.T) {
	got, err := ResolveCredential(context.Background(), "")
	if err != nil || got != "" {
		t.Fatalf("empty ref = %q, %v", got, err)
	}
}

func TestMalformedReferencesAreRejected(t *testing.T) {
	for _, ref := range []string{"no-scheme", "env:", ":value", "bogus:thing"} {
		if _, err := ResolveCredential(context.Background(), ref); err == nil {
			t.Fatalf("reference %q should have been rejected", ref)
		}
	}
}

// Writing an env var would vanish on restart; say so rather than pretending.
func TestStoringToAnEnvReferenceIsRefusedWithGuidance(t *testing.T) {
	err := StoreCredential(context.Background(), EnvRef("SOME_KEY"), "sk-x")
	if err == nil {
		t.Fatal("writing to an environment variable should be refused")
	}
	if !strings.Contains(err.Error(), "keychain") {
		t.Fatalf("the error should point at a working alternative: %v", err)
	}
}

func TestFileCredentialRoundTripIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	ref := SchemeFile + ":" + path
	if err := StoreCredential(context.Background(), ref, "sk-secret"); err != nil {
		t.Fatalf("store: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credential file is %o, want 600", perm)
	}
	got, err := ResolveCredential(context.Background(), ref)
	if err != nil || got != "sk-secret" {
		t.Fatalf("round trip = %q, %v", got, err)
	}
	if err := DeleteCredential(context.Background(), ref); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("credential file was not removed")
	}
}

// Deleting something already gone is the desired end state, not a failure.
func TestDeletingAnAbsentCredentialSucceeds(t *testing.T) {
	ref := SchemeFile + ":" + filepath.Join(t.TempDir(), "never-existed")
	if err := DeleteCredential(context.Background(), ref); err != nil {
		t.Fatalf("delete of an absent credential should succeed: %v", err)
	}
}

func TestKeychainRefShape(t *testing.T) {
	if got := KeychainRef("openai"); got != "keychain:go-harness/openai" {
		t.Fatalf("keychain ref = %q", got)
	}
}

// The keychain is macOS-only; harnessd also runs on Linux, so every keychain
// path must degrade rather than assume.
func TestKeychainUnavailableIsReportedNotAssumed(t *testing.T) {
	if KeychainAvailable() {
		t.Skip("keychain is available here; the unavailable path is covered on Linux")
	}
	if _, err := ResolveCredential(context.Background(), KeychainRef("x")); err == nil {
		t.Fatal("expected a clear error when the keychain is unavailable")
	}
}

// A nil context from start-up code must not take the daemon down.
func TestNilContextDoesNotPanic(t *testing.T) {
	//nolint:staticcheck // deliberately passing nil: this is the regression.
	if _, err := ResolveCredential(nil, KeychainRef("no-such-entry")); err == nil {
		t.Log("no entry, as expected")
	}
	//nolint:staticcheck
	_ = DeleteCredential(nil, SchemeFile+":"+filepath.Join(t.TempDir(), "x"))
}
