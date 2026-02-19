#!/bin/bash

# AxonHub Restart Script
# This script restarts AxonHub by stopping and starting it

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

show_usage() {
    echo "Usage: $0 [--force]"
    echo
    echo "This script restarts AxonHub by stopping and starting it."
    echo "Options:"
    echo "  --force     Force kill before restart"
    echo "  --help, -h  Show this help message"
}

main() {
    print_info "Restarting AxonHub..."
    
    # Stop AxonHub
    print_info "Stopping AxonHub..."
    if [[ "$FORCE" == "true" ]]; then
        "$SCRIPT_DIR/stop.sh" --force || true
    else
        "$SCRIPT_DIR/stop.sh" || true
    fi
    
    # Brief pause to ensure clean shutdown
    sleep 5
    
    # Start AxonHub
    print_info "Starting AxonHub..."
    "$SCRIPT_DIR/start.sh"
    
    print_success "AxonHub has been restarted"
}

# Handle script arguments
FORCE="false"
case "${1:-}" in
    --force)
        FORCE="true"
        main
        ;;
    --help|-h)
        show_usage
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
