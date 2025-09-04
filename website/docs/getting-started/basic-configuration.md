# 基础配置

本指南将帮助您完成 AxonHub 的基础配置，确保系统能够正常运行。

## 📋 配置文件概述

AxonHub 使用 YAML 格式的配置文件 `config.yml`，支持环境变量覆盖。配置文件路径：

- **默认路径**: `./config.yml`
- **环境变量**: `AXONHUB_CONFIG_PATH`
- **示例文件**: `config.example.yml`

## ⚙️ 核心配置项

### 1. 服务器配置

```yaml
server:
  port: 8090                    # 服务端口 (1-65535)
  name: "AxonHub"               # 服务名称，用于日志和监控
  base_path: ""                 # API 基础路径，如 "/api/v1"
  request_timeout: "30s"        # HTTP 请求超时时间
  llm_request_timeout: "600s"   # LLM API 请求超时时间
  debug: false                  # 调试模式，启用详细日志
  trace:
    trace_header: "AH-Trace-Id" # 分布式追踪头名称
```

**配置说明：**
- `port`: 服务监听端口，确保端口未被占用
- `name`: 服务实例名称，在集群部署时用于区分不同实例
- `base_path`: API 路径前缀，用于反向代理或多服务部署
- `request_timeout`: 普通 HTTP 请求超时，建议 30-60 秒
- `llm_request_timeout`: AI 模型请求超时，建议 300-600 秒
- `debug`: 开发环境可启用，生产环境建议关闭

### 2. 数据库配置

```yaml
db:
  dialect: "postgres"           # 数据库类型
  dsn: "connection_string"      # 数据库连接字符串
  debug: false                  # 数据库调试日志
```

**支持的数据库类型：**

| 数据库 | dialect 值 | DSN 示例 |
|--------|------------|----------|
| **SQLite** | `sqlite3` | `file:axonhub.db?cache=shared&_fk=1` |
| **PostgreSQL** | `postgres` | `postgres://user:pass@host:5432/dbname?sslmode=disable` |
| **MySQL** | `mysql` | `user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True` |

**不同环境的数据库配置示例：**

**开发环境 (SQLite):**
```yaml
db:
  dialect: "sqlite3"
  dsn: "file:./data/axonhub_dev.db?cache=shared&_fk=1"
  debug: true
```

**生产环境 (PostgreSQL):**
```yaml
db:
  dialect: "postgres"
  dsn: "postgres://axonhub:secure_password@db.example.com:5432/axonhub_prod?sslmode=require"
  debug: false
```

### 3. 日志配置

```yaml
log:
  name: "axonhub"               # 日志器名称
  debug: false                  # 调试日志开关
  level: "info"                 # 日志级别
  level_key: "level"            # 日志级别字段名
  time_key: "time"              # 时间戳字段名
  caller_key: "label"           # 调用者信息字段名
  encoding: "json"              # 日志编码格式
  includes: []                  # 包含的日志器列表
  excludes: []                  # 排除的日志器列表
```

**日志级别说明：**
- `debug`: 详细调试信息，仅开发环境使用
- `info`: 一般信息，推荐生产环境使用
- `warn`: 警告信息，需要关注但不影响运行
- `error`: 错误信息，需要立即处理
- `panic`: 严重错误，程序可能崩溃
- `fatal`: 致命错误，程序将退出

**日志编码格式：**
- `json`: JSON 格式，适合日志收集系统
- `console`: 控制台格式，适合开发调试
- `console_json`: 控制台 JSON 格式，兼顾可读性和结构化

### 4. 监控配置

```yaml
metrics:
  enabled: true                 # 启用监控
  exporter:
    type: "prometheus"          # 导出器类型
```

**监控导出器类型：**
- `prometheus`: Prometheus 格式，端点 `/metrics`
- `console`: 控制台输出，用于调试
- `stdout`: 标准输出，用于容器化部署

### 5. 数据转储配置

```yaml
dumper:
  enabled: false                # 启用错误数据转储
  dump_path: "./dumps"          # 转储文件目录
  max_size: 100                 # 单个文件最大大小 (MB)
  max_age: "24h"                # 文件保留时间
  max_backups: 10               # 最大备份文件数
```

## 🌍 环境变量配置

AxonHub 支持通过环境变量覆盖配置文件中的设置，环境变量优先级高于配置文件。

