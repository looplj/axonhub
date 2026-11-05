# 事件

[插件系统](README.md)的一个扩展点。WebHook 是这里的内置订阅者，不是独立模块：新增一种事件只发布一次，WebHook 和 JS 插件都从总线上收到。

manifest 的 `events` 字段与权限词表见 [manifest.md](manifest.md)，运行时和数值限制见 [README](README.md#运行限制)。任务见 [任务表](README.md#9-任务管理)。

## 1. 背景

`internal/server/biz/webhook_notifier.go` 只有 4 个事件：`channel.auto_disabled`、`provider.quota.warning|exhausted|recovered`。每个事件一个 `NotifyXxx`，由 `channel_auto_disable.go` 的 `asyncNotifyChannelAutoDisabled` 和 `provider_quota.go` 的 `notifyQuotaStatusChange` 直接调用。订阅粒度只有事件名。配置在 `systems.webhook_notifier_config`。

缺口：`DisableAPIKey` 只有全部凭证都不可用时才发 channel 事件。故障转移只在 `llm/pipeline/pipeline.go` 和 `orchestrator/outbound.go` 的 `NextChannel`（990 行）打 Debug 日志。请求失败、API Key 配额耗尽没有任何事件。触发之后也不能执行禁用渠道这类动作。

## 2. 目标

1. 发布下面 12 种事件，使用统一信封。凭证只出现指纹。
2. 现有 WebHook 变成内置插件 `axonhub.webhook`。旧配置不用迁移，4 个旧事件的请求体与升级前逐字节一致，并能订阅新事件。
3. JS 插件用 `onEvent` 订阅，并通过已授予的宿主 API 处置。用户场景：某模型在某渠道 5 分钟内故障转移 10 次时发一次通知并禁用该渠道。

### 非目标

- 声明式规则引擎。阈值用 `ctx.counter`。
- 事件落库、重放。投递是最多一次。
- 管理操作审计表。动作记结构化日志。

## 3. User Stories

1. 作为运维，我希望订阅凭证禁用和故障转移，并按渠道、模型过滤，以便知道哪个上游出了问题。
2. 作为运维，我希望故障转移超过阈值后自动禁用该渠道并通知值班群，以便不用人工盯盘。
3. 作为升级用户，我希望已配置的 WebHook 照常发送，并且能改成订阅新事件。
4. 作为维护者，我希望新增一种事件只在业务处发布一次，以便 WebHook 和 JS 插件同时可用。

## 4. 决策

| 方案 | 结论 |
|------|------|
| 继续为每个事件写 `NotifyXxx` | 放弃。每加一个订阅者都要改所有发布点 |
| 进程内总线，订阅者各自有界队列和单 worker，`Publish` 非阻塞 | 采用。没有订阅者时不构造事件 |
| 数据库 outbox | 放弃。`request.*` 每请求写一次，写放大 |

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

`fingerprint = hex(sha256(key))[:12]`，`masked = 前 4 位 + "…" + 后 4 位`。OAuth 凭证固定 `fingerprint="oauth"`。宿主 API 按指纹在渠道当前凭证里反查明文，插件拿不到明文。

| 事件 | 级别 | 发布位置 | Subject | Data |
|------|------|----------|---------|------|
| `channel.auto_disabled` | warning | `biz/channel_auto_disable.go` `asyncNotifyChannelAutoDisabled` | channel | statusCode, threshold, actualCount, reason |
| `channel.auto_enabled` | info | `biz/channel_apikey.go` `cleanupExpiredAutoDisabledChannels` | channel | — |
| `channel.credential_disabled` | warning | `biz/channel_apikey.go` `DisableAPIKey` 写库成功后 | channel, credential | errorCode, reason, expiresAt |
| `channel.credential_enabled` | info | `EnableAPIKey` / `EnableAllAPIKeys` / `EnableSelectedAPIKeys` / `cleanupExpiredDisabledAPIKeys` | channel, credential | trigger（manual / expired） |
| `provider.quota.warning` / `exhausted` / `recovered` | warning / critical / info | `biz/provider_quota.go` `notifyQuotaStatusChange` | channel | previousStatus, newStatus, usageRatio, nextResetAt, limits |
| `request.attempt_failed` | warning | `orchestrator/request_execution.go` `persistRequestExecution` 的 `OnOutboundRawError` | channel, credential, model, apiKey, project, request | attempt, statusCode, errorType, errorMessage（截断 512 字节） |
| `request.failover` | warning | `orchestrator/outbound.go` `NextChannel`：在 `CurrentCandidateIndex++` 之前记下当前渠道，切换成功后发布 | channel（切换前）, model, apiKey, project, request | toChannelId, toChannelName, attempt |
| `request.failed` | error | `orchestrator/orchestrator.go` `Process` 错误分支，`UpdateRequestStatusFromError` 之后 | channel（最后一次）, model, apiKey, project, request | statusCode, errorType, errorMessage, attempts |
| `request.completed` | info | 非流式 `persistRequest` 的 `OnInboundRawResponse`；流式 `InboundPersistentStream` 结束 | channel, model, apiKey, project, request | latencyMs, promptTokens, completionTokens, attempts, stream |
| `api_key.quota_exceeded` | warning | `orchestrator/quota.go` `enforceQuota` 拒绝时 | apiKey, project, model, request | profile, window |

Subject 和 Data 交给 JS 和 WebHook 模板时，字段名是 camelCase。4 个旧事件原有的模板变量不变。

所有事件发布点（`channel.*`、`provider.quota.*`、`request.*`、`api_key.*`）都先检查 `Bus.HasSubscribers`；无订阅者不构造事件对象及其 Subject/Data。该检查只跳过事件构造与发布，不跳过原业务操作。

`Bus`：`Publish`、`Subscribe(name, types, queue, handler) (unsubscribe)`、`HasSubscribers`、`Stats`。每个订阅者一个有界队列和一个 worker。FX `OnStop` 关闭 worker。

## 6. 发布与 WebHook

`ChannelServiceParams`、`ProviderQuotaServiceParams` 增加 `EventBus *events.Bus`。原来的 `NotifyChannelAutoDisabled`、`NotifyProviderQuota*` 删除，调用方改为 `Publish`。`biz/fx_module.go` 不再把 `WebhookNotifier` 注入这些 service。

`WebhookRenderContext` 只增加 `Subject`、`Data`、`Message`，旧字段语义不变。`NotifyEvent` 对 4 个旧事件仍填充 Channel、Trigger、Quota；新事件填充 Channel 与 Subject、Data。`selectTargets` 不变。`SendToTarget` 给 `ctx.webhook.send` 用。

内置插件 `axonhub.webhook` 订阅全部事件类型，按 `WebhookNotifierConfig.Subscriptions` 选目标并渲染发送。它默认启用，Owner 可以在插件页停用。前端 `webhook-settings.tsx` 的事件选项追加 8 个新事件，文案进 `locales/en/system.json` 和 `locales/zh-CN/system.json`。

## 7. JS 处置

`onEvent` 的命名导出、上下文与示例见 [plugin.js.md](plugin.js.md)。这里接收业务事件，不接收流式响应 chunk。

启用且声明了 `events` 的 JS 插件，由 `dispatcher.go` 向 Bus 订阅。`Origin` 对应的插件不收到自己造成的事件。`Depth > 2` 丢弃。

```text
1. 渠道 A 失败，NextChannel 切到 B，发布 request.failover（channel=A, model=gpt-5）
2. axonhub.webhook（有订阅时）和 acme.failover-guard 各自入队
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

计数器：`redis` 或 `two-level` 时用 `xredis`，按秒分桶 `axonhub:plugin:cnt:{plugin_id}:{key}:{window_sec}:{unix_sec}`，key 组成部分编码以避免分隔符碰撞；同 key 的不同窗口独立。`INCR` 加 `EXPIRE(window+1s)`，读取时对窗口内求和。窗口大于 60 秒时按 10 秒分桶。`memory` 模式用进程内同样的环形分桶，因此多实例下阈值是「单实例达到阈值」，管理页要提示。

## 8. 验收

| 类别 | 标准 | 验证方式 |
|------|------|----------|
| 功能 | 12 种事件在上表位置各发布一次，字段符合白名单，凭证只有指纹 | `go test ./internal/server/events/... ./internal/server/biz/... ./internal/server/orchestrator/...` |
| 功能 | 四类事件在无订阅者时均不构造或发布事件，原业务操作仍正常执行；有匹配订阅者时正常投递 | 发布点单测，记录事件构造与投递次数 |
| 功能 | 单 worker 串行处理时，第 10 次故障转移恰好发一次 WebHook 并禁用渠道；第 11 次不重复。不要求多实例并发下恰好触发一次，见妥协 5 | 插件集成测试。内存计数器和 miniredis 各一组 |
| 权限 | 未授予的宿主 API 抛 `PERMISSION_DENIED` 且无副作用；manifest 含 `fetch` 时安装失败 | 插件单测 |
| 对外兼容 | 4 个旧事件在默认模板下的请求体与升级前一致 | `go test ./internal/server/biz/ -run TestWebhook` |

## 9. 风险与妥协

| 风险 | 触发条件 | 影响 | 对策 |
|------|----------|------|------|
| 事件风暴 | 慢插件订阅 `request.*` | 队列丢弃 | 有界队列和丢弃计数，管理页可见 |
| 动作循环触发 | 动作又产生事件 | 反复启停渠道 | `Origin` 不回投，`Depth > 2` 丢弃，写 API 每分钟 60 次 |
| WebHook 回归 | 发布点遗漏或模板上下文变化 | 用户收不到通知 | 快照测试。升级后第一次自动禁用时看 `notify webhook` 日志，并确认 `axonhub.webhook` 启用 |

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

5. 妥协：Redis 计数的递增与窗口求和不保证整体原子性
   - 原因：当前接受多实例并发下的阈值触发偏差，不增加 Lua 原子操作
   - 影响：多个实例同时处理事件时，`n === threshold` 可能漏触发或重复触发；Redis 共享计数不等于动作恰好执行一次
   - 升级触发：需要可靠的多实例阈值处置
   - 后续方向：将递增、过期设置和窗口求和合并为原子操作，再按业务需要设计动作去重
```
