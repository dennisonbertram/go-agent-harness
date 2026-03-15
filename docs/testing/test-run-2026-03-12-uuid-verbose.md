# Test Run: 2026-03-12

**Date**: 2026-03-12
**Commands**: `go test ./... -race` and `./scripts/test-regression.sh`
**Working Directory**: `/Users/dennisonbertram/Develop/go-agent-harness`

---

## 1. `go test ./... -race` Output

```
ok  	go-agent-harness/cmd/coveragegate	(cached)
ok  	go-agent-harness/cmd/cronctl	(cached)
ok  	go-agent-harness/cmd/cronsd	(cached)
# go-agent-harness/cmd/symphd.test
ld: warning: '/private/var/folders/_b/d094gwqd7d38jhjn_pvv_h380000gn/T/go-link-274523614/000013.o' has malformed LC_DYSYMTAB, expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
ok  	go-agent-harness/cmd/harnesscli	1.431s
# go-agent-harness/internal/symphd.test
ld: warning: '/private/var/folders/_b/d094gwqd7d38jhjn_pvv_h380000gn/T/go-link-2881587071/000013.o' has malformed LC_DYSYMTAB, expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
# go-agent-harness/internal/workspace.test
ld: warning: '/private/var/folders/_b/d094gwqd7d38jhjn_pvv_h380000gn/T/go-link-1760517049/000013.o' has malformed LC_DYSYMTAB, expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
ok  	go-agent-harness/cmd/harnessd	1.963s
ok  	go-agent-harness/cmd/symphd	(cached)
ok  	go-agent-harness/demo-cli	1.305s
ok  	go-agent-harness/internal/config	1.352s
ok  	go-agent-harness/internal/cron	(cached)
ok  	go-agent-harness/internal/deploy	(cached)
ok  	go-agent-harness/internal/harness	2.399s
ok  	go-agent-harness/internal/harness/tools	(cached)
ok  	go-agent-harness/internal/harness/tools/core	1.464s
ok  	go-agent-harness/internal/harness/tools/deferred	(cached)
ok  	go-agent-harness/internal/harness/tools/descriptions	(cached)
ok  	go-agent-harness/internal/harness/tools/recipe	(cached)
ok  	go-agent-harness/internal/harness/tools/script	(cached)
ok  	go-agent-harness/internal/observationalmemory	(cached)
ok  	go-agent-harness/internal/provider/anthropic	1.619s
ok  	go-agent-harness/internal/provider/catalog	(cached)
ok  	go-agent-harness/internal/provider/openai	1.487s
ok  	go-agent-harness/internal/provider/pricing	(cached)
ok  	go-agent-harness/internal/quality/coveragegate	(cached)
ok  	go-agent-harness/internal/rollout	1.674s
ok  	go-agent-harness/internal/server	2.358s
ok  	go-agent-harness/internal/skills	1.665s
ok  	go-agent-harness/internal/skills/packs	(cached)
ok  	go-agent-harness/internal/symphd	(cached)
ok  	go-agent-harness/internal/systemprompt	(cached)
ok  	go-agent-harness/internal/watcher	(cached)
ok  	go-agent-harness/internal/workspace	(cached)
ok  	go-agent-harness/skills	2.094s
```

### Summary: `go test ./... -race`

- **Total packages**: 32
- **Passed**: 32
- **Failed**: 0
- **Race conditions detected**: None
- **Linker warnings (non-fatal)**: 3 packages (`cmd/symphd`, `internal/symphd`, `internal/workspace`) emitted `ld: warning: malformed LC_DYSYMTAB` — this is a macOS linker warning, not a test failure

---

## 2. `./scripts/test-regression.sh` Output

### Phase 1: Standard test run (`go test ./internal/... ./cmd/...`)

```
[regression] go test ./internal/... ./cmd/...
ok  	go-agent-harness/internal/config	0.148s
ok  	go-agent-harness/internal/cron	2.089s
ok  	go-agent-harness/internal/deploy	0.334s
ok  	go-agent-harness/internal/harness	0.347s
ok  	go-agent-harness/internal/harness/tools	8.471s
ok  	go-agent-harness/internal/harness/tools/core	0.368s
ok  	go-agent-harness/internal/harness/tools/deferred	(cached)
ok  	go-agent-harness/internal/harness/tools/descriptions	(cached)
ok  	go-agent-harness/internal/harness/tools/recipe	(cached)
ok  	go-agent-harness/internal/harness/tools/script	3.463s
ok  	go-agent-harness/internal/observationalmemory	(cached)
ok  	go-agent-harness/internal/provider/anthropic	0.481s
ok  	go-agent-harness/internal/provider/catalog	(cached)
ok  	go-agent-harness/internal/provider/openai	0.776s
ok  	go-agent-harness/internal/provider/pricing	(cached)
ok  	go-agent-harness/internal/quality/coveragegate	(cached)
ok  	go-agent-harness/internal/rollout	0.918s
ok  	go-agent-harness/internal/server	1.726s
ok  	go-agent-harness/internal/skills	0.981s
ok  	go-agent-harness/internal/skills/packs	0.816s
ok  	go-agent-harness/internal/symphd	(cached)
ok  	go-agent-harness/internal/systemprompt	(cached)
ok  	go-agent-harness/internal/watcher	(cached)
ok  	go-agent-harness/internal/workspace	6.186s
ok  	go-agent-harness/cmd/coveragegate	0.981s
ok  	go-agent-harness/cmd/cronctl	(cached)
ok  	go-agent-harness/cmd/cronsd	(cached)
ok  	go-agent-harness/cmd/harnesscli	1.068s
ok  	go-agent-harness/cmd/harnessd	1.119s
ok  	go-agent-harness/cmd/symphd	(cached)
```

