# 配置指南

## 概述

AxonHub 使用灵活的配置系统，支持 YAML 配置文件和环境变量。本指南涵盖了所有可用的配置选项以及针对不同部署场景的最佳实践。

## 配置方法

### 配置优先级

AxonHub 使用 Viper 进行配置管理，它可以从多个配置源读取并将其合并为一组配置键值对。Viper 使用以下优先级进行合并（从高到低）：

1. **环境变量** - 系统环境变量
2. **配置文件** - YAML 配置文件
3. **外部键/值存储** - 外部配置存储
4. **默认值** - 内置默认值

这意味着环境变量将覆盖配置文件中的值，而命令行标志将覆盖环境变量。

### 1. YAML 配置文件

创建一个 `config.yml` 文件：

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

### 2. 环境变量

所有配置选项都可以通过环境变量设置：

```bash
export AXONHUB_SERVER_PORT=8090
export AXONHUB_DB_DIALECT="sqlite3"
export AXONHUB_DB_DSN="file:axonhub.db?cache=shared&_fk=1"
export AXONHUB_LOG_LEVEL="info"
```

### 3. 混合配置

环境变量会覆盖 YAML 配置值。

## 配置参考

### 服务器配置

```yaml
server:
  port: 8090                    # 服务器端口
  name: "AxonHub"               # 服务器名称
  base_path: ""                 # API 路由的基础路径
  request_timeout: "30s"        # 请求超时时间
  llm_request_timeout: "600s"   # LLM 请求超时时间
  trace:
    thread_header: "AH-Thread-Id" # 线程 ID 请求头名称
    trace_header: "AH-Trace-Id" # 追踪 ID 请求头名称
    extra_trace_headers: []     # 额外的追踪请求头
    claude_code_trace_enabled: false # 启用 Claude Code 追踪提取
    codex_trace_enabled: false # 启用 Codex 追踪提取
  debug: false                  # 启用调试模式
```

**环境变量：**
- `AXONHUB_SERVER_PORT`
- `AXONHUB_SERVER_NAME`
- `AXONHUB_SERVER_BASE_PATH`
- `AXONHUB_SERVER_REQUEST_TIMEOUT`
- `AXONHUB_SERVER_LLM_REQUEST_TIMEOUT`
- `AXONHUB_SERVER_TRACE_THREAD_HEADER`
- `AXONHUB_SERVER_TRACE_TRACE_HEADER`
- `AXONHUB_SERVER_TRACE_EXTRA_TRACE_HEADERS`
- `AXONHUB_SERVER_TRACE_CLAUDE_CODE_TRACE_ENABLED`
- `AXONHUB_SERVER_TRACE_CODEX_TRACE_ENABLED`
- `AXONHUB_SERVER_DEBUG`

### 数据库配置

```yaml
db:
  dialect: "sqlite3"            # sqlite3, postgres, mysql, tidb
  dsn: "file:axonhub.db?cache=shared&_fk=1"  # 连接字符串
  debug: false                  # 启用数据库调试日志
```

**支持的数据库：**
- **SQLite**: `sqlite3` (开发环境)
- **PostgreSQL**: `postgres` (生产环境)
- **MySQL**: `mysql` (生产环境)
- **TiDB**: `tidb` (生产环境/云端)

**环境变量：**
- `AXONHUB_DB_DIALECT`
- `AXONHUB_DB_DSN`
- `AXONHUB_DB_DEBUG`

### 缓存配置

```yaml
cache:
  mode: "memory"                # memory, redis, two-level
  
  # 内存缓存配置
  memory:
    expiration: "5s"            # 内存缓存 TTL
    cleanup_interval: "10m"     # 内存缓存清理间隔

  # Redis 缓存配置
  redis:
    url: ""                     # Redis 连接 URL (redis:// 或 rediss://)
    addr: ""                    # Redis 地址: 127.0.0.1:6379
    username: ""                # 如果设置，将覆盖 URL 中的用户名
    password: ""                # 如果设置，将覆盖 URL 中的密码
    db: 0                       # 如果设置，将覆盖 URL 路径中的数据库编号 (/0)
    tls: false                  # 启用 TLS (rediss:// 也会自动启用)
    tls_insecure_skip_verify: false # 跳过 TLS 证书验证 (自签名证书)
    expiration: "30m"           # Redis 缓存 TTL
```

**环境变量：**
- `AXONHUB_CACHE_MODE`
- `AXONHUB_CACHE_MEMORY_EXPIRATION`
- `AXONHUB_CACHE_MEMORY_CLEANUP_INTERVAL`
- `AXONHUB_CACHE_REDIS_URL`
- `AXONHUB_CACHE_REDIS_ADDR`
- `AXONHUB_CACHE_REDIS_USERNAME`
- `AXONHUB_CACHE_REDIS_PASSWORD`
- `AXONHUB_CACHE_REDIS_DB`
- `AXONHUB_CACHE_REDIS_TLS`
- `AXONHUB_CACHE_REDIS_TLS_INSECURE_SKIP_VERIFY`
- `AXONHUB_CACHE_REDIS_EXPIRATION`

### 日志配置

