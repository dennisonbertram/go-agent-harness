# Coding Harness Comparison: Codex vs OpenCode vs Crush vs Pi vs go-agent-harness

**Date**: 2026-03-05
**Goal**: Build the best coding harness — fast, lightweight, smart, long time horizons, persistent memory, universal runtime

---

## The Scorecard

Evaluated against your stated priorities: lightweight binary (run thousands), rich tool system, long time horizons (repo-wide historical understanding), persistent memory/learning, universal runtime (one core, many personas), and run-anywhere deployment.

| Dimension | Codex CLI | OpenCode | Crush | Pi / omp | go-agent-harness |
|-----------|-----------|----------|-------|----------|-----------------|
| **Language** | Rust | TS/Bun + Zig | Go | TS/Bun + Rust | Go |
| **Binary** | Single static | Bun runtime needed | Single static | Bun runtime needed | Single static |
| **License** | Apache-2.0 | Open source | FSL-1.1-MIT (!) | MIT | Yours |
| **Stars** | 63k | 112k | 20.9k | ~5k | -- |
| **Built-in Tools** | ~12 | ~12 + custom TS | ~20 | 4 (upstream) / 30+ (omp) | 30+ |
| **Custom Tool API** | MCP only | TS files + MCP | MCP only (!) | Full extension API | Go interface + MCP |
| **Model Providers** | OpenAI only | 75+ (via AI SDK) | 10+ (via Fantasy) | All major + local | OpenAI only (planned multi) |
| **Sandboxing** | OS-level (best) | Permission prompts | Permission prompts | None (by design) | Workspace scope + blocklist |
| **Context Compaction** | Auto @ 95% + encrypted | Auto @ 95% | Auto (buggy) | Auto on overflow | None yet (bounded steps) |
| **Persistent Memory** | Session JSONL replay | SQLite sessions | SQLite sessions | JSONL tree sessions | SQLite observational memory |
| **Sub-agents** | Experimental | Sequential only | Read-only only | Full parallel (omp) | Yes (agent tool) |
| **Headless/API Mode** | Yes (app-server) | Yes (HTTP/SSE) | Broken (`crush run`) | Yes (RPC/SDK) | Native (HTTP/SSE) |
| **Long Session Stability** | Memory leaks reported | Degrades over time | CPU spikes in long use | Stable (minimal) | Untested at scale |
| **Run 1000s Concurrently** | Possible (Rust) | No (Bun overhead) | Possible (Go) | No (Bun overhead) | Yes (Go, designed for it) |

---

## Deep Analysis by Your Priorities

### 1. Lightweight & Fast (Run Thousands)

**Winner: Codex (Rust) and go-agent-harness (Go) tied**

| Harness | Process Weight | Verdict |
|---------|---------------|---------|
| **Codex** | Single Rust binary, no GC, no runtime. Sub-second startup. BUT: Windows memory leak (90GB!), heavy workload memory pressure reported | Best raw performance, concerning stability issues |
| **go-agent-harness** | Single Go binary, GC but tunable, goroutine-native concurrency. HTTP server model means one process handles many runs | Purpose-built for this use case |
| **Crush** | Go binary, but Bubble Tea TUI is CPU-heavy. 30-40% CPU after extended use. Not designed for headless mass deployment | TUI overhead kills it for headless |
| **OpenCode** | Bun runtime required. Client/server arch adds overhead. Not a single binary | Disqualified for mass deployment |
| **Pi** | Bun runtime required. Lightweight in practice but runtime dependency | Disqualified for mass deployment |

**Takeaway**: Go is the right choice. Single binary, goroutine concurrency model is ideal for running thousands. Rust would be marginally faster but Go's simplicity and goroutine model is better for this server-of-agents pattern.

---

### 2. Smart with Lots of Tools

**Winner: go-agent-harness (30+ tools, most comprehensive built-in set)**

