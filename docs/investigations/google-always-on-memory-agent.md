# Investigation: Google Always-On Memory Agent vs Local Observational Memory

**Date**: 2026-03-10
**Source**: https://github.com/GoogleCloudPlatform/generative-ai/tree/main/gemini/agents/always-on-memory-agent
**Local system**: `/Users/dennisonbertram/Develop/go-agent-harness/internal/observationalmemory/`

---

## 1. Google Always-On Memory Agent — Architecture Overview

The Google system (authored by Shubhamsaboo, hosted in the GoogleCloudPlatform generative-ai monorepo) is a standalone Python application built on Google ADK (Agent Development Kit) with Gemini 3.1 Flash-Lite.

### High-Level Design

```
                     ┌─────────────────────────────┐
                     │      memory_orchestrator      │
                     │  (routes requests to agents)  │
                     └────────────┬────────────────┘
            ┌───────────┬─────────┴──────────┐
            │           │                     │
      ingest_agent  consolidate_agent    query_agent
            │           │                     │
      store_memory  read_uncons. +       read_all_memories +
                    store_consol.        read_consol_history
                            │
                        SQLite DB
                    (memories + consolidations
                     + processed_files)
```

### Storage Schema

Two primary tables:

**`memories`** — individual memory entries:
- `id`, `source`, `raw_text`, `summary`, `entities` (JSON list of strings), `topics` (JSON list of strings), `connections` (JSON list of linked memory pairs), `importance` (float 0.0–1.0), `created_at`, `consolidated` (boolean)

**`consolidations`** — higher-level cross-memory insights:
- `id`, `source_ids` (JSON list of memory IDs), `summary`, `insight`, `created_at`

**`processed_files`** — deduplication log for the file watcher:
- `path`, `processed_at`

---

## 2. How Google's Memory System Works

### Ingestion (Trigger: external push)

Ingestion is **externally driven** — a human or process must explicitly feed information in via one of three channels:

1. **File watcher**: Drop a file into `./inbox/`. A background coroutine polls every 5 seconds and picks up any new file. Supports 27 file types including text, images, audio, video, and PDFs. Media files are sent to Gemini multimodally.
2. **HTTP API**: `POST /ingest` with `{"text": "...", "source": "..."}`.
3. **Streamlit dashboard**: Upload button for files or typed text.

