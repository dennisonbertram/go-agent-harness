## When to Use
- Reading a specific file you already know exists
- Targeted reads of a known portion of a file (use offset and limit)
- Reading a file before editing it (always required before edit)
- Reading configuration files, source files, or data files

## When NOT to Use
- Searching for files by name — use the glob tool
- Searching for content across multiple files — use the grep tool
- Bulk exploration of a directory — use ls first, then read specific files
- Reading an entire large file when only a section is needed — use offset/limit

## Behavioral Rules
1. Prefer offset+limit when you know which section of a large file you need
2. Use grep to locate the relevant section first, then read with offset if the file is large
3. Never read a file just to check if it exists — use glob or ls
4. Reading a file that does not exist returns an error, not empty content

## Common Mistakes
- **BulkRead**: Reading all files in a directory with the read tool one by one instead of using grep to find the relevant content
- **NoOffsetOnLargeFile**: Reading a 5000-line file from offset 0 when you only need lines 200-250
- **ReadToCheckExistence**: Using read to check if a file exists — use glob instead

## Examples
### WRONG
Read every file in the directory looking for a function definition.

### RIGHT
Use grep with the function name pattern to find the file and line number, then read that specific file with an appropriate offset.