### 完整环境变量映射表

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| **服务器配置** | | | |
| `server.port` | `AXONHUB_SERVER_PORT` | `8090` | 服务端口 |
| `server.name` | `AXONHUB_SERVER_NAME` | `AxonHub` | 服务名称 |
| `server.base_path` | `AXONHUB_SERVER_BASE_PATH` | `""` | API 基础路径 |
| `server.request_timeout` | `AXONHUB_SERVER_REQUEST_TIMEOUT` | `30s` | 请求超时 |
| `server.llm_request_timeout` | `AXONHUB_SERVER_LLM_REQUEST_TIMEOUT` | `600s` | LLM 请求超时 |
| `server.debug` | `AXONHUB_SERVER_DEBUG` | `false` | 调试模式 |
| **数据库配置** | | | |
| `db.dialect` | `AXONHUB_DB_DIALECT` | `sqlite3` | 数据库类型 |
| `db.dsn` | `AXONHUB_DB_DSN` | `file:axonhub.db` | 数据库连接串 |
| `db.debug` | `AXONHUB_DB_DEBUG` | `false` | 数据库调试 |
| **日志配置** | | | |
| `log.name` | `AXONHUB_LOG_NAME` | `axonhub` | 日志器名称 |
| `log.debug` | `AXONHUB_LOG_DEBUG` | `false` | 调试日志 |
| `log.level` | `AXONHUB_LOG_LEVEL` | `info` | 日志级别 |
| `log.encoding` | `AXONHUB_LOG_ENCODING` | `json` | 日志格式 |
| **监控配置** | | | |
| `metrics.enabled` | `AXONHUB_METRICS_ENABLED` | `false` | 启用监控 |
| `metrics.exporter.type` | `AXONHUB_METRICS_EXPORTER_TYPE` | `stdout` | 导出器类型 |
| **转储配置** | | | |
| `dumper.enabled` | `AXONHUB_DUMPER_ENABLED` | `false` | 启用转储 |
| `dumper.dump_path` | `AXONHUB_DUMPER_DUMP_PATH` | `./dumps` | 转储路径 |

### 环境变量使用示例

**Docker 部署：**
```bash
# docker-compose.yml 中的环境变量
environment:
  - AXONHUB_SERVER_PORT=8090
  - AXONHUB_DB_DIALECT=postgres
  - AXONHUB_DB_DSN=postgres://axonhub:${DB_PASSWORD}@postgres:5432/axonhub
  - AXONHUB_LOG_LEVEL=info
  - AXONHUB_LOG_ENCODING=json
```

**Kubernetes 部署：**
```yaml
# deployment.yaml 中的环境变量
env:
- name: AXONHUB_SERVER_PORT
  value: "8090"
- name: AXONHUB_DB_DSN
  valueFrom:
    secretKeyRef:
      name: axonhub-db-secret
      key: dsn
- name: AXONHUB_LOG_LEVEL
  value: "info"
```

## 🎯 不同场景配置示例

### 1. 开发环境配置

```yaml
# config.yml - 开发环境
server:
  port: 8090
  name: "AxonHub-Dev"
  debug: true
  request_timeout: "60s"
  llm_request_timeout: "300s"

db:
  dialect: "sqlite3"
  dsn: "file:./data/axonhub_dev.db?cache=shared&_fk=1"
  debug: true

log:
  level: "debug"
  encoding: "console"
  debug: true

metrics:
  enabled: true
  exporter:
    type: "console"

dumper:
  enabled: true
  dump_path: "./dumps"
  max_size: 50
  max_age: "1h"
  max_backups: 5
```

### 2. 生产环境配置

```yaml
# config.yml - 生产环境
server:
  port: 8090
  name: "AxonHub-Prod"
  debug: false
  request_timeout: "30s"
  llm_request_timeout: "600s"
  trace:
    trace_header: "X-Trace-ID"

db:
  dialect: "postgres"
  dsn: "postgres://axonhub:${DB_PASSWORD}@postgres.internal:5432/axonhub?sslmode=require"
  debug: false

log:
  level: "info"
  encoding: "json"
  debug: false
  excludes: ["gorm.io/gorm"]

metrics:
  enabled: true
  exporter:
    type: "prometheus"

dumper:
  enabled: false
```

### 3. 高性能环境配置

```yaml
# config.yml - 高性能环境
server:
  port: 8090
  name: "AxonHub-HPC"
  debug: false
  request_timeout: "15s"
  llm_request_timeout: "300s"

db:
  dialect: "postgres"
  dsn: "postgres://axonhub:password@pgpool:5432/axonhub?sslmode=require&pool_max_conns=50&pool_min_conns=10"
  debug: false

log:
  level: "warn"
  encoding: "json"
  debug: false
  excludes: ["gorm.io/gorm", "net/http"]

metrics:
  enabled: true
  exporter:
    type: "prometheus"

dumper:
  enabled: false
```

## ✅ 配置验证

### 1. 配置文件语法验证

**验证 YAML 语法：**
```bash
# 使用 yq 验证 YAML 语法
yq eval '.' config.yml > /dev/null && echo "配置文件语法正确" || echo "配置文件语法错误"

# 使用 Python 验证
python -c "import yaml; yaml.safe_load(open('config.yml'))" && echo "YAML 格式正确"
```

### 2. 配置加载测试

