## When to Use
- Modifying an existing file when you know the exact content to change
- Making targeted, surgical edits to specific lines or blocks
- Renaming symbols or making the same change in multiple places (use replace_all)

## When NOT to Use
- Creating a new file — use the write tool for new files
- Making changes without reading the file first
- Replacing more content than necessary to uniquely identify the location
- Rewriting the entire file — use write for complete rewrites

## Behavioral Rules
1. Always read the file with the read tool before editing — never edit blind
2. Use the smallest unique old_string that unambiguously identifies the location (2-4 lines of context)
3. Preserve the exact indentation (tabs/spaces) as shown in the read output — do not alter whitespace
4. Use replace_all=true when renaming a variable or making the same change everywhere
5. Do not include line number prefixes in old_string or new_string
6. Verify the edit compiled or is syntactically valid after applying

## Common Mistakes
- **EditWithoutReading**: Attempting to edit a file without calling the read tool first — the edit will fail
- **TooNarrowContext**: Using a one-word old_string that could match multiple locations, causing ambiguous edits
- **IndentationDrift**: Changing indentation style (tabs to spaces) while making a different edit
- **FullFileRewrite**: Providing the entire file as old_string and new_string when only a small section changed

## Examples
### WRONG
Edit with old_string = "Enabled" — this is too short and will match the wrong location.

### RIGHT
Edit with old_string spanning 3 lines:
```
	Memory: MemoryConfig{
		Enabled: true,
		Mode:    "auto",
```
This uniquely identifies the exact location without ambiguity.
