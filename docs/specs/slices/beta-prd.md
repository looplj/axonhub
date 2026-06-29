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
- **状态**:⏳ 待开始。

## β-3 · #1b — builtin 工具静默丢
- **Problem**:file_search/mcp/computer_use/bash_/text_editor_ 等原生工具在 anthropic 出站被静默过滤(作者 outbound_convert.go:227-228 注释已表态仅留 web_search),无告警致客户端不知情功能缺失。
- **Solution**:倾向 warn(日志或响应 metadata 标记)而非强行映射;待 grill 确认作者过滤点与既有 warn 机制后定。
- **Testing**:待定。
- **状态**:⏳ 待开始。
