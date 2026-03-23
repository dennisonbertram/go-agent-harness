package trigger

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DeriveExternalThreadID computes a stable, deterministic conversation ID
// from external source identifiers. The algorithm is:
//
//   - If repoOwner and repoName are both non-empty:
//     SHA256("source\x00repoOwner\x00repoName\x00threadID")
//   - Otherwise:
//     SHA256("source\x00threadID")
//
// The source is normalized to lowercase before hashing. The returned ID has
// the form "source:hexhash" so it is both human-readable and globally unique.
func DeriveExternalThreadID(source, repoOwner, repoName, threadID string) ExternalThreadID {
	normalized := strings.ToLower(source)
	var preimage string
	if repoOwner != "" && repoName != "" {
		preimage = normalized + "\x00" + repoOwner + "\x00" + repoName + "\x00" + threadID
	} else {
		preimage = normalized + "\x00" + threadID
	}
	hash := sha256.Sum256([]byte(preimage))
	return ExternalThreadID(normalized + ":" + hex.EncodeToString(hash[:]))
}
