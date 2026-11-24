# Model catalog check / 模型目录检查

This example checks one catalog provider on a schedule. The task queries the site's upstream catalog and local models with `ctx.graphql`, keeps the latest successful result in plugin KV and puts the total on a header action with `ctx.ui.setBadge`. The dialog shows separate counts and links to the model library for manual review.

示例定时检查一个服务商。任务用 `ctx.graphql` 读取站点的上游目录和全局模型，把最近一次成功结果保存到插件 KV，再用 `ctx.ui.setBadge` 更新顶部角标。点击顶部按钮可查看两个计数，并前往全局模型库手动复核。

## Run / 运行

From the repository root / 在仓库根目录打包：

```sh
go run -C cmd/pack-plugin . ../../plugins/examples/model-catalog-check
```

1. Upload `plugins/examples/model-catalog-check.axplugin` on an AxonHub version with task GraphQL and UI badges. Enable it with the `read_channels` grant.
2. The provider defaults to `openai`. Open the plugin details and run the `check` task, or leave it scheduled for 08:00 UTC.
3. Open **Model catalog check** in the top bar or its overflow menu. A successful check displays the counts and sets or clears the badge. A failed upstream fetch keeps the previous result.

1. 在支持任务 GraphQL 和 UI 角标的 AxonHub 上传 `plugins/examples/model-catalog-check.axplugin`，启用并授予 `read_channels`。
2. 默认检查 `openai`，每天 UTC 08:00 运行。也可在插件详情中立即运行 `check` 任务。
3. 从顶部工具栏或更多菜单打开「模型目录检查」。成功后显示两个计数并更新角标，上游读取失败时保留上次结果。

The example matches exact model IDs and saved aliases for the chosen provider. Each local model with a changed followed rate contributes one item, even if multiple rates changed. The example leaves all model and price changes to the administrator.

示例只比较选定服务商，按模型 ID 和已保存的别名精确匹配。一个已有模型的多项跟随价变化只计一项。模型及价格由管理员手动更新。
