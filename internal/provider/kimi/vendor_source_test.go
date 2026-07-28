package kimi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeCredential(t *testing.T, expiresAt int64, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kimi-code.json")
	body, _ := json.Marshal(map[string]any{
		"access_token": token, "expires_at": expiresAt, "token_type": "Bearer",
	})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestVendorSourceReturnsTheCLIsCurrentToken(t *testing.T) {
	path := writeCredential(t, time.Now().Add(10*time.Minute).Unix(), "tok-current")
	got, err := NewVendorTokenSource(path).Token(context.Background())
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if got != "tok-current" {
		t.Fatalf("token = %q", got)
	}
}

// The point of reading through: whatever the CLI writes next is picked up
// without any import step. A rotated refresh token can no longer strand us.
func TestVendorSourceSeesTheCLIsNextRefresh(t *testing.T) {
	path := writeCredential(t, time.Now().Add(time.Minute).Unix(), "tok-first")
	source := NewVendorTokenSource(path)
	if got, _ := source.Token(context.Background()); got != "tok-first" {
		t.Fatalf("first read = %q", got)
	}

	// The CLI refreshes on its own schedule and rewrites the file.
	body, _ := json.Marshal(map[string]any{
		"access_token": "tok-rotated",
		"expires_at":   time.Now().Add(15 * time.Minute).Unix(),
	})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if got != "tok-rotated" {
		t.Fatalf("second read = %q — the source cached instead of reading through", got)
	}
}

// An expired token is something only the user can fix, so say what to do
// rather than letting the request go out and reporting a bare 401.
func TestExpiredVendorTokenSaysHowToFixIt(t *testing.T) {
	path := writeCredential(t, time.Now().Add(-time.Minute).Unix(), "tok-stale")
	_, err := NewVendorTokenSource(path).Token(context.Background())
	if err == nil {
		t.Fatal("an expired token should be reported")
	}
	if !strings.Contains(err.Error(), "run `kimi`") {
		t.Fatalf("error should name the fix: %v", err)
	}
}

func TestMissingVendorCredentialIsExplained(t *testing.T) {
	_, err := NewVendorTokenSource(filepath.Join(t.TempDir(), "absent.json")).
		Token(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "sign in") {
		t.Fatalf("error should tell the user what to do: %v", err)
	}
}

// The source must read the CLI's own file, not the harness's private copy.
// Reading the copy would reintroduce the staleness this type exists to avoid,
// and the two paths are easy to confuse: DefaultStorePath is the copy.
func TestDefaultPathIsTheVendorFileNotTheHarnessCopy(t *testing.T) {
	got := NewVendorTokenSource("").path
	if got != VendorCredentialPath() {
		t.Fatalf("default path = %q, want the CLI's own file %q", got, VendorCredentialPath())
	}
	if got == DefaultStorePath() {
		t.Fatal("the source is reading the harness's private copy")
	}
}

// A stub CLI that rewrites the credential file, standing in for the real one.
func refresherWriting(t *testing.T, path string, newExpiry time.Time, token string) *CLIRefresher {
	t.Helper()
	script := filepath.Join(t.TempDir(), "fake-kimi")
	body := "#!/bin/sh\ncat > " + path + " <<'JSON'\n" +
		`{"access_token":"` + token + `","expires_at":` +
		strconv.FormatInt(newExpiry.Unix(), 10) + "}\nJSON\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return &CLIRefresher{Binary: script, Prompt: "ok", Timeout: 20 * time.Second}
}

// An idle subscription should recover on its own rather than making the user
// go and run the CLI by hand.
func TestExpiredTokenIsRefreshedAutomatically(t *testing.T) {
	path := writeCredential(t, time.Now().Add(-time.Minute).Unix(), "tok-stale")
	source := NewVendorTokenSource(path).
		WithRefresher(refresherWriting(t, path, time.Now().Add(15*time.Minute), "tok-fresh"))

	got, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if got != "tok-fresh" {
		t.Fatalf("token = %q — the refreshed value was not picked up", got)
	}
}

// A valid token must not trigger a refresh: each one costs a real completion.
func TestValidTokenDoesNotInvokeTheCLI(t *testing.T) {
	path := writeCredential(t, time.Now().Add(10*time.Minute).Unix(), "tok-good")
	marker := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "fake-kimi")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := NewVendorTokenSource(path).
		WithRefresher(&CLIRefresher{Binary: script, Timeout: 10 * time.Second})

	if _, err := source.Token(context.Background()); err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the CLI was invoked for a token that was still valid")
	}
}

// A refresh that does not actually help must not spawn a process per request.
func TestFailingRefreshIsRateLimited(t *testing.T) {
	path := writeCredential(t, time.Now().Add(-time.Minute).Unix(), "tok-stale")
	counter := filepath.Join(t.TempDir(), "count")
	script := filepath.Join(t.TempDir(), "fake-kimi")
	// Writes a still-expired credential, i.e. a refresh that achieves nothing.
	body := "#!/bin/sh\necho x >> " + counter + "\ncat > " + path + " <<'JSON'\n" +
		`{"access_token":"still-stale","expires_at":` +
		strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10) + "}\nJSON\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	source := NewVendorTokenSource(path).WithRefresher(&CLIRefresher{
		Binary: script, Timeout: 10 * time.Second, MinInterval: time.Hour,
	})

	for i := 0; i < 4; i++ {
		if _, err := source.Token(context.Background()); err == nil {
			t.Fatal("a refresh that does not help should still be an error")
		}
	}
	data, _ := os.ReadFile(counter)
	if runs := strings.Count(string(data), "x"); runs != 1 {
		t.Fatalf("the CLI ran %d times across 4 requests; the rate limit did not hold", runs)
	}
}

// A missing CLI must be reported plainly, not as a token problem.
func TestMissingCLIIsReported(t *testing.T) {
	path := writeCredential(t, time.Now().Add(-time.Minute).Unix(), "tok-stale")
	source := NewVendorTokenSource(path).WithRefresher(&CLIRefresher{
		Binary: filepath.Join(t.TempDir(), "absent"), Timeout: 5 * time.Second,
	})
	_, err := source.Token(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("expected a clear not-installed error, got: %v", err)
	}
}
