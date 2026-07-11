# G4 Slice Ledger

## Workflow rule
- Each G is split into ~5-minute vertical slices.
- Each slice: TDD -> implement -> verify -> self-review.
- After all slices in a G pass self-review, run a real multi-axis sub-agent review.
- Only if module review PASSes may the next G begin.

## G4 slices

| Slice | Outcome | Evidence | Self-review | Status |
|---|---|---|---|---|
| S1 top-level `audio` raw preserve | Chat->Chat re-emits original `audio` including unknown nested fields | fixture `openai-audio.request.json`; test `TestOpenAIChatRequestOutputControlsRawRoundTrip/audio` | pass | completed |
| S2 top-level `prediction` variants | Chat->Chat re-emits string and content[] prediction variants | fixtures `openai-prediction-string.request.json`, `openai-prediction-parts.request.json` | pass | completed |
| S3 top-level `moderation` raw preserve | Chat->Chat re-emits moderation object including unknown nested fields | fixture `openai-moderation.request.json` | pass | completed |
| S4 cross-protocol unsupported/lossy | Responses does not synthesize fields; Anthropic records field-level lossy diagnostics | `TestOpenAIChatRequestOutputControlsNotSynthesizedForResponses`; `TestOutboundTransformer_TransformRequest_DiagnosesChatOutputControlsLoss` | pass | completed |
| S5 scope cleanup after review FAIL | Removed web_search_options/functions/function_call from G4 preserve/lossy lists | commit `7cd64f9f`; review note `g4-module-review-boyle-fail.md` | pass | completed |
| S6 package verification | openai + anthropic package tests green | `go test ./transformer/openai -count=1`; `go test ./transformer/anthropic -count=1` | pass | completed |

## Module review gate
- first review: FAIL by Boyle `019f5271-70a5-7663-bb34-e2185c7ea405`
- re-review: PASS by Boyle on commits `9a2692ed` + `7cd64f9f`
- report: `research/reviews/g4-module-review-boyle-pass.md`
