# Claude Agent SDK -- Complete Reference

**Date:** 2026-03-14
**Package:** `@anthropic-ai/claude-agent-sdk` (current) / `@anthropic-ai/claude-code` (legacy, still works)
**Sources:** Context7 (`/anthropics/claude-code`, `/ben-vargas/ai-sdk-provider-claude-code`, `/websites/code_claude`, `/llmstxt/code_claude_llms_txt`), official docs at `code.claude.com`

---

## 1. Installation and Package Identity

The SDK was originally published as `@anthropic-ai/claude-code` and has been renamed to `@anthropic-ai/claude-agent-sdk`. Both package names work and export the same API.

```bash
# Current name
npm install @anthropic-ai/claude-agent-sdk

# Legacy name (still works)
npm install @anthropic-ai/claude-code
```

There is also a Python SDK:

```bash
pip install claude-agent-sdk
# Legacy: pip install claude-code-sdk
```

### Key Exports

```typescript
import {
  query,              // Primary function -- runs the agent loop
  tool,               // Define custom in-process tools
  createSdkMcpServer, // Create an in-process MCP server from tools
  ClaudeCode,         // Class-based constructor (newer API surface)
} from "@anthropic-ai/claude-agent-sdk";
// OR
import { query, tool, createSdkMcpServer } from "@anthropic-ai/claude-code";
```

There is **no** `claude()` function export. The primary API is `query()`.

---

## 2. The `query()` Function

### Signature

```typescript
function query({
  prompt,
  options,
}: {
  prompt: string | AsyncIterable<SDKUserMessage>;
  options?: Options;
}): Query;
```

### Parameters

- **`prompt`** (`string | AsyncIterable<SDKUserMessage>`) -- The user's message or a stream of user messages for multi-turn input.
- **`options`** (`Options`) -- Configuration object (see section 3).

### Return Value: `Query`

`Query` extends `AsyncGenerator<SDKMessage, void>` -- it is an async iterable that yields `SDKMessage` events. It also exposes control methods:

```typescript
interface Query extends AsyncGenerator<SDKMessage, void> {
  /** Interrupt the current generation */
  interrupt(): Promise<void>;

  /** Rewind file changes to a specific user message */
  rewindFiles(userMessageId: string, options?: { dryRun?: boolean }): Promise<RewindFilesResult>;

  /** Change permission mode mid-session */
  setPermissionMode(mode: PermissionMode): Promise<void>;

  /** Change model mid-session */
  setModel(model?: string): Promise<void>;

  /** Get initialization details (tools, model, etc.) */
  initializationResult(): Promise<SDKControlInitializeResponse>;

  /** List all supported slash commands */
  supportedCommands(): Promise<SlashCommand[]>;

  /** List all supported models */
  supportedModels(): Promise<ModelInfo[]>;

  /** Get MCP server connection statuses */
  mcpServerStatus(): Promise<McpServerStatus[]>;

  /** Get account info (plan, usage) */
  accountInfo(): Promise<AccountInfo>;

  /** Push additional user messages into an active stream */
  streamInput(stream: AsyncIterable<SDKUserMessage>): Promise<void>;

  /** Stop a running subagent task */
  stopTask(taskId: string): Promise<void>;

  /** Close the query and clean up */
  close(): void;
}
```

### Basic Usage

```typescript
import { query } from "@anthropic-ai/claude-agent-sdk";

for await (const message of query({
  prompt: "Find and fix the bug in auth.py",
  options: { allowedTools: ["Read", "Edit", "Bash"] },
})) {
  if (message.type === "assistant") {
    for (const block of message.message.content) {
      if (block.type === "text") process.stdout.write(block.text);
    }
  }
  if (message.type === "result") {
    console.log("Done. Cost:", message.total_cost_usd);
  }
}
```

### Alternative: `ClaudeCode` Class

A newer class-based API also exists:

```typescript
import { ClaudeCode } from "@anthropic-ai/claude-agent-sdk";
const client = new ClaudeCode();
```

---

## 3. `Options` -- All Configuration Parameters

