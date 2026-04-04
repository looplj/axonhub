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

restart_axonclaw() {
    print_info "Restarting AxonClaw..."
    
    local script_dir
    script_dir=$(cd "$(dirname "$0")" && pwd)
    
    print_info "Stopping AxonClaw..."
    if [[ -x "$script_dir/stop.sh" ]]; then
        "$script_dir/stop.sh" || print_warning "Stop script returned non-zero, continuing..."
    else
        print_warning "stop.sh not found or not executable"
    fi
    
    sleep 1
    
    print_info "Starting AxonClaw..."
    if [[ -x "$script_dir/start.sh" ]]; then
        "$script_dir/start.sh"
    else
        print_error "start.sh not found or not executable"
        exit 1
    fi
}

case "${1:-}" in
    --help|-h)
        echo "Usage: $0"
        echo
        echo "Restart AxonClaw by stopping and starting the service."
        echo
        echo "Environment variables (passed to start.sh):"
        echo "  AXONCLAW_BASE_URL      Optional. AxonHub server URL"
        echo "  AXONCLAW_API_KEY       Optional. Agent API key for authentication"
        echo "  AXONCLAW_AUTO_SYNC_CONFIG Optional. Set to 'true' to enable --auto-sync-config"
        echo "  DEBUG_MODE             Optional. Set to 'true' to enable debug logging"
        echo
        echo "Example:"
        echo "  ./restart.sh"
        exit 0
        ;;
    "")
        restart_axonclaw
        ;;
    *)
        print_error "Unknown option: $1"
        print_info "Use --help for usage information"
        exit 1
        ;;
esac
