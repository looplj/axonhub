# 切片 β · PRD(Anthropic 工具链)

## β-1 · #3+#1e — parallel_tool_calls ↔ disable_parallel_tool_use 极性映射
- **Problem**:canonical `ParallelToolCalls`(true=允许)与 Anthropic `ToolChoice.DisableParallelToolUse`(true=禁用,且是 tool_choice 子字段非顶层)语义反相;修复前 anthropic 转换器对两者零读写,跨格式并行控制丢失。
- **Solution**:出站 `convertToAnthropicRequestWithConfig` 在 Tools 非空且 ParallelToolCalls 非空时,合成/补 ToolChoice(缺则 {Type:"auto"}),Type!="none" 时注入 `DisableParallelToolUse=lo.ToPtr(!*ParallelToolCalls)`;入站 `convertToLLMRequest` 读 `ToolChoice.DisableParallelToolUse` 反相写 `ParallelToolCalls`。
- **Testing**:出站三态极性(explicit disable/allow/unset)+ tool_choice=none 跳过注入守卫;入站三态极性(disable true/false/absent)。seam = `TransformRequest`(既有)。
- **Out of scope**:tool_choice=none 时的并行语义(Anthropic 本就不允许 disable_parallel_tool_use 与 none 同存);chat/responses 侧既有 ParallelToolCalls 逻辑不动。
- **diagnose 跳过理由**:红→绿一次过,六项验收 APPROVED,无残留。
- **状态**:✅ 已完成·同模验收 Banach APPROVED,commit `2a06d2a9`,fix-tracker 已归档。

## β-2 · #1c-D11 — 非流式/history custom_tool_call namespace
- **Problem**:非流式与 history 重建路径的 custom_tool_call 漏填 namespace(约4处,镜像 D12 但在 responses 非流侧),跨格式往返 namespace 丢失。
- **Solution**:待 grill 用 MCP 定位具体站点后补 `.Namespace`(源值同 D12 模式)。
- **Testing**:待定,镜像 D12 红绿但走非流 `TransformRequest` seam。
- **状态**:✅ 已完成·同模验收 Archimedes APPROVED(7 标准,含范围完整性扫描,11 构造点无残留),待 commit。

## β-3 · #1b — builtin 工具静默丢
- **Problem**:Anthropic 转换器只建模 function+web_search,其余 Anthropic 原生工具(bash_20250124/text_editor_20250124/image_generation 等)双向静默丢、无告警:入站 `convertToolToLLM`(`inbound_convert.go:770`),`default:`→`return llm.Tool{},false`;出站 `convertToolsAnthropic`(`outbound_convert.go:265+`),`default:`→`continue`。作者有意(注释明示),但违 Anthropic spec(定义了这些工具)——规范可见性缺口,非偶发 bug。
- **Solution**:两处 `default:` 加 slog 告警使丢工具可见;不动 canonical(守 D1 红线),不强行映射。warn 机制 TDD 定(包用 slog;函数现无 ctx,优先 slog.Warn 或调用方收集丢弃项告警,避免改签名)。
- **Testing**:红:含 bash_/text_editor_ 的 anthropic 请求,断言丢工具有 warn;绿:补 warn 后通过。
- **Out of scope**:强行映射到 canonical(违红线);Responses 侧 RawFragments 透传(另切片);web_search 在非 Anthropic 平台丢弃(作者有意、平台不支持)。
- **diagnose**:不跳过——作者有意但违 spec 可见性,需补;TDD 复现"静默丢无 warn"。
- **状态**:⏭ 不修(按作者设计)。用户确认:服务商原生工具(builtin)无需透传,沿用作者仅 function+web_search 的设计;非 bug,关闭。
