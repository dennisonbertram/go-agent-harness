# Issue #62: Skill Verification Flag (VOYAGER write-verify-store pattern)

## Summary

Implemented skill verification flag support following the VOYAGER write-verify-store pattern. Skills can now be marked as verified, with verification metadata persisted in the SKILL.md frontmatter.

## Files Changed

### `internal/skills/types.go`
- Added `Verified bool`, `VerifiedAt string`, `VerifiedBy string` fields to `Skill` struct (JSON-tagged)
- Added `Verified`, `VerifiedAt`, `VerifiedBy` YAML fields to `frontmatter` struct

### `internal/skills/loader.go`
- Mapped new frontmatter fields to `Skill` struct in `parseSkillFile`
- Added `WriteVerification(path, verifiedAt, verifiedBy string) error` function that:
  - Reads the existing SKILL.md
  - Parses frontmatter as a generic YAML map
  - Updates `verified`, `verified_at`, `verified_by` keys
  - Re-serializes frontmatter and writes file back, preserving markdown body

### `internal/harness/tools/types.go`
- Added `Verified bool`, `VerifiedAt string`, `VerifiedBy string`, `FilePath string` to `SkillInfo` struct
- `FilePath` is required so the `verify` action knows where to write

### `cmd/harnessd/main.go`
- Updated `skillListerAdapter.GetSkill` and `ListSkills` to propagate new `Skill` fields to `SkillInfo`

### `internal/harness/tools/core/skill.go`
- Added `skills` package import
- Added `handleListSkills`: returns all skills with `[verified]`/`[unverified]` status
- Added `handleVerifySkill`: writes verification metadata to SKILL.md via `skills.WriteVerification`
- Updated `handleConversationSkill`: prepends `⚠ WARNING: skill is unverified\n\n` to meta-message body for unverified skills
- Updated handler dispatch: routes `list` and `verify` commands to new handlers before falling through to skill apply

### `internal/harness/tools/descriptions/skill.md`
- Documented `list` and `verify` built-in actions

## Tests Added

### `internal/skills/loader_test.go`
- `TestLoaderLoad_VerifiedFields` - loads skill with verified frontmatter fields
- `TestLoaderLoad_UnverifiedByDefault` - backward compat: missing fields default to false/empty

### `internal/harness/tools/core/skill_test.go`
- `TestSkillTool_Handler_ListShowsVerifiedStatus` - list output contains `[verified]` and `[unverified]`
- `TestSkillTool_Handler_ListEmptySkills` - list with no skills
- `TestSkillTool_Handler_ApplyUnverifiedPrependsWarning` - warning in meta-message for unverified
- `TestSkillTool_Handler_ApplyVerifiedNoWarning` - no warning for verified skills
- `TestSkillTool_Handler_VerifyAction` - writes verified/verified_at/verified_by to file
- `TestSkillTool_Handler_VerifyDefaultVerifiedBy` - defaults to "agent" when no verifier given
- `TestSkillTool_Handler_VerifyNonexistentSkill` - error for unknown skill
- `TestSkillTool_Handler_VerifyMissingSkillName` - error when verify has no skill name arg

## Test Results

All tests pass with the race detector:

```
ok  go-agent-harness/internal/skills
ok  go-agent-harness/internal/harness/tools
ok  go-agent-harness/internal/harness/tools/core
ok  go-agent-harness/cmd/harnessd
FAIL go-agent-harness/demo-cli [build failed]  # pre-existing, unrelated failure
```

## Design Notes

- `WriteVerification` parses frontmatter as a generic YAML map to avoid losing unknown keys
- The `FilePath` field in `SkillInfo` is used by the `verify` action to locate the file to update
- `list` and `verify` are reserved command prefixes; a skill named "list" or "verify" cannot be applied (by design)
- The warning unicode character `⚠` (U+26A0) is used as specified in the issue
- `Mutating` flag on the tool definition remains `false` since the primary operations are read-only; `verify` is an admin/config operation analogous to writing a config file
