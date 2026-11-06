# 事件

[插件系统](README.md)的一个扩展点。当前阶段 JS 插件从进程内总线上接收下面 8 种新事件。现有 WebHook 的 4 条 `NotifyXxx` 保持原样，不改成订阅者。内置插件见 [builtin-plugins.md](builtin-plugins.md)，未排期。

manifest 的 `events` 字段与权限词表见 [manifest.md](manifest.md)，运行时和数值限制见 [README](README.md#运行限制)。任务见 [任务表](README.md#9-任务管理)，本篇对应 T-08、T-09、T-12。

## 1. 背景

`internal/server/biz/webhook_notifier.go` 只有 4 个事件：`channel.auto_disabled`、`provider.quota.warning|exhausted|recovered`。每个事件一个 `NotifyXxx`，由 `channel_auto_disable.go` 的 `asyncNotifyChannelAutoDisabled` 和 `provider_quota.go` 的 `notifyQuotaStatusChange` 直接调用。订阅粒度只有事件名。配置在 `systems.webhook_notifier_config`。

缺口：`DisableAPIKey` 只有全部凭证都不可用时才发 channel 事件。故障转移只在 `llm/pipeline/pipeline.go` 和 `orchestrator/outbound.go` 的 `NextChannel`（990 行）打 Debug 日志。请求失败、API Key 配额耗尽没有任何事件。触发之后也不能执行禁用渠道这类动作。

## 2. 目标

1. 一次发布下面 8 种新事件，使用统一信封。事件里要点名某一把上游 Key 时，只放指纹和掩码，不放明文。
2. 现有 4 条 WebHook 调用不改。`channel.auto_disabled` 与三条 `provider.quota.*` 仍由 `NotifyXxx` 发送，不在本阶段进入总线。
3. JS 插件用 `onEvent` 订阅这 8 种里声明过的类型，并通过已授予的宿主 API 处置。用户场景：某模型在某渠道 5 分钟内故障转移 10 次时发一次已有 WebHook 目标，并禁用该渠道。

### 非目标

- 声明式规则引擎。阈值用 `ctx.counter`。
- 事件落库、重放。投递是最多一次。
- 管理操作审计表。动作记结构化日志。

## 3. User Stories

1. 作为运维，我希望订阅凭证禁用和故障转移，并按渠道、模型过滤，以便知道哪个上游出了问题。
2. 作为运维，我希望故障转移超过阈值后自动禁用该渠道并通知值班群，以便不用人工盯盘。
3. 作为升级用户，我希望已配置的 WebHook 照常发送。让设置页订阅新事件留到 [builtin-plugins.md](builtin-plugins.md)。
4. 作为插件作者，我希望这 8 种事件用同一份信封，以便 `onEvent` 按类型过滤。

## 4. 决策

| 方案                                                        | 结论                                   |
| ----------------------------------------------------------- | -------------------------------------- |
| 继续为每个事件写 `NotifyXxx`                                | 放弃。每加一个订阅者都要改所有发布点   |
| 进程内总线，订阅者各自有界队列和单 worker，`Publish` 非阻塞 | 采用。没有订阅者时不构造事件           |
| 数据库 outbox                                               | 放弃。`request.*` 每请求写一次，写放大 |

## 5. 事件

`internal/server/events` 不依赖 biz。

```go
type Event struct {
    ID         string
    Type       string
    Severity   string // info / warning / error / critical
    OccurredAt time.Time
    Subject    Subject
    Data       map[string]any
    Origin     string // "" 或 "plugin:<plugin_id>"
    Depth      int
}

type Subject struct {
    Channel    *ChannelRef    // id, name, type, baseUrl, status
    Credential *CredentialRef // fingerprint, masked
    Model      string
    APIKey     *APIKeyRef     // id, name
    Project    *ProjectRef    // id
    Request    *RequestRef    // id
}
```

只有 `channel.credential_disabled`、`channel.credential_enabled`、`request.attempt_failed` 带 `Credential`。`fingerprint = hex(sha256(key))[:12]`，`masked` 复用渠道测试的脱敏规则：长度不超过 8 个字符时为 `****`，否则为前 4 位 + `****` + 后 4 位。OAuth 没有密钥串，`fingerprint` 与 `masked` 都是 `oauth`。宿主 API 按指纹在该渠道当前凭证里反查明文，插件拿不到明文。其余事件不填 `Credential`。

本阶段一次发布这 8 种。manifest 的 `events` 也只接受这 8 个名字。

| 事件                          | 级别    | 发布位置                                                                                                | Subject                                              | Data                                                          |
| ----------------------------- | ------- | ------------------------------------------------------------------------------------------------------- | ---------------------------------------------------- | ------------------------------------------------------------- |
| `channel.auto_enabled`        | info    | `biz/channel_apikey.go` `cleanupExpiredAutoDisabledChannels`                                            | channel                                              | —                                                             |
| `channel.credential_disabled` | warning | `biz/channel_apikey.go` `DisableAPIKey` 写库成功后                                                      | channel, credential                                  | errorCode, reason, expiresAt                                  |
| `channel.credential_enabled`  | info    | `EnableAPIKey` / `EnableAllAPIKeys` / `EnableSelectedAPIKeys` / `cleanupExpiredDisabledAPIKeys`         | channel, credential                                  | trigger（manual / expired）                                   |
| `request.attempt_failed`      | warning | `orchestrator/request_execution.go` `persistRequestExecution` 的 `OnOutboundRawError`                   | channel, credential, model, apiKey, project, request | attempt, statusCode, errorType, errorMessage（截断 512 字节） |
| `request.failover`            | warning | `orchestrator/outbound.go` `NextChannel`：在 `CurrentCandidateIndex++` 之前记下当前渠道，切换成功后发布 | channel（切换前）, model, apiKey, project, request   | toChannelId, toChannelName, attempt                           |
| `request.failed`              | error   | `orchestrator/orchestrator.go` `Process` 错误分支，`UpdateRequestStatusFromError` 之后                  | channel（最后一次）, model, apiKey, project, request | statusCode, errorType, errorMessage, attempts                 |
| `request.completed`           | info    | 非流式 `persistRequest` 的 `OnInboundRawResponse`；流式 `InboundPersistentStream` 结束                  | channel, model, apiKey, project, request             | latencyMs, promptTokens, completionTokens, attempts, stream   |
| `api_key.quota_exceeded`      | warning | `orchestrator/quota.go` `enforceQuota` 拒绝时                                                           | apiKey, project, model, request                      | profile, window                                               |

仍只走现有 WebHook、本阶段不发布：

| 事件                                                 | 现有调用                         |
| ---------------------------------------------------- | -------------------------------- |
| `channel.auto_disabled`                              | `asyncNotifyChannelAutoDisabled` |
| `provider.quota.warning` / `exhausted` / `recovered` | `notifyQuotaStatusChange`        |

Subject 和 Data 交给 JS 时，字段名是 camelCase。`Event` 提供这份映射，发布点填 Go 结构，不各自拼 map。

每个发布点先检查 `Bus.HasSubscribers(事件名)`。没有订阅者就不构造事件对象及其 Subject/Data。`EventBus` 为 nil 时同样跳过。这两项都不跳过原业务操作。

`Bus`：`Publish`、`Subscribe(name, types, queue, handler) (unsubscribe)`、`HasSubscribers`、`Stats`。每个订阅者一个有界队列和一个 worker。FX `OnStop` 关闭 worker。

## 6. 发布

`ChannelService` 和编排请求的 orchestrator 增加可选的 `*events.Bus`。`ProviderQuotaService` 不注入总线：配额的 3 个旧事件仍调用原来的 `NotifyProviderQuota*`。`channel_auto_disable.go` 仍调用 `NotifyChannelAutoDisabled`。

`ctx.webhook.send` 在 T-12 给 `WebhookNotifier` 增加按已有目标名发送的方法。它不改变 `NotifyXxx`，也不读取订阅列表去决定发不发；插件点名目标，宿主按该目标已有模板渲染。渲染上下文保留现有字段，由本次事件填充 `Event`、`Severity`、`OccurredAt`，并将 `event.Subject.Channel` 映射到 `Channel`；`Data` 为插件传入的 payload，字符串 `payload.message` 同时填充 `Message`。上下文不提供 `Subject`，也不自动将 `event.Data` 填入 `Data`。不改 `webhook-settings.tsx`。

WebHook 改为订阅者、以及把上表 4 个旧事件搬上总线，见 [builtin-plugins.md](builtin-plugins.md)。

## 7. JS 处置

`onEvent` 的命名导出、上下文与示例见 [plugin.js.md](plugin.js.md)。这里接收业务事件，不接收流式响应 chunk。

启用且声明了 `events` 的 JS 插件，由 `dispatcher.go` 向 Bus 订阅。`Origin` 对应的插件不收到自己造成的事件。`Depth > 2` 丢弃。

```text
1. 渠道 A 失败，NextChannel 切到 B，发布 request.failover（channel=A, model=gpt-5）
2. 声明了 `request.failover` 的 JS 插件入队。没有内置 WebHook 订阅者
3. worker 从该插件的 VM 池借一个 VM 调用 onEvent(event, ctx)，返回后归还。
   池子被请求钩子占满时 worker 等到有空闲 VM，事件留在队列里，不丢弃、不计失败
4. 插件：
   const n = ctx.counter.add(`failover:${ch.id}:${event.subject.model}`, 300);
   if (n === ctx.config.threshold) {
     ctx.webhook.send(ctx.config.target, { message: `${ch.name} 5 分钟内故障转移 ${n} 次` });
     ctx.axonhub.channels.disable(ch.id, { reason: "failover threshold" });
   }
5. disable 先查 granted_permissions 里有 write_channels，再查写限流，
   然后 WithSystemBypass(ctx, "plugin:acme.failover-guard") 调 UpdateChannelStatus。
   日志记 plugin_id、action、channel_id、result
6. 该操作再产生的事件带 Origin=plugin:acme.failover-guard、Depth=1，不回投该插件
```

失败：`onEvent` 抛错或超时则丢弃、不重试，只记失败次数和最近错误。无权限抛 `PluginHostError`（code 为 `PERMISSION_DENIED`）且无副作用。队列满则丢弃并计数。连续 20 次失败熔断 5 分钟。

`hostapi.go` 按 [宿主 API 签名](plugin.js.md#公共类型与调用规则) 注入；渠道管理与 WebHook 发送只在事件调用开放：

- `ctx.axonhub.channels.{get,list,disable,enable,disableCredential,enableCredential}`
- `ctx.axonhub.requests.get`，只返回元数据
- `ctx.webhook.send`，需要权限 `webhook`
- `ctx.counter.{add,get}`、`ctx.kv.{get,set,delete,list}`、`ctx.log.{info,warn,error}`

写操作走 `ChannelService.UpdateChannelStatus`、`DisableAPIKey`、`EnableAPIKey`。`ctx.kv` 不查权限词表。

计数器：窗口不超过 60 秒时按秒分桶，更长窗口按 10 秒分桶；同一插件、key、窗口组成一个独立计数序列。只存有计数的桶并维护累计总数，不在每次读取时遍历整个窗口。

- `redis` 或 `two-level`：每个序列使用一个 Hash 保存桶计数与总数、一个 Sorted Set 索引桶的过期时间。两个 key 使用同一 hash tag，支持 Redis Cluster。一次 Lua 调用原子完成过期桶扣减、可选递增和总数返回，使用 Redis 服务端时间避免实例时钟偏差。仅清理已过期桶，正常读取不扫描未过期桶。最后一次递增后最多 `window+1s` 自动释放存储；读取不续期，也不创建空计数器。
- `memory`：保留相同的时间桶与过期语义，访问时增量扣除过期桶，闲置序列最多每分钟随访问清理一次，不启动后台任务。多实例下阈值是「单实例达到阈值」，管理页要提示。

启动时按缓存模式只创建一个计数后端：内存容器，或计数专用 Redis 客户端（Redis 不可达时启动失败）。Redis 并发递增返回各自原子操作后的计数；这不保证随后的禁用渠道、WebHook 等动作恰好执行一次。

## 8. 验收

| 类别     | 标准                                                                                                                                         | 验证方式                                                                                            |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| 功能     | 8 种新事件在上表位置各发布一次，字段符合白名单；带凭证的 3 种只有指纹和掩码                                                                  | `go test ./internal/server/events/... ./internal/server/biz/... ./internal/server/orchestrator/...` |
| 功能     | 无订阅者或 `EventBus` 为 nil 时不构造、不发布，原业务操作仍执行；有匹配订阅者时正常投递                                                      | 发布点单测                                                                                          |
| 功能     | 单 worker 串行处理时，第 10 次故障转移恰好通过 `ctx.webhook.send` 发一次并禁用渠道；第 11 次不重复。不要求多实例并发下恰好触发一次，见妥协 5 | 插件集成测试。内存计数器和 miniredis 各一组                                                         |
| 权限     | 未授予的宿主 API 抛 `PERMISSION_DENIED` 且无副作用；manifest 含 `fetch` 或未开放的事件名时安装失败                                           | 插件单测                                                                                            |
| 对外兼容 | 4 个旧 WebHook 事件的请求体与改动前一致，因为调用点未改                                                                                      | `go test ./internal/server/biz/ -run TestWebhook`                                                   |

## 9. 风险与妥协

| 风险         | 触发条件                      | 影响           | 对策                                                                        |
| ------------ | ----------------------------- | -------------- | --------------------------------------------------------------------------- |
| 事件风暴     | 慢插件订阅 `request.*`        | 队列丢弃       | 有界队列和丢弃计数，管理页可见                                              |
| 动作循环触发 | 动作又产生事件                | 反复启停渠道   | `Origin` 不回投，`Depth > 2` 丢弃，写 API 每分钟 60 次                      |
| WebHook 回归 | 改动旧的 `NotifyXxx` 或设置页 | 已有通知体变化 | 本阶段不改这些路径。迁移时的快照见 [builtin-plugins.md](builtin-plugins.md) |

```text
1. 妥协：事件最多投递一次
   - 原因：进程内总线，不落库
   - 影响：重启、队列满、熔断期间的事件丢失
   - 升级触发：要基于事件做对账或计费
   - 后续方向：低频事件（channel.*、provider.quota.*）增加 outbox

2. 妥协：memory 模式下计数器按实例计数
   - 原因：不把 Redis 变成强制依赖
   - 影响：多实例时阈值是单实例阈值
   - 升级触发：多实例用户反馈阈值不准
   - 后续方向：要求多实例配置 Redis，或改用数据库计数

3. 妥协：无管理操作审计表
   - 原因：仓库没有审计实体
   - 影响：插件动作只能从结构化日志和 bypass 审计日志追溯
   - 升级触发：合规要求
   - 后续方向：新增通用审计实体，插件动作首批接入

4. 妥协：无声明式规则引擎
   - 原因：JS 加计数器已覆盖阈值场景
   - 影响：不会写代码的用户要用现成插件
   - 升级触发：大量同类阈值需求
   - 后续方向：做成内置插件，仍走这个事件扩展点

5. 妥协：计数与后续处置不是同一事务
   - 原因：Redis 计数虽为原子操作，但渠道变更和 WebHook 是独立的外部副作用
   - 影响：达到阈值后插件退出或动作失败，处置可能未完成；窗口计数回落后再次达到阈值也可能再次触发
   - 升级触发：需要可靠的多实例阈值处置
   - 后续方向：按业务设计动作幂等标识、处置状态和重试，不把原子计数等同于动作恰好执行一次
```