| Harness | Tool Count | Tool Quality | Extensibility |
|---------|-----------|-------------|---------------|
| **go-agent-harness** | 30+ | LSP, Sourcegraph, MCP, git, fetch, todos, ask_user, observational memory, sub-agents | Go interface + MCP |
| **Crush** | ~20 | LSP, MCP, Sourcegraph, shell (POSIX emulated) | MCP only -- no plugin API! |
| **Codex** | ~12 | Strong apply_patch (model-trained), ripgrep, web search, JS REPL | MCP only |
| **OpenCode** | ~12 | Basic file ops + custom TS files | TS files + MCP |
| **Pi upstream** | 4 | Minimal by design | Extension API (powerful but BYO) |
| **omp fork** | 30+ | Hashline edits, AST tools, LSP, browser automation, SSH | Full extension system |

**Takeaway**: Your harness already leads here. The key insight from competitors:
- **Hashline edits** (omp): Content-hash anchored editing. 10x improvement on some models. Worth stealing.
- **AST tools** (omp): `ast_grep` and `ast_edit` for syntax-aware operations. High value for refactoring personas.
- **Codex's trained apply_patch**: The model is specifically trained for their patch format. You can't replicate this without model training, but you can optimize your patch tool ergonomics.

---

### 3. Long Time Horizons & Repo-Wide Historical Understanding

**Winner: Nobody does this well yet. This is your opportunity to differentiate.**

| Harness | Context Strategy | Repo History | Memory Persistence | Verdict |
|---------|-----------------|-------------|-------------------|---------|
| **Codex** | Compaction with encrypted blob. ~258k usable tokens | `git` tool available but no deep integration | Session JSONL, can resume | Compaction is lossy -- forgets earlier context |
| **OpenCode** | Auto-compact at 95% | Git snapshots for rollback | SQLite sessions | Summarization loses nuance |
| **Crush** | Auto-summarize (buggy) | No deep git integration | SQLite sessions | Context bugs in long sessions |
| **Pi/omp** | Auto-compact on overflow + session tree branching | `git-overview`, `git-file-diff`, `git-hunk` tools (omp) | JSONL tree with fork/branch | Session trees are novel but no cross-session learning |
| **go-agent-harness** | Bounded steps (no compaction yet) | `git_status`, `git_diff` tools | SQLite observational memory with LLM-driven reflection | Best memory architecture, needs compaction + git depth |

**The gap**: Every harness treats repo history as "run `git log`." None of them build a **persistent, queryable model of the repository's evolution** -- why decisions were made, what patterns emerged, what regressions occurred.

**What "best in class" would look like**:

1. **Git History Agent Persona** -- Same core harness, equipped with deep git tools:
   - `git_log_search` -- semantic search over commit messages and diffs
   - `git_blame_context` -- who changed what and why (with linked issue/PR context)
   - `git_evolution` -- trace how a file/function evolved over time
   - `git_regression_detect` -- find when a behavior changed

2. **Accumulated Repo Knowledge** -- Your observational memory system, extended:
   - Per-repo memory that persists across conversations
   - Architectural decisions, recurring patterns, known pitfalls
   - Cross-conversation reflection: "I've seen this pattern 3 times, here's what works"

3. **Proactive Context Loading** -- Before each task:
   - Load relevant repo memory (past decisions about these files)
   - Load recent git history for touched files
   - Load related issues/PRs

4. **Conversation Compaction** -- You need this for unlimited steps:
   - Summarize older turns while preserving key decisions
   - Keep full transcript in SQLite for replay
   - omp's TTSR pattern (pattern-triggered context injection at zero cost until fired) is worth adopting

---

### 4. Persistent Memory / Cross-Session Learning

**Winner: go-agent-harness (already has the best architecture for this)**

| Harness | Memory Type | Cross-Session | Learning |
|---------|------------|--------------|----------|
| **Codex** | Session replay (JSONL) | Resume sessions, but no cross-session memory | None |
| **OpenCode** | SQLite session persistence | Can reload sessions | None |
| **Crush** | SQLite messages | Session continuity | None |
| **Pi** | JSONL session tree | Fork/branch/resume | None |
| **go-agent-harness** | SQLite observational memory with LLM-driven observation + reflection | **Yes** -- scoped by tenant/conversation/agent with dedicated cheap LLM | **Yes** -- automatic reflection synthesis |

**Your observational memory is genuinely novel among these harnesses.** None of the competitors have:
- Automatic observation after each step
- LLM-driven reflection synthesis at token thresholds
- Dedicated cheap LLM for memory (cost-isolated from main run)
- Scoped memory (tenant/conversation/agent dimensions)

