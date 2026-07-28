package tools

// Direct tests for the shared infrastructure this package exports to
// tools/core and tools/deferred.
//
// These helpers are used constantly, but only ever indirectly — through the
// tools that call them. That left them with no coverage of their own here and,
// more importantly, no test that states what they are supposed to do. A
// behaviour change in one of them would surface as a puzzling failure in some
// unrelated tool's test, if it surfaced at all.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripSudo(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain sudo", "sudo rm -rf /tmp/x", "rm -rf /tmp/x"},
		{"valueless flags are stripped", "sudo -H -n whoami", "whoami"},
		// KNOWN LIMITATION, pinned deliberately: the pattern strips flags but
		// not the VALUES of flags that take one, so "sudo -u root whoami"
		// leaves the value behind and yields "root whoami" — a different,
		// nonsense command rather than a clean strip. Stripping correctly
		// would mean knowing which sudo flags consume an argument (-u, -g, -p,
		// -C, ...). Recorded here so the behaviour is visible; if it is ever
		// fixed, this case moves up to the block above.
		{"flag values are NOT stripped", "sudo -u root whoami", "root whoami"},
		{"flag values are NOT stripped, multi-flag", "sudo -H -u root whoami", "root whoami"},
		{"uppercase sudo", "SUDO whoami", "whoami"},
		{"sudo mid-command", "echo hi && sudo whoami", "echo hi && whoami"},
		{"no sudo present is unchanged", "rm -rf /tmp/x", "rm -rf /tmp/x"},
		{"a word merely containing sudo is untouched", "pseudonym --list", "pseudonym --list"},
		{"empty command", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripSudo(tc.in); got != tc.want {
				t.Errorf("StripSudo(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestForkResultExecutionError(t *testing.T) {
	if err := ForkResultExecutionError(ForkResult{}); err != nil {
		t.Errorf("an empty ForkResult.Error must produce no error, got %v", err)
	}
	if err := ForkResultExecutionError(ForkResult{Error: "   \t\n "}); err != nil {
		t.Errorf("a whitespace-only ForkResult.Error must produce no error, got %v", err)
	}
	err := ForkResultExecutionError(ForkResult{Error: "child run exceeded max steps"})
	if err == nil {
		t.Fatal("a populated ForkResult.Error must produce an error")
	}
	if err.Error() != "child run exceeded max steps" {
		t.Errorf("error text = %q, want the ForkResult.Error verbatim", err.Error())
	}
}

func TestForkDepthContextRoundTrip(t *testing.T) {
	if got := ForkDepthFromContext(nil); got != 0 { //nolint:staticcheck // nil is the case under test
		t.Errorf("ForkDepthFromContext(nil) = %d, want 0", got)
	}
	if got := ForkDepthFromContext(context.Background()); got != 0 {
		t.Errorf("an unset depth must read as 0, got %d", got)
	}
	ctx := WithForkDepth(context.Background(), 3)
	if got := ForkDepthFromContext(ctx); got != 3 {
		t.Errorf("ForkDepthFromContext after WithForkDepth(3) = %d, want 3", got)
	}
	// Depth must be overridable, since each nested spawn re-stamps it.
	if got := ForkDepthFromContext(WithForkDepth(ctx, 7)); got != 7 {
		t.Errorf("re-stamping the depth = %d, want 7", got)
	}
}

func TestMessageReplacerFromContext(t *testing.T) {
	if _, ok := MessageReplacerFromContext(nil); ok { //nolint:staticcheck // nil is the case under test
		t.Error("a nil context must not yield a replacer")
	}
	if _, ok := MessageReplacerFromContext(context.Background()); ok {
		t.Error("an unset replacer must not be reported as present")
	}

	var got []map[string]any
	ctx := context.WithValue(context.Background(), ContextKeyMessageReplacer,
		func(msgs []map[string]any) { got = msgs })
	fn, ok := MessageReplacerFromContext(ctx)
	if !ok {
		t.Fatal("expected the replacer to be found")
	}
	fn([]map[string]any{{"role": "user", "content": "hi"}})
	if len(got) != 1 || got[0]["content"] != "hi" {
		t.Errorf("the retrieved replacer did not receive the messages: %+v", got)
	}

	// A value of the wrong type stored under the same key must not be
	// returned as if it were a replacer.
	wrong := context.WithValue(context.Background(), ContextKeyMessageReplacer, "not a function")
	if _, ok := MessageReplacerFromContext(wrong); ok {
		t.Error("a wrongly-typed context value must not be reported as a replacer")
	}
}

func TestFileVersionFromBytesAndReadFileVersion(t *testing.T) {
	v1 := FileVersionFromBytes([]byte("hello"))
	if len(v1) != 16 {
		t.Errorf("version hash length = %d, want 16 hex chars (8 bytes)", len(v1))
	}
	if v1 != FileVersionFromBytes([]byte("hello")) {
		t.Error("the same content must produce the same version")
	}
	if v1 == FileVersionFromBytes([]byte("hello!")) {
		t.Error("different content must produce a different version")
	}
	if FileVersionFromBytes(nil) != FileVersionFromBytes([]byte{}) {
		t.Error("nil and empty content must hash identically")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := ReadFileVersion(path)
	if err != nil {
		t.Fatalf("ReadFileVersion: %v", err)
	}
	if got != v1 {
		t.Errorf("ReadFileVersion = %q, want the same hash as the content %q", got, v1)
	}

	if _, err := ReadFileVersion(filepath.Join(dir, "does-not-exist")); err == nil {
		t.Error("ReadFileVersion must fail for a missing file")
	}
}

// --- GuardedWebFetcher -------------------------------------------------
//
// Fetch is the agent-supplied-destination surface, so its guard is tested as
// an attack: every branch that must refuse, plus the allowlist escape hatch
// that must still work.

// recordingFetcher stands in for a real WebFetcher and records whether it was
// ever reached, which is how the tests below prove the guard rejected a
// destination BEFORE delegating.
type recordingFetcher struct {
	called bool
	url    string
}

func (f *recordingFetcher) Fetch(_ context.Context, url string) (string, error) {
	f.called = true
	f.url = url
	return "delegated:" + url, nil
}

func (f *recordingFetcher) Search(_ context.Context, query string, _ int) ([]map[string]any, error) {
	return []map[string]any{{"query": query}}, nil
}

func TestGuardedWebFetcher_FetchRejectsBeforeDelegating(t *testing.T) {
	attacks := []struct {
		name string
		url  string
	}{
		{"loopback by name", "http://localhost:8080/x"},
		{"loopback by address", "http://127.0.0.1:8080/x"},
		{"ipv6 loopback", "http://[::1]:8080/x"},
		{"cloud metadata link-local", "http://169.254.169.254/latest/meta-data/"},
		{"rfc1918 private", "http://10.0.0.5/x"},
		{"rfc1918 private 192.168", "http://192.168.1.1/x"},
	}
	for _, a := range attacks {
		t.Run(a.name, func(t *testing.T) {
			base := &recordingFetcher{}
			g := NewGuardedWebFetcher(base, nil)
			if _, err := g.Fetch(context.Background(), a.url); err == nil {
				t.Fatalf("expected %s to be refused", a.name)
			}
			if base.called {
				t.Error("the guard must refuse BEFORE delegating to the base fetcher")
			}
		})
	}
}

func TestGuardedWebFetcher_FetchRejectsMalformedDestinations(t *testing.T) {
	base := &recordingFetcher{}
	g := NewGuardedWebFetcher(base, nil)

	for _, tc := range []struct{ name, url, wantErrContains string }{
		{"unsupported scheme", "file:///etc/passwd", "scheme"},
		{"ftp scheme", "ftp://example.com/x", "scheme"},
		{"no host", "http://", "host"},
		{"unparseable url", "http://[::1", "invalid url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := g.Fetch(context.Background(), tc.url)
			if err == nil {
				t.Fatalf("expected %s to error", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error %q should mention %q", err.Error(), tc.wantErrContains)
			}
			if base.called {
				t.Error("a malformed destination must not reach the base fetcher")
			}
		})
	}
}

func TestGuardedWebFetcher_AllowlistPermitsAndStillDelegates(t *testing.T) {
	base := &recordingFetcher{}
	g := NewGuardedWebFetcher(base, []string{"localhost"})

	out, err := g.Fetch(context.Background(), "http://localhost:9999/thing")
	if err != nil {
		t.Fatalf("an allowlisted host should be permitted: %v", err)
	}
	if !base.called {
		t.Error("an allowlisted destination must be delegated to the base fetcher")
	}
	if out != "delegated:http://localhost:9999/thing" {
		t.Errorf("the base fetcher's response should be returned verbatim, got %q", out)
	}

	// A host NOT on the allowlist is still refused by the same fetcher.
	base.called = false
	if _, err := g.Fetch(context.Background(), "http://127.0.0.1:9999/thing"); err == nil {
		t.Error("allowlisting one host must not permit a different private destination")
	}
}

func TestGuardedWebFetcher_PublicIPLiteralIsPermitted(t *testing.T) {
	base := &recordingFetcher{}
	g := NewGuardedWebFetcher(base, nil)
	// A public IP literal needs no DNS, so this exercises the accept path of
	// the destination check without depending on network access.
	if _, err := g.Fetch(context.Background(), "http://8.8.8.8/x"); err != nil {
		t.Fatalf("a public destination must be permitted: %v", err)
	}
	if !base.called {
		t.Error("a permitted destination must be delegated to the base fetcher")
	}
}

func TestGuardedWebFetcher_UnresolvableHostIsRefused(t *testing.T) {
	base := &recordingFetcher{}
	g := NewGuardedWebFetcher(base, nil)
	// .invalid is reserved by RFC 2606 and must never resolve.
	_, err := g.Fetch(context.Background(), "http://this-host-does-not-exist.invalid/x")
	if err == nil {
		t.Fatal("an unresolvable host must be refused rather than delegated")
	}
	if base.called {
		t.Error("an unresolvable host must not reach the base fetcher")
	}
}

func TestGuardedWebFetcher_DirectFetchTruncatesOversizedBodies(t *testing.T) {
	// base == nil selects the direct path, which owns its transport and
	// applies the response size cap. Loopback is allowlisted so the dial-time
	// guard permits the test server.
	oversize := guardedWebFetchMaxBytes + 4096
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", oversize)))
	}))
	defer srv.Close()

	g := NewGuardedWebFetcher(nil, []string{"127.0.0.1"})
	body, err := g.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("direct fetch: %v", err)
	}
	if len(body) != guardedWebFetchMaxBytes {
		t.Errorf("body length = %d, want it truncated to the %d-byte cap", len(body), guardedWebFetchMaxBytes)
	}
}

func TestGuardedWebFetcher_DirectFetchReturnsSmallBodyIntact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s", r.URL.Path)
	}))
	defer srv.Close()

	g := NewGuardedWebFetcher(nil, []string{"127.0.0.1"})
	body, err := g.Fetch(context.Background(), srv.URL+"/hello")
	if err != nil {
		t.Fatalf("direct fetch: %v", err)
	}
	if body != "path=/hello" {
		t.Errorf("body = %q, want the server's response intact", body)
	}
}

func TestGuardedWebFetcher_Search(t *testing.T) {
	// Search has no agent-supplied destination, so it delegates unchanged...
	base := &recordingFetcher{}
	g := NewGuardedWebFetcher(base, nil)
	results, err := g.Search(context.Background(), "golang", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0]["query"] != "golang" {
		t.Errorf("search should delegate unchanged, got %+v", results)
	}

	// ...but without a base there is nothing to delegate to.
	if _, err := NewGuardedWebFetcher(nil, nil).Search(context.Background(), "golang", 3); err == nil {
		t.Error("search without a configured base must return an error")
	}
}
