#!/bin/bash

# E2E Backend Server Management Script
# This script manages the backend server for E2E testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
E2E_DB="${SCRIPT_DIR}/axonhub-e2e.db"
E2E_PORT=8099
BINARY_NAME="axonhub-e2e"
BINARY_PATH="${SCRIPT_DIR}/${BINARY_NAME}"
PID_FILE="${SCRIPT_DIR}/.e2e-backend.pid"
LOG_FILE="${SCRIPT_DIR}/e2e-backend.log"
DB_TYPE_FILE="${SCRIPT_DIR}/.e2e-backend-db-type"
DB_TYPE="${AXONHUB_E2E_DB_TYPE:-sqlite}"
MYSQL_CONTAINER="axonhub-e2e-mysql"
MYSQL_PORT=13306
MYSQL_ROOT_PASSWORD="axonhub_test_root"
MYSQL_DATABASE="axonhub_e2e"
MYSQL_USER="axonhub"
MYSQL_PASSWORD="axonhub_test"
POSTGRES_CONTAINER="axonhub-e2e-postgres"
POSTGRES_PORT=15432
POSTGRES_DATABASE="axonhub_e2e"
POSTGRES_USER="axonhub"
POSTGRES_PASSWORD="axonhub_test"
USE_EXISTING_DB="${AXONHUB_E2E_USE_EXISTING_DB:-false}"

check_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "Docker is required for ${DB_TYPE} database." >&2
    exit 1
  fi

  if ! docker info >/dev/null 2>&1; then
    echo "Docker daemon is not running." >&2
    exit 1
  fi
}

setup_mysql() {
  check_docker

  if docker ps -a --format '{{.Names}}' | grep -q "^${MYSQL_CONTAINER}$"; then
    docker rm -f "$MYSQL_CONTAINER" >/dev/null 2>&1 || true
  fi

  docker run -d \
    --name "$MYSQL_CONTAINER" \
    -e MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
    -e MYSQL_DATABASE="$MYSQL_DATABASE" \
    -e MYSQL_USER="$MYSQL_USER" \
    -e MYSQL_PASSWORD="$MYSQL_PASSWORD" \
    -p "${MYSQL_PORT}:3306" \
    mysql:8.0 \
    --character-set-server=utf8mb4 \
    --collation-server=utf8mb4_unicode_ci \
    >/dev/null

  for i in {1..30}; do
    if docker exec "$MYSQL_CONTAINER" mysqladmin ping -h localhost -u root -p"$MYSQL_ROOT_PASSWORD" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  docker logs "$MYSQL_CONTAINER" >&2 || true
  echo "MySQL failed to start." >&2
  exit 1
}

setup_postgres() {
  check_docker

  if docker ps -a --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
    docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
  fi

  docker run -d \
    --name "$POSTGRES_CONTAINER" \
    -e POSTGRES_DB="$POSTGRES_DATABASE" \
    -e POSTGRES_USER="$POSTGRES_USER" \
    -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
    -p "${POSTGRES_PORT}:5432" \
    postgres:15-alpine \
    >/dev/null

  for i in {1..30}; do
    if docker exec "$POSTGRES_CONTAINER" pg_isready -U "$POSTGRES_USER" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  docker logs "$POSTGRES_CONTAINER" >&2 || true
  echo "PostgreSQL failed to start." >&2
  exit 1
}

cleanup_database() {
  local type="$1"

  case "$type" in
    mysql)
      if [ "$USE_EXISTING_DB" != "true" ] && [ "${AXONHUB_E2E_KEEP_DB:-false}" != "true" ] && command -v docker >/dev/null 2>&1; then
        docker rm -f "$MYSQL_CONTAINER" >/dev/null 2>&1 || true
      fi
      ;;
    postgres)
      if [ "$USE_EXISTING_DB" != "true" ] && [ "${AXONHUB_E2E_KEEP_DB:-false}" != "true" ] && command -v docker >/dev/null 2>&1; then
        docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
      fi
      ;;
    sqlite)
      if [ "${AXONHUB_E2E_KEEP_DB:-false}" != "true" ]; then
        rm -f "$E2E_DB"
      fi
      ;;
  esac
}

load_db_type() {
  if [ -f "$DB_TYPE_FILE" ]; then
    DB_TYPE=$(cat "$DB_TYPE_FILE")
  else
    DB_TYPE="${AXONHUB_E2E_DB_TYPE:-sqlite}"
  fi
}

cd "$SCRIPT_DIR"

