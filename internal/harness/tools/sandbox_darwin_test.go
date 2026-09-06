//go:build darwin

package tools

import (
	"context"
	"strings"
	"testing"
)

// TestSeatbeltProfileNetworkPolicy verifies that seatbeltProfile emits
// "(deny network*)" only for SandboxScopeWorkspace/SandboxScopeLocal, and
// only when the caller asks for it (issue #1397). Before this change the
// seatbelt profile always denied network regardless of caller intent.
func TestSeatbeltProfileNetworkPolicy(t *testing.T) {
	t.Parallel()

	for _, scope := range []SandboxScope{SandboxScopeWorkspace, SandboxScopeLocal} {
		profile := seatbeltProfile(scope, t.TempDir(), NetworkPolicyDeny)
		if !strings.Contains(profile, "(deny network*)") {
			t.Errorf("scope %q, network=deny: expected profile to contain \"(deny network*)\", got:\n%s", scope, profile)
		}

		profile = seatbeltProfile(scope, t.TempDir(), NetworkPolicyAllow)
		if strings.Contains(profile, "(deny network*)") {
			t.Errorf("scope %q, network=allow: expected profile to omit \"(deny network*)\", got:\n%s", scope, profile)
		}
	}
}

// TestBuildSandboxedCommandDarwinNetworkPolicyFromContext verifies that
// buildSandboxedCommand reads the network policy from ctx (issue #1397) and
// reports the applied policy on SandboxExecResult so bash tool output can
// surface it.
func TestBuildSandboxedCommandDarwinNetworkPolicyFromContext(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	ctx := WithNetworkPolicy(context.Background(), NetworkPolicyDeny)
	_, cleanup, res, err := buildSandboxedCommand(ctx, SandboxScopeWorkspace, workspace, "echo hi")
	if err != nil {
		t.Fatalf("buildSandboxedCommand: %v", err)
	}
	defer cleanup()
	if res.NetworkPolicy != NetworkPolicyDeny {
		t.Errorf("expected SandboxExecResult.NetworkPolicy=%q, got %q", NetworkPolicyDeny, res.NetworkPolicy)
	}

	ctx = WithNetworkPolicy(context.Background(), NetworkPolicyAllow)
	_, cleanup2, res2, err := buildSandboxedCommand(ctx, SandboxScopeWorkspace, workspace, "echo hi")
	if err != nil {
		t.Fatalf("buildSandboxedCommand: %v", err)
	}
	defer cleanup2()
	if res2.NetworkPolicy != NetworkPolicyAllow {
		t.Errorf("expected SandboxExecResult.NetworkPolicy=%q, got %q", NetworkPolicyAllow, res2.NetworkPolicy)
	}

	// No network policy on the context at all must default to allow.
	_, cleanup3, res3, err := buildSandboxedCommand(context.Background(), SandboxScopeWorkspace, workspace, "echo hi")
	if err != nil {
		t.Fatalf("buildSandboxedCommand: %v", err)
	}
	defer cleanup3()
	if res3.NetworkPolicy != NetworkPolicyAllow {
		t.Errorf("expected default SandboxExecResult.NetworkPolicy=%q, got %q", NetworkPolicyAllow, res3.NetworkPolicy)
	}

	// Unrestricted scope is untouched by the network policy: no seatbelt
	// profile is generated at all.
	_, cleanup4, res4, err := buildSandboxedCommand(context.Background(), SandboxScopeUnrestricted, workspace, "echo hi")
	if err != nil {
		t.Fatalf("buildSandboxedCommand: %v", err)
	}
	defer cleanup4()
	if res4.Mechanism != "none" {
		t.Errorf("expected unrestricted scope to skip sandboxing, got mechanism %q", res4.Mechanism)
	}
}
