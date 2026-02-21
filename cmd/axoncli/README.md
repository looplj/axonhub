# AxonCli

> ⚠️ 这是一个验证性的 Demo 项目

AxonCli 是一个基于 [axon](../../axon) 包构建的命令行 AI 助手演示程序，使用 [Bubbletea](https://github.com/charmbracelet/bubbletea) TUI 框架。

## 功能

- 基于 Bubbletea 的终端 UI（可滚动对话区、多行输入、加载动画）
- 支持文件读取、写入、编辑
- 支持 Bash 命令执行
- 支持 Grep 和 Glob 文件搜索
- 会话线程管理（JSONL 持久化）
- 会话中途 Steering（打断/纠正）
- Skills 管理子命令
- Memory 管理子命令

## 配置

在 `~/.axoncli/config.yml` 中配置：

```yml
base_url: http://localhost:8090
api_key: your-api-key
model: deepseek-chat
```

也可以使用环境变量覆盖：

- `AXONCLI_BASE_URL`
- `AXONCLI_API_KEY`
- `AXONCLI_MODEL`

## 运行

```bash
go run .
```

## 快捷键

| 快捷键       | 功能           |
|--------------|---------------|
| `Enter`      | 发送消息       |
| `Shift+Enter`| 换行           |
| `Ctrl+Enter` | 发送消息       |
| `Ctrl+J`     | 换行           |
| `Ctrl+C`     | 快速双击退出    |
| `Esc`        | 取消当前处理    |

## 复制/选中文本

- 鼠标左键拖动选择对话内容，松开后自动复制到系统剪贴板
- `Esc` 清除选择高亮
- 也可用 `Ctrl+Shift+C` / `Ctrl+Y` 复制当前选择

## 命令

| 命令        | 功能               |
|-------------|-------------------|
| `/help`     | 显示帮助信息       |
| `/clear`    | 清屏               |
| `/messages` | 显示对话历史       |
| `/quit`     | 退出               |
