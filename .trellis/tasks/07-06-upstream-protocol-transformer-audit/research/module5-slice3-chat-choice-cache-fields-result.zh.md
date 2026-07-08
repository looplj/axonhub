# Module 5 Slice 5.3：OpenAI Chat n / prompt_cache_retention 保真结果

## 目标

补齐 OpenAI Chat Completions 官方顶层请求字段中当前缺失的两个字段：

- `n`
- `prompt_cache_retention`

要求：

- 入站 OpenAI Chat 请求转换到 `llm.Request` 时不丢失。
- 出站仅在 `PlatformOpenAI` same-protocol 路径回放。
- 不恢复/启用共享 `llm.Request.N`，因为当前共享模型中 `N` 被明确注释为 `NOTE: Not supported, always 1`。
- `prompt_cache_retention` 需要保留显式 `null`，不能用 `*string` 导致 null 与缺失混淆。

## 实现路径

沿用 Module 5 的 Chat 原生扩展 seam：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/model.go`
  - `Request.N json.RawMessage`
  - `Request.PromptCacheRetention json.RawMessage`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/chat_extensions.go`
  - 入站保存 `n` / `prompt_cache_retention`
  - 出站回放 `n` / `prompt_cache_retention`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/inbound_test.go`
  - 入站保真测试
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/outbound_test.go`
  - OpenAI-only 出站策略测试

## TDD 证据

先添加红测，确认当前行为确实丢字段：

```text
TestInboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFields
失败：ProviderExtensions.OpenAIChat 为 nil / fields 缺失

TestOutboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFieldsForOpenAIOnly
失败：openAIBody["n"] / ["prompt_cache_retention"] 为空
```

实现后目标测试通过：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm
go test ./transformer/openai -run 'Test(InboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFields|OutboundTransformer_TransformRequest_PreservesOpenAIChatChoiceAndPromptCacheFieldsForOpenAIOnly)$' -count=1
```

结果：

```text
ok  github.com/looplj/axonhub/llm/transformer/openai
```

包级验证通过：

```bash
go test ./transformer/openai -count=1
```

结果：

```text
ok  github.com/looplj/axonhub/llm/transformer/openai
```

差异格式检查通过：

```bash
git -C /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean diff --check
```

结果：退出码 0。

## 字段矩阵复查

用已保存的官方 OpenAI 字段文档与当前 `openai.Request` 结构对照：

```text
official_count: 35
model_count: 37
missing_official_in_model: []
extra_model_not_official: [reasoning_budget, reasoning_summary]
```

解释：

- Chat request 官方顶层字段当前已全部存在于 OpenAI Chat native request model。
- `reasoning_budget` / `reasoning_summary` 是项目已有扩展字段，不属于本 slice 新增。

## 自审结论

通过。

审查项：

- 字段归属：`n` 和 `prompt_cache_retention` 是 OpenAI Chat 原生字段，不进入共享 `llm.Request`，保存在 OpenAI Chat 扩展中。
- null 保真：`prompt_cache_retention` 使用 `json.RawMessage`，测试覆盖显式 `null`。
- OpenAI-only 策略：出站测试确认 `PlatformGoogle` 不输出这两个字段。
- 架构一致性：没有修改共享 `RequestFromLLM`，没有扩大所有 OpenAI-compatible provider 的输出面。
- 验证范围：目标测试、OpenAI transformer 包测试、diff check、字段矩阵复查均通过。

## 下一步

Module 5 的 Chat request 顶层字段已补齐。下一步应做 Module 5 模块级审查：

1. 检查当前 Chat 原生扩展 seam 是否仍符合作者架构；
2. 检查是否存在死代码、重复逻辑或过宽回放；
3. 检查官方 Chat request 字段矩阵与当前实现是否一致；
4. 审查通过后再决定进入 Chat response / stream 或下一模块。
