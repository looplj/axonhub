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
    
    local args=()
    
    if [[ -n "$AXONCLAW_BASE_URL" ]]; then
        args+=("--base-url" "$AXONCLAW_BASE_URL")
    fi
    
    if [[ -n "$AXONCLAW_API_KEY" ]]; then
        args+=("--api-key" "$AXONCLAW_API_KEY")
    fi
    
    if [[ -n "$AXONCLAW_AUTO_SYNC_CONFIG" ]]; then
        args+=("--auto-sync-config")
    fi
    
    if [[ -n "$DEBUG_MODE" ]]; then
        args+=("--debug")
    fi
    
    mkdir -p .axonclaw/logs

    print_info "Starting AxonClaw process..."
    if [[ -n "$AXONCLAW_BASE_URL" ]]; then
        print_info "  Base URL: $AXONCLAW_BASE_URL"
    fi
    if [[ -n "$AXONCLAW_AUTO_SYNC_CONFIG" ]]; then
        print_info "  Auto Sync Config: enabled"
    fi

    nohup "$binary_path" "${args[@]}" >> "$LOG_FILE" 2>&1 &
    
    local pid=$!
    echo "$pid" > "$PID_FILE"
    
    sleep 2
    
    if kill -0 "$pid" 2>/dev/null; then
        print_success "AxonClaw started successfully (PID: $pid)"
        print_info "Process information:"
        echo "  • PID: $pid"
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
        echo "Environment variables (all optional if config exists):"
        echo "  AXONCLAW_BASE_URL      Optional. AxonHub server URL"
        echo "  AXONCLAW_API_KEY       Optional. Agent API key for authentication"
        echo "  AXONCLAW_AUTO_SYNC_CONFIG Optional. Set to 'true' to enable --auto-sync-config"
        echo "  DEBUG_MODE             Optional. Set to 'true' to enable debug logging"
        echo
        echo "Example (first start):"
        echo "  AXONCLAW_API_KEY=your-key ./start.sh"
        echo
        echo "Example (subsequent starts with saved config):"
        echo "  ./start.sh"
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
