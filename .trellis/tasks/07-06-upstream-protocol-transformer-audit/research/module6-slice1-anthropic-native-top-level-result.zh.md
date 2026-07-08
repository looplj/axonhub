# Module 6 Slice 1: Anthropic native top-level request fields

日期：2026-07-07
工作树：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`

## 范围

Anthropic Messages same-protocol request 顶层字段：

- `container`
- `inference_geo`

不包含本轮未处理字段：

- `mcp_servers`
- `tools[].type=mcp_toolset`
- response `container`
- response `stop_details`
- stream event fidelity

## TDD seam

公开边界：

- `anthropic.InboundTransformer.TransformRequest`
- `anthropic.OutboundTransformer.TransformRequest`

新增测试：

- `TestAnthropicSameProtocolRequestPreservesNativeTopLevelFields`
- `TestAnthropicRequestNativeTopLevelFieldsAreDirectOnly`

红测表现：`container` / `inference_geo` 在 same-protocol 出站 body 中缺失。

## 修复

- 在 `MessageRequest` 增加 Anthropic native raw 字段：`Container`、`InferenceGeo`。
- 新增 `ProviderExtensions.Anthropic.Request.RawTopLevelFields`，避免把 Anthropic-only 字段塞进 common `llm.Request`。
- inbound 保存 native raw 字段到 Anthropic provider extension。
- outbound 只对 direct Anthropic same-protocol replay；DeepSeek 等 Anthropic-format adapter 不 replay。

## 验证

已执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm

go test ./transformer/anthropic -run 'TestAnthropicSameProtocolRequestPreservesNativeTopLevelFields|TestAnthropicRequestNativeTopLevelFieldsAreDirectOnly' -count=1

go test ./transformer/anthropic -count=1

git diff --check
```

结果：全部通过。

索引：

```bash
codebase-memory-mcp cli --json index_repository '{"repo_path":"/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean","mode":"fast"}'
```

结果：状态 `ready`，nodes 39775，edges 203175。

## 自审结论

通过本切片自审：

- 未扩大 common `llm.Request`。
- 未扩展 `TransformerMetadata` magic-key bus。
- 协议专属字段归属到 `ProviderExtensions.Anthropic`。
- same-protocol direct Anthropic 保真。
- Anthropic-format 适配器负向保护存在。
- 当前仍未进入 Module 6 大审查/提交，后续还需继续 Anthropic MCP/response 字段切片。
