# Toolbar notes / 工具栏随手记

An actions-only plugin: the global toolbar opens a dialog containing a shared scratchpad. No sidebar page, backend JavaScript or external network access is required. Plugin KV is shared across users; saving requires `write_plugins` (or Owner).

仅声明 `ui.actions` 的示例：从全局工具栏打开共享笔记弹窗，不增加侧栏页面，不需要后端脚本或出站权限。笔记属于插件，不属于某个项目或个人；保存需要 `write_plugins` 或 Owner。

From the repository root / 在仓库根目录打包：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/toolbar-notes
```

Upload `plugins/examples/toolbar-notes.axplugin` in plugin management and enable it. The puzzle icon appears in the top-right toolbar (or its overflow menu). No permissions need to be granted to read this plugin's own KV.

在插件管理上传 `plugins/examples/toolbar-notes.axplugin` 后启用，右上角工具栏或其更多菜单中出现「随手记」。读取本插件 KV 不需要额外插件授权。
