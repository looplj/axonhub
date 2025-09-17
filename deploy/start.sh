#!/bin/bash

# AxonHub Start Script
# This script starts AxonHub directly (no systemd), with proper error handling and logging

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="axonhub"
CONFIG_FILE="/etc/axonhub/config.yml"
BINARY_PATH="/usr/local/bin/axonhub"
PID_FILE="/var/run/axonhub.pid"
LOG_FILE="/var/log/axonhub/axonhub.log"

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

# Note: systemd-related logic removed for simplicity; this script always starts directly

start_directly() {
    print_info "Starting AxonHub directly..."
    
    # Check if already running
    if [[ -f "$PID_FILE" ]]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            print_warning "AxonHub is already running (PID: $pid)"
            return 0
        else
            print_info "Removing stale PID file"
            rm -f "$PID_FILE"
        fi
    fi
    
    # Check if binary exists
    if [[ ! -x "$BINARY_PATH" ]]; then
        print_error "AxonHub binary not found at $BINARY_PATH"
        print_info "Please run the install script first: ./deploy/install.sh"
        return 1
    fi
    
    # Check if config exists
    if [[ ! -f "$CONFIG_FILE" ]]; then
        print_warning "Configuration file not found at $CONFIG_FILE"
        print_info "Starting with default configuration..."
        CONFIG_ARGS=""
    else
        CONFIG_ARGS="--config $CONFIG_FILE"
    fi
    
    # Create log directory if it doesn't exist
    mkdir -p "$(dirname "$LOG_FILE")"
    
    # Start AxonHub in background
    print_info "Starting AxonHub process..."
    
    if [[ $EUID -eq 0 ]]; then
        # Running as root, switch to axonhub user if it exists
        if id "axonhub" &>/dev/null; then
            print_info "Running as axonhub user..."
            sudo -u axonhub "$BINARY_PATH" $CONFIG_ARGS > "$LOG_FILE" 2>&1 &
        else
            print_warning "Running as root (not recommended for production)"
            "$BINARY_PATH" $CONFIG_ARGS > "$LOG_FILE" 2>&1 &
        fi
    else
        "$BINARY_PATH" $CONFIG_ARGS > "$LOG_FILE" 2>&1 &
    fi
    
    local pid=$!
    echo "$pid" > "$PID_FILE"
    
    # Wait a moment and check if process is still running
    sleep 2
    
    if kill -0 "$pid" 2>/dev/null; then
        print_success "AxonHub started successfully (PID: $pid)"
        print_info "Process information:"
        echo "  • PID: $pid"
        echo "  • Log file: $LOG_FILE"
        echo "  • Config: ${CONFIG_FILE:-"default"}"
        echo "  • Web interface: http://localhost:8090"
        echo
        print_info "To stop AxonHub: ./deploy/stop.sh"
        print_info "To view logs: tail -f $LOG_FILE"
    else
        print_error "AxonHub failed to start"
        if [[ -f "$LOG_FILE" ]]; then
            print_info "Last few log lines:"
            tail -n 10 "$LOG_FILE"
        fi
        rm -f "$PID_FILE"
        return 1
    fi
}

check_port() {
    local port=${1:-8090}
    
    if command -v netstat >/dev/null 2>&1; then
        if netstat -tuln | grep -q ":$port "; then
            print_warning "Port $port is already in use"
            print_info "Processes using port $port:"
            netstat -tulnp | grep ":$port " || true
            return 1
        fi
    elif command -v ss >/dev/null 2>&1; then
        if ss -tuln | grep -q ":$port "; then
            print_warning "Port $port is already in use"
            print_info "Processes using port $port:"
            ss -tulnp | grep ":$port " || true
            return 1
        fi
    fi
    
    return 0
}

main() {
    print_info "Starting AxonHub..."
    
    # Check if port is available
    if ! check_port 8090; then
        print_error "Cannot start AxonHub: port 8090 is already in use"
        return 1
    fi
    
    # Always start directly
    start_directly
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0"
        echo
        echo "This script starts AxonHub directly (no systemd)."
        echo "Logs: $LOG_FILE"
        echo "PID file: $PID_FILE"
        exit 0
        ;;
    "")
        main
        ;;
    *)
        print_error "Unknown option: $1"
        print_info "Use --help for usage information"
        exit 1
        ;;
esac
