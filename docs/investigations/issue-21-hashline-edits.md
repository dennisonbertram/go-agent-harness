# Issue #21: Hashline Edits — Reliable File Editing via Content-Hash Anchoring

**Date**: 2026-03-14
**Status**: Research
**Benchmark**: 6.7% → 68.3% success improvement (reported for Grok Code Fast 1)

---

## 1. What Are Hashline Edits?

Hashline edits are a file editing technique from the `oh-my-pi` (omp) fork of Pi
agent. Instead of matching `old_text` by exact string comparison, edits are
anchored to **content hashes of specific lines**.

From the Pi research doc:
> "Content-hash anchored line editing (reported 6.7% to 68.3% success
> improvement on Grok Code Fast 1)"
> "omp's hashline edits — content-hash anchored editing eliminates whitespace
> ambiguity. Measured 10x improvement on some models."

The core insight: LLMs frequently hallucinate or slightly misquote code when
constructing the `old_text` parameter for an edit. A single extra space,
different indentation, or missing trailing newline causes the entire edit to
fail silently (or with an unhelpful "text not found" error). Hashline edits
replace fragile string matching with hash-based line addressing.

---

## 2. The Problem With Current File Editing

### Current Implementation

From `internal/harness/tools/edit.go`:

```go
// The critical path:
if args.ReplaceAll {
    replacements = strings.Count(original, args.OldText)
    updated = strings.ReplaceAll(original, args.OldText, args.NewText)
} else {
    if strings.Contains(original, args.OldText) {
        replacements = 1
        updated = strings.Replace(original, args.OldText, args.NewText, 1)
    }
}
if replacements == 0 {
    return "", fmt.Errorf("old_text not found in %s", args.Path)
}
```

The entire edit mechanism is `strings.Contains` / `strings.Replace`. This is
exact byte-for-byte matching with no tolerance for any deviation.

### Failure Modes

**1. Whitespace mismatches** (most common)
- LLM quotes 4-space indentation; file uses 2-space or tabs
- LLM omits trailing spaces that exist in the file
- LLM adds a trailing newline where none exists

**2. LLM hallucination of content**
- LLM partially misquotes a long function signature
- LLM inserts a comment that isn't in the original
- LLM quotes a different version of the function than currently exists

**3. Multi-line quote boundary errors**
- LLM starts/ends the quote at the wrong line (off by one)
- Inconsistent line endings (CRLF vs LF) on Windows paths

**4. File changed between read and edit**
- The `expected_version` field (SHA256 of file content) already handles this
  case — but only if the LLM remembers to use it

**5. Duplicate text**
- `strings.Replace` (first occurrence) is ambiguous when the text appears
  multiple times. The LLM may intend to change the second occurrence.
- The `apply_patch` tool's `occurrence` field mitigates this partially

### How Often Do Edits Fail in Practice?

The 6.7% baseline figure from omp represents the success rate with standard
string-match editing on one particular benchmark using Grok Code Fast 1.
This is an extreme case, but even for strong models (Claude, GPT-4) edit
failures are a meaningful fraction of runs:

- Anthropic's Claude Code uses anchor-context matching (see §4) specifically
  to address this
- The harness currently returns an error that the LLM must then recover from
  by re-reading the file and retrying — this wastes 1-2 LLM turns per failure

---

## 3. Hashline Edit Mechanism

### Core Algorithm

1. **At read time**: when the agent reads a file, each line is annotated with a
   short hash derived from its content. The hashes are exposed in the tool
   output.

2. **At edit time**: instead of providing exact `old_text`, the agent provides:
   - The starting line hash (first line of the section to replace)
   - The ending line hash (last line of the section to replace)
   - The replacement content

3. **At execution time**: the tool finds lines matching the provided hashes,
   validates they exist and are contiguous (or accepts a range), and replaces
   the content between them.

### Hash Definition

The hash of a line is typically computed as:
```
hash = sha256(line_content)[:6_bytes_hex]  ← 12 hex chars
```

Content is the raw line text, typically **without** trailing newline but
**with** leading whitespace (so indentation is part of the hash identity).

In the oh-my-pi implementation, the hash is used as a stable address:
- If the file content at that line has changed (different whitespace, different
  text), the hash won't match and the edit is rejected with a useful error
- The LLM cannot hallucinate a valid hash for a line it hasn't seen

### What the Agent Sees in Read Output

Instead of plain text, the file is returned with per-line hashes:

```
[a3f2c1] func processOrder(id string) error {
[b7e4d9]     if id == "" {
[c1a8f3]         return errors.New("empty id")
[d2b9e0]     }
[e5c7f2]     return db.Process(id)
[f8d1a4] }
```

