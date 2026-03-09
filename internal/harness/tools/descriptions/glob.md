Find files by name pattern using glob syntax. Use this to locate files when you know part of the filename or extension but not the exact path. Glob matches file and directory NAMES only — it does NOT search file contents (use grep for content search).

IMPORTANT: This uses Go's filepath.Glob, which does NOT support recursive "**" patterns. Each "*" matches within a single directory level only. To find files in deeply nested directories, use multiple levels of "*".

Common patterns:
- */*.go — Go files one level deep (e.g. cmd/main.go)
- */*/*.go — Go files two levels deep (e.g. internal/harness/runner.go)
- */*/*/*.go — Go files three levels deep
- docs/*.md — Markdown files directly in docs/
- docs/*/*.md — Markdown files one level under docs/
- */*_test.go — test files one level deep
- */*/*_test.go — test files two levels deep
- prompts/*.yaml — YAML files in prompts/

To find ALL files of a type across the entire project, call glob multiple times with increasing depth: */*.ext, */*/*.ext, */*/*/*.ext, etc. Or use ls with recursive=true and then filter by extension.

The pattern is relative to the workspace root. Returns a list of matching file paths. Use max_matches to limit results (default 500, max 2000).