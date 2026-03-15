# mgrep Research (mixedbread-ai/mgrep)

## What Is It?

mgrep is a semantic code search CLI tool — a drop-in replacement for `grep` that finds code by **meaning** rather than text pattern. Instead of regex matching, it embeds your query and your codebase into vector space, then retrieves the most semantically relevant results.

Built by Mixedbread AI (makers of the mxbai embedding models).

## Architecture

- **Language**: TypeScript (Node.js)
- **Embeddings**: Mixedbread cloud API (`mxbai-embed-large` or `mxbai-embed-small`) — **requires API key, has cost**
- **Reranking**: Optional Mixedbread reranker for precision boost
- **Index format**: Local vector store (file-backed, stored in project or `~/.mgrep/`)
- **Watch mode**: `mgrep watch` for live incremental indexing as files change
- **CLI**: `mgrep search <query>`, `mgrep index`, `mgrep watch`

## How It Differs from grep/ripgrep

| Feature | grep/rg | mgrep |
|---------|---------|-------|
| Match type | Exact/regex | Semantic (meaning) |
| Query | Pattern | Natural language |
| Speed | Very fast | Slower (network round-trip for embeddings) |
| Privacy | 100% local | Code sent to Mixedbread API |
| Index required | No | Yes |
| Finds synonyms/concepts | No | Yes |

## Benchmark Claims

- **2x fewer LLM tokens** in agentic workflows vs. grep-based search (their benchmark)
- Agents find the right code faster → shorter context windows → lower cost per run

## Licensing & Distribution

- Apache 2.0 license
- Published to npm: `@mixedbread-ai/mgrep`
- Requires `MXBAI_API_KEY` environment variable

## Integration Effort for Go Harness

**Approach**: Register as a deferred/optional tool that spawns `mgrep` subprocess.

**Steps**:
1. Tool handler: `internal/harness/tools/mgrep.go`
2. Description: `internal/harness/tools/descriptions/mgrep.md`
3. Subprocess invocation: `mgrep search <query> --format json`
4. Index management: either pre-indexed by user or trigger `mgrep index` on workspace init
5. Config: `MXBAI_API_KEY` env, `MGREP_INDEX_PATH` per workspace

**Effort**: ~2-3 days (subprocess wrapper + lifecycle management + tests)

## Key Concerns

1. **Cloud dependency**: Code is sent to Mixedbread API — privacy issue for sensitive codebases
2. **API cost**: Every query costs money (Mixedbread pricing)
3. **Index staleness**: Requires either watch mode or periodic re-index
4. **Requires Node.js**: External binary dependency
5. **npm install**: User must install `@mixedbread-ai/mgrep` globally

## Comparison vs. Local Alternative (lgrep/lss)

| | mgrep | lgrep/lss |
|--|-------|-----------|
| Embeddings | Cloud (Mixedbread) | Local (Ollama) |
| Cost | Per-query API cost | Free (local GPU/CPU) |
| Privacy | Code leaves machine | 100% local |
| Setup | npm install + API key | npm install + Ollama running |
| Speed | Fast (managed API) | Depends on hardware |
| Quality | High (production models) | Good (mxbai-embed-large local) |
