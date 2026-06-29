# 切片 α · PRD(暖身)

> 折中纪律:每原子在此写 Problem/Solution/Testing/Out-of-scope/Status,作为 PRD;单点不拆 issues;TDD 红→绿;同模验收 6 标准;绿项跳 diagnose 但显式说明;切片末统一 /handoff。

## α-1 · D12 — 流式 custom_tool_call namespace 往返
- **Problem**:responses→chat-chunk 流式桥接中,`case "custom_tool_call"` 三处构造 `ResponseCustomToolCall{}` 漏填 `.Namespace`,namespace 身份在往返里丢失(CONTEXT.md BridgeAsymmetry/NamespaceQualifier)。
- **Solution**:A/B 点(状态初始化、首 delta emit)取 `item.Namespace`;C 点(增量 delta emit)从状态对象 `tc.ResponseCustomToolCall.Namespace` 取。与同文件 function 分支(:236/:255)既有写法一致。
- **Testing**:新增 `TestOutboundTransformer_TransformStream_CustomToolCallPreservesNamespace` 红绿,断言 output_item.added→input.delta→done 全程 namespace 保持;seam = outbound `TransformStream`(既有 harness)。
- **Out of scope**:非流式/history 路径(归 β-2 #1c-D11);namespace 容器压扁还原(归 η D1/#1a)。
- **diagnose 跳过理由**:红→绿一次过,无残留 bug 需复现诊断。
- **状态**:✅ 已完成·同模验收 APPROVED,commit `98fb0a16`,fix-tracker 已归档。
