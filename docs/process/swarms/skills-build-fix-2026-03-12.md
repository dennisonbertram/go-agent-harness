# Swarm: Fix skills/ Package Build Failure

**Started**: 2026-03-12
**Team**: skills-build-fix-swarm
**Scope**: `skills/skills_validation_*.go` test files
**Trigger**: `skills_validation_83_84_85_test.go` re-declared `skillsDir` and `loadAllSkillsNew` already declared in sibling test files

---

## Wave 1 — Remove duplicate declarations
**Teammates**: implementer
**Files changed**: `skills/skills_validation_83_84_85_test.go`
**What**: Deleted duplicate `skillsDir` and `loadAllSkillsNew` function declarations + removed unused imports
**Result**: `go build ./skills/...` and `go test ./skills/...` pass

## Wave 2 — Consolidate to canonical helpers
**Teammates**: implementer-2
**Files changed**: `skills/skills_validation_83_84_85_test.go`
**What**: Replaced 37 calls of `loadAllSkillsNew(t)` → `loadAllSkills(t)` (canonical pair in skills_validation_test.go)
**Reason**: GPT-5.2 review flagged hidden cross-file dependency on `skills_validation_56_57_73_76_test.go`

## Wave 3 — Add duplicate name detection
**Teammates**: implementer-3
**Files changed**: all 4 `loadAllSkills*` helpers
**What**: Added `t.Fatalf("duplicate skill name %q returned by loader", s.Name)` before map assignment
**Reason**: GPT-5.2 found silent overwrite on duplicate skill names (real integrity hole)

## Wave 4 — Full helper consolidation
**Teammates**: implementer-4
**Files changed**: `skills_validation_56_57_73_76_test.go`, `skills_validation_74_86_test.go`, `skills_validation_78_80_81_82_test.go`
**What**: Removed all per-file helper variants; migrated all call sites to canonical `loadAllSkills(t)` + `skillsDir(t)`
- `56_57_73_76`: removed `skillsDirNew`, `loadAllSkillsNew`; 37 call replacements + 1 dir helper
- `74_86`: removed `skillsDir7486`, `loadAllSkills7486`; 16 call replacements
- `78_80_81_82`: removed `skillsDir7882`, `loadAllSkills7882`; 42 call replacements
- `65_66_68_69`: already canonical, skipped

## Wave 5 — Fix string literal constant
**Who**: Team lead (direct edit)
**Files changed**: `skills/skills_validation_83_84_85_test.go`
**What**: Added `go-agent-harness/internal/skills` import; replaced `s.Context == "fork"` with `s.Context == skills.ContextFork`

---

## Ralph Loop

**Pass 1 (Adversarial)**: CRITICAL: 0 / HIGH: 0 / MEDIUM: 0 / APPROVED: YES
**Pass 2 (Skeptical User)**: CRITICAL: 0 / HIGH: 0 / MEDIUM: 2 / APPROVED: YES
**Pass 3 (Correctness)**: CRITICAL: 0 / HIGH: 0 / MEDIUM: 3 / APPROVED: YES
**Result**: COMPLETE — 3 consecutive clean passes

---

## Final Status
- [x] 3 consecutive clean passes
- [x] All tests passing (`go test ./skills/... -race`)
- [x] Build clean
- [x] Committed: `e77ffa3` — fix(skills): resolve build failure from duplicate test helper declarations
