# Configuration Guide

## Overview

AxonHub uses a flexible configuration system that supports both YAML configuration files and environment variables. This guide covers all available configuration options and best practices for different deployment scenarios.

## Configuration Methods

### Configuration Priority

AxonHub uses Viper for configuration management, which can read from multiple configuration sources and merges them together into one set of configuration keys and values. Viper uses the following precedence for merging (highest to lowest):

1. **Environment variables** - System environment variables
2. **Config files** - YAML configuration files
3. **External key/value stores** - External configuration stores
4. **Defaults** - Built-in default values

This means that environment variables will override values in the configuration file, and command line flags will override environment variables.

### 1. YAML Configuration File

Create a `config.yml` file:

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

### 2. Environment Variables

All configuration options can be set via environment variables:

```bash
export AXONHUB_SERVER_PORT=8090
export AXONHUB_DB_DIALECT="sqlite3"
export AXONHUB_DB_DSN="file:axonhub.db?cache=shared&_fk=1"
export AXONHUB_LOG_LEVEL="info"
```

### 3. Mixed Configuration

Environment variables override YAML configuration values.

## Configuration Reference

### Server Configuration

```yaml
server:
  port: 8090                    # Server port
  name: "AxonHub"               # Server name
  base_path: ""                 # Base path for API routes
  request_timeout: "30s"        # Request timeout duration
  llm_request_timeout: "600s"   # LLM request timeout duration
  trace:
    thread_header: "AH-Thread-Id" # Thread ID header name
    trace_header: "AH-Trace-Id" # Trace ID header name
    extra_trace_headers: []     # Extra trace headers
    claude_code_trace_enabled: false # Enable Claude Code trace extraction
    codex_trace_enabled: false # Enable Codex trace extraction
  debug: false                  # Enable debug mode
```

**Environment Variables:**
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

### Database Configuration

```yaml
db:
  dialect: "sqlite3"            # sqlite3, postgres, mysql, tidb
  dsn: "file:axonhub.db?cache=shared&_fk=1"  # Connection string
  debug: false                  # Enable database debug logging
```

**Supported Databases:**
- **SQLite**: `sqlite3` (development)
- **PostgreSQL**: `postgres` (production)
- **MySQL**: `mysql` (production)
- **TiDB**: `tidb` (production/cloud)

**Environment Variables:**
- `AXONHUB_DB_DIALECT`
- `AXONHUB_DB_DSN`
- `AXONHUB_DB_DEBUG`

### Cache Configuration

```yaml
cache:
  mode: "memory"                # memory, redis, two-level

  # Memory cache configuration
  memory:
    expiration: "5s"            # Memory cache TTL
    cleanup_interval: "10m"     # Memory cache cleanup interval

  # Redis cache configuration
  redis:
    url: ""                     # Redis connection URL(redis:// or rediss://)
    addr: ""                    # Redis address: 127.0.0.1:6379
    username: ""                # Overrides username in URL if set
    password: ""                # Overrides password in URL if set
    db: 0                       # Overrides DB in URL path (/0)
    tls: false                  # Enable TLS (also auto-enabled for rediss://)
    tls_insecure_skip_verify: false # Skip TLS cert verification (self-signed)
    expiration: "30m"           # Redis cache TTL
```

**Environment Variables:**
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

### Logging Configuration

```yaml
log:
  name: "axonhub"               # Logger name
  debug: false                  # Enable debug logging
  level: "info"                 # debug, info, warn, error, panic, fatal
  level_key: "level"            # Key name for log level field
  time_key: "time"              # Key name for timestamp field
  caller_key: "label"           # Key name for caller info field
  function_key: ""              # Key name for function field
  name_key: "logger"            # Key name for logger name field
  encoding: "json"              # json, console, console_json
  includes: []                  # Logger names to include
  excludes: []                  # Logger names to exclude
  output: "stdio"               # file or stdio
  file:                         # File-based logging
    path: "logs/axonhub.log"   # Log file path
    max_size: 100               # Max size in MB before rotation
    max_age: 30                 # Max age in days to retain
    max_backups: 10             # Max number of old log files
    local_time: true            # Use local time for rotated files
```

**Environment Variables:**
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

### Metrics Configuration

```yaml
metrics:
  enabled: false                 # Enable metrics collection
  exporter:
    type: "oltphttp"            # prometheus, console
    endpoint: "localhost:8080"  # Metrics exporter endpoint
    insecure: true              # Enable insecure connection
```

**Environment Variables:**
- `AXONHUB_METRICS_ENABLED`
- `AXONHUB_METRICS_EXPORTER_TYPE`
- `AXONHUB_METRICS_EXPORTER_ENDPOINT`
- `AXONHUB_METRICS_EXPORTER_INSECURE`

### Garbage Collection Configuration

```yaml
gc:
  cron: "0 2 * * *"              # Cron expression for GC execution
```

**Environment Variables:**
- `AXONHUB_GC_CRON`

## Configuration Examples

### Development Configuration

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

### Production Configuration

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

## Database Connection Strings

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

### Tidb 

```
<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true&parseTime=true&multiStatements=true&charset=utf8mb4
```

## Best Practices

### Security

1. **Use environment variables for secrets**
   ```bash
   export AXONHUB_DB_DSN="postgres://axonhub:$(cat /run/secrets/db-password)@localhost:5432/axonhub"
   ```

2. **Enable TLS for database connections**
   ```yaml
   dsn: "postgres://user:pass@host:5432/axonhub?sslmode=verify-full"
   ```

3. **Use file-based logging in production**
   ```yaml
   log:
     output: "file"
     file:
       path: "/var/log/axonhub/axonhub.log"
   ```

### Performance

1. **Use Redis for caching in production**
   ```yaml
   cache:
     mode: "redis"
     redis:
       addr: "redis:6379"
       expiration: "30m"
   ```

2. **Configure appropriate timeouts**
   ```yaml
   server:
     request_timeout: "30s"
     llm_request_timeout: "600s"
   ```

3. **Enable metrics for monitoring**
   ```yaml
   metrics:
     enabled: true
     exporter:
       type: "prometheus"
   ```

### Troubleshooting

1. **Enable debug mode for development**
   ```yaml
   server:
     debug: true
   log:
     level: "debug"
   ```

2. **Enable data dumping for error analysis**
   ```yaml
   dumper:
     enabled: true
     dump_path: "./dumps"
   ```

## Validation

Validate your configuration:

```bash
./axonhub config check
```

This command will validate your configuration file and report any errors.

## Related Documentation

- [Docker Deployment](docker.md)
- [Quick Start Guide](../getting-started/quick-start.md)
- [API Reference](../api-reference/unified-api.md)