| Property | Type | Default | Description |
|:---------|:-----|:--------|:------------|
| `abortController` | `AbortController` | `new AbortController()` | For cancelling the query |
| `allowedTools` | `string[]` | `[]` | Tools to auto-approve without user confirmation |
| `disallowedTools` | `string[]` | `[]` | Tools to always deny. Cannot use with `allowedTools` simultaneously |
| `model` | `string` | Default from CLI/env | Claude model to use (`"opus"`, `"sonnet"`, `"haiku"`, or full model ID) |
| `maxTurns` | `number` | `undefined` (unlimited) | Maximum agentic tool-use round trips |
| `maxBudgetUsd` | `number` | `undefined` | Spending cap in USD |
| `systemPrompt` | `string \| SystemPromptPreset` | `undefined` (minimal) | System prompt configuration |
| `permissionMode` | `PermissionMode` | `'default'` | How tool permissions are handled |
| `settingSources` | `SettingSource[]` | `[]` (none) | Which filesystem settings to load (CLAUDE.md, settings.json) |
| `cwd` | `string` | `process.cwd()` | Working directory for the agent |
| `additionalDirectories` | `string[]` | `[]` | Extra directories the agent can access |
| `mcpServers` | `Record<string, McpServerConfig>` | `{}` | MCP server configurations |
| `resume` | `string` | `undefined` | Session ID to resume (loads conversation history) |
| `sessionId` | `string` | Auto-generated UUID | Explicit session UUID (does NOT load history) |
| `includePartialMessages` | `boolean` | `false` | Emit token-level streaming events (`stream_event`) |
| `effort` | `'low' \| 'medium' \| 'high' \| 'max'` | `'high'` | Reasoning depth / thinking effort |
| `maxThinkingTokens` | `number` | `undefined` | Max tokens for extended thinking (ignored on Opus 4.6/Sonnet 4.6 which use adaptive reasoning) |
| `hooks` | `Partial<Record<HookEvent, HookCallbackMatcher[]>>` | `{}` | Lifecycle hook callbacks |
| `agents` | `Record<string, AgentDefinition>` | `undefined` | Programmatic subagent definitions |
| `stderr` | `(data: string) => void` | `undefined` | Callback for stderr output from the subprocess |
| `env` | `Record<string, string>` | `undefined` | Environment variables for the agent process |
| `executable` | `string` | `"node"` | Executable to run the agent |
| `executableArgs` | `string[]` | `[]` | Arguments to the executable |
| `pathToClaudeCodeExecutable` | `string` | Auto-detected | Custom path to the claude CLI binary |
| `fallbackModel` | `string` | `undefined` | Model to use if the primary fails |
| `extraArgs` | `Record<string, string>` | `{}` | Additional CLI arguments |

### System Prompt Configuration

The `systemPrompt` option accepts either a plain string or a preset object:

```typescript
// Plain string
systemPrompt: "You are an expert TypeScript developer."

// Preset with optional append
systemPrompt: {
  type: 'preset',
  preset: 'claude_code',           // Use Claude Code's full system prompt
  append: 'Always explain your reasoning step by step.',
}
```

When `systemPrompt` is not provided, the SDK uses a minimal system prompt (not the full Claude Code interactive prompt).

### Permission Modes

```typescript
type PermissionMode =
  | 'default'            // Ask user for each permission
  | 'acceptEdits'        // Auto-approve file edits, ask for others
  | 'bypassPermissions'  // Skip all permission prompts (dangerous)
  | 'plan'               // Read-only planning mode
  | 'dontAsk'            // Never ask, deny what isn't explicitly allowed
  | 'alwaysAsk'          // Always ask, even for auto-approved tools
  | 'auto';              // Equivalent to bypassPermissions in SDK context
```

### Setting Sources

```typescript
type SettingSource = 'user' | 'project' | 'local';
```

Controls which filesystem settings (CLAUDE.md files, settings.json) are loaded:
- `'user'` -- `~/.claude/` settings
- `'project'` -- `.claude/` in the project root
- `'local'` -- `.claude.local/` settings

Default is `[]` (no filesystem settings loaded), which means the SDK operates with explicit configuration only.

### Tool Name Patterns

Tools can be specified with optional argument patterns:

```typescript
allowedTools: [
  'Read',                        // Allow all Read operations
  'Bash(git log:*)',             // Allow only git log bash commands
  'Bash(git status)',            // Allow exact command
  'mcp__filesystem__read_file', // Allow specific MCP tool
  'mcp__git__status',           // MCP tools use mcp__<server>__<tool> format
]
```

---

## 4. `SDKMessage` Event Types

All messages yielded by the `query()` async generator are typed as `SDKMessage`, a discriminated union:

```typescript
type SDKMessage =
  | SDKSystemMessage              // type: "system"
  | SDKAssistantMessage           // type: "assistant"
  | SDKUserMessage                // type: "user"
  | SDKUserMessageReplay          // type: "user" (with isReplay: true)
  | SDKResultMessage              // type: "result"
  | SDKPartialAssistantMessage    // type: "stream_event"
  | SDKCompactBoundaryMessage     // type: "system", subtype: "compact_boundary"
  | SDKStatusMessage              // Status updates
  | SDKHookStartedMessage         // Hook lifecycle
  | SDKHookProgressMessage        // Hook lifecycle
  | SDKHookResponseMessage        // Hook lifecycle
  | SDKToolProgressMessage        // Tool execution progress
  | SDKAuthStatusMessage          // Authentication status
  | SDKTaskNotificationMessage    // Task notifications (subagents)
  | SDKTaskStartedMessage         // Task started (subagents)
  | SDKTaskProgressMessage        // Task progress (subagents)
  | SDKFilesPersistedEvent        // Files saved to disk
  | SDKToolUseSummaryMessage      // Summary of tool usage
  | SDKRateLimitEvent             // Rate limit hit
  | SDKPromptSuggestionMessage;   // Suggested follow-up prompts
```