All 30 packages passed.

### Phase 2: Race detector (`go test ./internal/... ./cmd/... -race`)

```
[regression] go test ./internal/... ./cmd/... -race
ok  	go-agent-harness/internal/config	(cached)
ok  	go-agent-harness/internal/cron	(cached)
ok  	go-agent-harness/internal/deploy	(cached)
ok  	go-agent-harness/internal/harness	(cached)
ok  	go-agent-harness/internal/harness/tools	(cached)
ok  	go-agent-harness/internal/harness/tools/core	(cached)
ok  	go-agent-harness/internal/harness/tools/deferred	(cached)
ok  	go-agent-harness/internal/harness/tools/descriptions	(cached)
ok  	go-agent-harness/internal/harness/tools/recipe	(cached)
ok  	go-agent-harness/internal/harness/tools/script	(cached)
ok  	go-agent-harness/internal/observationalmemory	(cached)
ok  	go-agent-harness/internal/provider/anthropic	(cached)
ok  	go-agent-harness/internal/provider/catalog	(cached)
ok  	go-agent-harness/internal/provider/openai	(cached)
ok  	go-agent-harness/internal/provider/pricing	(cached)
ok  	go-agent-harness/internal/quality/coveragegate	(cached)
ok  	go-agent-harness/internal/rollout	(cached)
# go-agent-harness/internal/symphd.test
ld: warning: '/private/var/folders/_b/d094gwqd7d38jhjn_pvv_h380000gn/T/go-link-2881587071/000013.o' has malformed LC_DYSYMTAB, expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
# go-agent-harness/cmd/symphd.test
ld: warning: '/private/var/folders/_b/d094gwqd7d38jhjn_pvv_h380000gn/T/go-link-274523614/000013.o' has malformed LC_DYSYMTAB, expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
ok  	go-agent-harness/internal/server	(cached)
ok  	go-agent-harness/internal/skills	(cached)
ok  	go-agent-harness/internal/skills/packs	(cached)
# go-agent-harness/internal/workspace.test
ld: warning: '/private/var/folders/_b/d094gwqd7d38jhjn_pvv_h380000gn/T/go-link-1760517049/000013.o' has malformed LC_DYSYMTAB, expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
ok  	go-agent-harness/internal/symphd	(cached)
ok  	go-agent-harness/internal/systemprompt	(cached)
ok  	go-agent-harness/internal/watcher	(cached)
ok  	go-agent-harness/internal/workspace	(cached)
ok  	go-agent-harness/cmd/coveragegate	(cached)
ok  	go-agent-harness/cmd/cronctl	(cached)
ok  	go-agent-harness/cmd/cronsd	(cached)
ok  	go-agent-harness/cmd/harnesscli	(cached)
ok  	go-agent-harness/cmd/harnessd	1.696s
ok  	go-agent-harness/cmd/symphd	(cached)
```

All 30 packages passed with race detector. No race conditions detected.

### Phase 3: Coverage profiling (`go test ./internal/... ./cmd/... -coverprofile=coverage.out`)

