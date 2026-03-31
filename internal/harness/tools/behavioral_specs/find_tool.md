## When to Use
- Before concluding that a capability does not exist in the harness
- When you need a tool whose name you don't know but whose behavior you can describe
- Discovering deferred tools that are not visible in the default catalog

## When NOT to Use
- When you already know the exact tool name — call it directly
- As a substitute for reading the tool description — use the tool, don't just find it

## Behavioral Rules
1. Search before assuming a tool doesn't exist — many tools are deferred and invisible by default
2. Use descriptive search terms related to the capability, not the tool name
3. After finding a tool, activate it in the same step so it becomes available

## Common Mistakes
- **AssumeAbsent**: Telling the user a capability doesn't exist without calling find_tool first
- **NameSearch**: Searching for tool names instead of capability descriptions (search "cron" instead of "schedule recurring tasks")

## Examples
### WRONG
Tell the user "there is no tool for scheduling tasks" without checking.

### RIGHT
Call find_tool with query "schedule recurring tasks" to discover the cron_create tool.