### 4.1 `SDKSystemMessage` (type: `"system"`, subtype: `"init"`)

The first message emitted. Contains session metadata.

```typescript
type SDKSystemMessage = {
  type: "system";
  subtype: "init";
  uuid: UUID;
  session_id: string;
  agents?: string[];
  apiKeySource: ApiKeySource;
  betas?: string[];
  claude_code_version: string;
  cwd: string;
  tools: string[];
  mcp_servers: { name: string; status: string }[];
  model: string;
  permissionMode: PermissionMode;
  slash_commands: string[];
  output_style: string;
  skills: string[];
  plugins: { name: string; path: string }[];
};
```

Use this to capture the `session_id` for later `resume` calls.

### 4.2 `SDKAssistantMessage` (type: `"assistant"`)

The main response from Claude. Contains the full Anthropic API message.

```typescript
type SDKAssistantMessage = {
  type: "assistant";
  uuid: UUID;
  session_id: string;
  message: BetaMessage;            // Full Anthropic API message object
  parent_tool_use_id: string | null;  // Non-null when inside a subagent
  error?: SDKAssistantMessageError;
};
```

The `message.content` array contains content blocks:
- `{ type: "text", text: string }` -- Text response
- `{ type: "tool_use", id: string, name: string, input: object }` -- Tool call
- `{ type: "thinking", thinking: string }` -- Extended thinking (when enabled)

Error types on assistant messages:

```typescript
type SDKAssistantMessageError =
  | 'authentication_failed'
  | 'billing_error'
  | 'rate_limit'
  | 'invalid_request'
  | 'server_error'
  | 'unknown';
```

### 4.3 `SDKUserMessage` (type: `"user"`)

Echoed user input or synthetic tool results fed back in the agent loop.

```typescript
type SDKUserMessage = {
  type: "user";
  uuid?: UUID;
  session_id: string;
  message: MessageParam;            // Anthropic SDK MessageParam
  parent_tool_use_id: string | null;
  isSynthetic?: boolean;            // true for tool result messages
  tool_use_result?: unknown;
};
```

### 4.4 `SDKResultMessage` (type: `"result"`)

The final message, always the last event. Two categories: success and error.

```typescript
// Success
type SDKResultSuccess = {
  type: "result";
  subtype: "success";
  uuid: UUID;
  session_id: string;
  duration_ms: number;
  duration_api_ms: number;
  is_error: boolean;
  num_turns: number;
  result: string;                   // Final text result
  stop_reason: string | null;
  total_cost_usd: number;
  usage: NonNullableUsage;
  modelUsage: { [modelName: string]: ModelUsage };
  permission_denials: SDKPermissionDenial[];
  structured_output?: unknown;
};

// Error variants
type SDKResultError = {
  type: "result";
  subtype:
    | "error_max_turns"
    | "error_during_execution"
    | "error_max_budget_usd"
    | "error_max_structured_output_retries";
  uuid: UUID;
  session_id: string;
  duration_ms: number;
  duration_api_ms: number;
  is_error: boolean;
  num_turns: number;
  total_cost_usd: number;
  usage: NonNullableUsage;
  modelUsage: { [modelName: string]: ModelUsage };
  errors: string[];
};
```

### 4.5 `SDKPartialAssistantMessage` (type: `"stream_event"`)

Only emitted when `includePartialMessages: true`. These are raw Anthropic streaming events with token-level deltas.

```typescript
type SDKPartialAssistantMessage = {
  type: "stream_event";
  event: BetaRawMessageStreamEvent;
  parent_tool_use_id: string | null;
  uuid: UUID;
  session_id: string;
};
```

The `event` field contains standard Anthropic streaming events:
- `{ type: "message_start", message: {...} }`
- `{ type: "content_block_start", content_block: {...}, index: number }`
- `{ type: "content_block_delta", delta: { type: "text_delta", text: string }, index: number }`
- `{ type: "content_block_stop", index: number }`
- `{ type: "message_delta", delta: {...}, usage: {...} }`
- `{ type: "message_stop" }`

### 4.6 `SDKCompactBoundaryMessage`

```typescript
type SDKCompactBoundaryMessage = {
  type: "system";
  subtype: "compact_boundary";
};
```

Emitted when the conversation context has been compacted (summarized to save tokens).

### Typical Event Sequence

