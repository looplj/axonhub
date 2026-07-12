# Design — latest Codex delta handling

## Classification first

本任务先把源差异分为四类，再决定是否实现：

```text
Codex source delta
  ├─ Existing official Responses field + actual outbound change
  │    └─ G13/G14/G15 → audit AxonHub same-protocol seam
  ├─ Existing tool family with no new wire shape
  │    └─ existing G6/G8/matrix backlog; no new implementation from this delta
  ├─ Codex app/control-plane change
  │    └─ explicit exclusion; no Hub transformer
  └─ Wire-neutral refactor / telemetry
       └─ explicit exclusion; no Hub transformer
```

## G13 — request reasoning/encrypted-content preservation

- Source fact: Codex now always sends an existing `reasoning` object and `include: ["reasoning.encrypted_content"]`.
- Hub contract: preserve only values actually present in the source Responses body. It must not inject those fields merely because Codex normally does.
- Seam: `Responses inbound HTTP body -> llm.Request -> Responses outbound HTTP body`.

## G14 — summary and stream-options capability compatibility

- Source fact: Codex now decides from its model metadata whether to send existing `reasoning.summary` / `stream_options.reasoning_summary_delivery`.
- Hub contract: preservation, not replication of Codex’s private capability decision. A request that includes the values must retain them on same-family replay; an omitted value must remain omitted.
- Seam: same as G13, with explicit typed + raw nested coexistence fixture when relevant.

## G15 — item identity/presence

- Source fact: Codex uses a typed string internally, but only emits known-prefix IDs outbound; empty/unprefixed values are omitted.
- Hub contract: treat item id as source identity, not Hub/Codex-generated identity. Preserve supplied non-empty ID exactly; preserve absence. Do not import Codex prefix validation into generic transformer.
- Seam: typed Responses input item → canonical message/tool state → emitted Responses `input[]` item.

## Architecture rules

- No new universal AST and no new common effort enum.
- Do not move Codex-specific model capability/prefix rules into `llm.Request` or shared transformer helpers.
- Raw fields stay owned by the Responses adapter and existing raw-preserve policy.
- If a G has no code defect, commit only a targeted fixture/doc proof; do not refactor production code.

## Compatibility / rollback

Each G is independently testable and revertible. Existing G9–G12 code remains untouched unless a G13–G15 fixture proves an actual bug in its public behavior.
