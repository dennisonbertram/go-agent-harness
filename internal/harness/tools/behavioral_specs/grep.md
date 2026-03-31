## When to Use
- Searching for a string or pattern across multiple files
- Finding where a function, variable, or symbol is defined or used
- Locating files that contain specific content
- Code navigation: finding all usages of an interface or type

## When NOT to Use
- Finding files by name pattern — use the glob tool
- Reading a specific file you already know — use the read tool
- Listing directory contents — use the ls tool

## Behavioral Rules
1. Use the glob parameter to narrow the search to relevant file types (e.g., "*.go", "**/*.ts")
2. Use output_mode="files_with_matches" when you only need file paths
3. Use output_mode="content" with context (-C) when you need surrounding lines
4. Prefer grep over bash-based `grep` or `rg` commands

## Common Mistakes
- **BashGrep**: Running `grep -r pattern .` via the bash tool instead of using the grep tool directly
- **NoGlobFilter**: Searching all files (including binaries, vendor/) when only source files are needed
- **ReadThenSearch**: Reading entire files to search for content instead of using grep

## Examples
### WRONG
```bash
grep -rn "BehavioralSpec" . --include="*.go"
```

### RIGHT
Use the grep tool with pattern="BehavioralSpec" and glob="*.go".
