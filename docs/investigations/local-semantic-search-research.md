# Local Semantic Search Research (dennisonbertram/local-semantic-search)

## Overview

**local-semantic-search** (`lss`) is a privacy-first TypeScript semantic search tool that indexes local files using Ollama embeddings and LanceDB. It's owned by the same user as go-agent-harness.

**Status**: DEPRECATED — the README points to [lgrep](https://github.com/dennisonbertram/lgrep) as the active successor with chunking, multi-provider support, and web UI.

## Architecture

- **Language**: TypeScript (Node.js), ES modules
- **Embeddings**: Ollama API (`mxbai-embed-large`, local, free, requires Ollama running)
- **Vector DB**: LanceDB (Apache Arrow, local file-backed at `~/.local/share/local-semantic-search/`)
- **MCP Server**: Exposed via `lss mcp-server` (stdio transport) — Claude Code compatible
- **CLI**: `lss index`, `lss search`, `lss status`, `lss doctor`

## Indexing

- Recursively scans directories, respects `.gitignore` and `.lssignore`
- SHA-256 hash change detection for incremental updates
- Batch embeds with concurrency control (`p-limit`, default 5 concurrent)
- Stores full file content + embedding + metadata in LanceDB
- ~100 files/second throughput (Ollama-dependent)

## MCP Interface (4 tools)

1. `local_search` — semantic query with limit + directory filter
2. `local_ask` — Q&A (searches top 5 results as context)
3. `index_status` — doc count, last-updated, tracked directories
4. `index_directory` — trigger indexing with optional force-reindex

## Performance (from SPEC.md)

- Search latency: <500ms for 10 results
- Memory: <200MB
- Disk: ~2KB per indexed file

## Integration with Go Harness

**Best approach**: Spawn `lss mcp-server` as subprocess, use MCP stdio protocol.

```
Go Runner → spawn `lss mcp-server` (Node.js process)
         → MCP stdio: call local_search / index_directory
         → subprocess calls LanceDB + Ollama
```

**Workspace isolation**: Set `LSS_DATA_DIR` env var per workspace (local, worktree, container).

**Deprecation note**: Since lss is deprecated, prefer integrating `lgrep` instead — it's the active project with chunking support and multiple embedding providers.

## Key Trade-offs

| Pro | Con |
|-----|-----|
| 100% local, no API cost | Requires Ollama running locally |
| MCP protocol ready | Deprecated in favor of lgrep |
| Privacy-first | No file chunking (whole-file embeddings) |
| LanceDB scales to millions of docs | Node.js subprocess dependency |
