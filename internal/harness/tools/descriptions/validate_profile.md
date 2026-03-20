Validate a profile TOML string without writing it to disk.

Parse and validate a profile configuration given as a TOML string. This is a dry-run check useful before calling `create_profile` or `update_profile`. No files are created or modified.

**Required fields:**
- `toml` — the full TOML content of the profile to validate

**Validation checks performed:**
1. TOML syntax is valid and parseable.
2. `[meta].name` is present and non-empty.
3. `[meta].description` is present and non-empty.
4. Profile name does not contain path separators or traversal sequences.

**Returns:**
- On success: `{"status":"valid","name":"<profile-name>"}` indicating the profile is well-formed.
- On error: an error message describing the first validation failure found.
