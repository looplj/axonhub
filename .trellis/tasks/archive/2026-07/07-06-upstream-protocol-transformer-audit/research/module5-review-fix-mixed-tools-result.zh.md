# Module 5 review fix: OpenAI Chat mixed tools same-protocol preservation

日期：2026-07-07
工作树：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`

## 问题

Spec 复审指出：OpenAI Chat `tools` 在同协议往返时，如果原始请求包含 function + custom 混合数组，当前实现会把 function 转入 common `llm.Request.Tools`，但因为 outbound 看到 structured `tools` 已存在而不 replay raw `tools`，导致 raw custom tool 丢失。

## TDD seam

公开边界：`openai.OutboundTransformer.TransformRequest` 输出的 OpenAI Chat 请求体。

新增红测：

- `TestOutboundTransformer_TransformRequest_OpenAIChatMixedToolsPreservesRawCustomTools`

红测结果：失败，实际输出只有 function tool，缺少 raw custom tool。

## 修复

保持作者架构和当前 Module 5 seam：

- 不把 Chat custom tool 塞进 common `llm.Tool`。
- 不让 stale raw function 覆盖 common structured function。
- 在 OpenAI same-protocol outbound merge 阶段，把 raw `tools` 中非 function 的 Chat 原生工具元素补回 structured `tools` 数组。
- `tool_choice` 仍沿用已有规则：common structured 已存在时，raw `tool_choice` 不覆盖。

改动点：

- `llm/transformer/openai/chat_extensions.go`
  - `mergeOpenAIChatToolUnionFields` 改为调用 `mergeOpenAIChatTools`。
  - 新增 `mergeOpenAIChatTools` 和 `isOpenAIChatFunctionToolRaw`。
- `llm/transformer/openai/outbound_test.go`
  - 新增 mixed tools 保真测试。
  - 调整旧 stale raw 工具测试，让它真正验证 raw function 不覆盖 common function。

## 验证

已执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm

go test ./transformer/openai -run 'Test(InboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFields|InboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFields|InboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFields|InboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFields|InboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFields|InboundTransformer_TransformRequest_PreservesOpenAIChatStreamOptionsRawFields|OutboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesDeprecatedOpenAIChatFunctionFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesOpenAIChatExplicitNullSamplingFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesOpenAIChatCustomToolsRawFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_PreservesOpenAIChatStreamOptionsRawFieldsForOpenAIOnly|OutboundTransformer_TransformRequest_OpenAIChatRawNonNullSamplingDoesNotOverrideCommonFields|OutboundTransformer_TransformRequest_OpenAIChatRawStreamOptionsDoesNotOverrideCommonIncludeUsage|OutboundTransformer_TransformRequest_OpenAIChatRawToolsDoNotOverrideCommonStructuredTools|OutboundTransformer_TransformRequest_OpenAIChatMixedToolsPreservesRawCustomTools)$' -count=1

go test ./transformer/openai -count=1

git diff --check
```

结果：全部通过。

索引：

```bash
codebase-memory-mcp cli --json index_repository '{"repo_path":"/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean","mode":"fast"}'
```

结果：增量刷新 2 个文件，状态 `ready`，nodes 39763，edges 203290。

## 自审结论

通过本切片自审：

- 未扩大 common `llm.Request` 字段。
- 未引入跨协议假映射。
- same-protocol Chat raw-only 工具得到保真。
- common-owned function tool 仍优先，stale raw function 不覆盖。
- 改动集中在 OpenAI Chat request emission policy seam。
