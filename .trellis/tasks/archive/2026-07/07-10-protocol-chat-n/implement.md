# Implement — Chat `n`

1. Read task context and Chat baseline/draft evidence.
2. Locate OpenAI Chat inbound raw capture and outbound replay with codebase-memory MCP/CLI.
3. Add a failing same-protocol fixture for `n` before production code.
4. Make the smallest native/raw-preserve change required for Chat -> Chat replay.
5. Add cross-protocol test only for diagnostic/unsupported behavior; do not implement multiple choices.
6. Run targeted OpenAI transformer tests and `git diff --check`.
7. Self-review owner, raw field scope, and absence of common-model widening.

