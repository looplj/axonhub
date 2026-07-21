> **Historical PRD (planning baseline).** Request-only “first phase” wording is **ordering history**, not current freeze.
> Durable decisions live in `docs/adr/0001-separate-openai-responses-native-preservation.md` and `docs/adr/0002-protocol-field-ownership-and-first-preservation-slice.md` (refreshed 2026-07-22).
> Implementation progress (response `RawOutputItems` / stream raw / Chat & Anthropic same-protocol evidence) supersedes “phase 1 request only” as a scope ban.
> For wire facts use `docs/specs/protocols/*-protocol.md`; for completion use the strict verification matrix.

# OpenAI Responses Native Round-Trip PRD

## Problem Statement

AxonHub currently treats OpenAI Responses requests too much like an intermediate projection of the cross-protocol canonical request. That works for shared model semantics, but it damages OpenAIResponsesNative structures that strict clients such as Codex rely on for tool routing, tool discovery, reasoning replay, and future field compatibility.

The user-visible failure is that Codex using Responses mode can lose MCP/tool behavior, break tool discovery, or experience confusing stream/session behavior because Responses-only fields such as NamespaceQualifier, tool search, additional tools, client metadata, and unknown future fields are flattened, dropped, or rebuilt incorrectly. PassThroughBody proves the upstream can work when the original payload is preserved, but it is only a mitigation; the actual transformer/native layer must support FullNativeRoundTrip.

## Solution

Implement OpenAI Responses as a native protocol surface inside AxonHub's existing Responses transformer architecture. Keep CrossProtocolCanonical focused on shared semantics, and add a ResponsesNativeAST plus raw fallback path so same-protocol OpenAI Responses routes can parse, inspect, diagnose, and re-emit request payloads without losing OpenAIResponsesNative semantics.

The first implementation phase covers request native round-trip only. Later phases will extend the same model to response bodies and stream events. The implementation must stay aligned with the current transformer package structure, avoid a new framework, and avoid turning the canonical request into a dumping ground for Responses-only fields.

## User Stories

1. As a Codex user, I want MCP tools declared as namespaces to remain namespaces, so that Codex can route calls through its DispatchRegistry.
2. As a Codex user, I want LeafMethod names inside a NamespaceQualifier to remain unchanged, so that local tool handlers resolve correctly.
3. As a Codex user, I want tool search declarations to remain tool search declarations, so that deferred MCP tools can be discovered correctly.
4. As a Codex user, I want deferred tool metadata to keep its defer-loading information, so that large tool inventories do not have to be fully loaded upfront.
5. As a Codex user, I want additional tools injected through Responses input items to survive transformation, so that loaded tools remain available in the next request.
6. As a Codex user, I want client metadata to survive same-protocol routing, so that Codex session/request metadata is not silently discarded.
7. As a Codex user, I want reasoning encrypted-content includes to survive same-protocol routing, so that reasoning replay and state continuity are not broken by the gateway.
8. As a Codex user, I want function calls with namespace identity to preserve that identity, so that StructuralValidity also remains RoutableIdentity.
9. As a Codex user, I want custom/freeform tool structures to survive native routing, so that non-JSON tool calls such as patch-style tools are not forced into ordinary function calls.
10. As a gateway operator, I want OpenAI Responses requests to be parseable and inspectable, so that I can diagnose native field preservation without relying only on blind body pass-through.
11. As a gateway operator, I want unknown future OpenAI Responses fields to be preserved in same-protocol routing, so that new OpenAI fields do not immediately break behind the gateway.
12. As a gateway operator, I want known fields to override raw fallback during re-emission, so that model mapping and intentional policy changes cannot be undone by stale raw data.
13. As a gateway operator, I want raw fallback to preserve unknown fields, so that compatibility is maintained even before AxonHub understands a new field semantically.
14. As a gateway operator, I want lossy downgrades to be explicit and diagnosable, so that cross-protocol conversion failures are observable instead of silent.
15. As a gateway operator, I want Responses-to-Responses routing to avoid flattening namespaces by default, so that same-protocol routing remains native.
16. As a gateway operator, I want Responses-to-Chat routing to retain the option to flatten namespaces, so that non-Responses targets can still receive a compatible degraded representation.
17. As a gateway maintainer, I want the implementation to reuse the existing Responses transformer modules, so that the change fits the author's architecture.
18. As a gateway maintainer, I want Responses-only fields kept out of CrossProtocolCanonical, so that shared protocol abstractions remain deep and clean.
19. As a gateway maintainer, I want a small native request module with a clear interface, so that preservation logic has locality and does not scatter across inbound and outbound conversion.
20. As a gateway maintainer, I want behavior tests at the highest practical transformer seam, so that refactors can preserve behavior without testing private helpers.
21. As a gateway maintainer, I want each preservation behavior added through small TDD slices, so that review can catch scope creep and architecture drift early.
22. As a gateway maintainer, I want review findings routed back to TDD or debugging, so that failed reviews produce concrete red conditions rather than vague cleanup work.
23. As a gateway maintainer, I want milestone reports after groups of slices, so that verification, architecture notes, and remaining risks stay visible.
24. As a future implementer, I want the first phase limited to request round-trip, so that response and stream native preservation can be implemented later without making the first change unreviewable.
25. As a future implementer, I want the OpenAI official docs and Codex source snapshot referenced from local artifacts, so that field decisions can be checked without rediscovering sources.

