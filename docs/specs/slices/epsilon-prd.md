# ε 切片 PRD — 身份会话缓存

> 原子顺序(由清晰到复杂):#13 user 桥接 → #10 session_id body 变体 → #11 chat·responses 顶层 cache_control

## ε-1 · #13 — user 跨 anthropic 边界桥接缺失(🟠 P1 身份泄漏)
- **Problem**:master 表第13行:chat/responses 顶层 `user`(string)经 canonical `User *string` 往返闭合;anthropic 无顶层 `user`(native),身份仅经 `metadata.user_id`→`chatReq.Metadata['user_id']`(**不走 canonical.User 槽**)。**跨格式桥接缺口**:canonical.User 与 Metadata['user_id'] 互不拷贝——chat/responses 发来的 user 跨到 anthropic 上游会被丢(anthropic 出站只读 Metadata.user_id),反之 anthropic 发来的 user_id 跨到 chat/responses 上游 canonical.User 不被设置。
- **grill 证据(已坐实)**:
  - `anthropic/inbound_convert.go:73-75`:`chatReq.Metadata['user_id']=anthropicReq.Metadata.UserID`,**未设 chatReq.User**。
  - `anthropic/outbound_convert.go:141-144`:仅读 `chatReq.Metadata['user_id']`,**未读 chatReq.User**。
  - chat/responses 入出站均读写 canonical.User(inbound_convert.go:57 / outbound_convert.go:29 / responses/inbound.go:181 / responses/outbound.go:258),故 chat↔responses 已闭合;缺口仅在 anthropic 边界两端。
- **Solution**(守红线:不加 canonical 新顶层槽,单向兜底桥接):
  - anthropic 入站:设 `Metadata['user_id']` 后,同步 `chatReq.User = lo.ToPtr(UserID)`(UserID!=""时)。anthropic-native metadata 保留(同协议往返),canonical.User 桥接供跨格式路由。
  - anthropic 出站:构建 AnthropicMetadata 时,优先 `Metadata['user_id']`(anthropic-native 权威);为空则回退 `chatReq.User`。这样 chat/responses→anthropic 身份不再丢。
- **测试设计**:镜像 top_k 范式。(1)anthropic 入站发 metadata.user_id→断言 canonical.User 捕获 + Metadata['user_id'] 保留;(2)anthropic 出站仅 canonical.User(无 Metadata)→断言 AnthropicMetadata.UserID 还原;(3)chat→anthropic 跨格式:chat 入站 user→canonical.User→anthropic 出站还原 metadata.user_id;(4)缺省守卫:无 user 无 metadata→无 AnthropicMetadata。
- **状态**:grill 完成·待实现(TDD)。

## ε-2 · #10 — session_id body 变体(待 grill)
- **Problem**:master 表待复核:responses body 内 session_id 透传变体。
- **状态**:待 grill。

## ε-3 · #11 — chat·responses 顶层 cache_control(待 grill)
- **Problem**:master 表待复核:chat/responses 顶层 cache_control 与 anthropic cache_control 桥接。
- **状态**:待 grill。

---

## 切片进度
| 原子 | 状态 | 验收代理 |
|---|---|---|
| ε-1 #13 | ✅ 已完成·已验收 | Curie |
| ε-2 #10 | 待 grill | — |
| ε-3 #11 | 待 grill | — |
