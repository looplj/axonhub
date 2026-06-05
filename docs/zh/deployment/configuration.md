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
  dsn: "file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)"

log:
  level: "info"
  encoding: "json"
```

### 2. 环境变量

所有配置选项都可以通过环境变量设置：

```bash
export AXONHUB_SERVER_PORT=8090
export AXONHUB_DB_DIALECT="sqlite3"
export AXONHUB_DB_DSN="file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)"
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
  disable_ssl_verify: false     # 禁用上游请求的 SSL 证书校验（自签名证书）
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
- `AXONHUB_SERVER_DISABLE_SSL_VERIFY`

### 数据库配置

```yaml
db:
  dialect: "sqlite3"            # sqlite3, postgres, mysql, tidb
  dsn: "file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)"  # 主库连接字符串
  debug: false                  # 启用数据库调试日志
  read_replica:
    read_dsn: ""                # 从库连接字符串（留空则禁用读写分离，所有查询走主库）
    read_max_open_conns: 0      # 从库最大打开连接数（0 表示使用默认值）
    read_max_idle_conns: 0      # 从库最大空闲连接数（0 表示使用默认值）
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
- `AXONHUB_DB_READ_REPLICA_READ_DSN`
- `AXONHUB_DB_READ_REPLICA_READ_MAX_OPEN_CONNS`
- `AXONHUB_DB_READ_REPLICA_READ_MAX_IDLE_CONNS`

#### 读写分离

当配置了 `read_replica.read_dsn` 时，AxonHub 会自动根据 SQL 语句类型分流：

| 操作类型 | 目标 | 示例 |
|----------|------|------|
| 读（SELECT/WITH 等） | 从库 | 查询、列表、统计 |
| 写（INSERT/UPDATE/DELETE 等） | 主库 | 创建、更新、删除 |
| 事务（Tx） | 主库 | 所有事务操作强制走主库，避免复制延迟 |

**示例（PostgreSQL）：**
```yaml
db:
  dialect: "postgres"
  dsn: "postgres://axonhub:password@master.db:5432/axonhub?sslmode=disable"
  read_replica:
    read_dsn: "postgres://axonhub:password@replica.db:5432/axonhub?sslmode=disable"
```

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
    addr: ""                    # Redis 地址(已弃用): 127.0.0.1:6379
    addrs:                      # Redis 地址，支持standalone、sentinel和cluster模式
      - 127.0.0.1:7000
      - 127.0.0.1:7001
      - 127.0.0.1:7002
    username: ""                # 如果设置，将覆盖 URL 中的用户名
    password: ""                # 如果设置，将覆盖 URL 中的密码
    master_name: "mymaster"     # Redis Sentinel模式的 MasterName
    sentinel_username: ""       # Redis Sentinel模式的 Sentinel用户名
    sentinel_password: ""       # Redis Sentinel模式的 Sentinel密码
    is_cluster_mode: false      # Redis Cluster模式的 开启标识（如果addrs配置多个地址时则无需配置）
    route_randomly: false       # Redis Cluster模式的 路由策略-随机
    route_by_latency: false     # Redis Cluster模式的 路由策略-延迟优先
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
- `AXONHUB_CACHE_REDIS_ADDRS`
- `AXONHUB_CACHE_REDIS_USERNAME`
- `AXONHUB_CACHE_REDIS_PASSWORD`
- `AXONHUB_CACHE_REDIS_MASTER_NAME`
- `AXONHUB_CACHE_REDIS_SENTINEL_USERNAME`
- `AXONHUB_CACHE_REDIS_SENTINEL_PASSWORD`
- `AXONHUB_CACHE_REDIS_ROUTE_BY_LATENCY`
- `AXONHUB_CACHE_REDIS_ROUTE_RANDOMLY`
- `AXONHUB_CACHE_REDIS_IS_CLUSTER_MODE`
- `AXONHUB_CACHE_REDIS_DB`
- `AXONHUB_CACHE_REDIS_TLS`
- `AXONHUB_CACHE_REDIS_TLS_INSECURE_SKIP_VERIFY`
- `AXONHUB_CACHE_REDIS_EXPIRATION`

#### 使用URL配置更多参数
**standalone模式标准URL**
```
redis://127.0.0.1:6379/0
```

**sentinel模式标准URL**
```
redis://?master_name=mymaster&addrs=127.0.0.1:26379&addrs=127.0.0.1:26380&addrs=127.0.0.1:26381
```

**cluster模式标准URL**
```
redis://?addrs=127.0.0.1:7000&addrs=127.0.0.1:7001&addrs=127.0.0.1:7002
或
redis://127.0.0.1:7000?is_cluster_mode=true
```

**参数说明**
| 参数 | 说明 | 示例 |
|------|------|------|
| addrs | 指定多个地址，格式为 addrs=host:port，可重复 | addrs=127.0.0.1:7000&addrs=127.0.0.1:7001 |
| client_name | 客户端名称，会设置为 Redis 客户端的 ClientName | client_name=axonhub |
| db | 指定 Redis DB 序号（数字） | db=1 或 在路径中 /1 |
| protocol | 协议版本（整型，库内部使用） | protocol=3 |
| username | 连接用户名（用于 ACL） | username=default |
| password | 连接密码 | password=secret |
| sentinel_username | Sentinel 认证用户名 | sentinel_username=sentineluser |
| sentinel_password | Sentinel 认证密码 | sentinel_password=sentinelpass |
| max_retries | 最大重试次数 | max_retries=3 |
| min_retry_backoff | 重试的最小退避时间，支持 s/ms 等单位或秒整数 | min_retry_backoff=100ms |
| max_retry_backoff | 重试的最大退避时间 | max_retry_backoff=2s |
| dial_timeout | 建立连接超时 | dial_timeout=5s |
| read_timeout | 读超时 | read_timeout=3s |
| write_timeout | 写超时 | write_timeout=3s |
| context_timeout_enabled | 是否启用基于 context 的超时（true/false） | context_timeout_enabled=true |
| read_buffer_size | 读缓冲区大小（字节） | read_buffer_size=4096 |
| write_buffer_size | 写缓冲区大小（字节） | write_buffer_size=4096 |
| pool_fifo | 连接池是否 FIFO（true/false） | pool_fifo=true |
| pool_size | 连接池大小 | pool_size=10 |
| pool_timeout | 获取连接的超时 | pool_timeout=4s |
| min_idle_conns | 保持的最小空闲连接数 | min_idle_conns=2 |
| max_idle_conns | 最大空闲连接数 | max_idle_conns=10 |
| max_active_conns | 最大活动连接数（客户端特定） | max_active_conns=0 |
| conn_max_lifetime | 连接最大生命周期 | conn_max_lifetime=30m |
| conn_max_idle_time | 连接最大空闲时间 | conn_max_idle_time=5m |
| max_redirects | 最大重定向次数（cluster） | max_redirects=8 |
| read_only | 只读模式（true/false） | read_only=true |
| route_by_latency | 按延迟路由（true/false） | route_by_latency=true |
| route_randomly | 随机路由（true/false） | route_randomly=true |
| master_name | sentinel 模式下的 master 名称 | master_name=mymaster |
| disable_identity | 禁用客户端标识（true/false） | disable_identity=true |
| identity_suffix | 客户端标识后缀 | identity_suffix=-axonhub |
| failing_timeout_seconds | 失败检测超时（秒） | failing_timeout_seconds=30 |
| unstable_resp3 | 使用不稳定的 RESP3（true/false） | unstable_resp3=true |
| is_cluster_mode | 强制集群模式（true/false） | is_cluster_mode=true |
| tls_insecure_skip_verify | TLS 跳过验证（true/false）| tls_insecure_skip_verify=true |

> **提示：** 当 `addr`、`addrs` 配置项存在，并同时配置了`url`中的`host`和`addrs`参数，他们的取值顺序依次是: `addrs` > `addr` > `url`中的`addrs` > `url`中的`host`。取值优先级为：配置项 > URL 参数部分 > URL 主体部分。


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

### GitHub Copilot OAuth 配置

```yaml
copilot:
  client_id: ""                   # 自定义 GitHub OAuth 客户端 ID（可选）
