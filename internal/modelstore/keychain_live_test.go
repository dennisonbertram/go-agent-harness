package modelstore

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

const realKeychainOptInEnv = "HARNESS_TEST_REAL_KEYCHAIN"

func realKeychainMutationEnabled() bool {
	return os.Getenv(realKeychainOptInEnv) == "1"
}

func requireRealKeychainMutation(t *testing.T) {
	t.Helper()
	if !realKeychainMutationEnabled() {
		t.Skip("real login-Keychain mutation skipped; run the named host-live lane with HARNESS_TEST_REAL_KEYCHAIN=1")
	}
	if !KeychainAvailable() {
		t.Skip("real login-Keychain mutation requested but security(1) is unavailable")
	}
}

// realKeychainAccount scopes every live mutation to a test/process account so
// concurrent host test processes never contend for a shared self-test entry.
func realKeychainAccount(t *testing.T, purpose string) string {
	t.Helper()
	purpose = strings.NewReplacer("/", "-", " ", "-", ".", "-").Replace(purpose)
	return fmt.Sprintf("modelstore-%s-%d", purpose, os.Getpid())
}
