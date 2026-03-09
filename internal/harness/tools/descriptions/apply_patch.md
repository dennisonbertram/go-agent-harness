Apply a structured patch to one or more workspace files. Use this tool for bulk, multi-file, or complex edits -- NOT for single small edits (use the edit tool instead) and NOT for creating new files from scratch (use the write tool instead).

Supports three modes:
1. **Single find/replace** -- provide `path`, `find`, `replace`, and optionally `replace_all`. Best for renaming a symbol across one file.
2. **Multi-edit batch** -- provide `path` and an `edits` array of {old_text, new_text, replace_all} objects to apply multiple replacements to one file in a single call.
3. **Unified patch** -- provide a `patch` string in the custom patch format described below to add, update, or delete multiple files atomically.

## When to use apply_patch vs other tools
- **apply_patch**: renaming a symbol across many files, adding a header to multiple files, applying a pre-computed diff, or making several related edits in one file.
- **edit**: fixing a typo on one line, changing a single value, or any small targeted edit in one file.
- **write**: creating a brand-new file that does not yet exist.

## Unified patch format (IMPORTANT: this is NOT standard git diff)
This tool uses a CUSTOM patch format. Do NOT use standard git unified diff syntax (no `--- a/` or `+++ b/` lines). The format is:

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

Key rules:
- The patch MUST start with `*** Begin Patch` and end with `*** End Patch`.
- Each file section starts with `*** Update File: <path>`, `*** Add File: <path>`, or `*** Delete File: <path>`.
- Lines prefixed with `-` are removed, `+` are added, and ` ` (space prefix) are unchanged context.
- Each hunk starts with `@@` (the content after `@@` is ignored).
- For multi-file patches, include multiple `*** Update File:` sections.

### Concrete example: add a header to two files

```
*** Begin Patch
*** Update File: tools/helper.go
@@ top of file @@
 package tools
+
+// Added header comment
*** Update File: tools/utils.go
@@ top of file @@
 package tools
+
+// Added header comment
*** End Patch
```