**What to add for "best in class"**:
- **Cross-conversation memory** -- Currently scoped per-conversation. Add a repo-level memory scope that accumulates across all conversations in a workspace.
- **Memory retrieval** -- Before each run, query repo-level memory for relevant context about the files/patterns being worked on.
- **Memory decay/consolidation** -- Older memories should be periodically consolidated (like human memory consolidation during sleep). Stale facts should decay; confirmed patterns should strengthen.
- **Structured memory types** -- Separate stores for: architectural decisions, recurring patterns, known bugs, file relationships, team conventions.

---

### 5. Universal Runtime (One Core, Many Personas)

**Winner: go-agent-harness (YAML-driven prompt composition is the right architecture)**

| Harness | Persona System | Composability |
|---------|---------------|---------------|
| **Codex** | AGENTS.md + config.toml | Basic -- mostly just system prompt override |
| **OpenCode** | Custom agents via JSON/markdown | Good -- model, prompt, tools, permissions per agent |
| **Crush** | Coder vs Task (hardcoded) | Poor -- only 2 profiles |
| **Pi/omp** | Extensions + SYSTEM.md override + role-based model routing | Excellent -- full behavior replacement via extensions |
| **go-agent-harness** | YAML catalog: intents + model profiles + behaviors + talents | **Best** -- composable layers with validation |

**Your prompt composition system is already the most structured**:
```
Base prompt → Intent → Model profile → Task context → Behaviors → Talents → Custom
```

This is exactly the right architecture for "one harness, many personas." A git historian persona is:
- Intent: `git_historian`
- Behaviors: `["thorough", "analytical"]`
- Talents: `["git_deep", "code_archaeology"]`
- Tools: git-focused subset from catalog