The hashes are short enough to be cheap in the context window, and unique
enough (6 bytes) to avoid collisions within typical file sizes.

### Edit Request With Hashline Addressing

```json
{
  "path": "order.go",
  "start_hash": "b7e4d9",
  "end_hash": "d2b9e0",
  "new_text": "    if id == \"\" {\n        return ErrEmptyID\n    }"
}
```

The tool finds the line with hash `b7e4d9`, finds the line with hash `d2b9e0`,
validates they are in order, replaces the content between (inclusive) with
`new_text`.

**Key benefit**: the LLM does not need to quote the exact old text. It only
needs to correctly copy two short hashes from the read output.

---

## 4. Alternative Approaches

### Anchor-Context Editing (Claude Code Style)

Claude Code's `Edit` tool uses `old_string` / `new_string` but with important
UX conventions:
- The tool description explicitly requires sufficient surrounding context
- The `old_string` must be unique within the file
- The model is instructed to include uniquely identifying lines above/below the
  changed section

This is effectively "hashline without the hash" — relying on sufficient context
to make the match unique and unambiguous. The uniqueness requirement is
enforced by instruction, not by tooling.

**Weakness**: whitespace mismatches still cause failures; the model must read
the file exactly right.

### Line Number + Validation

```json
{
  "path": "file.go",
  "start_line": 42,
  "end_line": 47,
  "expected_content": "func foo() {",  // optional validation of first line
  "new_text": "..."
}
```

This is simple and LLMs can work with it, but line numbers are fragile:
they change as the file is edited, making sequential edits risky.

**Weakness**: sequential edits invalidate line numbers; LLM must re-read and
re-plan after every edit.

### AST-Based Editing

Edit at the level of syntax nodes (function bodies, struct fields, etc.) using
a language-aware parser. The `oh-my-pi` fork has `ast_grep` and `ast_edit` for
this.

**Weakness**: language-specific; complex to implement for 30+ languages; not
available in the current harness (no language-specific parsing); overkill for
most edits.

### Unified Diff Format

The `apply_patch` tool already supports standard unified diff (`--- a/file /
+++ b/file`) and the custom `*** Begin Patch` format. This is context-based
matching: the diff shows surrounding unchanged lines as context.

```diff
--- a/order.go
+++ b/order.go
@@ -5,7 +5,7 @@
     if id == "" {
-        return errors.New("empty id")
+        return ErrEmptyID
     }
```

**Strength**: context lines help locate the change; standard format.
**Weakness**: still string-match based; whitespace in context lines can cause
mismatches.

---

## 5. Comparison Table

| Approach | Whitespace Tolerance | Requires Read | Token Cost | Complexity |
|----------|---------------------|---------------|------------|------------|
| Current (`edit`, exact match) | None | No (LLM recalls) | Lowest | Low |
| Anchor-context (Claude Code) | None | Required | Low | Low |
| Hashline (omp) | Full | Required | +hashes in output | Medium |
| Line number + validation | None | Required | Low | Low |
| Unified diff | Low (context lines) | Optional | Medium | Medium |
| AST-based | Full | Required | High | Very High |

**For this harness**: Hashline is the most targeted improvement for the key
failure mode (whitespace/hallucination mismatches). The token cost increase
is bounded (12 chars per line) and the implementation is straightforward.

---

## 6. Implementation in This Codebase

### Token Cost Analysis

Current `read` output for a 100-line file:
```json
{"path": "foo.go", "content": "...", "version": "a3f2c1b8"}
```

Hashline `read` output for a 100-line file:
```
[a3f2c1] line 1 content
[b7e4d9] line 2 content
...
```

Additional tokens: ~5 tokens per line × 100 lines = 500 additional tokens per
file read. For the typical 200-line file this harness works with: ~1,000 tokens
overhead per read.

At gpt-4.1 pricing ($2/1M input): $0.002 per read. For a session with 10 file
reads: $0.02 extra. This is acceptable given the edit reliability improvement.

### How Hashes Would Be Exposed

Option A: Modify the `read` tool to return hashline-annotated content by
default or with a `hash_lines=true` parameter.

Option B: New `read_with_hashes` tool (separate from `read`).

Option C: The `edit` tool detects hashline-format old_text and switches
to hash-based addressing automatically.

**Recommendation**: Option A with an opt-in parameter. The LLM should use
`hash_lines=true` when it anticipates editing the file. This avoids increasing
token cost for read-only operations.

### New `edit_by_hash` Parameters

The cleanest implementation is a new parameter set on the existing `edit` tool:

