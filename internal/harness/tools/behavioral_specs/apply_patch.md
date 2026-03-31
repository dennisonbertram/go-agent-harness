## When to Use
- Making changes to multiple locations across one or more files in a single operation
- Applying a well-defined set of changes expressed as a unified diff
- Bulk refactoring where the changes are known in advance

## When NOT to Use
- Single-location changes — use the edit tool for one targeted edit
- Creating new files from scratch — use the write tool
- When the patch source is unknown or untrusted

## Behavioral Rules
1. Prefer apply_patch over multiple sequential edit calls when changing more than 2 locations
2. Verify the patch applies cleanly before relying on it — a failed patch leaves the file unchanged
3. Use unified diff format (-u) with sufficient context lines (3+) for the patch to apply correctly
4. After applying, verify the result with a read or build step

## Common Mistakes
- **MultipleEditsInsteadOfPatch**: Making 5 separate edit calls when a single patch would be cleaner and atomic
- **InsufficientContext**: Writing a patch with 0 context lines — it will fail if the file has even minor whitespace differences
- **PatchNewFile**: Using apply_patch to create a new file instead of the write tool

## Examples
### WRONG
Use 4 separate edit tool calls to update the same import block in 4 different files.

### RIGHT
Construct a unified diff that updates all 4 files in one apply_patch call, making the change atomic.
