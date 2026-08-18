# Docker Deployment

## Overview

This guide covers deploying AxonHub using Docker and Docker Compose. Docker provides an isolated, reproducible environment that simplifies deployment and scaling.

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/looplj/axonhub.git
cd axonhub
```

### 2. Configure Environment

Copy the example configuration file:

```bash
cp config.example.yml config.yml
```

Edit `config.yml` with your settings:

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

### 3. Start Services

Create the `.env` file described in [Security Considerations](#security-considerations) first. The Compose file intentionally requires both image references and `DB_PASSWORD` instead of supplying mutable image or weak password defaults.

```bash
docker compose --env-file .env up -d
```

### 4. Verify Deployment

Check service status:

```bash
docker-compose ps
```

Access the application:
- Web interface: http://localhost:8090
- Default admin: admin@example.com / admin123

## Docker Compose Configuration

### Basic docker-compose.yml

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
      - AXONHUB_DB_DSN=file:axonhub.db?cache=shared&_fk=1
    restart: unless-stopped

volumes:
  axonhub_data:
```

### Production Configuration

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

## Database Options

### SQLite (Development)

```yaml
axonhub:
  environment:
    - AXONHUB_DB_DIALECT=sqlite3
    - AXONHUB_DB_DSN=file:axonhub.db?cache=shared&_fk=1&_pragma=journal_mode(WAL)
```

### PostgreSQL (Production)

```yaml
axonhub:
  environment:
    - AXONHUB_DB_DIALECT=postgres
    - AXONHUB_DB_DSN=postgres://user:pass@host:5432/axonhub
```

### MySQL (Production)

```yaml
axonhub:
  environment:
    - AXONHUB_DB_DIALECT=mysql
    - AXONHUB_DB_DSN=user:pass@tcp(host:3306)/axonhub?charset=utf8mb4&parseTime=True
```

## Environment Variables

### Server Configuration

```bash
AXONHUB_SERVER_PORT=8090
AXONHUB_SERVER_NAME="AxonHub"
AXONHUB_SERVER_DEBUG=false
AXONHUB_SERVER_REQUEST_TIMEOUT="30s"
AXONHUB_SERVER_LLM_REQUEST_TIMEOUT="600s"
```

### Database Configuration

```bash
AXONHUB_DB_DIALECT="postgres"
AXONHUB_DB_DSN="postgres://user:pass@host:5432/axonhub"
AXONHUB_DB_DEBUG=false
```

### Logging Configuration

```bash
AXONHUB_LOG_LEVEL="info"
AXONHUB_LOG_ENCODING="json"
AXONHUB_LOG_OUTPUT="stdio"
```

## Security Considerations

The repository `docker-compose.yml` applies the following production-safe defaults:

- PostgreSQL is reachable only on the private Compose network; it is not published to the host.
- AxonHub binds to `127.0.0.1` by default. Set `AXONHUB_BIND_ADDRESS` explicitly when a reverse proxy or external listener is required.
- `DB_PASSWORD` is mandatory; there is no built-in weak password fallback.
- The application runs as a non-root user with dropped Linux capabilities, `no-new-privileges`, a read-only root filesystem, a small `/tmp` tmpfs, PID/resource limits, and bounded container logs.
- The application and database use separate networks so the database cannot reach configured upstream providers.

Before starting, create a local `.env` file with restrictive permissions:

```bash
umask 077
cat > .env <<'EOF'
DB_PASSWORD=replace-with-a-long-random-password
# Replace both placeholders with the exact published image digests.
AXONHUB_IMAGE=looplj/axonhub@sha256:replace-with-axonhub-digest
POSTGRES_IMAGE=postgres@sha256:replace-with-postgres-digest
EOF
```

Use digest-pinned values for `AXONHUB_IMAGE` and `POSTGRES_IMAGE`; replace the placeholders before starting. Keep `.env` out of version control and rotate `DB_PASSWORD` according to your deployment process.

### Network Security

```yaml
axonhub:
  networks:
    - axonhub_network
  ports:
    - "127.0.0.1:8090:8090"  # Bind to localhost only

networks:
  axonhub_network:
    driver: bridge
```

### Secrets Management

The Compose file needs the database password to construct AxonHub's PostgreSQL DSN, so `DB_PASSWORD` is supplied through the local environment rather than committed to YAML. For Swarm/Kubernetes or another secret manager, inject the value at deployment time and do not store it in the repository:

```bash
# .env file
DB_PASSWORD=your-secure-password
API_KEY_SECRET=your-api-key-secret
```

```bash
chmod 600 .env
docker compose --env-file .env up -d
```

## Monitoring and Logging

### Health Checks

```yaml
axonhub:
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8090/health"]
    interval: 30s
    timeout: 10s
    retries: 3
    start_period: 40s
```

### Log Collection

```yaml
axonhub:
  logging:
    driver: "json-file"
    options:
      max-size: "10m"
      max-file: "3"
```

## Scaling

### Horizontal Scaling

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

### Load Balancer Setup

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

## Backup and Recovery

### Database Backup

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

### Volume Backup

```bash
# Backup data volume
docker run --rm -v axonhub_data:/source -v $(pwd)/backup:/backup alpine \
  tar czf /backup/axonhub-data-$(date +%Y%m%d).tar.gz -C /source .

# Restore data volume
docker run --rm -v axonhub_data:/target -v $(pwd)/backup:/backup alpine \
  tar xzf /backup/axonhub-data-20231110.tar.gz -C /target
```

## Troubleshooting

### Common Issues

**Container fails to start**
- Check Docker logs: `docker-compose logs axonhub`
- Verify configuration file permissions
- Ensure database connection is working

**Port conflicts**
- Change the exposed port in docker-compose.yml
- Check if another service is using port 8090

**Database connection issues**
- Verify database credentials
- Check network connectivity between containers
- Ensure database container is running

### Debug Mode

Enable debug logging for troubleshooting:

```yaml
axonhub:
  environment:
    - AXONHUB_SERVER_DEBUG=true
    - AXONHUB_LOG_LEVEL=debug
```

## Next Steps

- [Configuration Guide](configuration.md)
- [OpenAI API](../api-reference/openai-api.md)
- [Anthropic API](../api-reference/anthropic-api.md)
- [Gemini API](../api-reference/gemini-api.md)
- [Architecture Documentation](../development/erd.md)
