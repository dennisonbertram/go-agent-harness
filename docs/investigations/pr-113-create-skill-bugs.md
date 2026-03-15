# PR #113 `create_skill` Tool — Bug Investigation

**Date:** 2026-03-10
**PR:** #113 (branch `issue-58-create-skill-tool`)
**Issue:** #58

---

## Overview

PR #113 adds a `create_skill` deferred tool (`internal/harness/tools/deferred/create_skill.go`) that lets agents create validated SKILL.md files. This investigation identifies **two bugs**: a trigger field mismatch and a TOCTOU race condition.

---

## Bug 1: `trigger` YAML Field Is Written But Never Read

### How `create_skill` writes the YAML frontmatter

The `create_skill` tool accepts a `trigger` parameter (required) and writes it as a YAML frontmatter field:

```go
// From create_skill.go, lines ~91-101
var fm strings.Builder
fm.WriteString("---\n")
fmt.Fprintf(&fm, "name: %s\n", name)
fmt.Fprintf(&fm, "description: %s\n", quoteYAMLString(args.Description))
if strings.TrimSpace(args.Trigger) != "" {
    fmt.Fprintf(&fm, "trigger: %s\n", quoteYAMLString(args.Trigger))
}
fm.WriteString("version: 1\n")
fm.WriteString("---\n")
```

This produces a SKILL.md file like:

```yaml
---
name: code-review
description: "Review code for quality"
trigger: "When user asks to review code"
version: 1
---
```

### How the skill loader reads triggers

The skill loader (`internal/skills/loader.go`, `parseSkillFile` function, line 130) extracts triggers from the **description** field, not from a dedicated `trigger` YAML field:

```go
triggers := ExtractTriggers(meta.Description)
```

The `ExtractTriggers` function (`internal/skills/trigger.go`) searches for `"Trigger:"` or `"Triggers:"` as a substring **within the description text**:

```go
func ExtractTriggers(description string) []string {
    lower := strings.ToLower(description)
    // Look for "triggers:" first, then "trigger:"
    if i := strings.Index(lower, "triggers:"); i >= 0 {
        idx = i + len("triggers:")
        found = true
    } else if i := strings.Index(lower, "trigger:"); i >= 0 {
        idx = i + len("trigger:")
        found = true
    }
    // ... splits remaining text on commas
}
```

### The `frontmatter` struct has no `trigger` field

The `frontmatter` struct in `internal/skills/types.go` (lines 44-57) defines all YAML-parseable fields:

```go
type frontmatter struct {
    Name         string   `yaml:"name"`
    Description  string   `yaml:"description"`
    Version      int      `yaml:"version"`
    AutoInvoke   *bool    `yaml:"auto-invoke"`
    AllowedTools []string `yaml:"allowed-tools"`
    ArgumentHint string   `yaml:"argument-hint"`
    Context      string   `yaml:"context"`
    Agent        string   `yaml:"agent"`
    Verified     bool     `yaml:"verified"`
    VerifiedAt   string   `yaml:"verified_at"`
    VerifiedBy   string   `yaml:"verified_by"`
}
```

There is **no `trigger` field** with a `yaml:"trigger"` tag. When the loader parses the YAML frontmatter, the `trigger` key is silently ignored by `gopkg.in/yaml.v3`.

### The mismatch

| Component | How triggers work |
|---|---|
| `create_skill` tool | Writes `trigger: "..."` as a **separate YAML frontmatter field** |
| Skill loader (`parseSkillFile`) | Calls `ExtractTriggers(meta.Description)` to find `"Trigger: ..."` **embedded in the description string** |
| `frontmatter` struct | Has **no `trigger` YAML tag** — the field is silently dropped |

**Result:** Skills created by `create_skill` will **never have working triggers**. The `trigger` field is written to disk but completely ignored on load. The loader only finds triggers embedded in the description text (e.g., `description: "Review code. Trigger: review my code"`).

### Evidence from existing tests

The loader test in `internal/skills/loader_test.go` confirms the expected pattern:

```go
const validSkillMD = `---
name: my-skill
description: "A test skill. Trigger: do my thing"
version: 1
---
`
// ...
if len(s.Triggers) != 1 || s.Triggers[0] != "do my thing" {
    t.Errorf("Triggers = %v, want [do my thing]", s.Triggers)
}
```

The trigger phrase `"do my thing"` is embedded **inside** the description string after `"Trigger: "`.

### Fix options

**Option A (recommended): Append trigger to description in `create_skill`.**
Instead of writing a separate `trigger:` YAML field, embed the trigger phrase in the description so `ExtractTriggers` can find it:

```go
// Instead of writing a separate trigger field:
desc := args.Description
if trigger := strings.TrimSpace(args.Trigger); trigger != "" {
    desc = desc + ". Trigger: " + trigger
}
fmt.Fprintf(&fm, "description: %s\n", quoteYAMLString(desc))
// Remove the separate trigger: line
```