## Implementation Decisions

- Treat OpenAI Responses as a native protocol surface, not just a serialization target for CrossProtocolCanonical.
- Keep CrossProtocolCanonical as the maximum common denominator for shared model semantics: model, ordinary messages, sampling parameters, ordinary function tools, and other stable cross-protocol concepts.
- Add a ResponsesNativeAST for known OpenAI Responses request fields and combine it with raw fallback for unknown or future fields.
- Use known-plus-raw merge when re-emitting OpenAI Responses requests. Known structured values win over same-name raw values; unknown raw values are preserved.
- Keep same-protocol Responses-to-Responses routing native. It must preserve request semantics even when it does not use blind PassThroughBody.
- Keep PassThroughBody as an operational mitigation and correctness baseline, not as the architectural fix.
- Keep cross-protocol routing explicitly lossy where necessary. Namespace flattening is allowed only as a downgrade for non-Responses targets, and it must be diagnosable.
- Implement the first phase inside the existing OpenAI Responses transformer package. Separate native request preservation, native tool preservation, native input preservation, and merge behavior by module-level responsibilities inside that package.
- Do not create a new OpenAI Responses protocol framework in the first phase. If response/stream phases prove the current package seam is too cramped, record that as a later architecture decision.
- Do not place Responses-only structures such as namespace tools, tool search, additional tools, client metadata, or raw unknown fields directly into CrossProtocolCanonical.
- Do not write Codex-specific one-off branches. CodexResponsesProfile is a high-coverage validation profile for OpenAIResponsesNative, not a separate private protocol.
- Preserve all standard OpenAI Responses request tool variants in same-protocol routing. Where AxonHub can only partially understand a tool variant, retain the full native representation for re-emission.
- Preserve request-side input item variants, including items that carry tools or future unknown item types.
- Preserve tool choice forms, including unknown or future forms, through native/raw handling.
- Preserve OpenAI and Codex request metadata fields in the native layer rather than conflating them with generic metadata unless they are truly cross-protocol.
- Generate diagnostics for lossy downgrade and unknown preservation decisions from a centralized place rather than scattering logs or flags across the converter.
- Keep implementation slices small enough for review. Every slice must have a behavior-level test or an explicit reason why no correct seam exists.

## Testing Decisions

- Use the highest practical seam that exercises request inbound conversion, native extension preservation, outbound reconstruction, and same-protocol OpenAI Responses emission.
- Prefer behavior-level round-trip tests over private helper tests. The main assertion is that an input Responses request emerges with the same native semantics after Hub processing.
- Add one red test per behavior. Do not write all preservation tests first and then implement them in bulk.
- The first tracer-bullet test should prove the round-trip test seam with a simple Responses request before introducing namespace/tool-search complexity.
- Test unknown top-level field preservation before large known-field coverage, because raw fallback is the safety net for future OpenAI fields.
- Test known-over-raw merge explicitly with a model mapping scenario, because model mapping must not be overwritten by raw fallback.
- Test client metadata as a native known field, not as generic request metadata.
- Test namespace tool preservation by asserting that same-protocol output keeps the namespace object and LeafMethod name rather than emitting a CompositeName function.
- Test that cross-protocol namespace flattening, where still needed, is explicitly a LossyDowngrade and not the same-protocol default.
- Test tool search preservation by asserting that it remains a tool-search tool, not an ordinary function named tool_search.
- Test defer-loading preservation on both ordinary function tools and namespace child tools where applicable.
- Test additional-tools input items by asserting they do not become ordinary messages and that nested tools survive.
- Test unknown tool and input item types by asserting complete raw preservation in same-protocol routing.
- Test complex or unknown tool-choice forms through raw preservation.
- After each implementation slice, run a review slice against the ADR, field classification document, and domain glossary.
- If a review slice fails, convert the finding into the smallest red condition and return to TDD or diagnosing-bugs before continuing.
- After the first-phase milestone, run a targeted architecture review using Module, Interface, Depth, Seam, Adapter, Leverage, and Locality vocabulary.

## Out of Scope

- Response body native round-trip is out of scope for the first implementation phase.
- SSE stream event native round-trip is out of scope for the first implementation phase.
- UI configuration for native preservation modes is out of scope for the first implementation phase.
- Full provider/channel routing redesign is out of scope.
- Replacing the current transformer architecture with a new protocol framework is out of scope.
- Broad refactors of Chat, Claude, Gemini, or unrelated protocol transformers are out of scope.
- Changing billing, accounting, or persistence behavior beyond what is necessary to preserve and diagnose request fields is out of scope.
- Publishing external issue tracker items is out of scope unless explicitly requested.

## Further Notes

- Domain glossary: `CONTEXT.md`.
- Architecture decision: `docs/adr/0001-separate-openai-responses-native-preservation.md`.
- Field classification and implementation constraints: `docs/specs/openai-responses-native-field-classification.md`.
- Official OpenAI/Codex local references: `docs/specs/vendor/openai/`.
- Workflow router skill: `/Users/asuan/.skills-manager/skills/agent-workflow-router/SKILL.md`.
- Current committed documentation baseline: `915962a8 docs: define OpenAI Responses native round-trip plan`.
