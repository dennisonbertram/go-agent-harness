Delete a user-created agent profile from the user profiles directory.

Permanently removes the profile TOML file from the user profiles directory. Built-in profiles (github, researcher, reviewer, bash-runner, file-writer, full) are protected and cannot be deleted — they are embedded in the binary.

**Required fields:**
- `name` — the name of the profile to delete

**Behavior:**
- Only profiles in the user profiles directory can be deleted.
- If a user profile shadows a built-in with the same name, the user file is deleted and the built-in becomes visible again.
- If no user file exists and the name matches a built-in, an error is returned indicating the profile is protected.

**Errors returned:**
- Profile not found in the user directory.
- Profile is a built-in and cannot be deleted.
- Invalid profile name.
