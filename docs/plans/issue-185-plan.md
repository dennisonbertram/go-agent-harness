# Plan: Issue #185 — VMWorkspace (Cloud VM-based)

## Recommendation: Hetzner Cloud (hcloud-go)
Simpler API, per-second billing, tighter SDK. Start here; DigitalOcean can be added via same VMProvider interface later.

## Architecture

```
VMWorkspace
    └── VMProvider (interface)
            └── HetznerProvider (implementation)
```

## Files to Create

### `internal/workspace/provider.go`
VMProvider interface + VM struct:
```go
type VMProvider interface {
    Create(ctx context.Context, opts VMCreateOpts) (*VM, error)
    Delete(ctx context.Context, id string) error
}

type VM struct {
    ID       string
    PublicIP string
    Status   string
}

type VMCreateOpts struct {
    Name      string
    UserData  string
    ImageName string    // default: "ubuntu-24.04"
    ServerType string   // default: "cx22"
}
```

### `internal/workspace/vm.go`
VMWorkspace struct implementing Workspace:
```go
type VMWorkspace struct {
    harnessURL    string
    workspacePath string  // path on remote VM
    vmID          string
    provider      VMProvider
}
```
- Provision: calls provider.Create(), polls until IP available, builds harnessURL
- HarnessURL/WorkspacePath: return stored values (empty before Provision)
- Destroy: calls provider.Delete()
- Registered as "vm" in init()

### `internal/workspace/hetzner.go`
HetznerProvider using hcloud-go:
- Client from HETZNER_API_KEY env var
- Create: ServerCreateOpts{UserData, ImageName, ServerType, Name}
- Polls server.Status == "active" every 3s, 5-minute timeout
- Delete: client.Server.Delete()

### `internal/workspace/bootstrap.go`
UserData generation:
```go
func harnessBootstrapScript(harnessPort int) string {
    // Returns cloud-init bash script that:
    // 1. apt installs curl, git
    // 2. Could download harnessd binary or build from source
    // 3. Writes systemd service file
    // 4. Starts harnessd
}
```
Note: For now, document that harnessd binary must be accessible (e.g., pre-built in image or downloaded from release URL). The bootstrap script creates a placeholder that polls for harnessd readiness.

### `internal/workspace/vm_test.go`
Unit tests (no build tag, mock provider):
- TestVMWorkspace_ImplementsWorkspace
- TestVMWorkspace_Provision_EmptyID
- TestVMWorkspace_Destroy_NotProvisioned
- TestVMWorkspace_HarnessURL_BeforeProvision
- TestVMWorkspace_RegisteredInFactory
- TestVMWorkspace_ProvisionSetsHarnessURL (mock provider)
- TestVMWorkspace_DestroyCallsProvider (mock provider)

### `internal/workspace/vm_integration_test.go`
```go
//go:build integration
```
Real Hetzner API tests. Skip without credentials.

### `internal/workspace/hetzner_test.go`
Unit tests for HetznerProvider with mock HTTP server or skipped if no HETZNER_API_KEY.

## go.mod additions
```
github.com/hetznercloud/hcloud-go/v2 latest
```

## Commit Strategy
1. `feat(#185): add VMProvider interface, VMWorkspace, and HetznerProvider`
