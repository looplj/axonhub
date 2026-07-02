---
alwaysApply: false
globs: "docs/specs/**/*.md"
---

# Spec Audit Method Rules

> 适用范围：三协议(chat_completions / anthropic_messages / openai_responses)字段命运总表的逐格审计。唯一目标——零误判。

## MCP 工具三层可信度

1. `get_code_snippet` 从磁盘现读真实源码，可作为定论依据。下任何结论都必须用它读到赋值右值或 return 行。
2. `search_graph` / `query_graph` / `trace_path` 查的是知识图谱快照，会过期；仅用于定位符号全名与大致调用关系，绝不能拿其 caller/callee 边直接下影响分析结论。
3. `search_code` 底层 grep 实时扫盘（命中的文件位置可信），但其上叠加的图谱 enrich 可能夹带噪音，附带的“属于哪个函数／有多重要”类解读打折扣看。

## 防误判四判据

1. 判某字段是否丢失，只认 canonical 结构体上的赋值行；不接受“grep 命中数为 0”当作缺失证据——字段整体搬运(`a.X=b.X`)根本不会产生值的字面量。
2. 凡是由 switch 的 case 标签或函数名顺推出来的行为判断，强制跳进去读它的 return 兜底。命名是最强的思维钩子，也是最稳的坑(例：`convertUserMessage` 实际服务所有非 assistant/tool 角色)。
3. 三协议 × 入站/出站 共六个象限，每向各自取证才算该字段闭环；严禁局部验证外推全局(“入站无损” ≠ “该字段无缺陷”)。
4. 任一格的 ✅/❌ 定性须双证并存：① 源码赋值点(file:line)；② OpenRouter `min.yaml` 中对应 schema 的枚举/类型允许性。缺其一只能标 ⚠️ 待复核。

## 图谱新鲜度

- 被 fix 提交动过的模块(现知 tools/namespace 片区)，图谱关系一律视为失效：对该 project 重跑 `index_repository(mode="fast")`，或 fall back 到 `rg`/`cat` 直读真文件。
- 连文档自身关于“图谱是否滞后”的声明都不可全信，须经实时查询验证后再采信。
