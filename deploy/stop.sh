#!/bin/bash

# AxonHub Stop Script
# This script stops AxonHub directly (no systemd), with proper error handling

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="axonhub"
PID_FILE="/var/run/axonhub.pid"
PROCESS_NAME="axonhub"

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

# Note: systemd-related logic removed for simplicity; this script always stops directly

stop_by_pid() {
    print_info "Stopping AxonHub using PID file..."
    
    if [[ ! -f "$PID_FILE" ]]; then
        print_warning "PID file not found at $PID_FILE"
        return 1
    fi
    
    local pid=$(cat "$PID_FILE")
    
    if [[ ! "$pid" =~ ^[0-9]+$ ]]; then
        print_error "Invalid PID in file: $pid"
        rm -f "$PID_FILE"
        return 1
    fi
    
    if ! kill -0 "$pid" 2>/dev/null; then
        print_warning "Process with PID $pid is not running"
        rm -f "$PID_FILE"
        return 1
    fi
    
    print_info "Sending SIGTERM to process $pid..."
    if kill -TERM "$pid" 2>/dev/null; then
        # Wait for graceful shutdown
        local timeout=10
        local count=0
        
        while kill -0 "$pid" 2>/dev/null && [[ $count -lt $timeout ]]; do
            sleep 1
            ((count++))
        done
        
        if kill -0 "$pid" 2>/dev/null; then
            print_warning "Process did not stop gracefully, sending SIGKILL..."
            kill -KILL "$pid" 2>/dev/null || true
            sleep 2
        fi
        
        if ! kill -0 "$pid" 2>/dev/null; then
            print_success "AxonHub stopped successfully (PID: $pid)"
            rm -f "$PID_FILE"
        else
            print_error "Failed to stop AxonHub process"
            return 1
        fi
    else
        print_error "Failed to send signal to process $pid"
        return 1
    fi
}

stop_by_process_name() {
    print_info "Stopping AxonHub by process name..."
    
    local pids
    pids=$(pgrep -f "$PROCESS_NAME" 2>/dev/null || true)
    
    if [[ -z "$pids" ]]; then
        print_warning "No AxonHub processes found"
        return 1
    fi
    
    print_info "Found AxonHub processes: $pids"
    
    for pid in $pids; do
        print_info "Stopping process $pid..."
        
        if kill -TERM "$pid" 2>/dev/null; then
            # Wait for graceful shutdown
            local timeout=10
            local count=0
            
            while kill -0 "$pid" 2>/dev/null && [[ $count -lt $timeout ]]; do
                sleep 1
                ((count++))
            done
            
            if kill -0 "$pid" 2>/dev/null; then
                print_warning "Process $pid did not stop gracefully, sending SIGKILL..."
                kill -KILL "$pid" 2>/dev/null || true
            fi
        fi
    done
    
    sleep 2
    
    # Check if any processes are still running
    local remaining_pids
    remaining_pids=$(pgrep -f "$PROCESS_NAME" 2>/dev/null || true)
    
    if [[ -z "$remaining_pids" ]]; then
        print_success "All AxonHub processes stopped successfully"
        rm -f "$PID_FILE"
    else
        print_error "Some AxonHub processes are still running: $remaining_pids"
        return 1
    fi
}

check_running_processes() {
    local pids
    pids=$(pgrep -f "$PROCESS_NAME" 2>/dev/null || true)
    
    if [[ -n "$pids" ]]; then
        print_info "Running AxonHub processes:"
        ps -p $pids -o pid,ppid,cmd --no-headers 2>/dev/null || true
        return 0
    else
        return 1
    fi
}

main() {
    print_info "Stopping AxonHub..."
    
    local stopped=false
    
    # Try PID file first
    if stop_by_pid; then
        stopped=true
    fi
    
    # If PID file method didn't work, try by process name
    if [[ "$stopped" != true ]]; then
        if stop_by_process_name; then
            stopped=true
        fi
    fi
    
    # Final check
    if [[ "$stopped" != true ]]; then
        if check_running_processes; then
            print_error "Failed to stop all AxonHub processes"
            return 1
        else
            print_info "No AxonHub processes were running"
        fi
    fi
    
    # Clean up PID file
    rm -f "$PID_FILE"
    
    print_success "AxonHub has been stopped"
}

# Handle script arguments
case "${1:-}" in
    --force)
        print_info "Force stopping all AxonHub processes..."
        if check_running_processes; then
            pkill -KILL -f "$PROCESS_NAME" 2>/dev/null || true
            sleep 2
            if ! check_running_processes; then
                print_success "All AxonHub processes force-stopped"
                rm -f "$PID_FILE"
            else
                print_error "Failed to force-stop some processes"
                exit 1
            fi
        else
            print_info "No AxonHub processes found"
        fi
        ;;
    --help|-h)
        echo "Usage: $0 [--force]"
        echo
        echo "This script stops AxonHub directly (no systemd)."
        echo "Options:"
        echo "  --force     Force kill all AxonHub processes"
        echo "  --help, -h  Show this help message"
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
