# Event Log 示例插件

> 此示例已使用修订后的接口，统一 ctx 在前，请求 hooks 使用显式 enabled 和范围列表，对象返回为 replace、数组为 JSON Patch；参见 [开发指南](../../../docs/zh/development/plugins.md)。pluginApiVersion 保持 1。

这个目录用来确认事件插件已经接上。它把少量容易触发的事件写进本插件的 KV，并在自己的页面里列出来。不禁用渠道，也不发送 WebHook。

故障转移阈值示例仍是 [Failover Guard](../failover-guard/README.md)。那个例子要等真实切流，而且会关掉渠道。

## 记录什么

| 事件                          | 怎么触发                                |
| ----------------------------- | --------------------------------------- |
| `channel.credential_disabled` | 渠道编辑里停用一把上游凭证              |
| `channel.credential_enabled`  | 再启用这把凭证                          |
| `request.completed`           | Playground 或任意客户端打一次成功的请求 |

每次事件追加一条记录，默认只留最新 20 条。记录里有事件名、时间、渠道名、模型、凭证掩码和一行摘要。凭证只保存掩码，不保存明文，也不保存指纹。

页面打开时读取这些记录，并用宿主推送的主题变量跟随控制台的明暗和配色。事件由后台 worker 写入，点击停用后如果列表还是空的，等一秒再按「刷新」。

## 目录

```text
manifest.json
plugin.js           onEvent，写入 KV
lib/record.js
ui/log/index.html   bridge 相对路径是 ../../_axonhub/bridge.js
ui/log/app.js
ui/log/format.js
ui/log/style.css
```

## 打包

在仓库根目录执行：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/event-log
```

默认产物是 `plugins/examples/event-log.axplugin`，这个文件已被 Git 忽略。

## 安装和验收

1. 用 Owner 打开「插件」›「插件管理」（`/plugins`），上传 `event-log.axplugin` 并启用。这个例子不申请渠道或 WebHook 权限。
2. 侧栏出现 Event Log。直接地址是 `/plugins/example.event-log/pages/log`。此时列表为空。
3. 打开一个有上游 Key 的渠道，停用其中一把凭证。
4. 回到 Event Log，按「刷新」。应出现一行 `channel.credential_disabled`，渠道名正确，凭证是掩码。
5. 把这把凭证启用。再刷新，应多一行 `channel.credential_enabled`，摘要是 `manual`。
6. 可选：在 Playground 发一次成功请求。再刷新，应出现 `request.completed`，摘要里有耗时和 token 数。

清空列表需要当前用户有 `write_plugins`。Owner 可以清空。清空会删掉记录和序号，下一次事件从 1 重新编号。
