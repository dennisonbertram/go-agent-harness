# Slash Command UX Review — Gemini 2.5 Pro

**Date:** 2026-03-19
**Model:** gemini-2.5-pro
**Prompt topic:** Terminal TUI slash command system for an AI chat client

---

Of course. As a terminal UX expert, here is my honest assessment of your slash command system.

### Overall Assessment

This is a strong start. The system leverages familiar TUI patterns (slash commands, autocomplete, help dialog) and provides a solid foundation. The discoverability through autocomplete is excellent. However, there is a critical flaw in the conceptual model of your configuration commands (`/model`, `/provider`, `/keys`) that creates significant confusion, cognitive load, and redundancy. This issue overshadows the rest of the otherwise well-designed system.

---

### What Works Well

*   **Standard Convention:** Using `/` for commands is instantly familiar to anyone who has used Slack, Discord, or other modern chat applications. This lowers the barrier to entry.
*   **Excellent Discoverability:** The autocomplete dropdown with descriptions is the single best feature of this system. It transforms the interface from one requiring memorization to one of recognition and exploration. Fuzzy matching is a great touch for power users.
*   **Efficiency for Common Tasks:** Core session management commands like `/clear` and `/quit` are simple, clear, and fast. `/export` is also a well-understood and valuable utility.
*   **Comprehensive Help System:** A dedicated `/help` command with tabs for Commands and Keybindings is a best practice for TUIs. It provides a necessary safety net for users.

---

### What is Confusing & Problematic

The primary issue is a **fractured and overlapping conceptual model** for configuration. The user is forced to guess which command is the "right" one for their task, because the responsibilities of `/model`, `/provider`, and `/keys` are not clearly delineated.

#### 1. The `/model`, `/provider`, `/keys` Tangle

This is the key UX concern you identified, and you are right to be worried. It's the most significant flaw.

*   **First-Time User Confusion:** A new user wanting to set their API key is faced with a choice:
    *   `/model` says it manages API keys.
    *   `/keys` says it manages API keys.
    *   Which one is it? The user has to experiment or read documentation, which is a failure of intuitive design.
    *   The description for `/provider` — "...(use /model for per-model config)" — is a huge red flag. A command's description should not have to immediately point to a *different* command to clarify its own purpose. It signals that the abstraction is wrong.

*   **Expert User Annoyance:** An expert user will quickly become annoyed by this.
    *   **Increased Cognitive Load:** Instead of a single, coherent mental model ("I need to configure things"), they have to remember the arbitrary rules of this specific application ("Oh right, for *adding* a key I use `/keys`, but for *associating* it with a model I use `/model`... I think?").
    *   **Inefficiency:** If a user wants to set up a new model from scratch, they might have to run three separate commands to configure the gateway, add a key, and select the model. This is inefficient and frustrating.

The root of the problem is that these three commands manipulate different facets of the **same underlying concept: the chat configuration**. They should not be three separate, top-level commands.

#### 2. Vague & Potentially Overlapping Information Commands

*   `/context` vs. `/stats`: The distinction isn't immediately clear. "Context usage grid" sounds like a statistic. "Usage statistics" is very broad. A user might wonder: "Does `/stats` include the context usage, or is that separate?" This adds minor but unnecessary cognitive friction.

#### 3. Jargon-heavy Advanced Commands

*   `/subagents`: This command is likely clear to the developer, but "subagents" and "isolation state" are jargon. For 95% of users, this just adds noise to the command list. A powerful feature like this is great, but its presence at the top level can be intimidating and confusing for newcomers.

---

### Specific, Critical Recommendations for Change

My recommendations focus on creating a clear, hierarchical, and logical command structure based on the **Single Responsibility Principle**. Commands should do one thing well, or they should act as a clear entry point to a related group of sub-tasks.

#### Proposal 1: Unify Configuration under a Single Command

Get rid of `/model`, `/provider`, and `/keys`. Replace them with a single, powerful `/config` command (or `/set`) that uses subcommands. This immediately clarifies the user's mental model: "To change a setting, I use `/config`."

**New Command Structure:**

*   **/config** `<setting>` `<value>`

**Examples:**

*   `/config model gpt-4-turbo` — (Most common use case) Quickly switch the active model.
*   `/config list models` — Show available models.
*   `/config list keys` — List configured API keys and their associated providers.
*   `/config add key openai` — Interactively prompts the user to paste their OpenAI API key.
*   `/config remove key anthropic` — Removes the key for Anthropic.
*   `/config gateway <url_or_alias>` — Sets the routing gateway/proxy.
*   `/config` — (with no arguments) Opens a full-screen, interactive TUI editor for all settings. This is the most user-friendly option for complex setup.

**Why this is better:**
*   **Logical Grouping:** All configuration actions are under one roof.
*   **Discoverable:** Typing `/config ` and hitting `Tab` would autocomplete the available subcommands (`model`, `key`, `gateway`, `list`, `add`...).
*   **Eliminates Ambiguity:** There is no longer a question of which command to use for keys or gateways.

#### Proposal 2: Consolidate Information Commands

Merge `/context` into `/stats` as a subcommand.

**New Command Structure:**

*   `/stats` — Shows a general overview of usage statistics (e.g., total tokens, cost).
*   `/stats context` — Shows the detailed context usage grid (the old `/context` view).
*   `/stats tokens` — Shows a detailed breakdown of token usage by model/session.

**Why this is better:**
*   Reduces top-level command clutter.
*   Creates a logical hierarchy: context usage is a *type* of statistic.

#### Proposal 3: Isolate or Rename Advanced Features

The `/subagents` command should be de-emphasized for the average user.

*   **Option A (Rename):** If possible, rename it to something more intuitive, like `/agents` or `/bots`.
*   **Option B (Nest):** Move it under a parent command. Perhaps `/agents list`, `/agents enable <name>`, etc. This reserves the top-level namespace for the most common commands.

### Revised Command List (Proposed)

| Current Command | Proposed Replacement/Change | Description |
| :-------------- | :-------------------------- | :------------------------------------------------------------------- |
| `/clear`        | `/clear`                    | *(No change)* Clear conversation history.                            |
| `/context`      | `/stats context`            | Show the detailed context usage grid.                                |
| `/export`       | `/export`                   | *(No change)* Export conversation transcript.                        |
| `/help`         | `/help`                     | *(No change)* Show help dialog.                                      |
| `/keys`         | `/config add/remove/list key` | Manage provider API keys within the unified config system.           |
| `/model`        | `/config model`             | Switch the active model. All other settings go under `/config`.      |
| `/provider`     | *(Removed)*                 | Redundant. Gateway is set via `/config gateway`. Model implies provider. |
| `/quit`         | `/quit`                     | *(No change)* Quit the TUI.                                          |
| `/stats`        | `/stats`                    | Show usage statistics. Use subcommands for details.                  |
| `/subagents`    | `/agents list`              | (Renamed & Nested) List managed agents and their state.            |
| *(New)*         | `/config`                   | The new, unified command for managing all settings.                  |

This revised structure is more logical, scalable, and significantly less confusing for both new and expert users. It clarifies the application's concepts and empowers the user by providing a predictable and consistent interface.
