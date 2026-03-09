Manage a structured, in-memory todo list for the current run. Use this tool whenever the user asks you to track tasks, create a checklist, manage a to-do list, or maintain a list of action items. This is the preferred way to store todos — do NOT write todo items to files with the write tool or manage them via bash.

To **create or replace** the todo list, pass a "todos" array. Each item has:
- id (string): unique identifier for the item (e.g. "1", "task-a")
- text (string): description of the task
- status (string): one of "pending", "in_progress", or "completed" (defaults to "pending" if omitted)

To **read** the current todo list, call this tool with no arguments (empty object or omit the todos parameter).

The todo list is scoped to the current run — each run has its own independent list. Calling with a todos array **replaces** the entire list, so always include all items (not just changed ones) when updating.

Example — create items:
```json
{"todos": [
  {"id": "1", "text": "Design API schema", "status": "in_progress"},
  {"id": "2", "text": "Write unit tests", "status": "pending"}
]}
```

Example — mark item 1 completed (note: full list must be sent):
```json
{"todos": [
  {"id": "1", "text": "Design API schema", "status": "completed"},
  {"id": "2", "text": "Write unit tests", "status": "pending"}
]}
```

Example — read current list:
```json
{}
```