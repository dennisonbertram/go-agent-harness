//go:build !darwin && !linux

package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// buildSandboxedCommand has no OS-level confinement mechanism implemented
// for this platform. SandboxScopeWorkspace/SandboxScopeLocal degrade to the
// string-heuristic checks in sandbox.go (or fail closed under
// SandboxEnforcementEnv); SandboxScopeUnrestricted is unaffected.
func buildSandboxedCommand(ctx context.Context, scope SandboxScope, workspaceRoot, command string) (*exec.Cmd, func(), SandboxExecResult, error) {
	noop := func() {}
	cmd := exec.CommandContext(ctx, "/bin/bash", "-lc", command)
	network, _ := NetworkPolicyFromContext(ctx)
	if network == "" {
		network = NetworkPolicyAllow
	}
	switch scope {
	case SandboxScopeUnrestricted, "":
		return cmd, noop, SandboxExecResult{Applied: false, Mechanism: "none", NetworkPolicy: network}, nil
	case SandboxScopeWorkspace, SandboxScopeLocal:
		res, err := resolveSandboxUnavailable(scope, "os-sandbox", "no OS-level sandbox mechanism implemented for this platform")
		if err != nil {
			return nil, nil, SandboxExecResult{}, err
		}
		res.NetworkPolicy = network
		return cmd, noop, res, nil
	default:
		return nil, nil, SandboxExecResult{}, fmt.Errorf("unknown sandbox scope %q", scope)
	}
}
