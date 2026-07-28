package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// VendorTokenSource serves the Kimi CLI's current access token by reading the
// CLI's own credential file on each request.
//
// Why read through instead of refreshing our own copy:
//
//   - Kimi rejects the harness's refresh request with "invalid_client". The
//     grant type and form encoding are correct and client_id is required, but
//     the identifier the harness sends is not a valid OAuth client, and the
//     real one is assembled at runtime inside the CLI rather than stored as a
//     literal. So the harness cannot refresh at all.
//   - Kimi rotates refresh tokens. Even with a working client, a copy taken at
//     import goes stale the moment the CLI refreshes on its own schedule,
//     because rotation invalidates the token the copy holds.
//
// Reading through sidesteps both: the CLI owns the refresh, and the harness
// always sees whatever token is current. The file is only ever read — the CLI
// remains the sole writer, matching how the Codex credential is treated.
type VendorTokenSource struct {
	path string
	now  func() time.Time
	// refresher renews an expired token by running the CLI. Optional: without
	// one an expired token is reported and the user runs the CLI themselves.
	refresher *CLIRefresher
}

// NewVendorTokenSource reads the Kimi CLI's credential file at the given path.
// An empty path uses the CLI's own location.
//
// Note this is VendorCredentialPath, not DefaultStorePath: the latter is the
// harness's private copy, and reading that would reintroduce exactly the
// staleness this type exists to avoid.
func NewVendorTokenSource(path string) *VendorTokenSource {
	if path == "" {
		path = VendorCredentialPath()
	}
	return &VendorTokenSource{path: path, now: time.Now}
}

type vendorCredential struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
	TokenType   string `json:"token_type"`
}

// Token returns the CLI's current access token.
//
// An expired token is reported rather than returned: the caller cannot fix it,
// but the user can, by running the Kimi CLI so it refreshes. Saying that is far
// more useful than letting the request go out and surfacing a bare 401.
func (s *VendorTokenSource) Token(ctx context.Context) (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"no Kimi CLI credential at %s; run `kimi` and sign in", s.path)
		}
		return "", fmt.Errorf("read Kimi CLI credential: %w", err)
	}

	var cred vendorCredential
	if err := json.Unmarshal(data, &cred); err != nil {
		return "", fmt.Errorf("parse Kimi CLI credential at %s: %w", s.path, err)
	}
	if cred.AccessToken == "" {
		return "", fmt.Errorf("Kimi CLI credential at %s has no access token", s.path)
	}

	if cred.ExpiresAt > 0 {
		expiry := time.Unix(cred.ExpiresAt, 0)
		if !expiry.After(s.now()) {
			if s.refresher == nil {
				return "", fmt.Errorf(
					"the Kimi CLI's token expired at %s; run `kimi` to refresh it",
					expiry.Format(time.Kitchen))
			}
			if err := s.refresher.Refresh(ctx); err != nil {
				return "", fmt.Errorf(
					"the Kimi CLI's token expired at %s and refreshing it failed: %w",
					expiry.Format(time.Kitchen), err)
			}
			// Re-read: the CLI has rewritten the file.
			return s.readFresh()
		}
	}
	return cred.AccessToken, nil
}

// readFresh re-reads after a refresh and insists the token is now usable, so a
// refresh that silently did nothing surfaces here instead of as a 401.
func (s *VendorTokenSource) readFresh() (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return "", fmt.Errorf("re-read Kimi CLI credential after refresh: %w", err)
	}
	var cred vendorCredential
	if err := json.Unmarshal(data, &cred); err != nil {
		return "", fmt.Errorf("parse Kimi CLI credential after refresh: %w", err)
	}
	if cred.AccessToken == "" {
		return "", fmt.Errorf("the Kimi CLI wrote no access token after refreshing")
	}
	if cred.ExpiresAt > 0 && !time.Unix(cred.ExpiresAt, 0).After(s.now()) {
		return "", fmt.Errorf("the Kimi CLI refreshed but its token is still expired")
	}
	return cred.AccessToken, nil
}