```go
// Existing params: path, old_text, new_text, replace_all, expected_version
// New params:
"start_line_hash": {"type": "string", "description": "hash of first line to replace"},
"end_line_hash":   {"type": "string", "description": "hash of last line to replace (inclusive)"},
```

When `start_line_hash` is provided (and `old_text` is empty), the tool switches
to hash-based addressing.

### Implementation Sketch

```go
// In editTool handler, after reading file content:
if args.StartLineHash != "" {
    lines := strings.Split(original, "\n")
    startIdx, endIdx := -1, -1
    for i, line := range lines {
        h := lineHash(line)
        if h == args.StartLineHash && startIdx == -1 {
            startIdx = i
        }
        if h == args.EndLineHash {
            endIdx = i
        }
    }
    if startIdx == -1 {
        return "", fmt.Errorf("start_line_hash %q not found in %s", args.StartLineHash, args.Path)
    }
    if args.EndLineHash != "" && endIdx == -1 {
        return "", fmt.Errorf("end_line_hash %q not found in %s", args.EndLineHash, args.Path)
    }
    if endIdx == -1 {
        endIdx = startIdx // single-line replacement
    }
    if endIdx < startIdx {
        return "", fmt.Errorf("end_line_hash appears before start_line_hash in %s", args.Path)
    }
    // Replace lines[startIdx:endIdx+1] with new_text
    newLines := strings.Split(args.NewText, "\n")
    updated = strings.Join(append(lines[:startIdx], append(newLines, lines[endIdx+1:]...)...), "\n")
    replacements = 1
}

// lineHash: 6-byte SHA256 prefix of the line content
func lineHash(line string) string {
    h := sha256.Sum256([]byte(line))
    return hex.EncodeToString(h[:6])
}
```

Note: the `read` tool already uses `FileVersionFromBytes` (8-byte SHA256) for
the file-level version hash (`versioning.go`). The line-level hash is the same
mechanism applied per-line. The existing infrastructure supports this.

### Integration With Existing Version Control

The `expected_version` field (file-level SHA256) should still be supported
alongside hashline edits. It prevents edits to stale file versions.

### Read Tool Modification

```go
// New parameter in readTool:
"hash_lines": {"type": "boolean", "description": "annotate each line with a content hash"}

// In handler, when hash_lines is true:
lines := strings.Split(text, "\n")
var annotated strings.Builder
for _, line := range lines {
    h := lineHash(line)
    annotated.WriteString(fmt.Sprintf("[%s] %s\n", h, line))
}
result["content"] = annotated.String()
result["hash_lines"] = true
```

---

## 7. Backward Compatibility

The hashline approach is fully backward-compatible:
- Existing `edit` calls with `old_text` continue to work unchanged
- New `start_line_hash` / `end_line_hash` parameters are additive
- `read` with `hash_lines=false` (default) returns unmodified content

LLMs can continue using `old_text` for simple, unambiguous edits and switch to
hashline for complex or whitespace-sensitive edits.

---

## 8. Prototype Plan

**Phase 1** (1 day): Core mechanic
- Add `lineHash()` to `versioning.go`
- Add `hash_lines` bool to `read` tool parameters
- When `hash_lines=true`, annotate content with `[hash] line` format
- Add `start_line_hash` / `end_line_hash` to `edit` tool
- Hash-based replacement logic in `edit` handler

**Phase 2** (0.5 day): Tests
- Unit tests: `lineHash()`, annotated read output, hash-based edit
- Integration test: read with `hash_lines=true`, edit with hash addresses
- Failure case tests: hash not found, end before start, whitespace variants

**Phase 3** (0.5 day): Description updates
- Update `descriptions/edit.md` and `descriptions/read.md`
- Add example showing hashline workflow

**Total**: ~2 days

---

## 9. Open Questions

1. **Hash length**: 12 hex chars (6 bytes) gives 2^48 possible values.
   Collision probability within a 10,000-line file is ~10^-8. Is this
   sufficient, or should we use 8 bytes (16 chars)?

2. **Trailing whitespace in hashes**: should line hashes include trailing
   spaces? Including them makes hashes file-format specific but more precise.
   Excluding them makes them more tolerant of editor auto-trimming.

3. **apply_patch integration**: should hashline addressing also be added to
   `apply_patch` for multi-edit operations?

4. **Benchmark**: what representative edit tasks should we use to measure the
   improvement in this harness? A synthetic set of whitespace-sensitive Go
   files with known edits would be a good start.

5. **Token overhead per session**: for a typical 10-read session, the overhead
   is ~10K tokens. Is this acceptable as the default, or should it be opt-in?
   Opt-in is recommended to avoid surprising existing users.
