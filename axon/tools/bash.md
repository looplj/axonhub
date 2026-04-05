Run a shell command and return stdout/stderr.

Inputs:
- command (required): The command string to execute (non-Windows uses `sh -c`).
- cwd (optional): Working directory for this command. Relative paths are resolved against the workspace root (and may be restricted to the workspace).

Behavior & limits:
- Safety filter rejects obviously dangerous commands (e.g., `rm -rf`, `mkfs`, `shutdown`, writing to `/dev/*`).
- Timeout is fixed at 60s.
- Output is truncated to 10,000 chars for stdout and 10,000 chars for stderr.
- If stderr is non-empty, it is appended with a `STDERR:` header.
- If the command exits non-zero, `Exit error: ...` is appended.

Tips:
- Use this for build/test/git or other CLI workflows.
- Prefer `Read`/`Write`/`Edit`/`Glob`/`Grep` for filesystem and search tasks.

Examples:
- {"command":"go test ./..."}
- {"command":"git status","cwd":"./"} 
