# G7 Slice Ledger — Responses reasoning / stream

## Workflow
TDD per micro-slice; self-review each; module review after 8A-8E.

| Slice | Outcome | Evidence | Self-review | Status |
|---|---|---|---|---|
| 8A reasoning.context | same-protocol preserve context | reasoning-context.request.json + TestResponsesReasoningContextSameProtocolRoundTrip | pass | completed |
| 8B generate_summary | keep deprecated identity separate from summary | generate-summary fixtures + Tests | pass | completed |
| 8C reasoning output content/reasoning_text | output item content[] preserved | reasoning-output-content.response.json + TestResponsesReasoningOutputContentPreserved | pass | completed |
| 8D reasoning stream events | reasoning_text.* when prefer-text metadata set; summary path preserved | TestResponsesReasoningTextStreamEvents + existing stream fixtures green | pass | completed |
| 8E unknown nested reasoning variants | no silent drop | reasoning-unknown-nested.request.json + TestResponsesReasoningUnknownNestedPreserved | pass | completed |

## Implementation notes
- Request: context / generate_summary origin-value / raw reasoning object via TransformerMetadata; not widened onto llm.Request body fields.
- Response: Item.Content conflict fixed with custom JSON for type=reasoning; content[] reasoning_text preferred over summary.
- Stream: default remains reasoning_summary_*; reasoning_text.* gated by prefer-text metadata to avoid fixture breakage.
- Golden inbound request compare now copies ProviderExtensions (sidecar-only).

## Module review gate
- first review: PASS-with-majors by Wegener `019f5336-480a-72a0-adb7-15a918f06f8c`
  - M1 outbound_stream/aggregator silent skip reasoning_text.*
  - M2 prefer-text production writer missing
- fix commit: pending (this follow-up)
- re-review: PASS by Harvey `019f5352-3118-7133-bd18-a6a293f593e1`
- report: research/reviews/g7-module-rereview.md
- commits: 7a1d1cfe, e6fe1a78