```yaml
log:
  name: "axonhub"               # 日志器名称
  debug: false                  # 启用调试日志
  level: "info"                 # debug, info, warn, error, panic, fatal
  level_key: "level"            # 日志级别字段的键名
  time_key: "time"              # 时间戳字段的键名
  caller_key: "label"           # 调用者信息字段的键名
  function_key: ""              # 函数名字段的键名
  name_key: "logger"            # 日志器名称字段的键名
  encoding: "json"              # json, console, console_json
  includes: []                  # 包含的日志器名称
  excludes: []                  # 排除的日志器名称
  output: "stdio"               # file 或 stdio
  file:                         # 基于文件的日志配置
    path: "logs/axonhub.log"   # 日志文件路径
    max_size: 100               # 轮转前的最大大小 (MB)
    max_age: 30                 # 保留的最大天数
    max_backups: 10             # 旧日志文件的最大数量
    local_time: true            # 轮转文件使用本地时间
```

**环境变量：**
- `AXONHUB_LOG_NAME`
- `AXONHUB_LOG_DEBUG`
- `AXONHUB_LOG_LEVEL`
- `AXONHUB_LOG_LEVEL_KEY`
- `AXONHUB_LOG_TIME_KEY`
- `AXONHUB_LOG_CALLER_KEY`
- `AXONHUB_LOG_FUNCTION_KEY`
- `AXONHUB_LOG_NAME_KEY`
- `AXONHUB_LOG_ENCODING`
- `AXONHUB_LOG_INCLUDES`
- `AXONHUB_LOG_EXCLUDES`
- `AXONHUB_LOG_OUTPUT`
- `AXONHUB_LOG_FILE_PATH`
- `AXONHUB_LOG_FILE_MAX_SIZE`
- `AXONHUB_LOG_FILE_MAX_AGE`
- `AXONHUB_LOG_FILE_MAX_BACKUPS`
- `AXONHUB_LOG_FILE_LOCAL_TIME`

### 指标配置

```yaml
metrics:
  enabled: false                 # 启用指标收集
  exporter:
    type: "oltphttp"            # prometheus, console
    endpoint: "localhost:8080"  # 指标导出器端点
    insecure: true              # 启用不安全连接
```

**环境变量：**
- `AXONHUB_METRICS_ENABLED`
- `AXONHUB_METRICS_EXPORTER_TYPE`
- `AXONHUB_METRICS_EXPORTER_ENDPOINT`
- `AXONHUB_METRICS_EXPORTER_INSECURE`

### 垃圾回收配置

```yaml
gc:
  cron: "0 2 * * *"              # GC 执行的 Cron 表达式
```

**环境变量：**
- `AXONHUB_GC_CRON`

## 配置示例

### 开发环境配置

```yaml
server:
  port: 8090
  name: "AxonHub Dev"
  debug: true

db:
  dialect: "sqlite3"
  dsn: "file:axonhub.db?cache=shared&_fk=1"
  debug: true

log:
  level: "debug"
  encoding: "console"
  output: "stdio"
```

### 生产环境配置

```yaml
server:
  port: 8090
  name: "AxonHub Production"
  debug: false
  request_timeout: "30s"
  llm_request_timeout: "600s"

db:
  dialect: "postgres"
  dsn: "postgres://axonhub:password@localhost:5432/axonhub?sslmode=disable"
  debug: false

cache:
  mode: "redis"
  redis:
    addr: "redis:6379"
    password: "redis-password"
    expiration: "30m"

log:
  level: "warn"
  encoding: "json"
  output: "file"
  file:
    path: "/var/log/axonhub/axonhub.log"
    max_size: 200
    max_age: 14
    max_backups: 7
```

## 数据库连接字符串

### SQLite

```
file:axonhub.db?cache=shared&_fk=1
```

### PostgreSQL

```
postgres://username:password@host:5432/database?sslmode=disable
```

### MySQL

```
username:password@tcp(host:3306)/database?parseTime=True&multiStatements=true&charset=utf8mb4
```

### TiDB

```
username.root:password@tcp(host:4000)/database?tls=true&parseTime=true&multiStatements=true&charset=utf8mb4
```

## 最佳实践

### 安全

1. **对敏感信息使用环境变量**
   ```bash
   export AXONHUB_DB_DSN="postgres://axonhub:$(cat /run/secrets/db-password)@localhost:5432/axonhub"
   ```

2. **为数据库连接启用 TLS**
   ```yaml
   dsn: "postgres://user:pass@host:5432/axonhub?sslmode=verify-full"
   ```

3. **在生产环境中使用基于文件的日志**
   ```yaml
   log:
     output: "file"
     file:
       path: "/var/log/axonhub/axonhub.log"
   ```

### 性能

1. **在生产环境中使用 Redis 进行缓存**
   ```yaml
   cache:
     mode: "redis"
     redis:
       addr: "redis:6379"
       expiration: "30m"
   ```

2. **配置适当的超时时间**
   ```yaml
   server:
     request_timeout: "30s"
     llm_request_timeout: "600s"
   ```

3. **启用指标进行监控**
   ```yaml
   metrics:
     enabled: true
     exporter:
       type: "prometheus"
   ```

### 故障排除

1. **在开发环境中启用调试模式**
   ```yaml
   server:
     debug: true
   log:
     level: "debug"
   ```

2. **启用数据转储进行错误分析**
   ```yaml
   dumper:
     enabled: true
     dump_path: "./dumps"
   ```

## 验证

验证您的配置：

```bash
./axonhub config check
```

此命令将验证您的配置文件并报告任何错误。

## 相关文档

- [Docker 部署](docker.md)
- [快速入门](../getting-started/quick-start.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
