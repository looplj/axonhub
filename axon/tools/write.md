Write a file (overwrites if it exists) and creates parent directories if needed.

Inputs:
- path (required): File path. Relative paths are resolved against the workspace root.
- content (required): Full file content to write.

Behavior:
- Creates parent directories automatically.
- Overwrites existing files.

Tips:
- Prefer `edit` over `write` for modifying existing files.
- Read the file first before overwriting to avoid data loss.

Examples:
- {"path":"./tmp/output.txt","content":"hello\n"}
