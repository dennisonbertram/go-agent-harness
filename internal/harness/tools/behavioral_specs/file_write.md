## When to Use
- Creating a brand new file that does not yet exist
- Performing a complete, intentional rewrite of an existing file
- Writing generated output (test fixtures, config templates, data files)

## When NOT to Use
- Modifying an existing file — use the edit tool for targeted changes
- Adding a small section to an existing file — use edit to append or insert

## Behavioral Rules
1. If the file already exists, you MUST read it first before overwriting
2. Prefer the edit tool for any modification to an existing file — only use write for new files or full rewrites
3. Never write secrets, credentials, or API keys to files
4. Use write for complete file creation; edit for surgical modifications

## Common Mistakes
- **OverwriteWithoutReading**: Writing to an existing file without reading it first — you will lose the original content
- **WriteInsteadOfEdit**: Using write to change 3 lines in a 200-line file — the edit tool is the right choice
- **PartialWrite**: Writing only part of the desired file content and expecting the rest to be preserved — write always replaces the entire file

## Examples
### WRONG
Use write to add a single function to an existing Go file, losing all the other functions.

### RIGHT
Use edit to add the function by specifying old_string as the closing brace of the last function and new_string as the new function plus the closing brace.
