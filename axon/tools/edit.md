Replace exact text in a file.

Inputs:
- path (required): File path (relative paths are resolved against the workspace root).
- old_text (required): Exact text to find.
- new_text (required): Replacement text.
- replace_all (optional): If true, replace all occurrences. Defaults to false.

Behavior & limits:
- When replace_all is false (default), the edit succeeds only if old_text is found exactly once.
- When replace_all is true, all occurrences are replaced.

Tips:
- Read the file contents first to confirm the exact text to replace.
- If old_text occurs multiple times, include more surrounding context to make it unique, or set replace_all to true.
- Keep whitespace exactly as-is in old_text (including newlines and indentation).
- Use replace_all for renaming variables or strings across the file.

Examples:
- {"path":"./README.md","old_text":"old","new_text":"new"}
- {"path":"./main.go","old_text":"oldVar","new_text":"newVar","replace_all":true}