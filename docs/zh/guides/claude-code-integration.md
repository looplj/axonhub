# Claude Code 集成指南

---

## 概览
AxonHub 可以作为 Anthropic 接口的直接替代方案，使 Claude Code 能够通过您自己的基础设施连接。本文将介绍配置方法，并说明如何结合 AxonHub 的模型配置文件功能实现灵活路由。

### 关键点
- AxonHub 支持多种 AI 协议/格式转换。你可以配置多个上游渠道（provider/channel），对外提供统一的 Anthropic 兼容接口，供 Claude Code 使用。
- 你可以开启 Claude Code trace 聚合，将 Claude Code 同一次会话中的请求自动归并到同一条 Trace（见"配置 Claude Code"）。

### 前置要求
- 可访问的 AxonHub 实例。
- 拥有项目访问权限的 AxonHub API Key。
- Claude Code（Anthropic）的使用权限。
- （可选）已在 AxonHub 控制台配置好的一个或多个模型配置文件。

### 配置 Claude Code
1. 在 Shell 环境变量中写入 AxonHub 凭证：
   ```bash
   export ANTHROPIC_AUTH_TOKEN="<your-axonhub-api-key>"
   export ANTHROPIC_BASE_URL="http://localhost:8090/anthropic"
   ```
2. 启动 Claude Code，程序会自动读取上述变量并将所有 Anthropic 请求代理到 AxonHub。
3. （可选）触发一次对话并在 AxonHub 的 Traces 页面确认流量已成功记录。

#### Trace 聚合（可选）
若希望将 Claude Code 同一次会话的请求聚合到同一条 Trace，可在 `config.yml` 中开启：

```yaml
server:
  trace:
    claude_code_trace_enabled: true
```

#### 提示
- 请务必保密 API Key，可写入 shell profile 或使用密钥管理工具。
- 若 AxonHub 使用自签名证书，请在操作系统内添加信任配置。

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
- **模型结果异常**：检查 AxonHub 控制台中当前启用的配置文件映射，必要时禁用或调整规则。

### 相关文档
- [追踪指南](tracing.md)
- [Chat Completions 文档](../api-reference/unified-api.md#openai-chat-completions-api)
- [Codex 集成指南](codex-integration.md)
- README 中的 [使用指南](../../../README.zh-CN.md#使用指南--usage-guide)