```
1. SDKSystemMessage (type: "system", subtype: "init")     -- session info
2. SDKAssistantMessage (type: "assistant")                 -- thinking/response
3. SDKUserMessage (type: "user", isSynthetic: true)        -- tool result echoed
4. SDKAssistantMessage (type: "assistant")                 -- next response
5. ... (multiple turns of tool use)
6. SDKResultMessage (type: "result", subtype: "success")   -- always last
```

With `includePartialMessages: true`, `stream_event` messages are interleaved between assistant messages, providing token-level streaming.

---

## 5. Session Management

### Starting a New Session

Every `query()` call creates a new session unless `resume` or `sessionId` is provided. The `session_id` is returned in the `SDKSystemMessage` (first event).

### Resuming a Session

Use the `resume` option with a previous `session_id` to load conversation history and continue:

```typescript
// Turn 1
let sessionId: string;
const turn1 = query({ prompt: "My name is Bob", options: { maxTurns: 1 } });
for await (const event of turn1) {
  if (event.type === "system" && event.subtype === "init") {
    sessionId = event.session_id;
  }
}

// Turn 2 -- resumes with full conversation history
const turn2 = query({
  prompt: "What is my name?",
  options: { resume: sessionId },
});
```

### Key Distinctions

| Option | Behavior |
|:-------|:---------|
| `resume` | Loads conversation history from the session. Supports multi-turn. |
| `sessionId` | Sets an explicit UUID but does NOT load history. |
| `continue` | Continues the most recent conversation (CLI-oriented). |

**Important:** `resume` and `continue` are mutually exclusive. Do not use both.

---

## 6. Tool System

### Built-in Tools

Claude Code's agent has access to these built-in tool categories:

**File Operations:**
- `Read` -- Read file contents
- `Write` -- Write/create files
- `Edit` -- Edit existing files (surgical replacements)
- `MultiEdit` -- Multiple edits in one call
- `LS` -- List directory contents

**Search:**
- `Glob` -- Find files by pattern
- `Grep` -- Search file contents with regex

**Execution:**
- `Bash` -- Run shell commands

**Web:**
- `WebSearch` -- Search the web
- `WebFetch` -- Fetch URL contents

**Orchestration:**
- `Agent` -- Spawn subagents
- `Task` -- Task management
- `TodoWrite` -- Manage task checklists (non-interactive/SDK mode)

**Other:**
- `NotebookEdit` -- Edit Jupyter notebooks

### Tool Approval Patterns

```typescript
// Auto-approve specific tools
allowedTools: ["Read", "Glob", "Grep"]

// Auto-approve bash with specific commands
allowedTools: ["Bash(npm test)", "Bash(git log:*)"]

// Deny dangerous tools
disallowedTools: ["Write", "Edit", "Bash(rm:*)", "Bash(sudo:*)"]

// Empty array blocks ALL tools
allowedTools: []
```

### Custom In-Process Tools

Use `tool()` and `createSdkMcpServer()` to define custom tools that run in the SDK process:

```typescript
import { z } from 'zod';
import { createClaudeCode, createSdkMcpServer, tool } from 'ai-sdk-provider-claude-code';

// 1) Define a tool
const add = tool(
  'add',
  'Add two numbers',
  { a: z.number(), b: z.number() },
  async ({ a, b }) => ({
    content: [{ type: 'text', text: String(a + b) }],
  })
);

// 2) Create SDK MCP server
const sdkServer = createSdkMcpServer({ name: 'local', tools: [add] });

// 3) Wire into provider
const claude = createClaudeCode({
  defaultSettings: {
    mcpServers: { local: sdkServer },
    allowedTools: ['mcp__local__add'],
  },
});
```

### MCP Tool Naming Convention

MCP tools are referenced as `mcp__<serverName>__<toolName>`:

```typescript
allowedTools: [
  'mcp__filesystem__read_file',
  'mcp__git__status',
  'mcp__local__add',
]
```

---

## 7. MCP Server Configuration

### stdio Servers

```typescript
options: {
  mcpServers: {
    'my-server': {
      type: 'stdio',           // Optional, default
      command: 'node',
      args: ['./server.js'],
      env: {
        API_KEY: process.env.MY_API_KEY,
        LOG_LEVEL: 'debug',
      },
    },
  },
}
```

### HTTP Servers

```typescript
options: {
  mcpServers: {
    'weather-api': {
      type: 'http',
      url: 'https://api.weather.com/mcp',
      headers: {
        Authorization: `Bearer ${process.env.API_TOKEN}`,
      },
    },
  },
}
```

### In-Process SDK Servers

```typescript
import { createSdkMcpServer, tool } from '@anthropic-ai/claude-agent-sdk';

const server = createSdkMcpServer({
  name: 'my-tools',
  tools: [/* tool definitions */],
});

options: {
  mcpServers: { 'my-tools': server },
}
```

