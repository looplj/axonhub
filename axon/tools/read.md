Read a file or list a directory from the workspace.

Usage:
- `path` (required): File or directory path. Relative paths are resolved against the workspace root.
- `read_range` (optional): Inclusive line range `[start, end]` using 1-indexed line numbers.

Behavior:
- If `path` is a directory, returns one entry per line. Subdirectories end with `/` and directory listings do not include line numbers.
- If `path` is a file, contents are returned with each line prefixed as `N: <line>`.
- If `read_range` is omitted, this tool returns at most the first 500 lines.
- Any individual line longer than 2000 characters is truncated in-place and marked with `... (truncated)`.
- When a file is longer than the default 500-line window, the output ends with a truncation marker such as `... (truncated, showing lines 1-500 of 900; use read_range to read more)`.
- To continue reading a long file, call this tool again with a larger `read_range` instead of repeatedly requesting tiny slices.
- If you are unsure of the path, use `Glob` or `Grep` first.

Output format:
- Header: `<path> (<total> lines, <ext> file)`
- Body: `N: <line>`

Examples:
- `{"path":"./config.yml"}`
- `{"path":"./internal/server/server.go","read_range":[1,120]}`
- `{"path":"./internal/server/server.go","read_range":[121,260]}`
