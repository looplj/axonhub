# AxonHub Plugin

插件开发工具和示例的统一入口。开发指南见 [中文文档](../docs/zh/development/plugins.md)和 [English documentation](../docs/en/development/plugins.md)。

## 目录

- [`registry.json`](registry.json)：市场索引。AxonHub 从 `unstable` 分支的 raw URL 拉取。当前 `plugins` 为空。
- [`../cmd/pack-plugin`](../cmd/pack-plugin)：独立 Go 模块的打包器。`go install github.com/looplj/axonhub/cmd/pack-plugin@latest` 使用子目录标签 `cmd/pack-plugin/vX.Y.Z`。
- [`examples/notes/`](examples/notes/README.md)：可安装的 Notes UI 插件示例。
- [`examples/event-log/`](examples/event-log/README.md)：可安装的事件记录示例，用来确认事件已经送达。
- [`examples/failover-guard/`](examples/failover-guard/README.md)：可安装的故障转移事件插件示例。
- [`examples/star-history/`](examples/star-history/README.md)：可安装的 Star 历史示例，用来演示宿主代发的出站请求。
- [`examples/temperature-cap/`](examples/temperature-cap/README.md)：可安装的请求钩子示例，按配置写入 `temperature`。

## 打包示例

在仓库根目录执行：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/notes
go run -C cmd/pack-plugin . ../../plugins/examples/event-log
go run -C cmd/pack-plugin . ../../plugins/examples/failover-guard
go run -C cmd/pack-plugin . ../../plugins/examples/star-history
go run -C cmd/pack-plugin . ../../plugins/examples/temperature-cap
go run -C cmd/pack-plugin . ../../plugins/examples/notes -o /tmp/notes.axplugin
```

默认生成插件目录旁边的 `<目录名>.axplugin`，已被 Git 忽略。输出必须在插件目录外面。包内包含 `manifest.json`、`ui/` 下的文件、根目录 `plugin.js`，以及从 `plugin.js` 静态相对 import 到的文件。根目录 README、`.DS_Store` 和 `.git` 不会进入包。安装步骤见 [Notes 示例](examples/notes/README.md#安装)、[Event Log 示例](examples/event-log/README.md#安装和验收)、[Failover Guard 示例](examples/failover-guard/README.md#安装) 和 [Star 历史示例](examples/star-history/README.md#安装)。

打包器是独立模块，复制了服务端的 moejs 解析和模块链接来收集静态依赖，不执行插件代码，也不转换或合并源码。动态 `import()` 不会额外收集文件。服务端仍在安装时校验 manifest 兼容性和插件导出。