### Filesystem Configuration

MCP servers can also be configured via `.mcp.json` files at the project root:

```json
{
  "database-tools": {
    "command": "node",
    "args": ["./servers/db-server.js"],
    "env": {
      "DB_URL": "${DB_URL}"
    }
  }
}
```

### Subagent-Scoped MCP Servers

In YAML subagent definitions, MCP servers can be scoped to specific subagents:

```yaml
---
name: browser-tester
mcpServers:
  - playwright:
      type: stdio
      command: npx
      args: ["-y", "@playwright/mcp@latest"]
  - github  # Reference existing server by name
---
```

---

## 8. Hooks and Lifecycle Events

### Hook Events

The SDK supports these hook event types:

| Event | When It Fires | Can Block? |
|:------|:-------------|:-----------|
| `SessionStart` | Session starts or resumes | No (can inject context) |
| `SessionEnd` | Session ends | No |
| `PreToolUse` | Before a tool is executed | Yes (allow/deny/ask) |
| `PostToolUse` | After a tool completes | No (can provide feedback) |
| `Stop` | Agent considers stopping | Yes (approve/block) |
| `SubagentStop` | Subagent considers stopping | Yes (approve/block) |
| `UserPromptSubmit` | User sends a prompt | No |
| `PreCompact` | Before context compaction | No |
| `Notification` | Notification events | No |
| `PermissionRequest` | Permission dialog about to show | Yes (allow/deny) |

### Hook Types

Each hook can be either command-based or prompt-based:

**Command-based hooks** run a shell command:

```json
{
  "PreToolUse": [
    {
      "matcher": "Write|Edit",
      "hooks": [
        {
          "type": "command",
          "command": "bash ./hooks/scan-secrets.sh",
          "timeout": 30
        }
      ]
    }
  ]
}
```

**Prompt-based hooks** use an LLM for context-aware decisions:

```json
{
  "Stop": [
    {
      "matcher": "*",
      "hooks": [
        {
          "type": "prompt",
          "prompt": "Verify task completion: all tests pass, all requirements met. Return 'approve' to stop or 'block' with reason.",
          "timeout": 30
        }
      ]
    }
  ]
}
```

### Programmatic Hooks in SDK Options

```typescript
options: {
  hooks: {
    PreToolUse: [
      {
        matcher: "Bash",
        hooks: [
          {
            type: "prompt",
            prompt: "Evaluate if this bash command is safe. Return 'approve' or 'deny'.",
          },
        ],
      },
    ],
    Stop: [
      {
        matcher: "*",
        hooks: [
          {
            type: "prompt",
            prompt: "Check if all tasks are complete.",
          },
        ],
      },
    ],
  },
}
```

### Hook Decision Outputs

**PreToolUse** decisions:

```json
{
  "hookSpecificOutput": {
    "permissionDecision": "allow|deny|ask",
    "updatedInput": { "field": "modified_value" },
    "systemMessage": "Explanation for Claude"
  }
}
```

**Stop / SubagentStop** decisions:

```json
{
  "decision": "approve|block",
  "reason": "Explanation",
  "systemMessage": "Additional context"
}
```

**PermissionRequest** decisions:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PermissionRequest",
    "decision": {
      "behavior": "allow|deny",
      "updatedInput": {},
      "message": "Reason for denial"
    }
  }
}
```

### Hook-Related SDK Events

The SDK emits these events for hook lifecycle tracking:
- `SDKHookStartedMessage` -- A hook has begun executing
- `SDKHookProgressMessage` -- Hook execution progress
- `SDKHookResponseMessage` -- Hook returned a result

---

## 9. Model Selection

### Via Options

```typescript
options: {
  model: 'sonnet',  // Use default Sonnet model
}
```

### Available Model Aliases

| Alias | Description |
|:------|:------------|
| `opus` | Claude Opus (most capable, complex reasoning) |
| `sonnet` | Claude Sonnet (balanced performance/cost) |
| `haiku` | Claude Haiku (fastest, most cost-effective) |

Full model IDs are also accepted (e.g., `claude-opus-4-6`, `claude-sonnet-4-6`).

### Runtime Model Switching

```typescript
const q = query({ prompt: "...", options: {} });

// Change model during execution
await q.setModel('opus');
```

### Environment Variable Overrides

```bash
# Set primary model
export ANTHROPIC_MODEL='claude-opus-4-6'

# Set default models per tier
export ANTHROPIC_DEFAULT_OPUS_MODEL='claude-opus-4-6'
export ANTHROPIC_DEFAULT_SONNET_MODEL='claude-sonnet-4-6'
export ANTHROPIC_DEFAULT_HAIKU_MODEL='claude-haiku-4-5@20251001'