```

**描述：**
配置用于 GitHub Copilot 设备流程认证的 OAuth 客户端 ID。默认情况下，AxonHub 使用 VS Code 的公共客户端 ID。对于生产部署或为了遵守 GitHub 的服务条款，您应该注册自己的 OAuth 应用程序并配置自定义客户端 ID。

**环境变量：**
- `GITHUB_COPILOT_CLIENT_ID`

**默认值：** VS Code 公共客户端 ID（用于向后兼容）

**何时自定义：**
- **生产部署：** 注册您自己的 GitHub OAuth 应用程序以完全控制 OAuth 设置
- **合规性：** 使用您自己的客户端 ID 确保遵守 GitHub 的服务条款
- **速率限制：** 拥有自己的 OAuth 应用程序可以获得专用的速率限制

**如何注册您自己的 OAuth 应用程序：**
1. 前往 GitHub 设置 → 开发者设置 → OAuth 应用程序
2. 点击"新建 OAuth 应用程序"
3. 填写应用程序详细信息：
   - 应用程序名称：`您的 AxonHub 实例`
   - 主页 URL：`https://your-axonhub-domain.com`
   - 授权回调 URL：`https://your-axonhub-domain.com/api/copilot/oauth/callback`
4. 点击"注册应用程序"
5. 复制客户端 ID 并设置为环境变量

**示例：**
```yaml
copilot:
  client_id: "Iv1.your-custom-client-id"
```

```bash
export GITHUB_COPILOT_CLIENT_ID="Iv1.your-custom-client-id"
```

## 配置示例

### 开发环境配置

```yaml
server:
  port: 8090
  name: "AxonHub Dev"
  debug: true

db:
  dialect: "sqlite3"
  dsn: "file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)"
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
    # standalone模式
    addrs: 
      - "redis:6379"
    password: "redis-password"
    expiration: "30m"

    # sentinel模式
    addrs:
      - "redis:26379"
      - "redis:26380"
      - "redis:26381"
    master_name: mymaster
    password: "redis-password"
    sentinel_password: "sentinel-password"

    # cluster模式
    addrs:
      - "redis:7000"
      - "redis:7001"
      - "redis:7002"
    password: "redis-password"


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
   
   **standalone模式**
   ```yaml
   cache:
     mode: "redis"
     redis:
       addrs: 
         - "redis:6379"
       expiration: "30m"
   ```

   **sentinel模式**
   ```yaml
   cache:
     mode: "redis"
     redis:
       addrs:
         - "redis:26379"
         - "redis:26380"
         - "redis:26381"
       master_name: mymaster
       expiration: "30m"
   ```

   **cluster模式**
   ```yaml
   cache:
     mode: "redis"
     redis:
       addrs:
         - "redis:7000"
         - "redis:7001"
         - "redis:7002"
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
