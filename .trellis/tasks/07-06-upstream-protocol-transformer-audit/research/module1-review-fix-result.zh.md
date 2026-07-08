# Module 1 review-fix result

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`

## Review findings addressed

The first Module 1 review returned no P0/P1 blockers and these P2 findings:

1. `RawTopLevelFields` conflict branch was not truly tested.
2. `additional_tools` / `defer_loading` were not named in tests.
3. `openAIResponsesOwnedRequestFields` duplicated `responses.Request` JSON tags and could drift.
4. Provider extension field classification could be more self-explanatory.

## Fixes

- Strengthened `TestOutboundTransformer_TransformRequest_RawTopLevelDoesNotOverrideStructuredFields` by manually injecting conflicting `RawTopLevelFields["model"]` and `RawTopLevelFields["input"]`; outbound structured fields still win.
- Extended `TestOutboundTransformer_TransformRequest_ReplaysRawTopLevelFields` to assert `additional_tools` and `defer_loading` same-protocol unknown/profile top-level preservation.
- Replaced the manual owned top-level field map with `openAIResponsesRequestJSONFields()`, derived from `reflect.TypeOf(Request{})` JSON tags.
- Added comments to `OpenAIResponsesRequestExtensions` fields to distinguish named native preservation from unknown/profile top-level fallback.

## Verification

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

Result:

- `go test ./transformer/openai/responses -count=1` passed.
- `git diff --check` passed.
- Sanity check confirmed `openAIResponsesOwnedRequestFields` is derived from `Request` JSON tags and no longer a manual map literal.
