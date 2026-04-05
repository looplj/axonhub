Match files under a directory using a glob pattern.

Inputs:
- pattern (required): Glob pattern. Supports `**` plus standard `filepath.Match` wildcards (`*`, `?`, `[...]`).
- path (optional): Directory to search in. Defaults to workspace root.

Behavior & limits:
- Searches regular files and skips common vendor dirs (e.g., `.git`, `node_modules`).
- Results are sorted by modification time (newest first).
- Results may be truncated (currently shows up to 200 matches).

Examples:
- {"pattern":"**/*.go"}
- {"pattern":"frontend/**/routes/**/*.tsx","path":"./"}