**What to add**:
- **Tool profiles** -- Associate tool subsets with intents. When intent=`git_historian`, only load git-related tools. This is your deferred tools problem (GitHub #4) solved via composition.
- **Persona packages** -- Bundle intent + behaviors + talents + tool profile + memory scope into a single deployable persona config.
- **Runtime persona switching** -- Allow the running agent to switch its own persona mid-task (e.g., switch from `coder` to `reviewer` when done implementing).

---

### 6. Run Anywhere

**Winner: Codex (Rust binary) and go-agent-harness (Go binary) tied**

| Harness | Platforms | Dependencies |
|---------|----------|-------------|
| **Codex** | macOS, Linux, Windows | Zero (static binary) |
| **go-agent-harness** | Anywhere Go compiles | Zero (static binary) |
| **Crush** | macOS, Linux, Windows, Android, FreeBSD, OpenBSD, NetBSD | Zero (static binary) -- widest platform support |
| **OpenCode** | macOS, Linux, Windows | Bun runtime |
| **Pi** | macOS, Linux, Windows | Bun runtime |

Go and Rust are equivalent here. Go cross-compiles trivially (`GOOS=linux GOARCH=arm64 go build`). The harness already runs as an HTTP server, so it's naturally containerizable.

---

## Architectural Ideas Worth Stealing

### From Codex
1. **OS-level sandboxing** -- Seatbelt/Landlock/seccomp. Real security, not permission prompts. Worth implementing for untrusted code execution personas.
2. **Op/EventMsg protocol** -- Clean separation of frontend and core. Your SSE events are similar but less formalized.
3. **Head-tail buffer** -- Capture first N and last M lines of command output. Simple, effective memory control for shell tools.
4. **Encrypted compaction blob** -- Preserves "latent understanding" during context compaction. Clever if the provider supports it.

### From OpenCode
1. **Git-based snapshots** -- `git add . && write-tree` for rollback without polluting history. Zero-cost safety net.
2. **Custom tools via file drop-in** -- `.opencode/tools/mytool.ts` becomes a tool automatically. UX pattern worth replicating (`.harness/tools/mytool.go`?).
3. **Per-agent tool permissions** -- Fine-grained control over which tools each agent type can use.

### From Crush
1. **Fantasy provider abstraction** -- Clean multi-provider interface. Your provider interface is ready for this.
2. **POSIX shell emulation** -- `mvdan.cc/sh/v3` for shell execution without spawning real shells. Safer, more portable.
3. **Model switching mid-session** -- Preserving context while changing models. Critical for cost optimization.

### From Pi / oh-my-pi
1. **Hashline edits** -- Content-hash anchored line editing. 10x edit reliability improvement on some models. **Top priority to steal.**
2. **TTSR (Time Traveling Streamed Rules)** -- Pattern-triggered context injection at zero token cost until fired. Brilliant for long sessions.
3. **Session tree branching** -- Fork conversations at any point, navigate history tree. Low implementation cost, high value.
4. **Mid-session model switching with context transformation** -- Adapt message formats between providers on the fly.
5. **AST tools** -- `ast_grep` and `ast_edit` for syntax-aware code operations. Higher reliability than text-based editing.
6. **Role-based model routing** -- Different models for different tasks (cheap for titles, strong for coding, medium for planning).

---

## The "Best Coding Harness" Architecture

Based on this analysis, here's what the ideal harness looks like -- and how go-agent-harness maps to it:

### Core Runtime (you have this)
- [x] Deterministic step loop
- [x] Event-driven (SSE streaming)
- [x] HTTP API (headless-first)
- [x] Single Go binary
- [x] Multi-tenant scoped
- [ ] Conversation compaction (needed for unlimited steps)
- [ ] Multi-provider support
- [ ] Streaming tool output

### Tool System (you're ahead)
- [x] 30+ built-in tools
- [x] MCP integration
- [x] LSP integration
- [x] Sub-agent spawning
- [ ] Tool profiles (load subsets per persona)
- [ ] Hashline edits (steal from omp)
- [ ] AST tools (steal from omp)
- [ ] Deep git tools (git_log_search, git_blame_context, git_evolution)

### Prompt Composition (you're ahead)
- [x] YAML-driven modular composition
- [x] Intent → Model profile → Behaviors → Talents
- [x] Runtime context injection (time, tokens, cost)
- [ ] Persona packages (bundle everything)
- [ ] TTSR-style pattern-triggered injection (steal from omp)
- [ ] Runtime persona switching

### Memory (you're uniquely positioned)
- [x] SQLite-backed observational memory
- [x] LLM-driven observation + reflection
- [x] Dedicated cheap LLM for memory
- [x] Scoped by tenant/conversation/agent
- [ ] Cross-conversation repo-level memory
- [ ] Memory retrieval before each run
- [ ] Memory consolidation/decay
- [ ] Structured memory types (decisions, patterns, bugs, conventions)
- [ ] Git history as memory source

### Scale (designed for it)
- [x] Go binary -- goroutine concurrency
- [x] HTTP server model -- one process, many runs
- [x] Minimal resource footprint
- [ ] Persistence layer (runs survive restart)
- [ ] Cost ceilings per run
- [ ] Backpressure / rate limiting

---

## Verdict

**go-agent-harness is in the strongest architectural position to become the best coding harness**, specifically because:

1. **Go** -- Right language for running thousands of lightweight agents. Not TS (runtime overhead), not Rust (development velocity tradeoff for a system that's mostly waiting on API calls).

2. **HTTP-first** -- Every competitor started as a TUI and struggles with headless mode. You started headless and can add TUI later. This is correct for a "universal runtime."

3. **Observational memory** -- No competitor has anything close. This is your moat for long time horizons.

4. **Prompt composition** -- Most structured and composable system of any competitor. The "one core, many personas" architecture is already partially implemented.

5. **Tool richness** -- Largest built-in tool set. Combined with MCP and sub-agents, the most capable out of the box.

**Critical gaps to close** (in priority order):
1. Conversation compaction (for unlimited steps / long time horizons)
2. Multi-provider support (at minimum Anthropic + OpenAI)
3. Cross-conversation persistent memory (repo-level knowledge accumulation)
4. Deep git tools (for the repo historian persona)
5. Hashline edits (steal from omp for edit reliability)
6. Persistence layer (runs survive process restart)

---

## Detailed Reports

- [Codex CLI Review](codex-cli-review.md)
- [OpenCode Review](opencode-review.md)
- [Crush Review](crush-review.md)
- [Pi / oh-my-pi Review](pi-review.md)