**Option B: Add `trigger` to the frontmatter struct and use it in the loader.**
Add `Trigger string \`yaml:"trigger"\`` to the `frontmatter` struct and modify `parseSkillFile` to use it alongside or instead of `ExtractTriggers`. This is a larger change that affects the loader, types, registry, and possibly tests.

---

## Bug 2: TOCTOU Race Condition in Duplicate Detection

### The vulnerable code

In `create_skill.go`, lines ~88-100:

```go
// Duplicate detection
skillDir := filepath.Join(skillsDir, name)
skillFile := filepath.Join(skillDir, "SKILL.md")
if _, err := os.Stat(skillFile); err == nil {          // <-- CHECK
    return "", fmt.Errorf("skill %q already exists at %s", name, skillFile)
}

// Build YAML frontmatter...
// ...

// Create directory and write file
if err := os.MkdirAll(skillDir, 0o755); err != nil {   // <-- USE (gap)
    return "", fmt.Errorf("create skill directory %s: %w", skillDir, err)
}
if err := os.WriteFile(skillFile, []byte(fullContent), 0o644); err != nil { // <-- USE
    return "", fmt.Errorf("write skill file %s: %w", skillFile, err)
}
```

### The race window

1. **Thread A** calls `os.Stat(skillFile)` — file does not exist, proceeds.
2. **Thread B** calls `os.Stat(skillFile)` — file does not exist, proceeds.
3. **Thread A** calls `os.MkdirAll` + `os.WriteFile` — creates the skill.
4. **Thread B** calls `os.MkdirAll` + `os.WriteFile` — **silently overwrites** Thread A's skill.

This is a classic Time-of-Check-to-Time-of-Use (TOCTOU) race. Between the `os.Stat` check and the `os.WriteFile` write, another goroutine (or process) can create the file.

### Severity

- The tool is marked `ParallelSafe: false`, which provides some protection at the harness level (the tool loop serializes calls to non-parallel-safe tools within a single run).
- However, concurrent runs or external processes could still trigger the race.
- The existing `TestCreateSkillToolConcurrentCreation` test creates **different** skill names concurrently, so it does not catch this race for the **same** name.

### The same pattern exists in `create_prompt_extension.go`

The `create_prompt_extension.go` tool has the identical TOCTOU pattern (lines 144-155):

```go
if _, err := os.Stat(absPath); err == nil {
    if !args.Overwrite {
        return "", fmt.Errorf("extension file %q already exists; ...", filename)
    }
} else if !os.IsNotExist(err) {
    return "", fmt.Errorf("check existing extension: %w", err)
}
// ... (gap)
if err := os.WriteFile(absPath, []byte(args.Content), 0o644); err != nil {
```

### Fix

Use `os.OpenFile` with `O_CREATE|O_EXCL` flags, which atomically creates a file only if it does not already exist. If the file exists, the call returns an error:

```go
// Atomic create: fails if file already exists
f, err := os.OpenFile(skillFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
if err != nil {
    if os.IsExist(err) {
        return "", fmt.Errorf("skill %q already exists at %s", name, skillFile)
    }
    return "", fmt.Errorf("create skill file %s: %w", skillFile, err)
}
defer f.Close()

if _, err := f.Write([]byte(fullContent)); err != nil {
    return "", fmt.Errorf("write skill file %s: %w", skillFile, err)
}
```

This eliminates the race window entirely. The `os.MkdirAll` call before it is safe because `MkdirAll` is idempotent.

---

## Summary of Required Changes

| Bug | File | Fix |
|---|---|---|
| Trigger field mismatch | `internal/harness/tools/deferred/create_skill.go` | Embed trigger in description string (e.g., `desc + ". Trigger: " + trigger`) instead of writing a separate `trigger:` YAML field |
| TOCTOU race condition | `internal/harness/tools/deferred/create_skill.go` | Replace `os.Stat` + `os.WriteFile` with `os.OpenFile(O_CREATE\|O_EXCL)` for atomic create |
| (Same TOCTOU pattern) | `internal/harness/tools/deferred/create_prompt_extension.go` | Same fix: use `O_CREATE\|O_EXCL` instead of Stat-then-Write |

### Files read during this investigation

- `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/deferred/create_skill.go` (from branch `issue-58-create-skill-tool`)
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/deferred/create_skill_test.go` (from branch)
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/descriptions/create_skill.md` (from branch)
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/skills/loader.go` — `parseSkillFile`, `WriteVerification`, `splitFrontmatter`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/skills/trigger.go` — `ExtractTriggers`, `MatchTrigger`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/skills/trigger_test.go`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/skills/types.go` — `Skill` struct, `frontmatter` struct
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/skills/registry.go` — `MatchTriggers`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/skills/loader_test.go`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/skill.go`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/core/skill.go`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/verify_skill.go`
- `/Users/dennisonbertram/Develop/go-agent-harness/internal/harness/tools/deferred/create_prompt_extension.go`
