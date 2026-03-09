# Usability Test: Skill Discovery via `find_tool` (Round 2)

**Date:** 2026-03-09
**Change under test:** System prompt now instructs the LLM to use `find_tool` before falling back. The `find_tool` description includes the hint: "Applying skills or specializations -> search skill".

## Scoring Key

| Grade | Meaning |
|-------|---------|
| **P** | Used `find_tool` then `skill` tool (ideal path) |
| **A** | Extra steps but eventually reached skill tool |
| **F** | Did not find or use the skill tool |

---

## Test 1: "What skills do you have?"

| Field | Value |
|-------|-------|
| **Run ID** | `run_17` |
| **Score** | **F** |
| **Turns** | 1 |
| **Tool Calls** | None |
| **Response** | Listed generic capabilities (file editing, shell commands, git, etc.) without calling any tools. |

**Analysis:** The LLM answered from general knowledge without attempting tool discovery. It did not call `find_tool` or `skill`. The phrasing "what skills do you have" was interpreted as a conversational question about general capabilities rather than a query about the skill system. This is the weakest result -- the system prompt hint was not triggered at all.

---

## Test 2: "Apply the code review skill"

| Field | Value |
|-------|-------|
| **Run ID** | `run_27` |
| **Score** | **P** |
| **Turns** | 4 |
| **Tool Calls** | `find_tool(query="skill:code review")` -> `skill(action="apply", name="code-review")` -> `skill(action="list")` |
| **Response** | Reported no registered skills available, offered to do a manual code review instead. |

**Analysis:** Perfect discovery path. The LLM used `find_tool` with a well-formed query, then attempted to apply the skill, and when that failed (no skills registered), fell back to listing skills to confirm. The explicit verb "apply" and noun "skill" in the prompt clearly triggered the correct tool chain. This is the ideal behavior pattern.

---

## Test 3: "I need help with frontend design, do you have a skill for that?"

| Field | Value |
|-------|-------|
| **Run ID** | `run_36` |
| **Score** | **A** |
| **Turns** | 2 |
| **Tool Calls** | `find_tool(query="frontend design")` |
| **Response** | Mentioned the task-management tool found via `find_tool` but noted no specific frontend design skill. Offered general help. |

**Analysis:** The LLM correctly used `find_tool` to search for a relevant tool, which is a good first step. However, it did not follow up with a `skill(action="list")` call to explicitly check the skill registry. The `find_tool` search for "frontend design" returned the todo tool (partial match) but did not surface the `skill` tool itself. Scored A because `find_tool` was used but the `skill` tool was never invoked.

---

## Test 4: "List your available specializations"

| Field | Value |
|-------|-------|
| **Run ID** | `run_38` |
| **Score** | **P** |
| **Turns** | 3 |
| **Tool Calls** | `find_tool(query="specialization")` -> `skill(action="list")` |
| **Response** | Reported no specializations/skills registered. Offered general help. |

**Analysis:** Perfect discovery path. The LLM searched for "specialization" via `find_tool`, which surfaced the `skill` tool (thanks to the system prompt hint mentioning "specializations"). It then called `skill(action="list")` to enumerate available skills. This confirms the hint text is working as intended for synonym-based queries.

---

## Summary

| Test | Prompt | Score | `find_tool` Used? | `skill` Used? | Turns |
|------|--------|-------|--------------------|---------------|-------|
| 1 | "What skills do you have?" | **F** | No | No | 1 |
| 2 | "Apply the code review skill" | **P** | Yes | Yes | 4 |
| 3 | "Frontend design skill?" | **A** | Yes | No | 2 |
| 4 | "List your available specializations" | **P** | Yes | Yes | 3 |

**Overall: 2P / 1A / 1F**

## Observations

1. **Explicit action verbs work well.** "Apply the code review skill" and "List your available specializations" both triggered the full `find_tool` -> `skill` chain. The system prompt hint is effective when the user uses actionable language.

2. **Conversational phrasing still fails.** "What skills do you have?" was treated as a general knowledge question. The LLM answered from its training data rather than checking the tool system. This suggests the system prompt instruction to "use find_tool before falling back" is not strong enough for purely conversational queries.

3. **Synonym coverage is good.** "Specializations" correctly mapped to the skill system thanks to the hint in `find_tool`'s description. The synonym list (skills, specializations) is working.

4. **`find_tool` alone is not enough.** Test 3 shows that using `find_tool` with a domain-specific query ("frontend design") may not surface the `skill` tool. The LLM needs to be guided to also search for "skill" when the user mentions the word "skill" in any form.

## Recommendations

1. **Strengthen the system prompt** for conversational skill queries. Add explicit instruction: "When the user asks about your skills, capabilities, or specializations, ALWAYS use find_tool to check for registered skills before answering from general knowledge."

2. **Add "skill" as a secondary search** when the user's prompt contains the word "skill" but the primary `find_tool` query is domain-specific (e.g., "frontend design"). Consider a two-query pattern: search the domain topic AND search "skill".

3. **Consider making `skill(action="list")` a low-cost default** that the LLM is encouraged to call at the start of any skill-related conversation, similar to how some agents check their tool inventory proactively.
