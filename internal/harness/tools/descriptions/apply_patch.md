Apply a structured patch to one or more workspace files. Use this tool for bulk, multi-file, or complex edits -- NOT for single small edits (use the edit tool instead) and NOT for creating new files from scratch (use the write tool instead).

Supports three modes:
1. **Single find/replace** -- provide `path`, `find`, `replace`, and optionally `replace_all`. Best for renaming a symbol across one file.
2. **Multi-edit batch** -- provide `path` and an `edits` array of {old_text, new_text, replace_all} objects to apply multiple replacements to one file in a single call.
3. **Unified patch** -- provide a `patch` string in either standard unified diff format or the custom patch format to add, update, or delete multiple files atomically.

## When to use apply_patch vs other tools
- **apply_patch**: renaming a symbol across many files, adding a header to multiple files, applying a pre-computed diff, or making several related edits in one file.
- **edit**: fixing a typo on one line, changing a single value, or any small targeted edit in one file.
- **write**: creating a brand-new file that does not yet exist.

## Unified patch formats

### Standard unified diff (recommended — produced by `git diff`, `diff -u`, and most tools)

```diff
--- a/path/to/file.go
+++ b/path/to/file.go
@@ -10,7 +10,7 @@
 unchanged context line
-old line to remove
+new line to add
 more context
```

- Use `--- /dev/null` / `+++ b/path` to create a new file.
- Use `--- a/path` / `+++ /dev/null` to delete a file.
- Multiple file sections may appear in a single patch string.
- The `a/` and `b/` prefixes are stripped automatically.

### Custom patch format (alternative)

```
*** Begin Patch
*** Update File: path/to/file.go
@@ context @@
 unchanged context line
-old line to remove
+new line to add
 more context
*** Add File: path/to/new_file.go
+first line of new file
+second line
*** Delete File: path/to/obsolete.go
*** End Patch
```

Key rules for the custom format:
- The patch MUST start with `*** Begin Patch` and end with `*** End Patch`.
- Each file section starts with `*** Update File: <path>`, `*** Add File: <path>`, or `*** Delete File: <path>`.
- Lines prefixed with `-` are removed, `+` are added, and ` ` (space prefix) are unchanged context.
- Each hunk starts with `@@` (the content after `@@` is ignored).

### Concrete example: add a header to two files (standard unified diff)

```diff
--- a/tools/helper.go
+++ b/tools/helper.go
@@ -1,3 +1,5 @@
 package tools
+
+// Added header comment
--- a/tools/utils.go
+++ b/tools/utils.go
@@ -1,3 +1,5 @@
 package tools
+
+// Added header comment
```