# Set subagent model
export CLAUDE_CODE_SUBAGENT_MODEL='claude-sonnet-4-6'

# Small/fast model (deprecated, use ANTHROPIC_DEFAULT_HAIKU_MODEL)
export ANTHROPIC_SMALL_FAST_MODEL='claude-haiku-4-5@20251001'
```

---

## 10. Token Limits and Budgets

### Max Turns

```typescript
options: {
  maxTurns: 10,  // Limit tool-use round trips
}
```

If exceeded, the result message will have `subtype: "error_max_turns"`.

### Budget Cap

```typescript
options: {
  maxBudgetUsd: 5.0,  // Stop if cost exceeds $5
}
```

If exceeded, the result message will have `subtype: "error_max_budget_usd"`.

### Extended Thinking

```typescript
options: {
  maxThinkingTokens: 10000,  // Cap thinking tokens
  effort: 'high',            // 'low' | 'medium' | 'high' | 'max'
}
```

**Note:** `maxThinkingTokens` is ignored on Opus 4.6 and Sonnet 4.6, which use adaptive reasoning controlled by `effort`. Setting `maxThinkingTokens: 0` disables thinking on any model.

### Environment Variables for Thinking

```bash
# Set max thinking tokens
export MAX_THINKING_TOKENS=10000

# Disable thinking entirely
export MAX_THINKING_TOKENS=0

# Disable adaptive thinking (revert to fixed budget)
export CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING=1
```

---

## 11. Environment Variables -- Complete Reference

### Authentication

| Variable | Description |
|:---------|:------------|
| `ANTHROPIC_API_KEY` | Direct API key for Anthropic API. **Warning:** If set (even invalid), overrides subscription auth |
| `ANTHROPIC_AUTH_TOKEN` | Static auth token for LLM gateways |

### Model Configuration

| Variable | Description |
|:---------|:------------|
| `ANTHROPIC_MODEL` | Primary model override |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Opus alias model ID |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Sonnet alias model ID |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Haiku alias model ID |
| `ANTHROPIC_SMALL_FAST_MODEL` | **Deprecated.** Use `ANTHROPIC_DEFAULT_HAIKU_MODEL` |
| `CLAUDE_CODE_SUBAGENT_MODEL` | Model for subagents |

### Provider Configuration (Vertex AI)

| Variable | Description |
|:---------|:------------|
| `CLAUDE_CODE_USE_VERTEX` | Set to `1` to enable Vertex AI |
| `CLOUD_ML_REGION` | Vertex AI region (e.g., `global`, `us-east5`) |
| `ANTHROPIC_VERTEX_PROJECT_ID` | GCP project ID |
| `ANTHROPIC_VERTEX_BASE_URL` | Custom Vertex AI endpoint |
| `CLAUDE_CODE_SKIP_VERTEX_AUTH` | Skip Vertex auth (when gateway handles it) |
| `VERTEX_REGION_CLAUDE_*` | Per-model region overrides |

### Provider Configuration (Amazon Bedrock)

| Variable | Description |
|:---------|:------------|
| `CLAUDE_CODE_USE_BEDROCK` | Set to `1` to enable Bedrock |
| `AWS_REGION` | AWS region |
| `AWS_BEARER_TOKEN_BEDROCK` | Bedrock API key |
| `ANTHROPIC_BEDROCK_BASE_URL` | Custom Bedrock endpoint |
| `CLAUDE_CODE_SKIP_BEDROCK_AUTH` | Skip Bedrock auth (when gateway handles it) |

### Provider Configuration (Microsoft Foundry)

| Variable | Description |
|:---------|:------------|
| `ANTHROPIC_FOUNDRY_API_KEY` | Azure API key for Foundry |

### Thinking/Reasoning

| Variable | Description |
|:---------|:------------|
| `MAX_THINKING_TOKENS` | Max thinking tokens (0 to disable) |
| `CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING` | Set to `1` to revert to fixed thinking budget |

### Caching

| Variable | Description |
|:---------|:------------|
| `DISABLE_PROMPT_CACHING` | Set to `1` to disable prompt caching for all models |
| `DISABLE_PROMPT_CACHING_HAIKU` | Disable caching for Haiku only |
| `DISABLE_PROMPT_CACHING_SONNET` | Disable caching for Sonnet only |
| `DISABLE_PROMPT_CACHING_OPUS` | Disable caching for Opus only |

### Network/Proxy

| Variable | Description |
|:---------|:------------|
| `HTTPS_PROXY` | HTTPS proxy URL |
| `HTTP_PROXY` | HTTP proxy URL |
| `NO_PROXY` | Hosts to bypass proxy |

### Key Gotcha

If `ANTHROPIC_API_KEY` is set in the environment (even to an invalid value), it overrides subscription-based authentication. To use subscription auth, either unset it or set it to an empty string:

```bash
unset ANTHROPIC_API_KEY
# OR
export ANTHROPIC_API_KEY=""
```

---

## 12. Error Handling

### Error Events on Assistant Messages

```typescript
if (event.type === "assistant" && event.error) {
  switch (event.error) {
    case 'authentication_failed':
      console.error("Auth failed -- check API key");
      break;
    case 'billing_error':
      console.error("Billing issue");
      break;
    case 'rate_limit':
      console.error("Rate limited");
      break;
    case 'invalid_request':
      console.error("Invalid request");
      break;
    case 'server_error':
      console.error("Server error");
      break;
    case 'unknown':
      console.error("Unknown error");
      break;
  }
}
```

### Error Result Subtypes

The `SDKResultMessage` indicates how the session ended:

| Subtype | Meaning |
|:--------|:--------|
| `success` | Completed normally |
| `error_max_turns` | Hit `maxTurns` limit |
| `error_during_execution` | Runtime error during tool execution |
| `error_max_budget_usd` | Hit `maxBudgetUsd` spending cap |
| `error_max_structured_output_retries` | Structured output generation failed after retries |

### Rate Limit Events

`SDKRateLimitEvent` is emitted when the API rate limit is hit, allowing you to implement backoff logic.

### Cancellation via AbortController

```typescript
const controller = new AbortController();

