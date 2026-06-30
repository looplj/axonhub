# ζ 切片 PRD — 流式杂项(F2)

## ζ-1 · F2 — stream_options 跨格式 + convertStreamOptions 早 return(⏭ 复核无 bug·不修)
- **Problem(原 handoff 推测)**:F2 stream_options 双向 usage 闭合 + convertStreamOptions 早 return 守卫。
- **grill 结论(不修)**:对照 OpenRouter 官方 openapi.yaml 实证:
  - `include_obfuscation`:**全 yaml 0 命中**——非 OpenRouter 规范字段,是 AxonHub 自有扩展(流式混淆功能)。responses 内透传正确(F2a ✅,inbound.go:273-276 stash → outbound_convert.go:489-501 convertStreamOptions restore)。chat 无此概念,跨格式至 chat 丢失合规。
  - `include_usage`:仅 `ChatStreamOptions`(chat,openapi.yaml:5235/5241)有;responses 规范无 stream_options(min.yaml ResponsesRequest 13051-13270 无该键)。故 include_usage 是 chat 独有,chat 内往返闭合(inbound_convert.go:113-115 → outbound_convert.go:91-93);responses 无此字段→chat→responses 不可表示,合规。
  - **跨格式无公共字段**:两协议 stream_options 子项不重叠(include_usage=chat 独有,include_obfuscation=AxonHub 扩展),跨格式丢失不可避免且不违规范。
  - **convertStreamOptions 早 return 守卫**:`if includeObfuscation==nil { return nil }`——responses StreamOptions 仅含 IncludeObfuscation,无该值时无可发字段,nil 正确(非 bug)。src==nil 早 return 亦不掩盖 include_obfuscation(responses inbound 设 metadata 时必同时设 canonical.StreamOptions 非空)。
- **状态**:⏭ 复核无 bug·不修(类 #5/#10)。master 表 F2 ⚠️P3、F2a ✅ 与本 grill 一致。

---

## 切片进度
| 原子 | 状态 | 验收代理 |
|---|---|---|
| ζ-1 F2 | ⏭ 复核无 bug·不修 | — |
