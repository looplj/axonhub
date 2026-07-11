# G4 Slice Ledger

## Workflow rule
- Each G is split into ~5-minute vertical slices.
- Each slice: TDD -> implement -> verify -> self-review.
- After all slices in a G pass self-review, run a real multi-axis sub-agent review.
- Only if module review PASSes may the next G begin.

## G4 slices

| Slice | Outcome | Evidence | Self-review | Status |
|---|---|---|---|---|
| S1 top-level `audio` raw preserve | Chat->Chat re-emits original `audio` object including unknown nested fields | fixture `openai-audio-prediction-moderation.request.json`; test `TestOpenAIChatRequestOutputControlsRawRoundTrip`; helper `openAIChatRawPreserveFields` | pass | completed |
| S2 top-level `prediction` raw preserve | Chat->Chat re-emits original `prediction` including unknown nested fields | same fixture/test | pass | completed |
| S3 top-level `moderation` raw preserve | Chat->Chat re-emits original `moderation` including unknown nested fields | same fixture/test | pass | completed |
| S4 cross-protocol unsupported/lossy | Responses does not synthesize fields; Anthropic records lossy diagnostics for audio/prediction/moderation | `TestOpenAIChatRequestOutputControlsNotSynthesizedForResponses`; `TestOutboundTransformer_TransformRequest_DiagnosesChatOutputControlsLoss` | pass | completed |
| S5 package verification | openai + anthropic package tests green | `go test ./transformer/openai -count=1`; `go test ./transformer/anthropic -count=1` | pass | completed |

## Module review gate
- pending real sub-agent review
- commit under review: `9a2692ed`
