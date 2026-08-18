# Docker 部署

## 概述

本指南涵盖了使用 Docker 和 Docker Compose 部署 AxonHub 的方法。Docker 提供了一个隔离的、可重复的环境，简化了部署和扩展。

## 快速入门

### 1. 克隆仓库

```bash
git clone https://github.com/looplj/axonhub.git
cd axonhub
```

### 2. 配置环境

复制示例配置文件：

```bash
cp config.example.yml config.yml
```

编辑 `config.yml` 以进行设置：

```yaml
# config.yml
server:
  port: 8090
  name: "AxonHub"

db:
  dialect: "sqlite3"
  dsn: "file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)"

log:
  level: "info"
  encoding: "json"
```

### 3. 启动服务

请先按[安全考虑](#安全考虑)创建 `.env` 文件。Compose 会强制要求显式提供镜像引用和 `DB_PASSWORD`，不会提供可变的镜像标签或弱密码默认值。

```bash
docker compose --env-file .env up -d
```

### 4. 验证部署

检查服务状态：

```bash
docker-compose ps
```

访问应用：
- Web 界面：http://localhost:8090
- 默认管理员：admin@example.com / admin123

## Docker Compose 配置

### 基础 docker-compose.yml

```yaml
version: '3.8'

services:
  axonhub:
    image: ${AXONHUB_IMAGE:?AXONHUB_IMAGE must be set}
    ports:
      - "127.0.0.1:8090:8090"
    volumes:
      - ./config.yml:/app/config.yml:ro
      - axonhub_data:/app/data
    environment:
      - AXONHUB_SERVER_PORT=8090
      - AXONHUB_DB_DIALECT=sqlite3
      - AXONHUB_DB_DSN=file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)
    restart: unless-stopped

volumes:
  axonhub_data:
```

### 生产环境配置

```yaml
version: '3.8'

services:
  axonhub:
    image: ${AXONHUB_IMAGE:?AXONHUB_IMAGE must be set}
    ports:
      - "127.0.0.1:8090:8090"
    volumes:
      - ./config.yml:/app/config.yml:ro
      - axonhub_data:/app/data
      - ./logs:/app/logs
    environment:
      - AXONHUB_SERVER_PORT=8090
      - AXONHUB_DB_DIALECT=postgres
      - AXONHUB_DB_DSN=postgres://axonhub:${DB_PASSWORD:?DB_PASSWORD must be set}@postgres:5432/axonhub
      - AXONHUB_LOG_LEVEL=warn
      - AXONHUB_LOG_OUTPUT=file
      - AXONHUB_LOG_FILE_PATH=/app/logs/axonhub.log
    depends_on:
      - postgres
    restart: unless-stopped

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=axonhub
      - POSTGRES_USER=axonhub
      - POSTGRES_PASSWORD=${DB_PASSWORD:?DB_PASSWORD must be set}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  axonhub_data:
  postgres_data:
```

## 数据库选项

### SQLite (开发环境)

```yaml
axonhub:
  environment:
    - AXONHUB_DB_DIALECT=sqlite3
    - AXONHUB_DB_DSN=file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)
```

### PostgreSQL (生产环境)

```yaml
axonhub:
  environment:
    - AXONHUB_DB_DIALECT=postgres
    - AXONHUB_DB_DSN=postgres://user:pass@host:5432/axonhub
```

### MySQL (生产环境)

```yaml
axonhub:
  environment:
    - AXONHUB_DB_DIALECT=mysql
    - AXONHUB_DB_DSN=user:pass@tcp(host:3306)/axonhub?charset=utf8mb4&parseTime=True
```

## 环境变量

### 服务器配置

```bash
AXONHUB_SERVER_PORT=8090
AXONHUB_SERVER_NAME="AxonHub"
AXONHUB_SERVER_DEBUG=false
AXONHUB_SERVER_REQUEST_TIMEOUT="30s"
AXONHUB_SERVER_LLM_REQUEST_TIMEOUT="600s"
```

### 数据库配置

```bash
AXONHUB_DB_DIALECT="postgres"
AXONHUB_DB_DSN="postgres://user:pass@host:5432/axonhub"
AXONHUB_DB_DEBUG=false
```

### 日志配置

```bash
AXONHUB_LOG_LEVEL="info"
AXONHUB_LOG_ENCODING="json"
AXONHUB_LOG_OUTPUT="stdio"
```

## 安全考虑

仓库中的 `docker-compose.yml` 已启用以下安全默认值：

- PostgreSQL 仅能通过 Compose 私有网络访问，不再发布到宿主机端口。
- AxonHub 默认只绑定 `127.0.0.1`；如果需要由反向代理或外部监听，请显式设置 `AXONHUB_BIND_ADDRESS`。
- `DB_PASSWORD` 为必填项，不再提供内置弱密码。
- 应用以非 root 用户运行，丢弃 Linux capabilities，启用 `no-new-privileges`、只读根文件系统、受限 `/tmp` tmpfs、PID/资源限制和容器日志轮转。
- 应用和数据库使用隔离网络，数据库无法访问上游 AI 服务。

启动前创建权限受限的本地 `.env` 文件：

```bash
umask 077
cat > .env <<'EOF'
DB_PASSWORD=替换为足够长的随机密码
# 请将两个占位符替换为实际发布镜像的 digest。
AXONHUB_IMAGE=looplj/axonhub@sha256:replace-with-axonhub-digest
POSTGRES_IMAGE=postgres@sha256:replace-with-postgres-digest
EOF
```

请将 `AXONHUB_IMAGE` 和 `POSTGRES_IMAGE` 的占位符替换为 digest 固定的镜像引用后再启动。`.env` 不应提交到版本库，并应按部署流程轮换 `DB_PASSWORD`。

### 网络安全

```yaml
axonhub:
  networks:
    - axonhub_network
  ports:
    - "127.0.0.1:8090:8090"  # 仅绑定到本地回环
    
networks:
  axonhub_network:
    driver: bridge
```

### 密钥管理

Compose 需要使用数据库密码构造 AxonHub 的 PostgreSQL DSN，因此通过本地环境注入 `DB_PASSWORD`，而不是把密码写入 YAML。对于 Swarm/Kubernetes 或其他密钥管理系统，请在部署时注入，不要将密钥存入仓库：

```bash
# .env 文件
DB_PASSWORD=your-secure-password
API_KEY_SECRET=your-api-key-secret
```

```bash
chmod 600 .env
docker compose --env-file .env up -d
```

## 监控和日志

### 健康检查

```yaml
axonhub:
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8090/health"]
    interval: 30s
    timeout: 10s
    retries: 3
    start_period: 40s
```

### 日志收集

```yaml
axonhub:
  logging:
    driver: "json-file"
    options:
      max-size: "10m"
      max-file: "3"
```

## 扩展

### 水平扩展

```yaml
axonhub:
  deploy:
    replicas: 3
    resources:
      limits:
        memory: 1G
        cpus: '0.5'
      reservations:
        memory: 512M
        cpus: '0.25'
```

### 负载均衡器设置

```yaml
services:
  axonhub:
    image: ${AXONHUB_IMAGE:?AXONHUB_IMAGE must be set}
    deploy:
      replicas: 3
    networks:
      - axonhub_network

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - axonhub
    networks:
      - axonhub_network
```

## 备份与恢复

### 数据库备份

```yaml
services:
  backup:
    image: postgres:15
    volumes:
      - ./backup:/backup
      - postgres_data:/var/lib/postgresql/data
    command: |
      bash -c '
        pg_dump -h postgres -U axonhub axonhub > /backup/axonhub-$(date +%Y%m%d).sql
      '
    depends_on:
      - postgres
    environment:
      - PGPASSWORD=password
```

### 卷备份

```bash
# 备份数据卷
docker run --rm -v axonhub_data:/source -v $(pwd)/backup:/backup alpine \
  tar czf /backup/axonhub-data-$(date +%Y%m%d).tar.gz -C /source .

# 恢复数据卷
docker run --rm -v axonhub_data:/target -v $(pwd)/backup:/backup alpine \
  tar xzf /backup/axonhub-data-20231110.tar.gz -C /target
```

## 故障排除

### 常见问题

**容器无法启动**
- 检查 Docker 日志：`docker-compose logs axonhub`
- 验证配置文件权限
- 确保数据库连接正常

**端口冲突**
- 更改 docker-compose.yml 中暴露的端口
- 检查是否有其他服务正在使用端口 8090

**数据库连接问题**
- 验证数据库凭据
- 检查容器间的网络连通性
- 确保数据库容器正在运行

### 调试模式

启用调试日志进行故障排除：

```yaml
axonhub:
  environment:
    - AXONHUB_SERVER_DEBUG=true
    - AXONHUB_LOG_LEVEL=debug
```

## 后续步骤

- [配置指南](configuration.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [架构文档](../development/erd.md)
