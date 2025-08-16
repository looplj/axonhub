# AxonHub Configuration Guide

AxonHub uses [Viper](https://github.com/spf13/viper) for configuration management, supporting both YAML configuration files and environment variables.

## Configuration Sources

Configuration is loaded in the following priority order (highest to lowest):

1. **Environment Variables** (highest priority)
2. **Configuration File** (`config.yml`)
3. **Default Values** (lowest priority)

## Configuration File Locations

AxonHub searches for configuration files in the following locations:

1. Current directory (`./config.yml`)
2. `./conf/config.yml`
3. `/etc/axonhub/config.yml`
4. `$HOME/.axonhub/config.yml`

## Configuration Structure

### Server Configuration

```yaml
server:
  port: 8090                    # Server port
  name: "AxonHub"               # Server name
  base_path: ""                 # Base path for API routes
  request_timeout: "30s"        # Request timeout duration
  debug: false                  # Enable debug mode
```

### Database Configuration

```yaml
db:
  dialect: "sqlite3"            # Database dialect: sqlite3, postgres, mysql
  dsn: "file:axonhub.db?cache=shared&_fk=1"  # Database connection string
  debug: false                  # Enable database debug logging
```

#### Database DSN Examples

**SQLite:**
```yaml
dsn: "file:axonhub.db?cache=shared&_fk=1"
```

**PostgreSQL:**
```yaml
dsn: "postgres://user:password@localhost:5432/axonhub?sslmode=disable"
```

**MySQL:**
```yaml
dsn: "user:password@tcp(localhost:3306)/axonhub?charset=utf8mb4&parseTime=True&loc=Local"
```

### Logging Configuration

```yaml
log:
  name: "axonhub"               # Logger name
  debug: false                  # Enable debug logging
  skip_level: 1                 # Skip levels for caller info
  level: "info"                 # Log level: debug, info, warn, error, panic, fatal
  level_key: "level"            # Key name for log level field
  time_key: "time"              # Key name for timestamp field
  caller_key: "label"           # Key name for caller info field
  function_key: ""              # Key name for function field
  name_key: "logger"            # Key name for logger name field
  encoding: "json"              # Log encoding: json, console, console_json
  includes: []                  # Logger names to include
  excludes: []                  # Logger names to exclude
```

#### Log Levels

- `debug`: Detailed information for debugging
- `info`: General information (default)
- `warn`: Warning messages
- `error`: Error messages
- `panic`: Panic messages (will panic after logging)
- `fatal`: Fatal messages (will exit after logging)

#### Log Encodings

- `json`: JSON format (default, recommended for production)
- `console`: Human-readable console format
- `console_json`: JSON format with console-friendly output

## Environment Variables

All configuration options can be overridden using environment variables with the `AXONHUB_` prefix. Nested configuration keys use underscores (`_`) as separators.

### Environment Variable Examples

```bash
# Server configuration
export AXONHUB_SERVER_PORT=8080
export AXONHUB_SERVER_NAME="MyAxonHub"
export AXONHUB_SERVER_DEBUG=true

# Database configuration
export AXONHUB_DB_DIALECT="postgres"
export AXONHUB_DB_DSN="postgres://user:pass@localhost/axonhub?sslmode=disable"
export AXONHUB_DB_DEBUG=true

# Logging configuration
export AXONHUB_LOG_LEVEL="debug"
export AXONHUB_LOG_ENCODING="console"
export AXONHUB_LOG_DEBUG=true
```

### Environment Variable Mapping

| Configuration Key | Environment Variable |
|-------------------|---------------------|
| `server.port` | `AXONHUB_SERVER_PORT` |
| `server.name` | `AXONHUB_SERVER_NAME` |
| `server.base_path` | `AXONHUB_SERVER_BASE_PATH` |
| `server.request_timeout` | `AXONHUB_SERVER_REQUEST_TIMEOUT` |
| `server.debug` | `AXONHUB_SERVER_DEBUG` |
| `db.dialect` | `AXONHUB_DB_DIALECT` |
| `db.dsn` | `AXONHUB_DB_DSN` |
| `db.debug` | `AXONHUB_DB_DEBUG` |
| `log.name` | `AXONHUB_LOG_NAME` |
| `log.debug` | `AXONHUB_LOG_DEBUG` |
| `log.level` | `AXONHUB_LOG_LEVEL` |
| `log.encoding` | `AXONHUB_LOG_ENCODING` |

## Getting Started

1. **Copy the example configuration:**
   ```bash
   cp config.example.yml config.yml
   ```

2. **Edit the configuration file:**
   ```bash
   nano config.yml
   ```

3. **Or use environment variables:**
   ```bash
   export AXONHUB_SERVER_PORT=8080
   export AXONHUB_LOG_LEVEL=debug
   ```

4. **Start AxonHub:**
   ```bash
   ./axonhub
   ```

## Configuration Validation

AxonHub validates configuration on startup and will panic with descriptive error messages if:

- Invalid log levels are specified
- Required configuration is missing
- Configuration file syntax is invalid

## Docker Configuration

When running AxonHub in Docker, you can:

1. **Mount a configuration file:**
   ```bash
   docker run -v /path/to/config.yml:/app/config.yml axonhub
   ```

2. **Use environment variables:**
   ```bash
   docker run -e AXONHUB_SERVER_PORT=8080 -e AXONHUB_LOG_LEVEL=debug axonhub
   ```

## Production Recommendations

- Use environment variables for sensitive configuration (database passwords, API keys)
- Set `log.encoding` to `json` for structured logging
- Set `log.level` to `info` or `warn` in production
- Enable `db.debug` only for troubleshooting
- Use external databases (PostgreSQL/MySQL) instead of SQLite for production