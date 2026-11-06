# Temperature Cap 示例插件

> 此示例已使用修订后的接口，统一 ctx 在前，请求 hooks 使用显式 enabled 和范围列表，对象返回为 replace、数组为 JSON Patch；参见 [开发指南](../../../docs/zh/development/plugins.md)。pluginApiVersion 保持 1。

这个目录是一个可以安装的请求钩子插件，对应当前已经开放的 `onRequest`。它把每个请求的 `temperature` 设成配置里的 `maxTemperature`。

事件插件见 [Failover Guard](../failover-guard/README.md)。页面示例仍是 [Notes](../notes/README.md)。

## 做什么

`onRequest` 在每个请求上执行一次，位于模型权限检查、模型映射和选路之前。脚本读取 `ctx.config.maxTemperature`。还没在管理页保存过时，脚本使用 schema 里的默认值 `0.2`。返回值是展开后的完整请求对象，只写入 `temperature`，并保持 `stream` 与输入一致。

这个脚本不声明 `events`、`network`、`permissions` 或 `onAttempt`。脚本不使用模块级变量。

渠道开启 pass-through 时，`model` 以外的改写会被客户端原始请求体覆盖。`temperature` 属于这类改写。管理页上该插件的 `overriddenByPassThrough` 增加时，让 Owner 关闭该渠道的 pass-through，或改用渠道 override body 写入固定温度。

## 目录

```text
manifest.json
plugin.js        命名导出 onRequest
```

## 打包

在仓库根目录执行：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/temperature-cap
```

脚本会打进 `manifest.json` 和根目录 `plugin.js`。根目录 `README.md` 不会进包。默认产物是 `plugins/examples/temperature-cap.axplugin`，这个文件已被 Git 忽略。

## 安装

1. 用 Owner 打开侧栏「插件」›「插件管理」（`/plugins`），上传 `temperature-cap.axplugin` 并启用。这个例子不申请权限。
2. 在同一页设置 Max temperature。保存后的值会在下一次请求里进入 `ctx.config`。还没保存时，钩子使用 `0.2`。
