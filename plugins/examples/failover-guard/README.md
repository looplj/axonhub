# Failover Guard 示例插件

> 此示例已使用修订后的接口，统一 ctx 在前，请求 hooks 使用显式 enabled 和范围列表，对象返回为 replace、数组为 JSON Patch；参见 [开发指南](../../../docs/zh/development/plugins.md)。pluginApiVersion 保持 1。

这个目录是一个可以安装的事件插件，对应当前已经开放的 JS 运行时：`onEvent`、`ctx.counter`、`ctx.axonhub.channels.disable`、`ctx.webhook.send`、`ctx.log`。要先确认事件能在页面上看到，用 [Event Log](../event-log/README.md)。

请求钩子的 `onRequest` 见 [Temperature Cap](../temperature-cap/README.md)，这里不声明 `hooks`，也没有 `onRequest` / `onAttempt`。页面示例仍是 [Notes](../notes/README.md)。

## 做什么

订阅 `request.failover`。同一渠道、同一模型在窗口内的次数由 `ctx.counter` 累加。次数刚好等于阈值时：

1. 禁用该渠道。渠道已经是禁用状态时，宿主把这次调用当作成功。
2. 若配置了 WebHook 目标名，向该已有目标发一次通知。`payload.message` 会填进模板的 Message。目标留空则只禁用渠道。

第 11 次不会再次动作。窗口滑走、计数回落后，再次等于阈值时才会再动作。`memory` 缓存下计数按实例计算；配置了 Redis 时各实例共享同一窗口。

脚本只使用事件里的渠道、模型和 `data.toChannelName`。不读取凭证，也没有请求体。

`ctx.config` 是已保存的配置。还没在管理页保存过时，脚本使用 schema 里的默认值：阈值 10，窗口 300 秒。

## 目录

```text
manifest.json
plugin.js        命名导出 onEvent
lib/key.js       包内相对 import，把模型名收成计数器允许的 key
```

## 打包

在仓库根目录执行：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/failover-guard
```

脚本会打进 `manifest.json`、根目录 `plugin.js`，以及 `plugin.js` 的包内相对 import。根目录 `README.md` 不会进包。默认产物是 `plugins/examples/failover-guard.axplugin`，这个文件已被 Git 忽略。

## 安装

1. 在系统设置里准备好 WebHook 目标。插件只能按目标名称发送，不能自己写 URL。
2. 用 Owner 打开侧栏「插件」›「插件管理」（`/plugins`），上传 `failover-guard.axplugin`。
3. 授权时勾选 `write_channels` 和 `webhook`，然后启用。少勾选时，对应的宿主调用会抛 `PERMISSION_DENIED`，这次事件不会重试。
4. 在同一页设置 Threshold、Window seconds 和 Webhook target。保存后的值会在下一次事件里进入 `ctx.config`。

禁用渠道会再发布后续事件，并带上 `origin: "plugin:example.failover-guard"`。这条来源不会回投给本插件。
