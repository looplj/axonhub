#!/bin/bash

# AxonHub Setup Script
# This script manages auto-start configuration for AxonHub on Linux and macOS

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="axonhub"
# Resolve non-root user's HOME when running via sudo
if [[ -n "$SUDO_USER" && "$SUDO_USER" != "root" ]]; then
    USER_HOME="$(eval echo ~${SUDO_USER})"
    TARGET_USER="$SUDO_USER"
else
    USER_HOME="$HOME"
    TARGET_USER="$USER"
fi
BASE_DIR="${USER_HOME}/.config/axonhub"
BINARY_PATH="/usr/local/bin/axonhub"
PID_FILE="${BASE_DIR}/axonhub.pid"
LOG_FILE="${BASE_DIR}/axonhub.log"

# Platform detection
OS=""
ARCH=""

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

detect_platform() {
    local arch=$(uname -m)
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')

    case $arch in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac

    case $os in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            print_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
}

usage() {
    cat 1>&2 <<EOF
AxonHub Setup

Usage:
  ./setup.sh [command] [options]

Commands:
  install-autostart    Install AxonHub to start automatically on boot
  uninstall-autostart  Remove AxonHub from automatic startup
  status               Check autostart status

Options:
  -h, --help           Show this help and exit

Examples:
  ./setup.sh install-autostart      # Enable auto-start on boot
  ./setup.sh uninstall-autostart    # Disable auto-start
  ./setup.sh status                 # Check current status

Platform-specific notes:
  Linux: Uses systemd user service (recommended) or systemd system service
  macOS: Uses launchd user agent
EOF
}

# ==================== Linux systemd functions ====================

get_systemd_user_service_path() {
    echo "${USER_HOME}/.config/systemd/user/${SERVICE_NAME}.service"
}

get_systemd_system_service_path() {
    echo "/etc/systemd/system/${SERVICE_NAME}.service"
}

create_systemd_service_file() {
    local service_path="$1"
    local is_user_service="$2"

    cat > "$service_path" << EOF
[Unit]
Description=AxonHub AI Gateway
After=network.target

[Service]
Type=simple
User=${TARGET_USER}
WorkingDirectory=${BASE_DIR}
ExecStart=${BINARY_PATH}
Restart=always
RestartSec=5
StandardOutput=append:${LOG_FILE}
StandardError=append:${LOG_FILE}

[Install]
WantedBy=default.target
EOF
}

install_systemd_user_service() {
    print_info "Installing systemd user service..."

    if [[ ! -x "$BINARY_PATH" ]]; then
        print_error "AxonHub binary not found at $BINARY_PATH"
        print_info "Please run install.sh first to install AxonHub"
        return 1
    fi

    local service_path
    service_path=$(get_systemd_user_service_path)
    local service_dir
    service_dir=$(dirname "$service_path")

    # Create systemd user directory if needed
    mkdir -p "$service_dir"

    # Create service file
    create_systemd_service_file "$service_path" true

    # Set ownership
    chown "$TARGET_USER:$(id -gn "$TARGET_USER" 2>/dev/null || echo "$TARGET_USER")" "$service_path" 2>/dev/null || true

    # Reload systemd user daemon
    sudo -u "$TARGET_USER" systemctl --user daemon-reload

    # Enable service
    sudo -u "$TARGET_USER" systemctl --user enable "$SERVICE_NAME.service"

    print_success "Systemd user service installed and enabled"
    print_info "Service will start automatically on next login"
    return 0
}

install_systemd_system_service() {
    print_info "Installing systemd system service..."

    if [[ $EUID -ne 0 ]]; then
        print_warning "Root privileges required for system service installation"
        print_info "Please run with sudo: sudo ./setup.sh install-autostart"
        return 1
    fi

    if [[ ! -x "$BINARY_PATH" ]]; then
        print_error "AxonHub binary not found at $BINARY_PATH"
        print_info "Please run install.sh first to install AxonHub"
        return 1
    fi

    local service_path
    service_path=$(get_systemd_system_service_path)

    # Create service file
    create_systemd_service_file "$service_path" false

    # Reload systemd
    systemctl daemon-reload

    # Enable service
    systemctl enable "$SERVICE_NAME.service"

    print_success "Systemd system service installed and enabled"
    print_info "Service will start automatically on next boot"
    return 0
}

