# Implement — Deprecated Chat Function Compatibility

1. Read official Chat deprecated schema and tools draft.
2. Split red fixtures into request `functions`, request `function_call`, response `message.function_call`.
3. Implement smallest compatibility/raw preserve seam; avoid broad parser rewrite.
4. Add explicit legacy/modern precedence tests.
5. Run scoped OpenAI tool transformer tests and diff check.
6. Self-review no contamination of modern tools path.

