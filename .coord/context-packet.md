# Social Agent — Session Context

## Current State: V1 MERGED, UNTESTED LIVE

V1 fully merged to main at commit 6b7440b. All code in `apps/socialagent/`. 79 unit tests passing, race-clean. V0 was tested end-to-end successfully ("Works great, super fast"). V1 adds MCP tools, summarizer, "The Connector" personality, but has NOT been tested live yet.

## Architecture
```
Telegram → ngrok → socialagent (:8081) → harnessd (:8080) → OpenAI
                        ↓                       ↓
                    Postgres (:5433)     MCP server (:8082)
```

- `apps/socialagent/` — NO internal/ imports, HTTP API only
- Harness handles conversation persistence + auto-compaction
- Socialagent handles user identity, Telegram I/O, MCP tools, summarization

## V1 Components
- **MCP Server** (:8082): 5 tools — search_users, get_user_profile, get_updates, save_insight, get_my_profile
- **System Prompt**: "The Connector" — warm social mediator, privacy rules, tool instructions
- **Summarizer**: Background worker, extracts profile summary/interests/looking_for from conversation
- **Activity Logger**: Tracks user actions for get_updates tool
- **Gateway**: Wires MCP config into RunRequest, renders system prompt with user context

## Running Locally
```bash
cd apps/socialagent
./scripts/setup.sh    # Postgres Docker, .env, build binaries
# Fill in TELEGRAM_BOT_TOKEN and OPENAI_API_KEY in .env
./scripts/dev.sh      # harnessd + ngrok + webhook + MCP server + socialagent
```

## Bot Credentials
- Bot: @goAgentHarnessBot (t.me/goAgentHarnessBot)
- Token: stored in apps/socialagent/.env

## Open Issues
- #520: SSE keep-alive pings (enhancement)
- #521: Telegram API simulator for testing
- #522: Restrict harness tool access (security — HIGH)
- #523: Llama Guard input/output safety screening

## Next Step
Test V1 live: rebuild binaries, start stack, send Telegram message, verify MCP tools + summarizer work.
