# 托管表待办清单

`todos` 是唯一的业务表。页面通过 `axonhub.tables.open("todos")` 直接读写这张表。使用者需要插件管理权限（`write_plugins` 或 Owner）。

1. 上传 `table-todos.axplugin`，授予 `tables` 并启用。已安装旧版时，确认升级到 1.2.0。
2. 从侧栏打开「待办清单」，填写标题并添加。勾选记录可以完成或重新打开待办，点击「删除」可以移除它。
3. 刷新页面验证状态持久化。卸载时保留数据，重装启用后打开页面，原记录仍应可见。

从仓库根目录打包：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/table-todos
```
