# Implement — OpenAI Cache/State

1. Read Chat and Responses protocol baselines plus limits/state/cache draft.
2. TDD: add Chat request fixture and prove missing outbound replay.
3. Extend only Chat native capture/replay seam.
4. Preserve raw representation for unknown values.
5. Run scoped OpenAI transformer tests plus existing Responses cache tests.
6. Self-review that defaults and provider-specific cache semantics were not normalized across protocols.

