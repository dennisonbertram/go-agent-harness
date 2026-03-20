# Issue #362 Implementation Plan: Remove OpenAI-Only Bootstrap

## Summary
Remove hard `OPENAI_API_KEY` requirement from server startup. Use provider registry to find first configured provider. Keep OpenAI as simplest happy path but not a hard requirement.

## Files to Change

### `cmd/harnessd/main.go`
1. **Remove lines 163-166**: Delete `apiKey := getenv("OPENAI_API_KEY")` and the `if apiKey == ""` error
2. **Defer default provider creation (lines 323-332)**: Instead of always creating OpenAI provider, use registry to find first configured provider
3. **Fix memory LLM API key (line 217)**: `memoryLLMAPIKey` should not fall back to a global `apiKey` that no longer exists at that scope
4. **Fix conclusion watcher bootstrap (line ~589)**: Remove silent fallback to `OPENAI_API_KEY`
5. **Add helper**: `getFirstConfiguredProvider(registry)` to iterate catalog providers

### `cmd/harnessd/main_test.go`
- Update `TestRunWithSignalsMissingAPIKey` → expect error about "no provider" not "OPENAI_API_KEY"
- Add `TestRunWithSignalsStartsWithoutOpenAI` — Anthropic configured, no OpenAI key
- Add `TestRunWithSignalsFailsCleanlyWithoutAnyProvider` — no providers at all
- Add `TestObservationalMemoryWithoutOpenAI` — memory mode="inherit" with Anthropic

## Key Constraint
Do NOT break the existing OpenAI path. If OPENAI_API_KEY is set, behavior is unchanged.

## Commit Strategy
Single commit: `fix(#362): remove OpenAI-only bootstrap requirement from server startup`

## Verification
- `go test ./cmd/harnessd/...`
- `go test ./cmd/harnessd/... -race`
- `./scripts/test-regression.sh`
