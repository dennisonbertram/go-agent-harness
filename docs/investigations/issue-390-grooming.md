# Grooming: Issue #390 — chore(tools): align the default registry, docs, and code-intel policy

## Already Addressed?

Partial — The codebase has significant drift between several sources of truth:

1. `internal/harness/tools/catalog.go` (the legacy `BuildCatalog` function) still lists LSP tools (`lsp_diagnostics`, `lsp_references`, `lsp_restart`) as conditionally registered under `EnableLSP`. However, `tools_default.go` has removed LSP tools entirely — the comment at line 207 reads: `// LSP tools removed — bash gopls/go-build are sufficient.`

2. `docs/design/tool-roadmap.md` still lists `lsp_diagnostics`, `lsp_references`, and `lsp_restart` as `implemented` (lines 24-26), with no indication they have been removed from the default registry.

3. `tools_contract_test.go` does not include any LSP tools in its `expected` slice (lines 26-48), which is consistent with the current `tools_default.go` state, but inconsistent with the roadmap doc.

4. `docs/investigations/tool-catalog-review.md` (dated 2026-03-09) correctly identifies LSP tools as candidates for removal (Section "CUT all three") and the `catalog.go` `BuildCatalog` function is the old path — `tools_default.go` is now the authoritative registry builder.

The misalignment: `catalog.go::BuildCatalog` is stale (references `lsp_diagnostics` etc.), `tool-roadmap.md` is stale (shows LSP as implemented), `tools_default.go` is current and correct. The contract test enforces the current state but doesn't document the policy decision. No explicit statement anywhere says "LSP tools are removed permanently."

## Clarity

Clear — The issue title and body make the goal explicit: reconcile the files, make the code-intel policy explicit. Related files are correctly identified.

## Acceptance Criteria

Missing — The issue body does not state explicit pass/fail criteria. Implementable acceptance criteria would be:
- `docs/design/tool-roadmap.md` updated to show LSP tools as `removed` (or `deferred`) with rationale
- `internal/harness/tools/catalog.go::BuildCatalog` either updated to match `tools_default.go` or marked deprecated with a comment
- A comment or doc note explicitly stating the chosen code-intel policy (removed/optional/planned)
- `tools_contract_test.go` documents why the expected set is what it is (e.g. a comment anchoring the policy)
- No functional code changes required — this is a docs/comments/dead-code cleanup

## Scope

Atomic — This is a pure reconciliation/cleanup ticket. No new functionality. All changes are confined to docs and non-functional code (stale `BuildCatalog`, comments). The only risk is accidentally changing the contract test expected list.

## Blockers

None — This ticket has no upstream dependencies. It can be executed independently.

## Recommended Labels

well-specified, small

## Effort

Small — Estimated 1-2 hours. Reading three files, writing a comment/doc update, deciding LSP policy (the main intellectual work), updating the roadmap doc.

## Recommendation

well-specified — Clear goal, correct related files, no blockers. Missing formal acceptance criteria but the implementation is obvious from the context. Could be executed immediately.

## Notes

Key findings:
- `tools_default.go` line 207: `// LSP tools removed — bash gopls/go-build are sufficient.` confirms the removal decision was made, but it is only in a comment.
- `catalog.go::BuildCatalog` at line 55 still has `if opts.EnableLSP { tools = append(tools, lspDiagnosticsTool...) }` — this function appears to be a legacy parallel path that is no longer used by `NewDefaultRegistryWithOptions`. Its relationship to the active code path should be clarified (is `BuildCatalog` dead code?).
- The tool-roadmap.md still shows LSP as `implemented` — this is the most visible public-facing stale state.
- The `tool-catalog-review.md` investigation from 2026-03-09 recommended cutting all three LSP tools and this recommendation appears to have been acted on in `tools_default.go`. The doc just needs to catch up.
- No profile-related content needs updating for this ticket — it is purely about the tool catalog/registry alignment.