uninstall_systemd_user_service() {
    print_info "Uninstalling systemd user service..."

    local service_path
    service_path=$(get_systemd_user_service_path)

    # Stop and disable service if running
    if sudo -u "$TARGET_USER" systemctl --user is-active --quiet "$SERVICE_NAME.service" 2>/dev/null; then
        sudo -u "$TARGET_USER" systemctl --user stop "$SERVICE_NAME.service"
    fi

    if [[ -f "$service_path" ]]; then
        sudo -u "$TARGET_USER" systemctl --user disable "$SERVICE_NAME.service" 2>/dev/null || true
        rm -f "$service_path"
        sudo -u "$TARGET_USER" systemctl --user daemon-reload
        print_success "Systemd user service uninstalled"
        return 0
    else
        print_warning "No systemd user service found"
        return 1
    fi
}

uninstall_systemd_system_service() {
    print_info "Uninstalling systemd system service..."

    if [[ $EUID -ne 0 ]]; then
        print_warning "Root privileges required"
        print_info "Please run with sudo: sudo ./setup.sh uninstall-autostart"
        return 1
    fi

    local service_path
    service_path=$(get_systemd_system_service_path)

    # Stop and disable service if running
    if systemctl is-active --quiet "$SERVICE_NAME.service" 2>/dev/null; then
        systemctl stop "$SERVICE_NAME.service"
    fi

    if [[ -f "$service_path" ]]; then
        systemctl disable "$SERVICE_NAME.service" 2>/dev/null || true
        rm -f "$service_path"
        systemctl daemon-reload
        print_success "Systemd system service uninstalled"
        return 0
    else
        print_warning "No systemd system service found"
        return 1
    fi
}

check_systemd_status() {
    local status="disabled"

    # Check user service
    local user_service_path
    user_service_path=$(get_systemd_user_service_path)
    if [[ -f "$user_service_path" ]]; then
        if sudo -u "$TARGET_USER" systemctl --user is-enabled "$SERVICE_NAME.service" 2>/dev/null; then
            status="enabled (user)"
        else
            status="disabled (user service exists)"
        fi
    fi

    # Check system service
    local system_service_path
    system_service_path=$(get_systemd_system_service_path)
    if [[ -f "$system_service_path" ]]; then
        if systemctl is-enabled "$SERVICE_NAME.service" 2>/dev/null; then
            status="enabled (system)"
        else
            status="disabled (system service exists)"
        fi
    fi

    echo "$status"
}

# ==================== macOS launchd functions ====================

get_launchd_plist_path() {
    echo "${USER_HOME}/Library/LaunchAgents/com.axonhub.axonhub.plist"
}

create_launchd_plist() {
    local plist_path="$1"

    cat > "$plist_path" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.axonhub.axonhub</string>
    <key>ProgramArguments</key>
    <array>
        <string>${BINARY_PATH}</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${BASE_DIR}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>StandardOutPath</key>
    <string>${LOG_FILE}</string>
    <key>StandardErrorPath</key>
    <string>${LOG_FILE}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
EOF
}

