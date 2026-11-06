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

## 3. 验收

排期后再测，不在当前任务表：

| 标准                                                                     | 验证方式                                          |
| ------------------------------------------------------------------------ | ------------------------------------------------- |
| 没有行时 `axonhub.webhook` 按默认启用；第一次停用才 upsert；不能删除     | 插件管理测试                                      |
| 4 个旧事件在默认模板下的请求体与迁移前逐字节一致                         | `go test ./internal/server/biz/ -run TestWebhook` |
| 停用 `axonhub.webhook` 后这 4 个事件不再发 HTTP；JS 插件仍能从总线上收到 | 总线与通知测试                                    |
| 设置页能订阅迁移后开放的事件                                             | 前端事件选项与中英文文案                          |

## 4. 不在本篇

- 进程内总线、8 种新事件、`onEvent` 和宿主 API：见 [events.md](events.md)。
- 自定义 channel provider 的 Go 注册也走同一套 `Apply` / `OnDispose`，见 [provider.md](provider.md)。Provider 未排期。
