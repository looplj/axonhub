# 安全功能

AxonHub 提供多层基于 IP 的访问控制，以保护您的 AI 网关。

## IP 访问控制（全局白名单）

IP 访问控制是一个全局中间件，限制对整个 AxonHub 实例的访问。启用后，只接受来自白名单中 IP 地址或 CIDR 范围的请求，其他请求将被拒绝。

当您想将 AxonHub 实例限制为仅接受来自已知网络（例如企业 VPN 或云 VPC）的流量时，此功能非常有用。

### 配置

IP 访问控制通过 AxonHub 配置文件或环境变量进行配置：

```yaml
ip_access_control:
  enabled: true
  allowed_ips:
    - "10.0.0.0/8"
    - "192.168.1.0/24"
    - "203.0.113.10"
  redirect_url: ""  # 可选：将拒绝的请求重定向到此 URL
```

### 行为

- **禁用时**（默认）：所有请求都允许通过。
- **启用时**：来自 IP 匹配 `allowed_ips` 中任何条目的请求被允许；其他请求收到 404（如果设置了 `redirect_url`，则为 302 重定向）。
- 客户端 IP 从 TCP 连接的远程地址确定。

## IP 黑名单（系统级黑名单）

IP 黑名单允许管理员阻止特定 IP 地址或 CIDR 范围发起 API 请求。这通过系统设置 UI 在运行时管理。

### 配置

IP 黑名单通过系统设置进行配置：

```json
{
  "blocked_ips": ["203.0.113.50", "198.51.100.0/24"]
}
```

### 行为

- 来自被阻止 IP 的请求收到 403 Forbidden 响应。
- 黑名单适用于所有外部 API 请求。
- 通过设置 UI 的更改立即生效。

## API Key IP 限制（每密钥白名单）

API Key IP 限制允许您将单个 API 密钥限制为仅接受来自指定源 IP 地址或 CIDR 范围的请求。这对于将 API 密钥的使用限制在特定服务器或网络非常有用。

### 工作原理

当 API 密钥配置了 `allowed_ips` 时，AxonHub 会检查使用该密钥发出的每个请求的源 IP。如果源 IP 不匹配白名单中的任何条目，请求将被拒绝并返回 403 Forbidden 响应。

### 源 IP 检测

AxonHub 从多个请求头中检查源 IP，按优先级顺序：

1. **X-Forwarded-For**：X-Forwarded-For 头中的第一个 IP
2. **X-Real-IP**：X-Real-IP 头的值
3. **Client IP**：直接的 TCP 连接 IP

这确保了当 AxonHub 位于反向代理（例如 Nginx、Cloudflare、AWS ALB）之后时，能正确检测 IP。

### 配置

您可以在创建或编辑 API 密钥时配置 IP 限制：

1. 在仪表板中导航到 **API 密钥**。
2. 点击 **创建 API 密钥** 或编辑现有密钥。
3. 启用 **IP 限制** 开关。
4. 输入一个或多个 IP 地址或 CIDR 范围，以逗号分隔（例如 `192.168.1.0/24, 10.0.0.5, 2001:db8::/32`）。
5. 保存 API 密钥。

### 支持的格式

- **单个 IP**：`203.0.113.10` 或 `2001:db8::10`
- **CIDR 范围**：`203.0.113.0/24` 或 `2001:db8::/32`

同时支持 IPv4 和 IPv6。

### 示例

```bash
# 创建带 IP 限制的 API 密钥
curl -X POST https://your-axonhub-instance/api/api-keys \
  -H "Authorization: Bearer your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "生产服务器密钥",
    "allowed_ips": ["10.0.0.0/8", "192.168.1.100"]
  }'

# 来自允许 IP 的请求 - 成功
curl https://your-axonhub-instance/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}'

# 来自不允许 IP 的请求 - 403 Forbidden
curl https://your-axonhub-instance/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}'
# 返回：403 IP address is not allowed for this API key
```

## 分层安全

这三个功能协同工作，提供深度防御：

| 功能 | 范围 | 动作 | 使用场景 |
|------|------|------|----------|
| IP 访问控制 | 全局 | 仅允许列表中的 IP | 锁定整个实例 |
| IP 黑名单 | 全局 | 阻止特定 IP | 运行时阻止恶意 IP |
| API Key IP 限制 | 每密钥 | 仅允许列表中的 IP | 限制单个 API 密钥 |

请求按以下顺序评估：IP 访问控制 → IP 黑名单 → API 密钥认证 → API 密钥 IP 限制。
