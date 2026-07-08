# Module 2 review-fix result

Date: 2026-07-07
Implementation worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
Branch: `codex/responses-top-level-preservation-clean`

## Review findings addressed

Module 2 review returned no P0/P1 blockers and these P2 findings:

1. `outbound_stream.go` central switch is acceptable for current diff but should get a small internal seam before adding more stream event families.
2. `response_extensions.go` should document the invariant for Responses-native response metadata keys.
3. `AnnotationIndex` is parsed but not currently used by outbound conversion; comment should explain current ordering behavior.
4. `response.output_text.annotation.added` test fixture used nested `url_citation`; official wire shape can be flat `url` / `title`, and existing `Annotation.UnmarshalJSON` supports that shape. Test should cover the real flat shape.

## Fixes

- Extracted small internal helpers in `outbound_stream.go`:
  - `applyReasoningDelta`
  - `applyOutputTextAnnotation`
  - `applyRefusalDelta`
- Added invariant comments to `response_extensions.go`: only Responses-native response fields without common `llm.Response` equivalents belong in those metadata keys; request native data belongs in `ProviderExtensions`.
- Added `AnnotationIndex` comment in `stream_event.go`: parsed for wire fidelity; outbound conversion preserves event order and emits annotation payload.
- Updated `TestOutboundTransformer_TransformStream_PreservesOutputTextAnnotationAdded` to use flat `url` / `title` annotation shape.

## Verification

Commands:

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai/responses -run 'TestOutboundTransformer_TransformStream_Preserves(OutputTextAnnotationAdded|ReasoningTextDelta|RefusalDelta)$' -count=1

go test ./transformer/openai/responses -count=1

cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean
git diff --check
```

Result:

- Focused stream tests passed.
- Responses package tests passed.
- `git diff --check` passed.