When ingestion fires, `ingest_agent` is invoked with a natural-language prompt. The agent:
1. Analyzes the raw content (using Gemini's multimodal capabilities for media)
2. Produces: `summary`, `entities[]`, `topics[]`, `importance` score
3. Calls the `store_memory` tool to write the record to SQLite

**Memory creation trigger**: explicit external push only. The agent does not watch its own conversations.

### Consolidation (Trigger: timer)

`consolidation_loop` fires every 30 minutes (configurable). When it runs:
1. It checks if there are >= 2 unconsolidated memories.
2. `consolidate_agent` reads all unconsolidated memories via `read_unconsolidated_memories` (up to 10 at a time).
3. The LLM finds patterns and cross-references.
4. It calls `store_consolidation` with: `source_ids[]`, `summary`, `insight`, `connections[]` (from/to/relationship triples).
5. Source memories are marked `consolidated = 1`.
6. The connection graph is updated bidirectionally on each memory record.

**Consolidation trigger**: time-based timer, not token-count based. Also can be triggered manually via `POST /consolidate`.

### Memory Structure

Each memory is richly structured:
- **Entities**: extracted named entities (people, companies, concepts)
- **Topics**: 2–4 categorical tags
- **Importance**: float score
- **Connections**: explicit bidirectional links to other memories with relationship labels
- **Raw text + summary**: both preserved

Consolidation records add a second layer: synthesized multi-memory insights.

### Retrieval / Injection

When answering a query, `query_agent`:
1. Calls `read_all_memories` — returns all memories ordered by `created_at DESC`, limit 50
2. Calls `read_consolidation_history` — returns last 10 consolidation records
3. The LLM synthesizes an answer with source citations (`[Memory N]`)

**No semantic search, no embeddings.** The full memory dump is passed directly into the LLM context. The agent description says explicitly: "No vector database. No embeddings. Just an LLM that reads, thinks, and writes structured memory."

---

## 3. Local Observational Memory — Architecture Overview

The local system (`internal/observationalmemory/`) is a Go library embedded in the agent harness. It operates as a passive observer of ongoing agent conversations, not an active ingestor of external documents.

### High-Level Design

```
Runner (agent loop)
    → calls Observe(messages[]) after each step
        → if unobserved tokens >= threshold
            → ModelObserver.Observe() → LLM produces text chunk
            → chunk appended to ActiveObservations[]
            → if ActiveObservationTokens >= reflect threshold
                → ModelReflector.Reflect() → LLM compresses all chunks
                → stored as ActiveReflection (single string)

Snippet() renders memory for injection:
    "<observational-memory>
      Reflection: ...
      Observations:
      - [1] chunk text
      - [2] chunk text
    </observational-memory>"
```

### Storage Schema

Three tables in SQLite (or Postgres with a separate adapter):

**`om_memory_records`** — one row per (tenant_id, conversation_id, agent_id) triple:
- `memory_id`, scope fields, `enabled`, `state_version`
- `last_observed_message_index` — cursor tracking which messages have been processed
- `active_observations_json` — JSON array of `ObservationChunk` objects
- `active_observation_tokens` — token count for budget tracking
- `active_reflection` — compressed reflection text (replaces chunks when threshold hit)
- `active_reflection_tokens`
- `last_reflected_observation_seq`
- `config_json`

**`om_operation_log`** — audit trail of every mutation:
- Records each `observe`, `reflect_now`, `enable`/`disable` operation with `queued` → `processing` → `applied`/`failed` lifecycle
- Stale operations are reset on startup

**`om_markers`** — positional markers linking memory chunks to message indices:
- `observation_start`, `observation_end`, `reflection_end` markers
- Maps memory activity back to specific conversation positions

### Memory Trigger

Observation fires automatically via `Observe(ctx, req)`, called by the runner after each step with the current message history. The trigger is **token-count based**:
- Only fires if `unobserved tokens >= ObserveMinTokens` (default: 1200)
- Reflection fires if `ActiveObservationTokens >= ReflectThresholdTokens` (default: 4000)

No timer, no polling. Memory creation is fully driven by conversation progress.

### Memory Structure

Observations are **free-form text chunks** produced by a model call. The prompt instructs the LLM to:
> "Extract concrete, durable observations that help future coding turns. Do not include fluff. Prefer facts, constraints, decisions, and durable context."

Chunks are stored sequentially with: `seq`, `content`, `token_count`, `created_at`, `source_start_index`, `source_end_index`.

There is no entity extraction, no topic tagging, no importance scoring, no explicit connection graph.

Reflection compresses all existing chunks into a single summary text string (plus the existing reflection for incremental updates).

### Memory Injection

`Snippet()` constructs an XML-wrapped string for injection into the system prompt:
```
<observational-memory>
Reflection:
[compressed summary text]

Observations:
- [1] chunk text
- [N] chunk text
</observational-memory>
```

Budget-aware: observations are selected newest-first until `SnippetMaxTokens` (default: 900) is reached, then re-sorted by seq for injection.

---

## 4. Comparison: Key Differences

### What They Have in Common

- Both use SQLite as the backing store
- Both use an LLM to process/compress memory (no vector embeddings, no RAG)
- Both expose plain-text summaries for injection into context
- Both have a reflection/consolidation step that compresses multiple observations

### Google's Approach vs Local Approach

| Dimension | Google Always-On Agent | Local Observational Memory |
|---|---|---|
| **Scope** | Standalone application | Embedded library in agent runner |
| **Memory source** | External files, HTTP API, Streamlit UI | Agent's own live conversation transcript |
| **Trigger** | Manual push + 30-min timer | Token-count threshold (automatic) |
| **Memory structure** | Structured: entities[], topics[], importance, connections graph | Unstructured: free-form text chunks |
| **Cross-memory connections** | Explicit bidirectional graph links with relationship labels | None — memories are isolated sequences |
| **Consolidation** | Timer-based, produces cross-reference insights | Token-threshold-based, compresses chunks into reflection |
| **Multimodal input** | Yes (images, audio, video, PDFs via Gemini) | No (text transcript only) |
| **Retrieval** | Dump all memories into LLM context (no search) | Budget-capped snippet injection (newest-first) |
| **Scoping** | Single global scope (no multi-tenant, no per-conversation isolation) | Full multi-tenant: tenant_id + conversation_id + agent_id |
| **Operation audit log** | None | Full operation log with lifecycle tracking |
| **Conversation tracking** | None (no cursor into conversation messages) | `last_observed_message_index` cursor tracks position |
| **Concurrency safety** | None (raw sqlite3 connections, no locking) | `LocalCoordinator` with per-scope mutex, WAL mode, busy timeout |
| **Delete/clear** | Yes — `DELETE /delete` and `DELETE /clear` | Not exposed (records stay; enabled can be toggled) |
| **Query interface** | HTTP API (`GET /query?q=...`) | `Snippet()` returns text for system prompt injection |
| **Tech stack** | Python, Google ADK, aiohttp, Streamlit | Go, net/http |

---

## 5. What Google's System Does That the Local Project Doesn't

### 1. Structured Entity Extraction

Google extracts `entities` (named entities), `topics` (tags), and `importance` (float) from each memory. This enables future filtering, ranking, or search by entity/topic — even without embeddings. The local system stores free-form text only.

### 2. Explicit Cross-Memory Connection Graph

During consolidation, Google creates bidirectional `connections` between memories with labeled relationships (e.g., `Memory #1 ↔ #3: "Agent reliability needs better memory architectures"`). This graph grows over time and can be traversed to answer relational queries. The local system has no such graph.

### 3. Multimodal Ingestion

Google uses Gemini's multimodal API to ingest images, audio, video, and PDFs — not just text. This is a significant capability difference for agents that work with files.

### 4. External Push Model

Google supports a file-watcher `inbox/` pattern, letting any process drop files for the agent to process asynchronously. This enables integration with external workflows without API calls.

### 5. Dashboard / Query Interface

Google exposes a Streamlit dashboard and HTTP `GET /query` endpoint, allowing humans to interact with the memory store directly. The local system has no such interface — it is purely embedded infrastructure.

### 6. Timer-Based Consolidation

Google's consolidation is time-driven (30-minute cycles), which ensures memory is consolidated even if the agent is idle. The local system only consolidates when the token threshold is crossed during an active run.

---

## 6. What the Local Project Does That Google's Doesn't

### 1. Conversation-Anchored Observation

The local system tracks a `last_observed_message_index` cursor and observes the live conversation transcript incrementally. It knows exactly which messages have been processed and which haven't. Google has no equivalent — it only processes explicitly pushed content.

### 2. Multi-Tenant Scoping

The local system isolates memory by `(tenant_id, conversation_id, agent_id)`. Google has a single global memory store with no isolation.

### 3. Operation Audit Log

Every mutation (`observe`, `reflect_now`, `enable`, `disable`) is recorded in `om_operation_log` with `queued → processing → applied/failed` lifecycle. Stale processing operations are reset on startup. Google has no audit trail.

### 4. Positional Markers

`om_markers` links memory activity back to specific conversation message positions. This enables debugging and auditing of exactly which conversation segment produced which memory chunk.

### 5. Concurrency Safety

The local system uses a `LocalCoordinator` with per-scope mutexes, SQLite WAL mode, and a busy timeout. Google uses raw synchronous `sqlite3` connections inside async coroutines — there is no locking and multiple concurrent ingests could corrupt memory records.

### 6. Configurable Token Budgets

The local system has three tunable thresholds (`ObserveMinTokens`, `SnippetMaxTokens`, `ReflectThresholdTokens`) per-record. This allows fine-grained control over when memory fires and how much context it consumes in prompts.

### 7. Production-Grade Storage

The local system supports both SQLite (with WAL mode) and PostgreSQL via a pluggable `Store` interface. The system handles migration, stale operation recovery, and write-race handling (`INSERT OR IGNORE` + re-query pattern). Google's system is prototype-grade: a single-threaded SQLite connection with no migration versioning.

---

## 7. Is Google's Memory System "Good"?

### Strengths

1. **Conceptually sound**: The ingest → consolidate → query loop mirrors how human episodic memory works. The brain's sleep-based consolidation metaphor is apt.
2. **Rich memory structure**: Entity extraction, topic tagging, importance scoring, and connection graphs are more information-dense than free-form text chunks. They enable future filtering and graph traversal.
3. **Multimodal**: The ability to ingest images, audio, and video is a genuine capability advantage for use cases that work with rich media.
4. **Self-contained**: It runs as a standalone service and is easy to understand and deploy.
5. **No embeddings needed**: The "just use an LLM to read everything" approach avoids the complexity of vector stores and embedding drift.

### Weaknesses

1. **Not suitable for production use**: No concurrency safety, no per-scope isolation, no audit trail, no migration versioning. Multiple concurrent agents sharing one store would corrupt data.
2. **Does not observe its own context**: The agent must be explicitly fed information. It cannot passively learn from the conversations it participates in. This is a fundamental architectural gap for agent harnesses.
3. **No injection budget management**: When answering queries, Google dumps all 50 memories into the LLM context with no budget cap. In a long-running system with hundreds of memories, this will hit context limits.
4. **Timer-based consolidation is imprecise**: Consolidating on a wall-clock timer rather than on content volume means it may consolidate too early (few memories) or too late (many memories). The local token-threshold approach is more adaptive.
5. **No per-conversation scoping**: This is a dealbreaker for multi-user or multi-agent deployments.
6. **Prototype code quality**: Raw `sqlite3.connect()` in tool functions, global `get_db()` pattern, no connection pooling, no transaction management beyond basic commit/rollback.

### Assessment

Google's system is a well-designed **demo and reference implementation**. It introduces genuinely useful ideas (structured memory fields, connection graphs, multimodal ingestion, timer-based consolidation) and is clear to understand. However, it is explicitly built for single-user, single-process use as an educational example. It is not designed for embedding into a production agent harness.

The local system is architecturally more mature: it is scoped, audited, concurrency-safe, and deeply integrated with the agent's conversation lifecycle. It lacks the structured metadata (entities, topics, connections) and multimodal capabilities of Google's design.

---

## 8. Summary of Key Gaps

The two most actionable ideas from Google's system that the local project could benefit from:

1. **Structured memory fields**: Adding entity extraction, topic tags, and importance scoring to observation chunks would make memories more queryable and filterable in future work.
2. **Cross-memory connection graph**: The consolidation step in the local system could be enhanced to produce explicit links between observations, rather than just compressing them into a flat reflection string. This would enable relational queries across conversation history.

The local system's conversation-anchored, multi-tenant, concurrency-safe design is the right foundation for a production agent harness. Google's design trades those properties for richness of memory structure and multimodal capability.
