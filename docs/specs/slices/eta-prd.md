# η 切片 PRD — namespace 工具组往返闭合(D1/#1a,P0 压轴)

> 唯一原子:η-1 · #1a namespace 往返(非流式 + 流式)。这是 23 项中最后 1 个待修。

## η-1 · #1a — namespace 工具组声明压扁 + 往返断裂(❌ CRITICAL P0)

- **Problem**:responses 客户端声明 namespace 工具组(如 `mcp__node_repl` 内含 function `leaf`)。入站 `convertToolsToLLM`(inbound.go:841-863)把每个子功能展开为 `llm.Function{Name=grp+"__"+leaf}`,**分组身份从不记录**。出站装配 function_call 时写 `Namespace: tc.Function.Namespace`(恒为空,因声明期从未写入)。结果:模型返回 `name="mcp__node_repl__leaf"` → 客户端收 `function_call.name="mcp__node_repl__leaf", namespace=""`,无法还原分组。作者此前删除了唯一还原机制 `splitNamespaceAndName`(且字符串切分本身错——组名含 `__` 不能切)。

- **grill 证据(MCP 图谱 + 实时源码坐实)**:
  - **声明侧**:`inbound.go:841-863` case "namespace" 展开 `tool.Name+"__"+subTool.Name`,无任何 grp→leaf 映射记录。repo-wide rg 按 "__" 拆分零命中。
  - **非流式响应侧**:`convertToResponsesAPIResponse`(inbound.go:957)→ function_call Item 装配(inbound.go:~1054)写 `Name: toolCall.Function.Name, Namespace: toolCall.Function.Namespace`(恒空)。有 `chatResp.TransformerMetadata` 可用。
  - **出站请求侧**:`convertAssistantMessage`(outbound_convert.go:195)→ function_call Item(outbound_convert.go:237-241)写 `Namespace: tc.Function.Namespace`(恒空)。**只收 msg 不收 metadata**——需穿透签名链。MCP trace 确认调用链:`TransformRequest`/`transformCompactRequest` → `convertInputFromMessages` → `convertAssistantMessage`。
  - **流式响应侧**:`initToolCall`(inbound_stream.go:601)→ function_call Item(inbound_stream.go:~664)写 `Namespace: tc.Function.Namespace`(恒空)。有 `s.transformerMetadata` 可用。
  - **流式 metadata 命脉(关键)**:outbound stream state 的 `transformerMetadata` 初始化为空(outbound_stream.go:84),**不包含请求的 namespace map**。pipeline 框架不自动传播请求 TransformerMetadata 到流式 chunk。但 inbound stream 的 `mergeTransformerMetadata`(inbound_stream.go:202)在 `handleToolCalls`→`initToolCall` **之前**执行——故只要 outbound 把 namespace map 放到 chunk 上,inbound 先 merge 再查表即可命中。
  - **canonical 红线**:`llm.Function`(tools.go:44-52)无 Namespace 字段——不加槽,不改架构。namespace 经 TransformerMetadata 往返。

- **Solution**(查表还原,禁字符串切分;守红线):
  1. **入站建表**:新增 metadata key `responsesNamespaceToolMapTransformerMetadataKey = "openai_responses_namespace_tool_map"`(仿 responses 包范式)。`convertToolsToLLM` 增加 `metadata map[string]any` 参数;case "namespace" 展开时把 `compositeName → {Leaf, Namespace}` 存入 `metadata[key]`(类型 `map[string]namespaceToolEntry`)。`llm.Function` 维持无 Namespace。
  2. **查表 helper**:`resolveNamespaceFromMetadata(metadata, compositeName) → (name, namespace)`:命中返回 leaf+namespace,未命中返回原名+空。
  3. **非流式响应侧**:`convertToResponsesAPIResponse` function_call 装配处查 `chatResp.TransformerMetadata`。
  4. **出站请求侧**:`convertInputFromMessages` 增加 `metadata` 参数穿透 → `convertAssistantMessage` 增加 `metadata` 参数查表。`TransformRequest` 传 `llmReq.TransformerMetadata`;`transformCompactRequest` 传 nil(compact 无 namespace 工具声明)。
  5. **流式 outbound 传播**:outbound `TransformStream` 从 `req.TransformerMetadata` 取 namespace map 存入 state;`transformStreamChunk` 在首个事件(response.created)把它放到 `resp.TransformerMetadata`。
  6. **流式 inbound 查表**:`mergeTransformerMetadata` 增加namespace map key 透传(直接拷贝,非 append);`initToolCall` 查 `s.transformerMetadata` 还原。

- **测试设计(TDD 垂直切片)**:
  1. namespace 往返(非流式):声明 `mcp__node_repl`{leaf:run} → 入站建表 → 模型回 `mcp__node_repl__run` → `convertToResponsesAPIResponse` 出 `Item{name:run, namespace:mcp__node_repl}`。
  2. 组名含 `__` 边界:`mcp__node_repl__run` 不可被切分,必须查表命中(非字符串拆)。
  3. 扁平工具(非 namespace)不在表里 → 出站保持原名、namespace 空(无回归)。
  4. 出站请求侧:llm assistant message 含 ToolCall(Function.Name="grp__leaf") → `convertAssistantMessage` 出 `Item{name:leaf, namespace:grp}`。
  5. 流式:outbound stream chunk 携带 namespace map → inbound `initToolCall` 还原 name/namespace。

- **Out-of-scope**:
  - canonical `llm.Function` 加 Namespace 字段(红线,排除)。
  - 字符串切分还原(组名含 `__`,排除)。
  - compact 路径(namespace 工具声明只在标准 inbound,compact 无 tools 声明;传 nil 即可)。
  - 跨协议(chat/anthropic 客户端 → responses 上游)的 namespace 还原(chat/anthropic 无 namespace 工具声明,无表可查,保持原名合规)。

- **状态: ✅ 实现完成·TDD 全绿·待验收。

---

## 切片进度
| 原子 | 状态 | 验收代理 |
|---|---|---|
| η-1 #1a | ✅ 已完成·待验收 | 待定 |
