#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVICE_NAME="axonclaw"
PID_FILE=".axonclaw/axonclaw.pid"
PROCESS_NAME="axonclaw"
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
EXPECTED_BINARY="$SCRIPT_DIR/$PROCESS_NAME"

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

stop_by_pid() {
    print_info "Stopping AxonClaw using PID file..."
    
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
            print_success "AxonClaw stopped successfully (PID: $pid)"
            rm -f "$PID_FILE"
        else
            print_error "Failed to stop AxonClaw process"
            return 1
        fi
    else
        print_error "Failed to send signal to process $pid"
        return 1
    fi
}

find_instance_pids() {
    ps -eo pid=,args= 2>/dev/null | while read -r pid args; do
        if [[ "$args" == *"$EXPECTED_BINARY"* ]]; then
            printf '%s\n' "$pid"
        fi
    done
}

stop_by_process_name() {
    print_info "Stopping AxonClaw by exact instance path..."

    local pids
    pids=$(find_instance_pids)

    if [[ -z "$pids" ]]; then
        print_warning "No AxonClaw process found for $EXPECTED_BINARY"
        return 1
    fi

    print_info "Found AxonClaw processes for this instance: $pids"

    for pid in $pids; do
        print_info "Stopping process $pid..."

        if kill -TERM "$pid" 2>/dev/null; then
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

    local remaining_pids
    remaining_pids=$(find_instance_pids)

    if [[ -z "$remaining_pids" ]]; then
        print_success "All AxonClaw processes for this instance stopped successfully"
        rm -f "$PID_FILE"
    else
        print_error "Some AxonClaw processes for this instance are still running: $remaining_pids"
        return 1
    fi
}

check_running_processes() {
    local pids
    pids=$(find_instance_pids)
    
    if [[ -n "$pids" ]]; then
        print_info "Running AxonClaw processes for this instance:"
        ps -p $pids -o pid,ppid,cmd --no-headers 2>/dev/null || true
        return 0
    else
        return 1
    fi
}

main() {
    print_info "Stopping AxonClaw..."
    
    local stopped=false
    
    if stop_by_pid; then
        stopped=true
    fi
    
    if [[ "$stopped" != true ]]; then
        if stop_by_process_name; then
            stopped=true
        fi
    fi
    
    if [[ "$stopped" != true ]]; then
        if check_running_processes; then
            print_error "Failed to stop all AxonClaw processes"
            return 1
        else
            print_info "No AxonClaw processes were running"
        fi
    fi
    
    rm -f "$PID_FILE"
    
    print_success "AxonClaw has been stopped"
}

case "${1:-}" in
    --force)
        print_info "Force stopping AxonClaw processes for this instance..."
        if check_running_processes; then
            local pids
            pids=$(find_instance_pids)
            if [[ -n "$pids" ]]; then
                kill -KILL $pids 2>/dev/null || true
            fi
            sleep 2
            if ! check_running_processes; then
                print_success "All AxonClaw processes for this instance force-stopped"
                rm -f "$PID_FILE"
            else
                print_error "Failed to force-stop some AxonClaw processes for this instance"
                exit 1
            fi
        else
            print_info "No AxonClaw processes found for this instance"
        fi
        ;;
    --help|-h)
        echo "Usage: $0 [--force]"
        echo
        echo "Options:"
        echo "  --force     Force kill all AxonClaw processes"
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