**测试配置加载：**
```bash
# 启动服务并检查配置加载
./axonhub --config config.yml --validate-config

# 查看配置加载日志
./axonhub 2>&1 | grep -i "config\|configuration"

# 使用调试模式查看详细配置信息
AXONHUB_SERVER_DEBUG=true ./axonhub
```

### 3. 数据库连接测试

**测试数据库连接：**
```bash
# PostgreSQL 连接测试
psql "postgres://user:pass@host:5432/dbname" -c "SELECT 1;"

# MySQL 连接测试  
mysql -h host -u user -p -e "SELECT 1;" dbname

# SQLite 文件检查
sqlite3 axonhub.db ".tables"

# 使用 AxonHub 内置健康检查
curl http://localhost:8090/health
```

## 🔧 配置管理最佳实践

### 1. 配置文件管理

**版本控制：**
```bash
# 将配置文件模板纳入版本控制
git add config.example.yml
git commit -m "Add configuration template"

# 排除生产配置文件
echo "config.yml" >> .gitignore
echo ".env" >> .gitignore
```

**配置文件组织：**
```bash
# 创建配置目录
mkdir -p config
mv config.example.yml config/

# 环境特定配置
config/
├── base.yml          # 基础配置
├── development.yml   # 开发环境配置
├── production.yml    # 生产环境配置
└── test.yml          # 测试环境配置
```

### 2. 敏感信息管理

**使用环境变量：**
```bash
# 创建环境变量文件
cat > .env << EOF
# 数据库配置
DB_PASSWORD=your_secure_password
DB_HOST=localhost
DB_USER=axonhub

# API 密钥
OPENAI_API_KEY=your_openai_key
ANTHROPIC_API_KEY=your_anthropic_key
EOF

# 设置文件权限
chmod 600 .env
```

**使用密钥管理工具：**
```bash
# 使用 HashiCorp Vault
vault kv put axonhub/config \
  db_password=secure_password \
  openai_api_key=your_key

# 使用 AWS Secrets Manager
aws secretsmanager create-secret \
  --name axonhub-config \
  --secret-string '{"db_password":"secure_password"}'
```

### 3. 配置验证脚本

**创建配置验证脚本：**
```bash
#!/bin/bash
# scripts/validate-config.sh

CONFIG_FILE=${1:-config.yml}

# 检查配置文件存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 配置文件 $CONFIG_FILE 不存在"
    exit 1
fi

# 验证 YAML 语法
if ! python -c "import yaml; yaml.safe_load(open('$CONFIG_FILE'))" 2>/dev/null; then
    echo "错误: 配置文件 YAML 语法错误"
    exit 1
fi

# 检查必需的配置项
required_keys=("server.port" "db.dialect" "db.dsn")
for key in "${required_keys[@]}"; do
    if ! yq eval ".$key" "$CONFIG_FILE" >/dev/null 2>&1; then
        echo "错误: 缺少必需的配置项 $key"
        exit 1
    fi
done

echo "配置文件验证通过"
```

## 🚨 常见配置问题

### 1. 端口冲突

**问题**：`Error: listen tcp :8090: bind: address already in use`

**解决方案：**
```bash
# 查找占用端口的进程
sudo lsof -i :8090
sudo netstat -tulpn | grep :8090

# 终止占用进程
sudo kill -9 <PID>

# 或修改配置使用其他端口
export AXONHUB_SERVER_PORT=8091
```

### 2. 数据库连接失败

**问题**：`Error: failed to connect to database`

**解决方案：**
```bash
# 检查数据库服务状态
sudo systemctl status postgresql
sudo systemctl status mysql

# 测试网络连通性
telnet db_host 5432
nc -zv db_host 3306

# 验证连接字符串
echo $AXONHUB_DB_DSN
```

### 3. 权限问题

**问题**：`Error: permission denied`

**解决方案：**
```bash
# 检查文件权限
ls -la config.yml
ls -la ./dumps/

# 修复权限
sudo chown axonhub:axonhub config.yml
sudo chmod 644 config.yml
sudo mkdir -p ./dumps && sudo chown axonhub:axonhub ./dumps
```

### 4. 配置文件格式错误

**问题**：`Error: yaml: unmarshal errors`

**解决方案：**
```bash
# 检查 YAML 缩进
cat -A config.yml | head -20

# 验证 YAML 语法
python -m yaml config.yml

# 查找特殊字符
grep -P "[\x80-\xFF]" config.yml
```

---

## 📞 下一步

配置完成后，您可以：

1. **启动服务**: 运行 `./axonhub` 或 `sudo systemctl start axonhub`
2. **访问管理界面**: 打开 `http://localhost:8090`
3. **配置 AI 提供商**: 在管理界面中添加 API 密钥
4. **测试功能**: 发送第一个 AI 请求

下一章节：[第一个 AI 请求](./first-request.md)

---

<div align="center">

**配置完成！** 🎉

AxonHub 已准备就绪，开始您的 AI 网关之旅！

</div>