# AxonHub Plugin

插件开发工具和示例的统一入口。后续插件 SDK 和开发文档分别放在本目录下的 `sdk/`、`docs/`，待开发时添加。

## 目录

- [`DEVELOPMENT.md`](DEVELOPMENT.md)：插件页面如何使用宿主主题。
- [`scripts/`](scripts/)：插件开发工具，当前提供 Go 打包工具。
- [`examples/notes/`](examples/notes/README.md)：可安装的 Notes UI 插件示例。
- [`examples/event-log/`](examples/event-log/README.md)：可安装的事件记录示例，用来确认事件已经送达。
- [`examples/failover-guard/`](examples/failover-guard/README.md)：可安装的故障转移事件插件示例。
- [`examples/star-history/`](examples/star-history/README.md)：可安装的 Star 历史示例，用来演示宿主代发的出站请求。

## 打包示例

需要仓库 `go.mod` 指定版本的 Go。在仓库根目录执行：

```bash
go run ./plugin/scripts/pack-plugin plugin/examples/notes
go run ./plugin/scripts/pack-plugin plugin/examples/event-log
go run ./plugin/scripts/pack-plugin plugin/examples/failover-guard
go run ./plugin/scripts/pack-plugin plugin/examples/star-history
go run ./plugin/scripts/pack-plugin plugin/examples/notes -o /tmp/notes.axplugin
```

默认生成 `plugin/examples/<目录名>.axplugin`，已被 Git 忽略。包内包含 `manifest.json`、`ui/` 下的文件、根目录 `plugin.js`，以及从 `plugin.js` 静态相对 import 到的文件。根目录 README 和 `.DS_Store` 不会进入包。安装步骤见 [Notes 示例](examples/notes/README.md#安装)、[Event Log 示例](examples/event-log/README.md#安装和验收)、[Failover Guard 示例](examples/failover-guard/README.md#安装) 和 [Star 历史示例](examples/star-history/README.md#安装)。

打包工具复用服务端的 moejs 解析器和模块链接逻辑收集静态依赖，不执行插件代码，也不转换或合并源码。动态 `import()` 不会额外收集文件；服务端仍在安装时校验 manifest 兼容性和插件导出。
