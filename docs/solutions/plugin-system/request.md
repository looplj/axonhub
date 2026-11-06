# 请求钩子

[插件系统](README.md)的一个扩展点。插件在统一的 `llm.Request` 上改写，一次实现对所有入口协议生效。当前交付只有 `onRequest` 和 `onAttempt`；改写响应的 `onResponse`、`onStreamChunk` 见 [response.md](response.md)，不在当前交付。manifest 的 `hooks` 字段见 [manifest.md](manifest.md)。数值限制见 [运行限制](README.md#运行限制)。任务见 [任务表](README.md#9-任务管理)。

## 1. 背景

请求中间件在 `internal/server/orchestrator/orchestrator.go` 279–357 行写死。按租户改写请求、注入参数都要改这条链。

`NewChatCompletionOrchestrator` 的固定前两项是 `cc.SystemCacheCompatibility()` 和 `stream.EnsureUsage()`。其后是 billing、`enforce-quota`、模型权限、模型映射、选路。`OnOutboundLlmRequest` 在 `orchestrator/outbound.go` 里、provider transformer 序列化之前调用，只有实现了 `OutboundLlmRequestMiddleware` 的中间件会进入这个列表。

## 2. 目标

1. JS 插件可声明 `onRequest`、`onAttempt`。
2. 钩子抛错、超时、返回非法值（含 Promise）时，请求按钩子前的值继续，响应状态码不变。
3. 取不到 VM 时返回 429 `plugin_overloaded`，不跳过插件，不发上游，不影响渠道健康统计和插件熔断。
4. 没有声明请求钩子的插件时，`Process` 的中间件数量和分配次数与现在相同。

### 非目标

- 用请求钩子实现新的 channel provider 或路由策略。自定义 Provider 见 [provider.md](provider.md)。
- 用钩子改 `stream`。客户端要的响应形态不能被插件换掉。
- 改写响应和流式 chunk。见 [response.md](response.md)。
- 宿主按 project、API Key、模型收窄钩子。见 [scope.md](scope.md)。

## 3. User Stories

1. 作为 API 使用方，我希望没装插件或插件出错时，响应和现在一样，以便插件不是新的故障点。
2. 作为插件作者，我希望 `onRequest` 改了 model 之后仍走原来的模型权限和映射，以便不能绕过 API Key 的模型限制。
3. 作为插件作者，我希望对某次 attempt 的改写不影响下一次重试，以便按渠道改参数时不会污染故障转移。
4. 作为系统 Owner，我希望插件处理不过来时请求被拒绝，而不是绕过插件发给上游，以便依赖插件的规则在高负载下也成立。

## 4. 钩子

脚本的导出形式、返回值语义、`ctx` 和示例见 [plugin.js.md](plugin.js.md)。本节说明钩子在请求链路中的位置及约束。

脚本来自包内的 `plugin.js` 及其 import 闭包，启用或升级加载时编译，按 [资源加载与释放](README.md#资源加载与释放) 保存在内存。每次钩子调用借用已初始化的 VM，池未满且没有空闲 VM 时可按需创建并初始化；调用路径不重新读库、读 UI 临时文件或解压编译。

`plugins.Hooks.Middleware(ctx)` 在 `orchestrator.Process` 组装中间件时调用。当前 Generation 里没有已启用、声明了 `hooks.request` 或 `hooks.attempt` 的插件时返回 nil，不追加。有则返回一个同时实现 `OutboundLlmRequestMiddleware` 的 `pipeline.Middleware`。

插入位置：`processor.Middlewares` 和 billing 中间件之后、`NewPersistentTransformers` 之前（`orchestrator.go` 283 行后）。这样 `onAttempt` 能进入 `outboundLlmRequestMiddlewares`。

钩子对全部请求生效，宿主不按请求收窄。每个钩子的 `ctx.request` 提供 `projectId`（`contexts.GetProjectID`）、`apiKeyId`（`contexts.GetAPIKey`，playground 为 null）、`model`（进入钩子中间件时的 `llm.Request.Model`，即客户端请求的模型）、`apiFormat`。插件按这些字段自行判断，不处理时返回 `undefined`。宿主侧收窄是可选能力，见 [scope.md](scope.md)。

```text
1. OnInboundLlmRequest，每请求一次，早于 enforce-quota、模型权限、模型映射、选路。
   按 priority 调用 onRequest(req, ctx)。
   - 返回对象：反序列化成 llm.Request，作为下一个插件的输入。
   - 修改 stream：非法，整次修改回滚。
   - 修改 model：合法。后面的 check-api-key-model-access 和 apply-model-mapping 按新 model 校验。
   - ctx.reject(status, message)：status 只能是 400、403、429。错误构造与 enforceQuota 返回 403 的方式相同。
2. OnOutboundLlmRequest，每次 attempt。输入是当次请求的副本。
   此时 model 已换成实际模型，渠道的 transform options、reasoning 映射、默认 max_tokens 已应用（outbound.go 741–756 行）。
   onAttempt(req, ctx)，ctx.attempt 含 channel、actual model 和上游 API 格式。
   返回值只作用于这次 attempt。修改 model 或 stream 视为非法，丢弃本次全部请求修改，后续插件收到该次调用前的值。
3. 每次钩子调用借一个 VM，返回即归还。
   同一请求内的钩子用 ctx.state 共享数据。ctx.state 由宿主按请求、按插件保存，请求结束释放。
```

一次请求从 `Process` 开始固定使用当时的 Generation，重试中途不换钩子列表。

attempt 隔离要求：宿主保存请求钩子及后续公共入站处理完成后的请求基线；每次尝试都在应用当前渠道变换和调用 onAttempt 之前，从基线建立独立的可变副本。同渠道重试或切换渠道必须重新建立副本，不能从上一次 attempt 的结果继续修改。隔离覆盖嵌套 slice、map 和指针指向的可变值；浅拷贝不足以满足要求。具体复制方式、内部字段保留及写回映射按用户决定留到 T-13 实施时明确，本节先固定“尝试之间不互相污染”的可观察结果。

失败：抛错、超时、非法返回、输入超过 2 MiB，都使用钩子前的值继续，并计一次失败。不改变响应状态码。熔断期间该插件的钩子直接跳过，不借 VM。

### VM 不足

取不到 VM 时拒绝请求，不跳过插件。插件可能负责脱敏、拦截这类规则，高负载时悄悄放行会让规则失效。

```text
1. 取 VM 最多等 100 ms，不计入 200 ms 的调用超时。等不到就是过载。
2. onRequest 过载：OnInboundLlmRequest 返回 llm.ResponseError
   {status 429, code plugin_overloaded, type rate_limit_error, message "plugin <id> is at capacity; please retry shortly"}。
   与 ctx.reject、enforceQuota 走同一条错误路径。请求不进选路，不发上游。
3. onAttempt 过载：OnOutboundLlmRequest 返回 PluginOverloadedError，它包装第 2 步的 429。
   - PersistentOutboundTransformer.TransformRequest 在 outboundLlmRequestMiddlewares 循环里（outbound.go 757–763 行）识别这个错误，
     设置 state.PluginOverloaded = true。
   - CanRetry 对它返回 false，HasMoreChannels 在 PluginOverloaded 时返回 false。
     pipeline 的重试循环因此既不做同渠道重试，也不切渠道，直接返回这个错误。不需要改 llm/。
     不切渠道的原因：VM 池按插件共享，换渠道也取不到 VM，切换只会消耗候选并发出错误的 request.failover。
   - 错误发生在 TransformRequest 里，不经过 OnOutboundRawError，渠道冷却、错误统计、模型熔断都不受影响。
   - 客户端收到和第 2 步相同的 429。
4. 两种情况都：
   - 不计入插件失败次数，不触发熔断。过载是容量不够，不是插件出错。
   - 运行状态里的 overloaded 加一，记 Warn 日志：plugin_id、hook、request_id。
```

用 429 而不用 503，沿用 `ChannelQueueError`（`channel_queue_error.go`）和本地 RPM（`rate_limit_admission_error.go`）的做法：本地容量不足返回 429，客户端 SDK 按退避重试，前置负载均衡也不会把它当成实例故障。

### 与渠道配置的先后

两个钩子都改的是 `llm.Request`。序列化之后，`OnOutboundRawRequest` 阶段还有两类渠道配置会改上游请求，顺序见 `orchestrator.go` 304–325 行：

1. **pass-through body**：渠道开启 pass-through 时，用客户端原始 body 替换序列化结果。插件改写在这类渠道上的处理见 [§5](#5-pass-through-渠道)。
2. **override body / headers**：`applyOverrideRequestBody`、`applyOverrideRequestHeaders` 在 pass-through 之后执行。插件和渠道 override 改了同一个字段时，以渠道配置为准。

## 5. pass-through 渠道

### 现状

渠道开启 pass-through，且 `allowPassThroughBody` 和 `passThroughBodySupported` 都通过时，`applyPassThroughRequestBody`（`pass_through.go` 215 行）用客户端的原始 body 替换序列化结果，再用 sjson 合并 model 和 prompt protection 的改动（241–245 行）。multipart 请求不走 pass-through。

所以在这类渠道上，`onRequest` 改的 model 经过模型权限和映射后照常到达上游；`onRequest`、`onAttempt` 对参数、消息、工具等其他字段的改写都会丢失。

Owner 开 pass-through，是为了保留 `llm.Request` 表达不了的上游字段。插件改写和这个目的冲突时，需要定一个优先级。

### 方案对比

| 方案 | 做法 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| A | 保持现有规则：除 model 外的插件改写丢弃。宿主记录被丢弃的插件，记 Warn 日志并计数 | 不改变 pass-through 语义；不增加序列化；改动集中在一个回调 | 插件在这类渠道上不能改参数和消息 | 采用 |
| B | 钩子前后各序列化一次，按顶层字段求差，用 sjson 把差异写回原始 body | 插件不用改，改写大多能生效 | 每次 attempt 多一次 `TransformRequest`；顶层字段整体覆盖，会把原始 body 里 `llm.Request` 表达不了的嵌套字段一起冲掉，正好破坏 pass-through 要保留的东西 | 放弃 |
| C | `ctx.attempt.passThrough` 告诉插件本次走 pass-through；`onAttempt` 可返回 `patch` / `remove`（上游格式的 JSON 路径），宿主用 sjson 应用到原始 body | 改动精确，不丢字段，合并方式与 prompt protection 一致 | 新增一套对外契约；插件要按上游格式写路径，并处理不同 API 格式；目前没有明确需求 | 暂缓，作为后续方向 |
| D | 插件改了 model 以外的字段时，本次 attempt 不走 pass-through，改用序列化结果 | 插件改写全部生效 | 丢掉 Owner 依赖 pass-through 保留的字段；装一个插件就能悄悄改变渠道配置的语义 | 放弃 |

### 决策

采用 A。

1. pass-through 是渠道 Owner 的显式配置，插件不应在 Owner 不知情时削弱它。
2. A 不增加热路径上的序列化，也不新增插件 API，当前交付范围最小。
3. A 不排斥 C。以后有需求时，C 只在 `ctx.attempt` 和 `onAttempt` 返回值上增加字段，A 的计数和日志继续保留。

当前交付不提供 `ctx.attempt.passThrough`，插件无法事先知道改写会被丢弃，只能从运行状态和日志看到。

### 处理流程

```text
1. 钩子中间件比较每个插件的返回值和输入（去掉 model）。
   不同则把插件记入改写列表：onRequest 的记录作用于整次请求，onAttempt 的记录只作用于当次 attempt。
2. applyPassThroughRequestBody 设置 PassThroughApplied = true 后，
   调用 RequestHookProvider.PassThroughApplied(ctx, channelID)。
3. 对改写列表里的每个插件：
   - 记 Warn 日志：plugin_id、channel_id、request_id。
   - 运行状态里的 overriddenByPassThrough 加一，同一 attempt 内同一插件最多加一。管理页显示该计数。
4. 请求本身不变：上游收到原始 body 加 model、prompt protection 的合并结果，响应状态码不受影响。
```

开发者文档写明：pass-through 渠道上只有 model 的改写生效，看到 `overriddenByPassThrough` 增长时，让 Owner 关闭该渠道的 pass-through；固定值的改写也可以改用渠道 override body。

## 6. 改动清单

### `internal/server/orchestrator/`
- `plugins.go`：`PluginIntegration{Hooks RequestHookProvider; Events *events.Bus}`、`RequestHookProvider`（`Middleware(ctx) pipeline.Middleware`、`PassThroughApplied(ctx, channelID int)`）。`Events` 的发布点在 [events.md](events.md)。
- `orchestrator.go`：`NewChatCompletionOrchestrator` 末尾增加参数 `plugins PluginIntegration`。283 行后插入钩子中间件。
- `plugin_overload_error.go`：新增 `PluginOverloadedError`，`Unwrap` 返回 429 的 `*llm.ResponseError`。写法参照 `channel_queue_error.go`。
- `outbound.go`：`TransformRequest` 遇到 `PluginOverloadedError` 时设置 `state.PluginOverloaded`；`CanRetry` 对它返回 false；`HasMoreChannels` 在 `PluginOverloaded` 时返回 false。
- `pass_through.go`：`applyPassThroughRequestBody` 增加 `RequestHookProvider` 参数，设置 `PassThroughApplied = true` 后调用 `PassThroughApplied`。参数为 nil 时跳过。
- `internal/server/plugin/hooks.go`：实现 `RequestHookProvider`。取 VM 失败时按 [VM 不足](#vm-不足) 返回错误、计 `overloaded`。插件返回新对象时，比较返回值和输入去掉 `model` 后是否相同，不同就记入改写列表；`PassThroughApplied` 按 [§5 处理流程](#处理流程) 记日志和计数。

### `internal/server/api/*.go`
- `openai.go`、`anthropic.go`、`gemini.go`、`aisdk.go`、`jina.go`、`doubao.go`、`playground.go`、`typesafe.go` 的 Params 增加 `Plugins`。生产代码里 23 处 `NewChatCompletionOrchestrator` 调用补上这个参数（openai 15、jina 2，其余各 1），另有 6 个测试文件里的 8 处一并更新。事件发布也用同一个参数，不要为事件再加一次构造函数参数。

## 7. 验收

| 类别 | 标准 | 验证方式 |
|------|------|----------|
| 对外兼容 | 钩子抛错、`while(true)` 超时、返回非法值时，OpenAI 和 Anthropic 入口的响应与无插件时一致 | orchestrator 集成测试，httptest 上游，覆盖流式和非流式 |
| 功能 | `onRequest` 修改 model 后，没有该模型权限的 API Key 被拒绝 | 同上，断言 403 |
| 功能 | `onAttempt` 对渠道 A 的改写不出现在切到渠道 B 的请求里；同渠道重试也从基线开始，嵌套消息、工具参数等可变对象不残留上次修改 | 同上，两个上游各记录收到的 body，增加同渠道重试用例 |
| 功能 | `onAttempt` 返回对象修改 model 或 stream：该次请求修改全部丢弃，后续插件与上游收到调用前的值；客户端流式/非流式响应形态保持原请求约定 | orchestrator 契约测试，覆盖流式和非流式 |
| 功能 | 开启 pass-through 的渠道：上游收到客户端原始 body，`onRequest` 改的 model 生效、其他字段不生效，改了其他字段的插件 `overriddenByPassThrough` 加一并有 Warn 日志，只改 model 的插件不计数 | 同上，pass-through 渠道加 httptest 上游 |
| 功能 | 插件和渠道 override 改同一字段时，上游收到渠道 override 的值 | 同上 |
| 错误处理 | 插件 VM 全部占用时：`onRequest` 声明的插件让请求返回 429 `plugin_overloaded`，上游没收到请求；`onAttempt` 过载时不切渠道，第二个上游也没收到请求；插件失败次数不变，`overloaded` 加一，渠道没有冷却和错误记录 | orchestrator 集成测试，用阻塞的插件占满 16 个 VM |
| 性能 | 无请求钩子插件时，中间件数量和分配次数与改动前一致 | `go test -bench Orchestrator ./internal/server/orchestrator/` |
| 性能 | 100 KiB 请求体上，改一个字段的 `onRequest` ≤ 5 ms（开发机） | `go test -bench Hook ./internal/server/plugin/` |

启用钩子后 24 小时内看请求延迟。p95 增加超过上面的单次预算时，禁用插件，或让插件按 `ctx.request` 尽早返回。

## 8. 风险与妥协

| 风险 | 触发条件 | 影响 | 对策 |
|------|----------|------|------|
| 钩子拖慢热路径 | 插件逻辑重或请求体大 | 延迟上升 | 超时、2 MiB 上限、VM 池、熔断。只服务部分请求的插件可启用 [scope.md](scope.md) 的宿主侧收窄 |
| 插件改写被渠道配置覆盖 | 渠道开启 pass-through，或 override 改了同一字段 | 插件以为生效，上游实际没收到 | 按 [§5](#5-pass-through-渠道) 计数并记 Warn 日志；开发者文档写明先后规则 |
| 慢插件把请求挡成 429 | 插件单次调用接近 200 ms，且请求并发高 | 每插件每实例 16 个 VM，单次 200 ms 时约 80 次每秒就会过载，之后的请求收到 429 | 超时计入熔断，连续 20 次超时后插件暂停 5 分钟，钩子跳过；没超时但持续过载时，管理页显示 `overloaded` 计数，Owner 禁用插件、优化插件或加实例。只服务部分请求的插件启用 [scope.md](scope.md) |

```text
1. 妥协：当前交付不能改写响应
   - 原因：流式 chunk 逐个进 JS 的调用次数和延迟需要单独设计，不阻塞请求钩子
   - 影响：需要改写或过滤响应的插件暂时只能做请求侧和事件侧
   - 升级触发：出现明确的响应改写需求
   - 后续方向：按 response.md 开放 onResponse 和按 chunk 类型过滤的 onStreamChunk

2. 妥协：pass-through 渠道上只有 model 的改写生效
   - 原因：pass-through 回放客户端原始 body，只合并 model 和 prompt protection
   - 影响：在这类渠道上，插件对参数、消息的改写不会到达上游
   - 升级触发：需要在 pass-through 渠道上改写参数
   - 后续方向：§5 方案 C，提供 `ctx.attempt.passThrough`，允许 `onAttempt` 返回 `patch` / `remove`，用 sjson 合并进原始 body
```