```
[regression] go test ./internal/... ./cmd/... -coverprofile=coverage.out
ok  	go-agent-harness/internal/config	0.145s	coverage: 89.9% of statements
ok  	go-agent-harness/internal/cron	(cached)	coverage: 87.2% of statements
ok  	go-agent-harness/internal/deploy	(cached)	coverage: 95.8% of statements
ok  	go-agent-harness/internal/harness	0.341s	coverage: 84.2% of statements
ok  	go-agent-harness/internal/harness/tools	(cached)	coverage: 85.7% of statements
ok  	go-agent-harness/internal/harness/tools/core	0.377s	coverage: 70.0% of statements
ok  	go-agent-harness/internal/harness/tools/deferred	(cached)	coverage: 76.0% of statements
ok  	go-agent-harness/internal/harness/tools/descriptions	(cached)	coverage: 100.0% of statements
ok  	go-agent-harness/internal/harness/tools/recipe	(cached)	coverage: 92.4% of statements
ok  	go-agent-harness/internal/harness/tools/script	(cached)	coverage: 84.5% of statements
ok  	go-agent-harness/internal/observationalmemory	(cached)	coverage: 86.6% of statements
ok  	go-agent-harness/internal/provider/anthropic	0.488s	coverage: 89.2% of statements
ok  	go-agent-harness/internal/provider/catalog	(cached)	coverage: 95.2% of statements
ok  	go-agent-harness/internal/provider/openai	0.629s	coverage: 79.8% of statements
ok  	go-agent-harness/internal/provider/pricing	(cached)	coverage: 85.5% of statements
ok  	go-agent-harness/internal/quality/coveragegate	(cached)	coverage: 81.6% of statements
ok  	go-agent-harness/internal/rollout	0.754s	coverage: 89.3% of statements
ok  	go-agent-harness/internal/server	1.702s	coverage: 83.4% of statements
ok  	go-agent-harness/internal/skills	0.844s	coverage: 95.5% of statements
ok  	go-agent-harness/internal/skills/packs	(cached)	coverage: 94.2% of statements
ok  	go-agent-harness/internal/symphd	(cached)	coverage: 87.9% of statements
ok  	go-agent-harness/internal/systemprompt	(cached)	coverage: 86.6% of statements
ok  	go-agent-harness/internal/watcher	(cached)	coverage: 87.3% of statements
ok  	go-agent-harness/internal/workspace	(cached)	coverage: 75.3% of statements
ok  	go-agent-harness/cmd/coveragegate	(cached)	coverage: 81.2% of statements
ok  	go-agent-harness/cmd/cronctl	(cached)	coverage: 82.8% of statements
ok  	go-agent-harness/cmd/cronsd	(cached)	coverage: 86.7% of statements
ok  	go-agent-harness/cmd/harnesscli	0.898s	coverage: 90.8% of statements
ok  	go-agent-harness/cmd/harnessd	1.120s	coverage: 80.1% of statements
ok  	go-agent-harness/cmd/symphd	(cached)	coverage: 34.1% of statements
```

### Phase 4: Coverage Gate

```
[regression] coverage gate: min total 80.0% + no zero-coverage functions
coveragegate: PASS (total=83.0%, min=80.0%, zero-functions=0)
[regression] PASS
```

---

## Package Coverage Summary

| Package | Coverage |
|---|---|
| `internal/harness/tools/descriptions` | 100.0% |
| `internal/deploy` | 95.8% |
| `internal/provider/catalog` | 95.2% |
| `internal/skills` | 95.5% |
| `internal/skills/packs` | 94.2% |
| `internal/harness/tools/recipe` | 92.4% |
| `cmd/harnesscli` | 90.8% |
| `internal/config` | 89.9% |
| `internal/provider/anthropic` | 89.2% |
| `internal/rollout` | 89.3% |
| `internal/cron` | 87.2% |
| `internal/watcher` | 87.3% |
| `cmd/cronsd` | 86.7% |
| `internal/observationalmemory` | 86.6% |
| `internal/systemprompt` | 86.6% |
| `internal/harness/tools` | 85.7% |
| `internal/provider/pricing` | 85.5% |
| `internal/harness` | 84.2% |
| `internal/server` | 83.4% |
| `cmd/cronctl` | 82.8% |
| `internal/quality/coveragegate` | 81.6% |
| `cmd/harnessd` | 80.1% |
| `cmd/coveragegate` | 81.2% |
| `internal/harness/tools/script` | 84.5% |
| `internal/provider/openai` | 79.8% |
| `internal/harness/tools/deferred` | 76.0% |
| `internal/workspace` | 75.3% |
| `internal/harness/tools/core` | 70.0% |
| `internal/symphd` | 87.9% |
| `cmd/symphd` | 34.1% |

**Notable**: `cmd/symphd` has 34.1% coverage but the gate passes on aggregate total (83.0%).

---

## Overall Result

| Check | Status |
|---|---|
| `go test ./... -race` (32 packages) | PASS |
| Regression standard tests (30 packages) | PASS |
| Regression race detector (30 packages) | PASS |
| Race conditions detected | None |
| Coverage gate (total=83.0%, min=80.0%) | PASS |
| Zero-coverage functions | 0 |
| **Final** | **PASS** |

### Notes

- Three packages emit a non-fatal macOS linker warning (`malformed LC_DYSYMTAB`) during race-enabled test binary linking: `cmd/symphd`, `internal/symphd`, `internal/workspace`. These are OS-level dynamic symbol table warnings and do not affect test correctness or outcomes.
- `cmd/symphd` has notably low coverage at 34.1% but the aggregate total (83.0%) satisfies the 80.0% gate minimum.
