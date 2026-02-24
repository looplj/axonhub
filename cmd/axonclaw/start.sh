#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVICE_NAME="axonclaw"
BINARY_NAME="axonclaw"
PID_FILE=".axonclaw/axonclaw.pid"
LOG_FILE=".axonclaw/logs/axonclaw.log"

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

find_binary() {
    local script_dir
    script_dir=$(cd "$(dirname "$0")" && pwd)
    
    if [[ -x "$script_dir/$BINARY_NAME" ]]; then
        echo "$script_dir/$BINARY_NAME"
        return 0
    fi
    
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        command -v "$BINARY_NAME"
        return 0
    fi
    
    return 1
}

start_axonclaw() {
    print_info "Starting AxonClaw..."
    
    if [[ -f "$PID_FILE" ]]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            print_warning "AxonClaw is already running (PID: $pid)"
            return 0
        else
            print_info "Removing stale PID file"
            rm -f "$PID_FILE"
        fi
    fi
    
    local binary_path
    if ! binary_path=$(find_binary); then
        print_error "AxonClaw binary not found"
        print_info "Please build the binary first: go build -o axonclaw ./cmd/axonclaw"
        return 1
    fi
    
    print_info "Using binary: $binary_path"
    
    if [[ -z "$AXONCLAW_INSTANCE_ID" ]]; then
        print_error "AXONCLAW_INSTANCE_ID environment variable is required"
        print_info "Usage: AXONCLAW_INSTANCE_ID=<your-instance-id> ./start.sh"
        return 1
    fi
    
    if [[ -z "$AXONHUB_BASE_URL" ]]; then
        print_warning "AXONHUB_BASE_URL not set, using default: http://localhost:8090"
        export AXONHUB_BASE_URL="${AXONHUB_BASE_URL:-http://localhost:8090}"
    fi
    
    if [[ -z "$AXONHUB_API_KEY" ]]; then
        print_error "AXONHUB_API_KEY environment variable is required"
        return 1
    fi
    
    mkdir -p .axonclaw/logs
    
    print_info "Starting AxonClaw process..."
    print_info "  Instance ID: $AXONCLAW_INSTANCE_ID"
    print_info "  Base URL: $AXONHUB_BASE_URL"
    if [[ -n "$AXONCLAW_NAME" ]]; then
        print_info "  Name: $AXONCLAW_NAME"
    fi
    
    "$binary_path" \
        --base-url "$AXONHUB_BASE_URL" \
        --api-key "$AXONHUB_API_KEY" \
        --instance-id "$AXONCLAW_INSTANCE_ID" \
        ${AXONCLAW_NAME:+--name "$AXONCLAW_NAME"} \
        ${DEBUG_MODE:+--debug} \
        >> "$LOG_FILE" 2>&1 &
    
    local pid=$!
    echo "$pid" > "$PID_FILE"
    
    sleep 2
    
    if kill -0 "$pid" 2>/dev/null; then
        print_success "AxonClaw started successfully (PID: $pid)"
        print_info "Process information:"
        echo "  • PID: $pid"
        echo "  • Instance ID: $AXONCLAW_INSTANCE_ID"
        echo "  • Log file: $LOG_FILE"
        echo
        print_info "To stop AxonClaw: ./stop.sh"
        print_info "To view logs: tail -f $LOG_FILE"
    else
        print_error "AxonClaw failed to start"
        if [[ -f "$LOG_FILE" ]]; then
            print_info "Last few log lines:"
            tail -n 20 "$LOG_FILE"
        fi
        rm -f "$PID_FILE"
        return 1
    fi
}

case "${1:-}" in
    --help|-h)
        echo "Usage: $0"
        echo
        echo "Environment variables:"
        echo "  AXONCLAW_INSTANCE_ID  Required. The unique instance identifier"
        echo "  AXONHUB_BASE_URL      Optional. AxonHub server URL (default: http://localhost:8090)"
        echo "  AXONHUB_API_KEY       Required. Agent API key for authentication"
        echo "  AXONCLAW_NAME         Optional. Agent instance name"
        echo "  DEBUG_MODE            Optional. Set to 'true' to enable debug logging"
        echo
        echo "Example:"
        echo "  AXONCLAW_INSTANCE_ID=abc123 AXONHUB_API_KEY=your-key ./start.sh"
        exit 0
        ;;
    "")
        start_axonclaw
        ;;
    *)
        print_error "Unknown option: $1"
        print_info "Use --help for usage information"
        exit 1
        ;;
esac
