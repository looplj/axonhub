# 快速入门指南

## 概述

本指南将帮助您快速开始使用 AxonHub。只需几分钟，您就可以运行 AxonHub 并发出第一个 API 调用。

## 先决条件

- Docker 和 Docker Compose（推荐）
- 或者 Go 1.24+ 和 Node.js 18+ 用于开发环境设置
- 来自 AI 提供商的有效 API 密钥（OpenAI、Anthropic 等）

## 快速设置方法

### 方法 1：Docker Compose（推荐）

1. **克隆仓库**
   ```bash
   git clone https://github.com/looplj/axonhub.git
   cd axonhub
   ```

2. **配置环境变量**
   ```bash
   cp config.example.yml config.yml
   # 使用您喜欢的编辑器编辑 config.yml
   ```

3. **启动服务**
   ```bash
   docker-compose up -d
   ```

4. **访问应用程序**
   - Web 界面：http://localhost:8090
   - 默认凭据：admin@example.com / admin123

### 方法 2：二进制下载

1. **下载最新版本**
   - 访问 [GitHub Releases](https://github.com/looplj/axonhub/releases)
   - 下载适合您操作系统的二进制文件

2. **解压并运行**
   ```bash
   unzip axonhub_*.zip
   cd axonhub_*
   chmod +x axonhub
   ./axonhub
   ```

3. **访问应用程序**
   - Web 界面：http://localhost:8090

## 第一步

### 1. 配置您的第一个渠道

1. 登录到 Web 界面
2. 导航到 **Channels（渠道）**
3. 点击 **Add Channel（添加渠道）**
4. 选择您的提供商（例如，OpenAI）
5. 输入您的 API 密钥和配置
6. 测试连接
7. 启用渠道

### 2. 创建 API 密钥

1. 导航到 **API Keys（API 密钥）**
2. 点击 **Create API Key（创建 API 密钥）**
3. 给出一个描述性名称
4. 选择适当的范围
5. 复制生成的 API 密钥

### 3. 发出您的第一个 API 调用

AxonHub 支持 OpenAI 聊天补全和 Anthropic 消息 API，允许您使用首选的 API 格式访问任何支持的模型。

#### 使用 OpenAI API 格式

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# 调用 OpenAI 模型
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[
        {"role": "user", "content": "Hello, AxonHub!"}
    ]
)
print(response.choices[0].message.content)

# 使用 OpenAI API 调用 Anthropic 模型
response = client.chat.completions.create(
    model="claude-3-5-sonnet",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ]
)
print(response.choices[0].message.content)
```

#### 使用 Anthropic API 格式

```python
import requests

# 调用 Anthropic 模型
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={
        "Content-Type": "application/json",
        "X-API-Key": "your-axonhub-api-key"
    },
    json={
        "model": "claude-3-5-sonnet",
        "max_tokens": 512,
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Hello, Claude!"}]
            }
        ]
    }
)
print(response.json()["content"][0]["text"])

# 使用 Anthropic API 调用 OpenAI 模型
response = requests.post(
    "http://localhost:8090/anthropic/v1/messages",
    headers={
        "Content-Type": "application/json",
        "X-API-Key": "your-axonhub-api-key"
    },
    json={
        "model": "gpt-4o",
        "max_tokens": 512,
        "messages": [
            {
                "role": "user",
                "content": [{"type": "text", "text": "Hello, GPT!"}]
            }
        ]
    }
)
print(response.json()["content"][0]["text"])
```

#### 统一 API 的主要优势

- **API 互操作性**：使用 OpenAI API 调用 Anthropic 模型，或使用 Anthropic API 调用 OpenAI 模型
- **零代码更改**：继续使用您现有的 OpenAI 或 Anthropic 客户端 SDK
- **自动翻译**：AxonHub 自动处理 API 格式转换
- **提供商灵活性**：使用您首选的 API 格式访问任何支持的 AI 提供商

### 4. 高级渠道配置

#### 模型映射

模型映射允许您将特定模型的请求重定向到不同的上游模型。这对于以下情况很有用：

- **成本优化**：将昂贵的模型映射到更便宜的替代品
- **遗留支持**：将已弃用的模型名称映射到当前模型
- **提供商切换**：将模型映射到不同的提供商
- **故障转移**：配置具有不同提供商的多个渠道

**模型映射配置示例：**

```yaml
# 在渠道设置中
settings:
  modelMappings:
    # 将产品特定的别名映射到上游模型
    - from: "gpt-4o-mini"
      to: "gpt-4o"

    # 将遗留模型名称映射到当前模型
    - from: "claude-3-sonnet"
      to: "claude-3.5-sonnet"

    # 映射到不同的提供商
    - from: "my-company-model"
      to: "gpt-4o"

    # 成本优化
    - from: "expensive-model"
      to: "cost-effective-model"
