Search file contents with a Go regular expression.

Inputs:
- pattern (required): Regex pattern (Go regexp syntax).
- path (optional): File or directory to search in. Defaults to workspace root.
- glob (optional): Filter files by glob. Supports brace expansion like `*.{ts,tsx}`.
- type (optional): File type shortcut (e.g., `go`, `ts`, `py`) mapped to common extensions.
- output_mode (optional): `files_with_matches` (default), `content`, or `count`.
- before/after/context (optional): Context lines (content mode only).
- line_number (optional): Show line numbers in content mode (default: true).
- ignore_case (optional): Case-insensitive search.
- multiline (optional): Let patterns span newlines (slower; content output becomes snippets, not line-based).
- literal (optional): Treat pattern as a literal string (no regex metacharacters).
- head_limit/offset (optional): Simple pagination over output entries.

Behavior & limits:
- Skips common vendor dirs (e.g., `.git`, `node_modules`) and common binary file types.
- Limits output size: up to 32 total entries, max 16 matches per file; long lines/snippets are truncated.

Examples:
- {"pattern":"New[A-Za-z]+Tool","type":"go","output_mode":"content","context":2}
- {"pattern":"interface\\{\\}","glob":"**/*.go","output_mode":"files_with_matches"}
