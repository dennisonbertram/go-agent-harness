Write a new behavior or talent markdown file to the system prompt extensions directory.

This tool lets the agent create new behavioral patterns or talent specializations that will be available in future sessions. Behaviors modify how the agent operates; talents add domain-specific knowledge or capabilities.

Parameters:
- extension_type: "behavior" or "talent"
- name: machine-readable identifier, lowercase letters and hyphens only (e.g., "prefer-short-answers")
- title: human-readable title for the extension
- content: markdown content of the extension
- overwrite: if true, overwrite an existing file with the same name (default: false)

The file is written to the configured extensions directory for the given type. The name is sanitized to ensure it is safe as a filename. If the extensions directory is not configured, the tool returns an error.
