# Module 2 Slice 2 result — Responses stream output_text.annotation.added

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`
Base commit for this module: `fe716145 fix: preserve responses request native fields`

## Scope

OpenAI Responses provider stream -> `llm.Response` stream preservation for the official event:

- `response.output_text.annotation.added`

Out of scope:

- Other missing stream event families.
- Chat.
- Anthropic.
- Cross-protocol diagnostics.

## Field audit

Official stream event extraction lists `ResponseOutputTextAnnotationAddedEvent` with:

- `type`
- `item_id`
- `output_index`
- `content_index`
- `annotation_index`
- `annotation`
- `sequence_number`

Current stream event literals did not include `response.output_text.annotation.added`; unknown events are skipped in `responsesOutboundStream.transformStreamChunk`, so this event lost annotation data.

## Red test

Added `TestOutboundTransformer_TransformStream_PreservesOutputTextAnnotationAdded`.

Initial focused failure after adding a terminal event:

- transformed stream contained no `llm.Annotation`.

## Implementation

Changed files:

- `llm/transformer/openai/responses/stream_event.go`
- `llm/transformer/openai/responses/outbound_stream.go`
- `llm/transformer/openai/responses/outbound_stream_test.go`

Implementation summary:

- Added `StreamEventTypeOutputTextAnnotationAdded`.
- Added `AnnotationIndex` and `Annotation` fields to `StreamEvent`.
- Converted the streamed annotation to `llm.Annotation` using existing `annotationToLLM` and emitted it as a delta annotation.

## Verification

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformStream_PreservesOutputTextAnnotationAdded$' -count=1

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

- Uses existing annotation conversion logic.
- Does not widen common `llm.Response`.
- Does not touch Chat, Anthropic, Gemini, OpenRouter, or cross-protocol diagnostics.
- Adds one official stream event only; no broad stream rewrite.

Follow-up:

- Next stream fidelity slices should cover missing reasoning_text events and tool/server event families independently.
