package tools

// toolchainWritableDirs returns the per-user temp and cache directories that
// language-toolchain invocations (go build/test, npm, cargo) need to write
// to even though nothing else about them lives inside the workspace (issue
// #1399). Without these, a `go build` run under SandboxScopeWorkspace fails
// with "failed to initialize build cache ... operation not permitted" (the
// build cache lives under the per-user cache dir) or "creating work dir:
// mkdir /var/folders/...: operation not permitted" (Go's scratch dir lives
// under the process temp dir) — neither of those roots is the workspace, so
// neither darwin's seatbelt "(subpath ...)" allow-list nor Linux's bwrap
// binds cover them today.
//
// TODO(#1399): implement. Currently a stub returning nil so the red-phase
// tests fail for the right reason (missing entries), not a compile error.
func toolchainWritableDirs() []string {
	return nil
}
