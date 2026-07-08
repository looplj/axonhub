# Protocol field ownership and first preservation slice

AxonHub will preserve the author's transformer framework and repair protocol field loss by assigning every field to a clear owner: cross-protocol common fields stay in `llm.Request` / `llm.Response`, protocol-native fields stay in their protocol transformer modules, same-protocol unknowns use raw fallback, and cross-protocol losses are reported as `LossyDowngrade` diagnostics. The first implementation slice is frozen to OpenAI Responses -> OpenAI Responses native preservation, because it directly covers Codex Responses, MCP/lazy-loading identity, and official Responses fields without expanding the blast radius into Chat, Anthropic, or stream-event fidelity.

## Considered Options

- Put every missing field into `llm.Request`: rejected because it turns the common view into a universal protocol AST and makes provider blast radius worse.
- Use `passThroughBody` as the fix: rejected because it can preserve bytes but does not give structured ownership, diagnostics, accounting, or safe downgrade behavior.
- Patch each provider adapter independently: rejected because it scatters field-loss decisions and makes later audits unreliable.
- Preserve the author framework and deepen native preservation seams: accepted because it is the smallest architecture change that gives locality, leverage, and testable preserve-or-diagnose behavior.

## Consequences

The first implementation must not fix Chat, Anthropic, or stream fidelity in the same slice. It must first prove same-protocol OpenAI Responses preservation, then later slices can reuse the field-ownership rules for Chat emission policy, Anthropic native preservation, LossyDowngrade diagnostics, and stream event fidelity.
