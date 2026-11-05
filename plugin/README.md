# AxonHub Plugin

插件开发工具和示例的统一入口。后续插件 SDK 和开发文档分别放在本目录下的 `sdk/`、`docs/`，待开发时添加。

## 目录

- [`scripts/`](scripts/)：插件开发工具，当前提供打包脚本。
- [`examples/notes/`](examples/notes/README.md)：可安装的 Notes UI 插件示例。

## 打包示例

需要 Python 3。在仓库根目录执行：

```bash
python3 plugin/scripts/pack-plugin.py plugin/examples/notes
python3 plugin/scripts/pack-plugin.py plugin/examples/notes -o /tmp/notes.axplugin
```

默认生成 `plugin/examples/notes.axplugin`，已被 Git 忽略。包内包含 `manifest.json` 和 `ui/` 下的文件；根目录 README 和 `.DS_Store` 不会进入包。安装步骤见 [Notes 示例](examples/notes/README.md#安装)。

