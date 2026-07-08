# Module 6 Slice 3: Anthropic response native top-level fields

日期：2026-07-07
工作树：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`

## 范围

Anthropic Messages response 顶层字段：

- `container`
- `stop_sequence`
- `stop_details`

## TDD seam

公开边界：

- `anthropic.OutboundTransformer.TransformResponse`
- `anthropic.InboundTransformer.TransformResponse`

新增测试：

- `TestAnthropicSameProtocolResponsePreservesNativeTopLevelFields`

红测表现：`container` / `stop_details` / `stop_sequence` 在 Anthropic -> llm.Response -> Anthropic same-protocol 往返后缺失。

## 修复

- `llm.Response` 增加 `ProviderExtensions *ProviderExtensions`，`json:"-"`，用于 response 私有协议字段，不进入 common JSON。
- `ProviderExtensions.Anthropic.Response.RawTopLevelFields` 保存 Anthropic response native raw 字段。
- `Message` 增加 raw `Container` / `StopDetails` / raw field map；`stop_sequence` 用原始字段恢复到 typed `StopSequence`。
- outbound response 转 common 时捕获 raw fields；inbound response 转 Anthropic 时 replay。

## 验证

已执行：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm

go test ./transformer/anthropic -run 'TestAnthropicSameProtocolRequestPreservesNativeTopLevelFields|TestAnthropicRequestNativeTopLevelFieldsAreDirectOnly|TestAnthropicSameProtocolRequestPreservesMCPConnectorFields|TestAnthropicSameProtocolResponsePreservesNativeTopLevelFields' -count=1

go test ./transformer/anthropic -count=1

git diff --check
```

结果：全部通过。

## 自审结论

通过本切片自审：

- 未使用 `TransformerMetadata` 承载新增 response native 字段。
- response provider extension 与 request provider extension 对称。
- `ProviderExtensions` 不参与 common response JSON 序列化。
- 保留 same-protocol Anthropic native 字段，不做跨协议语义映射。
