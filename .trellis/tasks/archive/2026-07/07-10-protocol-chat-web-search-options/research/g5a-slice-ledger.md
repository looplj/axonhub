# G5a Slice Ledger — Chat web_search_options

## Workflow
TDD per ~5-minute slice; self-review each slice; real module sub-agent review after all slices.

| Slice | Outcome | Evidence | Self-review | Status |
|---|---|---|---|---|
| S1 red fixture/test | proved current drop of web_search_options | failing TestOpenAIChatRequestWebSearchOptionsRawRoundTrip before preserve list change | pass | completed |
| S2 implement raw preserve | Chat->Chat re-emits original JSON including unknown nested | openAIChatRawPreserveFields includes web_search_options; fixture openai-web-search-options.request.json | pass | completed |
| S3 tools[] regression | tools[] unchanged when web_search_options present | same test asserts tools JSONEq | pass | completed |
| S4 cross-protocol | Responses no synth; Anthropic field-level lossy | TestOpenAIChatRequestWebSearchOptionsNotSynthesizedForResponses; DiagnosesChatWebSearchOptionsLoss | pass | completed |
| S5 package verification | openai/anthropic package tests green | go test ./transformer/openai -count=1; go test ./transformer/anthropic -count=1 | pass | completed |

## Module review gate
- review: PASS by Laplace `019f5296-0574-7c01-93d6-bda7880b4dc8`
- report: research/reviews/g5a-module-review-laplace-pass.md
- commit: 6525bb82
