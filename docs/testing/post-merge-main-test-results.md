# Post-Merge Main Test Results

**Date**: 2026-03-20
**Branch**: main
**Result**: PASS

## Command 1: `go test ./internal/... ./cmd/...`

All packages passed. Output:

```
ok  	go-agent-harness/internal/config	(cached)
ok  	go-agent-harness/internal/cron	(cached)
ok  	go-agent-harness/internal/deploy	(cached)
ok  	go-agent-harness/internal/forensics/audittrail	(cached)
ok  	go-agent-harness/internal/forensics/causalgraph	(cached)
ok  	go-agent-harness/internal/forensics/contextwindow	(cached)
ok  	go-agent-harness/internal/forensics/costanomaly	(cached)
ok  	go-agent-harness/internal/forensics/differ	(cached)
ok  	go-agent-harness/internal/forensics/errorchain	(cached)
ok  	go-agent-harness/internal/forensics/redaction	(cached)
ok  	go-agent-harness/internal/forensics/replay	(cached)
ok  	go-agent-harness/internal/forensics/requestenvelope	(cached)
ok  	go-agent-harness/internal/forensics/rollout	(cached)
ok  	go-agent-harness/internal/forensics/tooldecision	(cached)
ok  	go-agent-harness/internal/harness	(cached)
ok  	go-agent-harness/internal/harness/tools	(cached)
ok  	go-agent-harness/internal/harness/tools/core	(cached)
ok  	go-agent-harness/internal/harness/tools/deferred	(cached)
ok  	go-agent-harness/internal/harness/tools/descriptions	(cached)
ok  	go-agent-harness/internal/harness/tools/recipe	(cached)
ok  	go-agent-harness/internal/harness/tools/script	(cached)
ok  	go-agent-harness/internal/harnessmcp	(cached)
ok  	go-agent-harness/internal/mcp	(cached)
ok  	go-agent-harness/internal/mcpserver	(cached)
ok  	go-agent-harness/internal/observationalmemory	(cached)
ok  	go-agent-harness/internal/profiles	(cached)
ok  	go-agent-harness/internal/provider/anthropic	(cached)
ok  	go-agent-harness/internal/provider/catalog	(cached)
ok  	go-agent-harness/internal/provider/openai	(cached)
ok  	go-agent-harness/internal/provider/pricing	(cached)
ok  	go-agent-harness/internal/quality/coveragegate	(cached)
ok  	go-agent-harness/internal/rollout	(cached)
ok  	go-agent-harness/internal/server	(cached)
ok  	go-agent-harness/internal/skills	(cached)
ok  	go-agent-harness/internal/skills/packs	(cached)
ok  	go-agent-harness/internal/store	(cached)
ok  	go-agent-harness/internal/store/s3backup	(cached)
ok  	go-agent-harness/internal/subagents	(cached)
ok  	go-agent-harness/internal/symphd	(cached)
ok  	go-agent-harness/internal/systemprompt	(cached)
ok  	go-agent-harness/internal/training	(cached)
ok  	go-agent-harness/internal/watcher	(cached)
ok  	go-agent-harness/internal/workspace	(cached)
ok  	go-agent-harness/cmd/coveragegate	(cached)
ok  	go-agent-harness/cmd/cronctl	(cached)
ok  	go-agent-harness/cmd/cronsd	(cached)
ok  	go-agent-harness/cmd/forensics	(cached)
ok  	go-agent-harness/cmd/harness-mcp	(cached)
ok  	go-agent-harness/cmd/harnesscli	(cached)
ok  	go-agent-harness/cmd/harnesscli/config	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui	4.082s
ok  	go-agent-harness/cmd/harnesscli/tui/components/configpanel	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/contextgrid	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/costdisplay	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/diffview	0.932s
ok  	go-agent-harness/cmd/harnesscli/tui/components/helpdialog	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/inputarea	0.322s
ok  	go-agent-harness/cmd/harnesscli/tui/components/interruptui	0.435s
ok  	go-agent-harness/cmd/harnesscli/tui/components/layout	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/messagebubble	0.849s
ok  	go-agent-harness/cmd/harnesscli/tui/components/modelswitcher	1.049s
ok  	go-agent-harness/cmd/harnesscli/tui/components/outputmode	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/permissionprompt	0.548s
ok  	go-agent-harness/cmd/harnesscli/tui/components/permissionspanel	0.676s
ok  	go-agent-harness/cmd/harnesscli/tui/components/planoverlay	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/sessionpicker	1.153s
ok  	go-agent-harness/cmd/harnesscli/tui/components/slashcomplete	1.380s
ok  	go-agent-harness/cmd/harnesscli/tui/components/spinner	1.318s
ok  	go-agent-harness/cmd/harnesscli/tui/components/statspanel	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/statusbar	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/streamrenderer	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/thinkingbar	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/tooluse	1.495s
ok  	go-agent-harness/cmd/harnesscli/tui/components/transcriptexport	1.544s
ok  	go-agent-harness/cmd/harnesscli/tui/components/viewport	1.646s
ok  	go-agent-harness/cmd/harnesscli/tui/testhelpers	2.946s
ok  	go-agent-harness/cmd/harnessd	2.180s
ok  	go-agent-harness/cmd/symphd	(cached)
ok  	go-agent-harness/cmd/trainerd	(cached)
```

