# 内置插件

[插件系统](README.md)里尚未排期的一块。当前任务表不实现注册表，也不把现有 WebHook 改成事件订阅者。JS 插件订阅的事件见 [events.md](events.md)。

`axonhub.webhook` 这个 id 已经在 `ReservedPluginIDs` 里保留，上传的包不能占用。管理器上的 `Builtin` 接口（`Apply` / `OnDispose` / `RegisterBuiltin`）可以留着，当前没有任何内置插件注册。

## 1. 做什么

Go 内置插件没有 `.axplugin`。名称、版本和 manifest 来自代码注册表，随 AxonHub 升级更新，不在启动时同步进库。

```text
plugin/builtin.Register
  → 管理器对已启用的内置插件调 Apply
  → 停用或进程停止时倒序 OnDispose
```

库里的行只存 `plugin_id` 和 `enabled`，其余列为空。没有行时按注册表默认值；Owner 第一次启停时按 `plugin_id` upsert。管理页可以启停，不能删除，不走权限授予。`plugins` 查询把注册表里的内置插件和 JS 插件一起返回，带 `builtin: true`，没有 config 和权限。

v1 只有 `axonhub.webhook`。它默认启用。配置继续读 `systems.webhook_notifier_config`，不用 `plugins.config`。

第二个候选是 `axonhub.codex`（Codex 兼容），见 [第 3 节](#3-codex-兼容迁移候选)。

## 2. WebHook 迁移

排期前，这 4 个事件仍由 `WebhookNotifier` 的 `NotifyXxx` 直接发送，调用点不变：

| 事件                       | 现有调用                                                       |
| -------------------------- | -------------------------------------------------------------- |
| `channel.auto_disabled`    | `biz/channel_auto_disable.go` `asyncNotifyChannelAutoDisabled` |
| `provider.quota.warning`   | `biz/provider_quota.go` `notifyQuotaStatusChange`              |
| `provider.quota.exhausted` | 同上                                                           |
| `provider.quota.recovered` | 同上                                                           |

排期时：

1. `ChannelService`、`ProviderQuotaService` 改为向 [事件总线](events.md) `Publish` 这 4 个事件，删除对应的 `NotifyXxx`。`biz/fx_module.go` 不再把 `WebhookNotifier` 注入这两个 service。
2. `axonhub.webhook` 订阅全部已发布事件，按 `WebhookNotifierConfig.Subscriptions` 选目标并渲染发送。`selectTargets` 不变。
3. `WebhookRenderContext` 增加 `Subject`、`Data`、`Message`。4 个旧事件仍填充 Channel、Trigger、Quota，默认模板下的请求体与升级前逐字节一致。新事件填充 Channel 与 Subject、Data。
4. 前端 `webhook-settings.tsx` 的事件选项补上当时总线上有、设置页还没有的事件。文案进 `locales/en/system.json` 和 `locales/zh-CN/system.json`。

`ctx.webhook.send` 是 JS 插件的宿主 API，属于当前的 [events.md](events.md#7-js-处置)，不依赖本篇。它复用已配置的目标，不把 WebHook 变成订阅者。

## 3. Codex 兼容迁移（候选）

Codex 兼容让 Codex CLI 用 AxonHub 的 API Key 接入 `/codex`，模型目录取自 Owner 选定的 Codex 渠道。它可选、默认关闭、关闭时整组路由返回 404，与核心网关逻辑解耦，形态上就是一个内置插件。记为 `axonhub.webhook` 之后的第二个迁移对象，随注册表一起排期。

不做成 JS 插件：它要在网关上挂 HTTP 与 WebSocket 路由（`internal/server/codex_routes.go`），走 API Key 鉴权、配额和请求日志，并从渠道拉取上游目录。这些都不在 JS 插件的扩展点里。出站 HTTP 只覆盖已授权的路径，见 [出站请求](plugin.js.md#出站请求)，不能用来挂这组路由。放在模型请求关键路径上也不能接受 VM 不足时的 429。

排期时：

1. 注册 `axonhub.codex`，默认停用，并加入 `ReservedPluginIDs`。没有行时按停用处理，现有部署升级后由 `systems.codex_compatibility.enabled` 决定首次状态，避免已开启的实例被关掉。
2. 配置继续读 `systems.codex_compatibility`，开关之外只用到 `channelID`（目录来源渠道）。与 `axonhub.webhook` 一样不用 `plugins.config`，因此不需要先给内置插件加配置模型。目录来源渠道的选择留在原设置页。
3. `codex_routes.go` 的路由仍在启动时注册。`CodexCompatibilityHandlers.Enabled` 中间件的判断从读取设置改为查询插件是否启用，停用时照旧返回 404。
4. GraphQL `codexCompatibilityEnabled` 改为返回插件启用状态，保持对任意登录调用方开放：它只暴露开关，API Key 详情据此决定是否展示 Codex 配置示例。
5. 启用 `axonhub.codex` 前必须校验 `channelID` 非空、渠道存在且调用方可访问、类型为 Codex；插件管理入口和原设置入口复用同一校验。插件保持启用时，不允许清空渠道或换成不可访问、非 Codex 的渠道。一次操作同时停用并清空渠道可以成功，校验以操作后的状态为准。
6. 保留 `updateCodexCompatibilitySettings` 的 schema 和现有权限校验，作为兼容入口：输入 `enabled` 更新插件状态，`channelID` 更新原设置，并在同一事务内提交；校验或写入失败时全部回滚。`codexCompatibilitySettings.enabled`、`codexCompatibilityEnabled` 和路由中间件统一读取插件状态。`systems.codex_compatibility.enabled` 仅用于首次迁移，迁移完成后不再作为运行时开关。旧客户端继续调用原 mutation 即可启停，无需先升级；新插件管理入口也调用同一状态变更服务，避免两套开关分歧。

## 4. 验收

排期后再测，不在当前任务表：

| 标准                                                                     | 验证方式                                          |
| ------------------------------------------------------------------------ | ------------------------------------------------- |
| 没有行时 `axonhub.webhook` 按默认启用；第一次停用才 upsert；不能删除     | 插件管理测试                                      |
| 4 个旧事件在默认模板下的请求体与迁移前逐字节一致                         | `go test ./internal/server/biz/ -run TestWebhook` |
| 停用 `axonhub.webhook` 后这 4 个事件不再发 HTTP；JS 插件仍能从总线上收到 | 总线与通知测试                                    |
| 设置页能订阅迁移后开放的事件                                             | 前端事件选项与中英文文案                          |
| 升级前已开启 Codex 兼容的实例，升级后 `axonhub.codex` 为启用             | 插件管理测试                                      |
| 停用 `axonhub.codex` 后 `/codex/*` 返回 404，`codexCompatibilityEnabled` 为 false | `go test ./internal/server/ -run TestCodex`       |
| 两个启用入口均拒绝缺失、不可访问或非 Codex 的渠道；启用期间不能清空渠道 | 插件管理与设置 mutation 测试 |
| 旧 settings mutation 与插件管理入口启停后，两个 GraphQL 开关字段和 `/codex/*` 行为一致 | GraphQL 与路由集成测试 |
| 同时修改渠道与开关时，校验或写入失败不会留下部分更新 | 事务回滚测试 |

## 5. 不在本篇

- 进程内总线、8 种新事件、`onEvent` 和宿主 API：见 [events.md](events.md)。
- 自定义 channel provider 的 Go 注册也走同一套 `Apply` / `OnDispose`，见 [provider.md](provider.md)。Provider 未排期。