install_launchd_service() {
    print_info "Installing launchd user agent..."

    if [[ ! -x "$BINARY_PATH" ]]; then
        print_error "AxonHub binary not found at $BINARY_PATH"
        print_info "Please run install.sh first to install AxonHub"
        return 1
    fi

    local plist_path
    plist_path=$(get_launchd_plist_path)
    local launchagents_dir
    launchagents_dir=$(dirname "$plist_path")

    # Create LaunchAgents directory if needed
    mkdir -p "$launchagents_dir"
    chown "$TARGET_USER:$(id -gn "$TARGET_USER" 2>/dev/null || echo "$TARGET_USER")" "$launchagents_dir" 2>/dev/null || true

    # Create plist file
    create_launchd_plist "$plist_path"

    # Set ownership
    chown "$TARGET_USER:$(id -gn "$TARGET_USER" 2>/dev/null || echo "$TARGET_USER")" "$plist_path" 2>/dev/null || true

    # Load the service
    sudo -u "$TARGET_USER" launchctl load "$plist_path" 2>/dev/null || true

    print_success "Launchd user agent installed"
    print_info "Service will start automatically on next login"
    return 0
}

uninstall_launchd_service() {
    print_info "Uninstalling launchd user agent..."

    local plist_path
    plist_path=$(get_launchd_plist_path)

    if [[ -f "$plist_path" ]]; then
        # Unload the service
        sudo -u "$TARGET_USER" launchctl unload "$plist_path" 2>/dev/null || true

        # Remove plist file
        rm -f "$plist_path"

        print_success "Launchd user agent uninstalled"
        return 0
    else
        print_warning "No launchd user agent found"
        return 1
    fi
}

check_launchd_status() {
    local plist_path
    plist_path=$(get_launchd_plist_path)

    if [[ -f "$plist_path" ]]; then
        # Check if loaded
        if sudo -u "$TARGET_USER" launchctl list | grep -q "com.axonhub.axonhub"; then
            echo "enabled"
        else
            echo "disabled (plist exists)"
        fi
    else
        echo "disabled"
    fi
}

# ==================== Main functions ====================

install_autostart() {
    print_info "Installing AxonHub auto-start..."

    case "$OS" in
        linux)
            # Try user service first, fallback to system service
            if ! install_systemd_user_service; then
                print_info "Falling back to system service..."
                install_systemd_system_service
            fi
            ;;
        darwin)
            install_launchd_service
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac
}

uninstall_autostart() {
    print_info "Uninstalling AxonHub auto-start..."

    case "$OS" in
        linux)
            uninstall_systemd_user_service
            uninstall_systemd_system_service
            ;;
        darwin)
            uninstall_launchd_service
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac
}

show_status() {
    print_info "Checking AxonHub autostart status..."

    echo ""
    echo "Platform: $OS ($ARCH)"
    echo "User: $TARGET_USER"
    echo "Base Directory: $BASE_DIR"
    echo "Binary Path: $BINARY_PATH"
    echo ""

    case "$OS" in
        linux)
            local systemd_status
            systemd_status=$(check_systemd_status)
            echo "Autostart Status: $systemd_status"
            ;;
        darwin)
            local launchd_status
            launchd_status=$(check_launchd_status)
            echo "Autostart Status: $launchd_status"
            ;;
    esac

    # Check if running
    echo ""
    if [[ -f "$PID_FILE" ]]; then
        local pid
        pid=$(cat "$PID_FILE" 2>/dev/null || true)
        if kill -0 "$pid" 2>/dev/null; then
            echo "Process Status: Running (PID: $pid)"
        else
            echo "Process Status: Not running (stale PID file)"
        fi
    else
        # Try to find process
        local pids
        pids=$(pgrep -f "$BINARY_PATH" 2>/dev/null || true)
        if [[ -n "$pids" ]]; then
            echo "Process Status: Running (PID: $pids)"
        else
            echo "Process Status: Not running"
        fi
    fi
}

# ==================== Main execution ====================

main() {
    # Detect platform
    detect_platform

    # Parse arguments
    local command=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            install-autostart|enable-autostart)
                command="install"
                shift
                ;;
            uninstall-autostart|disable-autostart)
                command="uninstall"
                shift
                ;;
            status|check)
                command="status"
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    # Execute command
    case "$command" in
        install)
            install_autostart
            ;;
        uninstall)
            uninstall_autostart
            ;;
        status)
            show_status
            ;;
        *)
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
