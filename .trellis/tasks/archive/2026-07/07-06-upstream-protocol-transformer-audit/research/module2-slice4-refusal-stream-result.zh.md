# Module 2 Slice 4 result — Responses stream refusal.delta/done

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`
Base commit for this module: `fe716145 fix: preserve responses request native fields`

## Scope

OpenAI Responses provider stream -> `llm.Response` stream preservation for official refusal events:

- `response.refusal.delta`
- `response.refusal.done`

Out of scope:

- Other missing stream event families.
- Chat.
- Anthropic.
- Cross-protocol diagnostics.

## Field audit

Official stream event extraction defines:

- `ResponseRefusalDeltaEvent`: `type`, `item_id`, `output_index`, `content_index`, `delta`, `sequence_number`.
- `ResponseRefusalDoneEvent`: `type`, `item_id`, `output_index`, `content_index`, `refusal`, `sequence_number`.

Common `llm.Message` already has `Refusal string`, so this is a stable existing seam.

## Red test

Added `TestOutboundTransformer_TransformStream_PreservesRefusalDelta`.

Initial failure:

- transformed stream contained no `Delta.Refusal`.

## Implementation

Changed files:

- `llm/transformer/openai/responses/stream_event.go`
- `llm/transformer/openai/responses/outbound_stream.go`
- `llm/transformer/openai/responses/outbound_stream_test.go`

Implementation summary:

- Added `StreamEventTypeRefusalDelta` and `StreamEventTypeRefusalDone`.
- Added `Refusal` field to `StreamEvent` for `response.refusal.done` parsing.
- Mapped `response.refusal.delta` to `llm.Message.Refusal` delta.
- Skipped `response.refusal.done`, because refusal text has already streamed through deltas.

## Verification

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformStream_PreservesRefusalDelta$' -count=1

go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

Result:

- Focused test passed.
- Responses package tests passed.
- `git diff --check` passed.

## Self-review

Pass:

- Uses an existing common field (`llm.Message.Refusal`) instead of adding a new model.
- Keeps same-protocol stream fidelity scoped to the Responses transformer.
- No Chat, Anthropic, Gemini, OpenRouter, or cross-protocol files touched.

Follow-up:

- Remaining stream gaps include MCP, file search lifecycle, web search lifecycle, audio, and code interpreter events. These should be separate future slices.