const q = query({
  prompt: "Long running task...",
  options: { abortController: controller },
});

// Cancel after 30 seconds
setTimeout(() => controller.abort(), 30000);

for await (const event of q) {
  // Will stop yielding after abort
}
```

---

## 13. Subprocess and Sidecar Management

The SDK runs Claude Code as a subprocess. The main process communicates with it via JSON messages over stdio.

### stderr Handling

```typescript
options: {
  stderr: (data: string) => {
    console.error("[claude stderr]", data);
  },
}
```

### Executable Configuration

```typescript
options: {
  executable: 'node',
  executableArgs: ['--max-old-space-size=4096'],
  pathToClaudeCodeExecutable: '/usr/local/bin/claude',
}
```

### Process Cleanup

Call `query.close()` to clean up the subprocess:

```typescript
const q = query({ prompt: "...", options: {} });
try {
  for await (const event of q) { /* handle events */ }
} finally {
  q.close();
}
```

---

## 14. Subagent Definitions

### Programmatic Agents

```typescript
options: {
  agents: {
    'researcher': {
      name: 'researcher',
      description: 'Research agent with restricted capabilities',
      tools: ['Read', 'Grep', 'Glob', 'Bash'],
      disallowedTools: ['Write', 'Edit'],
      permissionMode: 'dontAsk',
    },
    'coder': {
      name: 'coder',
      description: 'Code writing agent',
      tools: ['Read', 'Write', 'Edit', 'Bash'],
      permissionMode: 'acceptEdits',
    },
  },
}
```

### YAML-Based Subagent Definitions

Subagents can also be defined as markdown files with YAML frontmatter:

```yaml
---
name: safe-researcher
description: Research agent with restricted capabilities
tools: Read, Grep, Glob, Bash
disallowedTools: Write, Edit
permissionMode: dontAsk
mcpServers:
  - github
---

# Research Agent Instructions

You are a research agent. Your job is to investigate codebases...
```

### Subagent-Related SDK Events

- `SDKTaskNotificationMessage` -- Notification from a subagent
- `SDKTaskStartedMessage` -- A subagent task has started
- `SDKTaskProgressMessage` -- Progress update from a subagent

---

## 15. Authentication

### Authentication Methods

1. **Subscription Auth (Default):** Uses the Claude subscription associated with the logged-in user. No API key needed.

2. **Direct API Key:**
   ```bash
   export ANTHROPIC_API_KEY=sk-ant-...
   ```

3. **Amazon Bedrock:**
   ```bash
   export CLAUDE_CODE_USE_BEDROCK=1
   export AWS_REGION=us-east-1
   # Uses AWS credentials (IAM, SSO, env vars, etc.)
   ```

4. **Google Vertex AI:**
   ```bash
   export CLAUDE_CODE_USE_VERTEX=1
   export CLOUD_ML_REGION=global
   export ANTHROPIC_VERTEX_PROJECT_ID=my-project
   # Uses Google Cloud credentials
   ```

5. **LLM Gateway / Proxy:**
   ```bash
   export ANTHROPIC_AUTH_TOKEN=sk-litellm-static-key
   export ANTHROPIC_BASE_URL=https://your-gateway.com/v1
   ```

6. **Microsoft Foundry:**
   ```bash
   export ANTHROPIC_FOUNDRY_API_KEY=your-azure-key
   ```

### API Key Source

The `SDKSystemMessage` includes `apiKeySource` which tells you how authentication was resolved.

---

## 16. Monitoring and Usage Events

### API Request Event

The `claude_code.api_request` event is logged for every API call with:

| Field | Description |
|:------|:------------|
| `model` | Model used |
| `cost_usd` | Estimated cost |
| `duration_ms` | Request duration |
| `input_tokens` | Input token count |
| `output_tokens` | Output token count |
| `cache_read_tokens` | Tokens read from cache |
| `cache_creation_tokens` | Tokens written to cache |
| `speed` | Whether fast mode was active |

### Usage in Result Messages

Every `SDKResultMessage` includes:

```typescript
{
  total_cost_usd: number;
  usage: {
    input_tokens: number;
    output_tokens: number;
    cache_read_tokens: number;
    cache_creation_tokens: number;
  };
  modelUsage: {
    [modelName: string]: {
      input_tokens: number;
      output_tokens: number;
      // ...
    };
  };
  duration_ms: number;
  duration_api_ms: number;
  num_turns: number;
}
```

---

## 17. Structured Output

The SDK supports structured output with guaranteed schema compliance:

```typescript
// Via Vercel AI SDK provider
import { generateObject } from 'ai';
import { claudeCode } from 'ai-sdk-provider-claude-code';
import { z } from 'zod';

