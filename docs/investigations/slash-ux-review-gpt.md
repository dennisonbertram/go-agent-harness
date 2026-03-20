# Slash Command UX Review — GPT-4.1 Assessment

**Date:** 2026-03-19
**Model:** GPT-4.1
**Purpose:** UX review of the terminal TUI slash command system

---

Here's a detailed, critical UX assessment from the perspective of both a first-time and an expert user:

---

## What Works Well

**1. Slash Command Discovery**
- Having `/` trigger an autocomplete dropdown with names and clearly-aligned descriptions is excellent.
- Fuzzy matching, arrow key, and Tab navigation are efficient and modern patterns.
- Immediate execution on Enter is appropriate for most commands.

**2. Clear, Minimal Set of Core Commands**
- Most commands are intuitive (`/help`, `/clear`, `/quit`, `/export`, `/stats`).
- Distinct labels for `/help`, `/export`, `/clear` are particularly user-friendly.

**3. Help Dialog Structure**
- Three-tab help dialog ("Commands | Keybindings | About") is clear.
- Tab/Shift-Tab to cycle between tabs works well for keyboard users.
- Aligned formatting (command → description) is readable.

---

## Confusing Points and Redundancies

### **The `/model`, `/provider`, and `/keys` Tangle**

#### 1. Blurred Responsibilities
- **/model:** "Switch model, gateway, and API keys"
- **/provider:** "Switch routing gateway (use /model for per-model config)"
- **/keys:** "Manage provider API keys"

A first-time user will immediately ask:
- What is a "gateway"? How is it different from a "provider"?
- Are "provider" and "gateway" the same? Why two commands?
- If I just want to change the model, do I have to change the provider as well?
- Why does `/model` also manage "API keys" and "gateway"?

#### 2. Redundancy/Overloading
- `/model` is overloaded: it does three things (model, gateway, keys). That's a lot for one command.
- `/provider` does what `/model` also claims to (gateway switching).
- If `/model` lets you do per-model config, then why use `/provider` at all, or vice versa?
- `/keys` is for "API keys" but `/model` and `/provider` can swap keys too. Conflict.

#### 3. Terminology
- "Gateway", "Provider", "Model" need DEFINED meanings in your UI. Their relationship is currently ambiguous.
- Expert users may be annoyed by having to mentally map three interrelated commands with documented overlaps and exceptions.

### **Other Issues**

- **/subagents:** The concept is advanced but maybe unclear if not explained somewhere.
- Export exports to "markdown"—is that the only option? (minor, but might annoy power users).
- No `/settings` catch-all; some might look for this instead of `/model` or `/provider`.

---

## Specific, Actionable Suggestions

### **1. Clarify and De-duplicate `/model`, `/provider`, `/keys`**

#### Option A: Unify with Subcommands
If possible, combine into a single `/config` or `/settings` umbrella:
```
/config model
/config provider
/config key
```
or let `/model` handle only models, `/provider` only routing gateways, `/keys` only API keys.

#### Option B: Clearer Scoping & Naming
- **/model:** Switches only the model (e.g., GPT-3.5, Claude-2, etc)
- **/provider:** Switches only the provider (OpenAI, Anthropic, etc), and perhaps sets a default model per provider.
- **/keys:** Add, remove, list API keys (never changed by `/model` or `/provider`).

**Remove** any implicit cross-command effects. Each command manages *one* thing.

#### Option C: Descriptions
Update command descriptions with clarifying examples, e.g.:

- `/model` — Switch conversation AI model (e.g., GPT-4, Claude-2)
- `/provider` — Switch service provider (OpenAI, Anthropic, etc)
- `/keys` — Add or manage API keys for all providers

And in each command, offer `--help` (or `/model help`) for detailed usage patterns.

#### Make Relationships Explicit
First-time users need to *see* associations: e.g., "Current Model: GPT-4 (Provider: OpenAI)".

### **2. Help Dialog Improvements**
- Have a "Quick Start" message, on first /help open, explaining "models vs providers vs keys".
- Link to docs about "gateways" if they're a complex/unique idea in your system.

### **3. Advanced Commands: Explicit Labeling**
- Group or visually separate "beginner/frequent" vs "advanced/rare" commands (`/subagents`, `/context`).

---

## TL;DR — UX Assessment

**Strengths:**
- Command autocomplete, aligned descriptions, keyboard navigation — excellent.
- Help dialog structure good.
- Common chat-related commands are discoverable.

**Weaknesses:**
- `/model`, `/provider`, `/keys` are overlapping/confusing both in description and likely in function.
- Terminology ("provider", "gateway", "model") is not well-disambiguated.
- First-time users will be confused. Power users will dislike redundancy and overloading.

**Recommendations:**
- Redefine scoping and descriptions, possibly combine into a single `/config` umbrella.
- Make terminology and relationships explicit in UI and help.
- Stick to *one-responsibility-per-command* unless using subcommands.
