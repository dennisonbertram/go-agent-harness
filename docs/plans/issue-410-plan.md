# Issue #410 Implementation Plan — feat(prompt): auto-load repo AGENTS.md into resolved system prompts

**Date**: 2026-03-23
**Branch**: issue-410-auto-load-repo-agents-md-into-prompts

## Summary

When a workspace/repo path is known, automatically read the root `AGENTS.md` file and inject it as a distinct, provenance-labeled section into the resolved system prompt, positioned after MODEL_PROFILE and before TASK_CONTEXT. Fail soft on absent files, fail closed on path-escape.

## Files to Create/Modify

| File | Change |
|------|--------|
| `internal/systemprompt/types.go` | Add `WorkspacePath string` to `ResolveRequest`; add `AgentsMdLoaded bool` to `ResolvedPrompt` |
| `internal/systemprompt/engine.go` | Add `readAgentsMd()` func; update `Resolve()` to inject AGENTS_MD section |
| `internal/systemprompt/engine_test.go` | Add 5 test cases covering happy path, absent, unreadable, path escape, empty path |
| `internal/harness/runner.go` | Thread workspace path from provisioned workspace into `ResolveRequest` |

## Approach

1. Add `WorkspacePath string` to `ResolveRequest` in types.go
2. Add `AgentsMdLoaded bool` to `ResolvedPrompt` for test/debug observability
3. Implement `readAgentsMd(workspaceRoot string) (string, error)` in engine.go:
   - Skip if `workspaceRoot == ""` (return `"", nil`)
   - Clean and validate path using `filepath.Clean` + `filepath.Rel` to prevent escape
   - Return `"", nil` on `os.IsNotExist` (soft fail)
   - Return `"", err` on other errors (logged as warning)
   - Return file content on success
4. In `Resolve()`, call `readAgentsMd(req.WorkspacePath)` and if content present, insert `promptSection{Name: "AGENTS_MD", Content: content}` after MODEL_PROFILE
5. In `runner.go`, after `ws.WorkspacePath()` is available, pass it to `ResolveRequest`

## Prompt Section Ordering

```
BASE → INTENT → MODEL_PROFILE → AGENTS_MD (new) → TASK_CONTEXT → BEHAVIORS → TALENTS → SKILLS → CUSTOM
```

## Testing Strategy

**Regression tests** (add before implementation):
- `TestResolveWithEmptyWorkspacePath` — no AGENTS_MD section, no error
- `TestResolveSkipsAgentsMdWhenAbsent` — file absent, no section, AgentsMdLoaded=false
- `TestResolveLoadsAgentsMdFromWorkspace` — file present, content in prompt, AgentsMdLoaded=true
- `TestResolveWarnsOnAgentsMdReadFailure` — unreadable file, warning emitted, no crash
- `TestResolveRejectsPathEscape` — path traversal attempt fails closed

**Existing tests** to pin:
- All existing systemprompt tests must continue passing
- runner integration tests with WorkspacePath="" must behave identically

## Path-Escape Protection

```go
func readAgentsMd(workspaceRoot string) (string, error) {
    if workspaceRoot == "" {
        return "", nil
    }
    absRoot, err := filepath.Abs(workspaceRoot)
    if err != nil {
        return "", fmt.Errorf("invalid workspace path: %w", err)
    }
    absRoot = filepath.Clean(absRoot)
    candidate := filepath.Join(absRoot, "AGENTS.md")
    rel, err := filepath.Rel(absRoot, candidate)
    if err != nil || rel != "AGENTS.md" {
        return "", fmt.Errorf("AGENTS.md path escape detected")
    }
    content, err := os.ReadFile(candidate)
    if os.IsNotExist(err) {
        return "", nil  // soft fail
    }
    if err != nil {
        return "", err
    }
    return string(content), nil
}
```

## Risk Areas

- Large AGENTS.md: no truncation for now (reasonable assumption)
- Workspace path availability: path is available after provisioning but before first LLM call — timing is safe
- Tests using temp dirs: use `t.TempDir()` for isolation

## Commit Strategy

Single commit:
```
feat(#410): auto-load repo AGENTS.md into resolved prompts
```
