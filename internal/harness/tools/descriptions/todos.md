Manage a structured, in-memory todo list for the current run. Use this tool whenever the user asks you to track tasks, create a checklist, manage a to-do list, or maintain a list of action items. This is the preferred way to store todos — do NOT write todo items to files with the write tool or manage them via bash.

## Actions

Use the optional `action` field to specify the operation. When omitted, the tool infers the action from the other fields.

### set (default when `todos` array is provided)
Replace the entire todo list with the provided array. Always include all items when using this action.

Each item in the `todos` array has:
- id (string): unique identifier for the item (e.g. "1", "task-a")
- text (string): description of the task
- status (string): one of "pending", "in_progress", or "completed" (defaults to "pending" if omitted)

### update
Update a single item by ID without resending the full list. Provide `id` plus the fields to change (`status`, `text`, or both).

### delete
Remove a single item by ID. Provide `id`. Returns an error if the ID does not exist.

### get (default when no `todos` array is provided)
Read the current todo list without making any changes. Call with an empty object `{}` or omit the `todos` parameter.

## Examples

**Create or replace the full list:**
```json
{"action": "set", "todos": [
  {"id": "1", "text": "Design API schema", "status": "in_progress"},
  {"id": "2", "text": "Write unit tests", "status": "pending"}
]}
```

**Update a single item's status:**
```json
{"action": "update", "id": "1", "status": "completed"}
```

**Update a single item's text:**
```json
{"action": "update", "id": "2", "text": "Write and run unit tests"}
```

**Delete a single item:**
```json
{"action": "delete", "id": "1"}
```

**Read the current list:**
```json
{}
```

The todo list is scoped to the current run — each run has its own independent list.
