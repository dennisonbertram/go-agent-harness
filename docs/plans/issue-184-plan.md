# Plan: Issue #184 — ContainerWorkspace (Docker-based)

## Summary
Implement `ContainerWorkspace` using the Docker Go SDK. Each workspace provisions a Docker container running harnessd, exposing it on a dynamically allocated host port.

## Docker SDK
- Import: `github.com/docker/docker/client` (add to go.mod)
- Also: `github.com/docker/docker/api/types/container`, `github.com/docker/go-connections/nat`
- Client init: `client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())`

## Files to Create

### `internal/workspace/container.go`
```go
type ContainerWorkspace struct {
    harnessURL    string
    workspacePath string  // host path mounted into container
    containerID   string  // set after Provision
    hostPort      string  // dynamically allocated, set after Provision
    dockerClient  *client.Client
    imageName     string  // defaults to "go-agent-harness:latest"
}
```

**Provision(ctx, opts)**:
1. Validate opts.ID non-empty
2. Create/ensure workspace dir on host: `filepath.Join(opts.BaseDir or TempDir, opts.ID)`
3. Find free port: `net.Listen("tcp", ":0")` → get port → close listener
4. Create docker client (or use existing)
5. `ContainerCreate` with:
   - Image from opts.Env["HARNESS_IMAGE"] or "go-agent-harness:latest"
   - PortBindings: `"8080/tcp" -> host port`
   - Bind mount: workspace dir → `/workspace` in container
   - Env: pass through opts.Env
6. `ContainerStart`
7. Poll `ContainerInspect` until health/running (max 30s with 1s intervals)
8. Set ws.containerID, ws.hostPort, ws.harnessURL = "http://localhost:<hostPort>"
9. Set ws.workspacePath

**HarnessURL()**: return ws.harnessURL
**WorkspacePath()**: return ws.workspacePath
**Destroy(ctx)**:
1. If containerID empty, return nil
2. `ContainerStop` (5s timeout)
3. `ContainerRemove` (Force: true)
4. Set containerID = ""

Register as `"container"` in init().

### `internal/workspace/container_test.go`
Use `//go:build docker` tag — skip when Docker not available.

Tests:
- `TestContainerWorkspace_ImplementsWorkspace` (no build tag — compile-time only)
- `TestContainerWorkspace_Provision_EmptyID` (no build tag — no Docker needed)
- `TestContainerWorkspace_FullLifecycle` (docker build tag)
- `TestContainerWorkspace_Destroy_NotProvisioned` (no build tag)
- `TestContainerWorkspace_RegisteredInFactory` (no build tag)

### `build/Dockerfile.harnessd`
Multi-stage build producing a minimal Alpine image with harnessd binary.

## Port Allocation Strategy
```go
func getFreePort() (int, error) {
    ln, err := net.Listen("tcp", ":0")
    if err != nil { return 0, err }
    defer ln.Close()
    return ln.Addr().(*net.TCPAddr).Port, nil
}
```

## Healthcheck Polling
Poll `ContainerInspect` every second for up to 30s, checking state is "running".
Alternatively, poll HTTP `GET /health` on harnessd endpoint.

## Add to go.mod
```
go get github.com/docker/docker@v27.x
go get github.com/docker/go-connections@latest
```

## Commit Strategy
1. `feat(#184): add ContainerWorkspace implementation (Docker-based)`
2. `feat(#184): add Dockerfile for harnessd container image`

## Risk Areas
- Docker daemon may not be running in CI → always skip tests with build tag
- Port binding race (port freed between listen and docker bind) → acceptable risk for now
- Image may not exist → document that user must build image first, or pull from registry
