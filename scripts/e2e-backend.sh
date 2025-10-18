#!/bin/bash

# E2E Backend Server Management Script
# This script manages the backend server for E2E testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
E2E_DB="${SCRIPT_DIR}/axonhub-e2e.db"
E2E_PORT=8099
BINARY_NAME="axonhub-e2e"
BINARY_PATH="${SCRIPT_DIR}/${BINARY_NAME}"
PID_FILE="${SCRIPT_DIR}/.e2e-backend.pid"
LOG_FILE="${SCRIPT_DIR}/e2e-backend.log"

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
    
    # Remove old E2E database
    if [ -f "$E2E_DB" ]; then
      echo "Removing old E2E database: $E2E_DB"
      rm -f "$E2E_DB"
    fi
    
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
    
    # Start backend server with E2E configuration
    echo "Starting backend on port $E2E_PORT with database $E2E_DB..."
    AXONHUB_SERVER_PORT=$E2E_PORT \
    AXONHUB_DB_DSN="file:${E2E_DB}?cache=shared&_fk=1" \
    AXONHUB_LOG_OUTPUT="stdio" \
    AXONHUB_LOG_LEVEL="debug" \
    AXONHUB_LOG_ENCODING="console" \
    nohup "$BINARY_PATH" > "$LOG_FILE" 2>&1 &
    
    BACKEND_PID=$!
    echo $BACKEND_PID > "$PID_FILE"
    
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
    rm -f "$E2E_DB" "$LOG_FILE" "$BINARY_PATH"
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