case "${1:-}" in
  start)
    echo "Starting E2E backend server..."
    
    # Check if server is already running
    if [ -f "$PID_FILE" ]; then
      PID=$(cat "$PID_FILE")
      if ps -p "$PID" > /dev/null 2>&1; then
        echo "E2E backend server is already running (PID: $PID)"
        exit 0
      else
        echo "Removing stale PID file"
        rm -f "$PID_FILE"
      fi
    fi
    
    load_db_type

    case "$DB_TYPE" in
      sqlite)
        if [ -f "$E2E_DB" ]; then
          echo "Removing old E2E database: $E2E_DB"
          rm -f "$E2E_DB"
        fi
        DB_DIALECT="sqlite3"
        DB_DSN="file:${E2E_DB}?cache=shared&_fk=1"
        ;;
      mysql)
        if [ "$USE_EXISTING_DB" = "true" ]; then
          echo "Using existing MySQL database for E2E..."
        else
          echo "Preparing MySQL database for E2E..."
          setup_mysql
        fi
        DB_DIALECT="${AXONHUB_E2E_DB_DIALECT:-mysql}"
        if [ -n "${AXONHUB_E2E_DB_DSN:-}" ]; then
          DB_DSN="$AXONHUB_E2E_DB_DSN"
        else
          DB_DSN="${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(localhost:${MYSQL_PORT})/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=True&loc=Local"
        fi
        ;;
      postgres)
        if [ "$USE_EXISTING_DB" = "true" ]; then
          echo "Using existing PostgreSQL database for E2E..."
        else
          echo "Preparing PostgreSQL database for E2E..."
          setup_postgres
        fi
        DB_DIALECT="${AXONHUB_E2E_DB_DIALECT:-postgres}"
        if [ -n "${AXONHUB_E2E_DB_DSN:-}" ]; then
          DB_DSN="$AXONHUB_E2E_DB_DSN"
        else
          DB_DSN="host=localhost port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DATABASE} sslmode=disable"
        fi
        ;;
      *)
        echo "Unsupported E2E database type: $DB_TYPE"
        exit 1
        ;;
    esac

    # Build backend if binary doesn't exist or is older than 30 minutes
    SHOULD_BUILD=false
    if [ ! -f "$BINARY_PATH" ]; then
      echo "Binary not found, will build..."
      SHOULD_BUILD=true
    else
      # Check if binary is older than 30 minutes (1800 seconds)
      CURRENT_TIME=$(date +%s)
      BINARY_TIME=$(stat -f %m "$BINARY_PATH" 2>/dev/null || stat -c %Y "$BINARY_PATH" 2>/dev/null)
      AGE=$((CURRENT_TIME - BINARY_TIME))
      
      if [ $AGE -gt 1800 ]; then
        echo "Binary is older than 30 minutes (age: $((AGE / 60)) minutes), will rebuild..."
        SHOULD_BUILD=true
      fi
    fi
    
    if [ "$SHOULD_BUILD" = true ]; then
      echo "Building E2E backend..."
      cd "$PROJECT_ROOT"
      go build -o "$BINARY_PATH" ./cmd/axonhub
      cd "$SCRIPT_DIR"
    fi
    
    echo "Using $DB_DSN database for E2E..."
    # Start backend server with E2E configuration
    echo "Starting backend on port $E2E_PORT using $DB_TYPE database..."
    AXONHUB_SERVER_PORT=$E2E_PORT \
    AXONHUB_DB_DIALECT="$DB_DIALECT" \
    AXONHUB_DB_DSN="$DB_DSN" \
    AXONHUB_LOG_OUTPUT="stdio" \
    AXONHUB_LOG_LEVEL="debug" \
    AXONHUB_LOG_ENCODING="console" \
    nohup "$BINARY_PATH" > "$LOG_FILE" 2>&1 &
    
    BACKEND_PID=$!
    echo $BACKEND_PID > "$PID_FILE"
    echo "$DB_TYPE" > "$DB_TYPE_FILE"

    echo "E2E backend server started (PID: $BACKEND_PID)"
    echo "Waiting for server to be ready..."
    
    # Wait for server to be ready (max 30 seconds)
    for i in {1..30}; do
      if curl -s "http://localhost:$E2E_PORT/health" > /dev/null 2>&1 || \
         curl -s "http://localhost:$E2E_PORT/" > /dev/null 2>&1; then
        echo "E2E backend server is ready!"
        exit 0
      fi
      sleep 1
    done
    
    echo "Warning: Server may not be ready yet. Check $LOG_FILE for details."
    exit 0
    ;;
    
  stop)
    echo "Stopping E2E backend server..."

    if [ ! -f "$PID_FILE" ]; then
      echo "No PID file found. Server may not be running."
      exit 0
    fi

    load_db_type

    PID=$(cat "$PID_FILE")

    if ps -p "$PID" > /dev/null 2>&1; then
      echo "Stopping server (PID: $PID)..."
      kill "$PID"
      
      # Wait for process to stop
      for i in {1..10}; do
        if ! ps -p "$PID" > /dev/null 2>&1; then
          break
        fi
        sleep 1
      done
      
      # Force kill if still running
      if ps -p "$PID" > /dev/null 2>&1; then
        echo "Force killing server..."
        kill -9 "$PID"
      fi
      
      echo "E2E backend server stopped"
    else
      echo "Server process not found (PID: $PID)"
    fi

    rm -f "$PID_FILE"

    if [ "${AXONHUB_E2E_KEEP_DB:-false}" != "true" ]; then
      cleanup_database "$DB_TYPE"
      rm -f "$DB_TYPE_FILE"
    else
      echo "Database preserved (--keep-db flag)"
      rm -f "$DB_TYPE_FILE"
    fi
    ;;
    
  restart)
    "$0" stop
    sleep 2
    "$0" start
    ;;
    
  status)
    if [ -f "$PID_FILE" ]; then
      PID=$(cat "$PID_FILE")
      if ps -p "$PID" > /dev/null 2>&1; then
        echo "E2E backend server is running (PID: $PID)"
        exit 0
      else
        echo "E2E backend server is not running (stale PID file)"
        exit 1
      fi
    else
      echo "E2E backend server is not running"
      exit 1
    fi
    ;;
    
  clean)
    echo "Cleaning E2E artifacts..."
    "$0" stop
    load_db_type
    cleanup_database "$DB_TYPE"
    rm -f "$E2E_DB" "$LOG_FILE" "$BINARY_PATH" "$DB_TYPE_FILE"
    echo "E2E artifacts cleaned"
    ;;
    
  *)
    echo "Usage: $0 {start|stop|restart|status|clean}"
    echo ""
    echo "Commands:"
    echo "  start   - Start E2E backend server (removes old DB, builds if needed)"
    echo "  stop    - Stop E2E backend server"
    echo "  restart - Restart E2E backend server"
    echo "  status  - Check E2E backend server status"
    echo "  clean   - Stop server and remove E2E database and logs"
    exit 1
    ;;
esac