```

**使用示例：**

```python
# 客户端请求 "gpt-4o-mini" 但获得 "gpt-4o"
response = client.chat.completions.create(
    model="gpt-4o-mini",  # 将被映射到 "gpt-4o"
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
```

#### 覆盖参数

覆盖参数允许您强制执行渠道特定的默认值，而不受传入请求负载的影响。这对于以下情况很有用：

- **安全性**：强制执行安全的参数值
- **一致性**：确保跨应用程序的一致行为
- **合规性**：满足组织要求
- **优化**：为特定用例设置最佳参数

**覆盖参数配置示例：**

```yaml
# 在渠道设置中
settings:
  overrideParameters: |
    {
      # 基本参数
      "temperature": 0.3,
      "max_tokens": 1024,
      "top_p": 0.9,

      # 强制 JSON 响应
      "response_format": {
        "type": "json_object"
      },

      # 安全参数
      "presence_penalty": 0.1,
      "frequency_penalty": 0.1,

      # 提供商特定参数
      "stop_sequences": ["\nHuman:", "\nAssistant:"]
    }
```

**高级覆盖示例：**

```yaml
# 为生产环境强制执行确定性响应
overrideParameters: |
  {
    "temperature": 0.1,
    "max_tokens": 500,
    "top_p": 0.95
  }

# 创意写作渠道
overrideParameters: |
  {
    "temperature": 0.8,
    "max_tokens": 2000,
    "frequency_penalty": 0.5
  }

# 代码生成渠道
overrideParameters: |
  {
    "temperature": 0.2,
    "max_tokens": 4096,
    "stop": ["```", "\n\n"]
  }
```

#### 组合示例：模型映射 + 覆盖参数

```yaml
# 完整的渠道配置
name: "openai-production"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "your-openai-key"
supported_models: ["gpt-4o", "gpt-4", "gpt-3.5-turbo"]
settings:
  modelMappings:
    - from: "chat-model"
      to: "gpt-4o"
    - from: "fast-model"
      to: "gpt-3.5-turbo"
  overrideParameters: |
    {
      "temperature": 0.3,
      "max_tokens": 1024,
      "response_format": {
        "type": "json_object"
      }
    }
```

#### 最佳实践

1. **模型映射**
   - 仅映射到 `supported_models` 中声明的模型
   - 使用描述性的映射名称以提高清晰度
   - 在生产使用前彻底测试映射
   - 为团队成员记录您的映射策略

2. **覆盖参数**
   - 从保守值开始，根据用例进行调整
   - 考虑对成本和性能的影响
   - 使用不同类型的请求测试覆盖
   - 监控使用模式以优化参数

3. **安全注意事项**
   - 避免在开发渠道中覆盖敏感参数
   - 为不同的安全要求使用单独的渠道
   - 定期审查和更新覆盖配置

## 配置示例

### 基本配置

```yaml
# config.yml
server:
  port: 8090
  name: "AxonHub"

db:
  dialect: "sqlite3"
  dsn: "file:axonhub.db?cache=shared&_fk=1"

log:
  level: "info"
  encoding: "json"
```

### 生产配置

```yaml
server:
  port: 8090
  name: "AxonHub Production"
  debug: false

db:
  dialect: "postgres"
  dsn: "postgres://user:pass@localhost/axonhub?sslmode=disable"

log:
  level: "warn"
  encoding: "json"
  output: "file"
  file:
    path: "/var/log/axonhub/axonhub.log"
```

## 下一步

### 探索功能
- **追踪**：设置请求追踪以实现可观测性
- **权限**：配置基于角色的访问控制
- **模型配置文件**：创建模型映射规则
- **使用分析**：监控 API 使用和成本

### 集成指南
- [Claude Code 集成](../guides/claude-code-integration.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [部署指南](../deployment/configuration.md)

## 故障排除

### 常见问题

**无法连接到 AxonHub**
- 检查服务是否正在运行：`docker-compose ps`
- 验证端口 8090 是否可用
- 检查防火墙设置

**API 密钥身份验证失败**
- 验证 API 密钥是否正确配置
- 检查渠道是否已启用
- 确保提供商 API 密钥有效

**请求超时**
- 在配置中增加 `server.llm_request_timeout`
- 检查与 AI 提供商的网络连通性

### 获取帮助

- 查看 [GitHub Issues](https://github.com/looplj/axonhub/issues)
- 查看 [架构文档](../architecture/erd.md)
- 加入社区讨论

## 下一步是什么？

现在您已经运行了 AxonHub，探索这些高级功能：

- 设置多个渠道以实现故障转移
- 配置模型映射以优化成本
- 实施请求追踪以进行调试
- 设置使用配额和速率限制
- 与现有的 CI/CD 流水线集成