const { object } = await generateObject({
  model: claudeCode('sonnet'),
  schema: z.object({
    name: z.string(),
    age: z.number(),
    skills: z.array(z.string()),
  }),
  prompt: 'Generate a developer profile',
});
```

This uses constrained decoding (native in Agent SDK v0.1.45+) to guarantee the output matches the Zod schema.

If structured output fails after retries, the result message will have `subtype: "error_max_structured_output_retries"`.

---

## 18. Multi-Turn Streaming Example

Complete example showing token-level streaming with session resumption:

```typescript
import { query } from "@anthropic-ai/claude-agent-sdk";

// Turn 1
const turn1 = query({
  prompt: "Explain TypeScript generics briefly",
  options: {
    includePartialMessages: true,
    maxTurns: 1,
  },
});

let sessionId: string;
for await (const event of turn1) {
  if (event.type === "system" && event.subtype === "init") {
    sessionId = event.session_id;
  }
  if (event.type === "stream_event" && event.event.type === "content_block_delta") {
    const delta = event.event.delta;
    if ('text' in delta) {
      process.stdout.write(delta.text);  // Token-level streaming
    }
  }
}

// Turn 2 -- resume with conversation history
const turn2 = query({
  prompt: "Now give an example with a generic function",
  options: {
    resume: sessionId,
    includePartialMessages: true,
  },
});

for await (const event of turn2) {
  if (event.type === "stream_event" && event.event.type === "content_block_delta") {
    const delta = event.event.delta;
    if ('text' in delta) {
      process.stdout.write(delta.text);
    }
  }
  if (event.type === "result") {
    console.log("\nCost:", event.total_cost_usd);
  }
}
```

---

## 19. CLI Equivalents

The SDK maps to CLI flags:

| SDK Option | CLI Flag |
|:-----------|:---------|
| `prompt` | `-p "..."` |
| `allowedTools` | `--allowedTools "Read,Edit,Bash"` |
| `maxTurns` | `--max-turns 10` |
| `model` | `--model sonnet` |
| `resume` | `--resume <session_id>` |
| `continue` (most recent) | `--continue` |
| `permissionMode: 'bypassPermissions'` | `--dangerously-skip-permissions` |
| `includePartialMessages: true` | `--include-partial-messages` (with `--output-format stream-json`) |

### CLI Stream JSON Format

```bash
claude -p "Explain recursion" \
  --output-format stream-json \
  --verbose \
  --include-partial-messages
```

Filter text deltas with jq:

```bash
claude -p "Write a poem" --output-format stream-json --verbose --include-partial-messages | \
  jq -rj 'select(.type == "stream_event" and .event.delta.type? == "text_delta") | .event.delta.text'
```

---

## 20. Permissions Configuration (settings.json)

Permissions can be configured in `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": [
      "Bash(npm test)",
      "Bash(git log:*)",
      "Read",
      "Glob",
      "Grep"
    ],
    "deny": [
      "Read(./.env)",
      "Read(./.env.*)",
      "Read(./secrets/**)",
      "Bash(rm -rf:*)",
      "Bash(sudo:*)"
    ]
  }
}
```

---

## Sources

- Official Agent SDK docs: https://code.claude.com/docs/en/headless
- Context7 `/anthropics/claude-code` (v2.1.39)
- Context7 `/ben-vargas/ai-sdk-provider-claude-code`
- Context7 `/websites/code_claude`
- Context7 `/llmstxt/code_claude_llms_txt`
- Existing project investigation: `docs/investigations/claude-code-sdk-vercel-ai.md`
