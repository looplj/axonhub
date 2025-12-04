# Claude Code 与 Codex 集成指南

---

## 概览
AxonHub 可以作为 Anthropic 或 OpenAI 接口的直接替代方案，使 Claude Code 与 Codex 能够通过您自己的基础设施连接。本文将介绍两者的配置方法，并说明如何结合 AxonHub 的模型配置文件功能实现灵活路由。

### 前置要求
- 可访问的 AxonHub 实例。
- 拥有项目访问权限的 AxonHub API Key。
- Claude Code（Anthropic）与/或 Codex（OpenAI 兼容工具）的使用权限。
- （可选）已在 AxonHub 控制台配置好的一个或多个模型配置文件。

### 配置 Claude Code
1. 在 Shell 环境变量中写入 AxonHub 凭证：
   ```bash
   export ANTHROPIC_AUTH_TOKEN="<your-axonhub-api-key>"
   export ANTHROPIC_BASE_URL="http://localhost:8090/anthropic"
   ```
2. 启动 Claude Code，程序会自动读取上述变量并将所有 Anthropic 请求代理到 AxonHub。
3. （可选）触发一次对话并在 AxonHub 的 Traces 页面确认流量已成功记录。

#### 提示
- 请务必保密 API Key，可写入 shell profile 或使用密钥管理工具。
- 若 AxonHub 使用自签名证书，请在操作系统内添加信任配置。

### 配置 Codex
1. 编辑 `${HOME}/.codex/config.toml`，将 AxonHub 注册为 provider：
   ```toml
   model = "gpt-5"
   model_provider = "axonhub-chat-completions"

   [model_providers.axonhub-chat-completions]
   name = "AxonHub using Chat Completions"
   base_url = "http://127.0.0.1:8090/v1"
   env_key = "AXONHUB_API_KEY"
   wire_api = "chat"
   query_params = {}
   ```
2. 导出供 Codex 读取的 API Key：
   ```bash
   export AXONHUB_API_KEY="<your-axonhub-api-key>"
   ```
3. 重启 Codex 以加载配置。

#### 验证
- 发送测试 Prompt，AxonHub 日志中应出现 `/v1/chat/completions` 调用。
- 启用 AxonHub 的追踪功能可查看提示词、回复及延迟信息。

### 使用模型配置文件
AxonHub 的模型配置文件支持将请求模型映射到具体提供商模型：
- 在 AxonHub 控制台创建配置文件并添加映射规则（精确名称或正则）。
- 将配置文件绑定到 API Key。
- 切换活动配置文件即可更改 Claude Code/Codex 的行为，无需调整本地工具设置。

<table>
  <tr align="center">
    <td align="center">
      <a href="../../screenshots/axonhub-profiles.png">
        <img src="../../screenshots/axonhub-profiles.png" alt="Model Profiles" width="250"/>
      </a>
      <br/>
      Model Profiles
    </td>
  </tr>
</table>

#### 示例
- 请求 `claude-sonnet-4-5` → 映射到 `deepseek-reasoner` 以获取更准确的回复。
- 请求 `claude-haiku-4-5` → 映射到 `deepseek-chat` 以降低成本。

### 常见问题
- **Claude Code 无法连接**：确认 `ANTHROPIC_BASE_URL` 指向 `/anthropic` 路径，且本地防火墙允许外部请求。
- **Codex 认证失败**：确保在启动 Codex 的同一 shell 会话中设置了 `AXONHUB_API_KEY`。
- **模型结果异常**：检查 AxonHub 控制台中当前启用的配置文件映射，必要时禁用或调整规则。

### 相关文档
- [追踪指南](tracing.md)
- [Chat Completions 文档](../api-reference/unified-api.md#openai-chat-completions-api)
- README 中的 [使用指南](../../../README.zh-CN.md#使用指南--usage-guide)
