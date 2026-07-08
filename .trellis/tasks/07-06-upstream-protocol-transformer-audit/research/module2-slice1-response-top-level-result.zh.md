# Module 2 Slice 1 result — Responses response top-level native fields

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`
Base commit for this slice: `fe716145 fix: preserve responses request native fields`

## Scope

OpenAI Responses same-protocol response top-level field preservation.

In scope:

- `completed_at`
- `output_text`

Out of scope:

- Stream event fidelity.
- Chat.
- Anthropic.
- Cross-protocol diagnostics.

## Field audit

Local official field extraction for `openai_responses_response` lists 33 top-level fields. Current `responses.Response` had 31 matching fields and missed:

- `completed_at`
- `output_text`

## Red test

Added `TestResponsesTransformResponse_RoundTripsNativeTopLevelFields` at the same-protocol seam:

```text
provider Responses HTTP body
  -> OutboundTransformer.TransformResponse
  -> llm.Response
  -> InboundTransformer.TransformResponse
  -> client Responses HTTP body
```

Initial failure:

- `completed_at` was absent from the final client body.
- `output_text` was absent from the final client body.

## Implementation

Changed files:

- `llm/transformer/openai/responses/model.go`
- `llm/transformer/openai/responses/outbound.go`
- `llm/transformer/openai/responses/inbound.go`
- `llm/transformer/openai/responses/outbound_test.go`
- `llm/transformer/openai/responses/response_extensions.go`

Implementation summary:

- Added `CompletedAt *int64` and `OutputText *string` to `responses.Response`.
- Stored those fields in `llm.Response.TransformerMetadata` during provider Responses -> `llm.Response` conversion.
- Replayed them during `llm.Response` -> client Responses conversion.
- Kept data scoped to the Responses transformer; did not widen common `llm.Response`.

## Verification

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestResponsesTransformResponse_RoundTripsNativeTopLevelFields$' -count=1

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

- Same-protocol response top-level fields now survive the full transformer seam.
- Common `llm.Response` was not widened for Responses-specific fields.
- No Chat, Anthropic, Gemini, OpenRouter, or stream code was changed.
- `completed_at` and `output_text` are narrow official response fields, so typed `*int64` / `*string` are appropriate.

Follow-up:

- Next Module 2 slice should audit stream event fidelity separately instead of mixing it with response body fields.
