## When to Use
- Finding files by name pattern (e.g., "**/*.go", "internal/*/config.go")
- Discovering all files of a specific type in a directory tree
- Checking if files matching a pattern exist before reading them

## When NOT to Use
- Searching file content — use the grep tool
- Listing a single directory — use the ls tool for simple directory listing
- Finding a specific known file — use the read tool directly

## Behavioral Rules
1. Use ** for recursive matching (e.g., "**/*.go" matches all Go files in any subdirectory)
2. Prefer glob over `find` via bash for file discovery
3. Results are sorted by modification time — most recently modified files appear first

## Common Mistakes
- **ContentSearch**: Using glob to find files, then reading each one to search for content — use grep instead
- **FindViaBash**: Running `find . -name "*.go"` via bash instead of using the glob tool

## Examples
### WRONG
Glob "*.go" expecting to find all Go files in subdirectories — this only matches the current level.

### RIGHT
Glob "**/*.go" to recursively match all Go files in the directory tree.