**Total packages**: 79
**Failed**: 0

## Command 2: `go test -race ./internal/... ./cmd/... | tail -30`

All packages passed under the race detector. Tail output:

```
ld: warning: '/private/var/folders/_b/.../000013.o' has malformed LC_DYSYMTAB, expected 98 undefined symbols to start at index 1626, found 95 undefined symbols starting at index 1626
ok  	go-agent-harness/cmd/harnesscli/tui	5.621s
ok  	go-agent-harness/cmd/harnesscli/tui/components/configpanel	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/contextgrid	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/costdisplay	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/diffview	1.161s
ok  	go-agent-harness/cmd/harnesscli/tui/components/helpdialog	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/inputarea	1.901s
ok  	go-agent-harness/cmd/harnesscli/tui/components/interruptui	2.142s
ok  	go-agent-harness/cmd/harnesscli/tui/components/layout	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/messagebubble	1.919s
ok  	go-agent-harness/cmd/harnesscli/tui/components/modelswitcher	1.437s
ok  	go-agent-harness/cmd/harnesscli/tui/components/outputmode	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/permissionprompt	2.267s
ok  	go-agent-harness/cmd/harnesscli/tui/components/permissionspanel	1.290s
ok  	go-agent-harness/cmd/harnesscli/tui/components/planoverlay	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/sessionpicker	2.023s
ok  	go-agent-harness/cmd/harnesscli/tui/components/slashcomplete	2.275s
ok  	go-agent-harness/cmd/harnesscli/tui/components/spinner	2.447s
ok  	go-agent-harness/cmd/harnesscli/tui/components/statspanel	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/statusbar	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/streamrenderer	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/thinkingbar	(cached)
ok  	go-agent-harness/cmd/harnesscli/tui/components/tooluse	2.554s
ok  	go-agent-harness/cmd/harnesscli/tui/components/transcriptexport	2.565s
ok  	go-agent-harness/cmd/harnesscli/tui/components/viewport	2.650s
ok  	go-agent-harness/cmd/harnesscli/tui/testhelpers	3.944s
ok  	go-agent-harness/cmd/harnessd	3.397s
ok  	go-agent-harness/cmd/symphd	(cached)
ok  	go-agent-harness/cmd/trainerd	(cached)
```

**Race conditions detected**: 0
**Note**: The `ld: warning: malformed LC_DYSYMTAB` message is a macOS linker warning unrelated to test correctness; it does not affect test outcomes.

## Summary

| Run | Packages | PASS | FAIL |
|-----|----------|------|------|
| Standard | 79 | 79 | 0 |
| Race detector | 79 | 79 | 0 |

**Overall result: PASS**
