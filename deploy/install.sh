#!/bin/bash

# AxonHub Installation Script
# This script downloads and installs the latest AxonHub release for direct start/stop usage (no systemd)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/usr/local/bin"
# Resolve non-root user's HOME when running via sudo
if [[ -n "$SUDO_USER" && "$SUDO_USER" != "root" ]]; then
    USER_HOME="$(eval echo ~${SUDO_USER})"
else
    USER_HOME="$HOME"
fi
BASE_DIR="${USER_HOME}/.config/axonhub"
CONFIG_DIR="${BASE_DIR}"
DATA_DIR="${BASE_DIR}"
LOG_DIR="${BASE_DIR}"
SERVICE_USER="axonhub"

# GitHub repository
REPO="looplj/axonhub"
GITHUB_API="https://api.github.com/repos/${REPO}"

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1" 1>&2
}

curl_gh() {
    # Curl helper for GitHub with proper headers and optional token
    local url="$1"
    local headers=(
        -H "Accept: application/vnd.github+json"
        -H "X-GitHub-Api-Version: 2022-11-28"
        -H "User-Agent: axonhub-installer"
    )
    if [[ -n "$GITHUB_TOKEN" ]]; then
        headers+=( -H "Authorization: Bearer $GITHUB_TOKEN" )
    fi
    curl -fsSL "${headers[@]}" "$url"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" 1>&2
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" 1>&2
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" 1>&2
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

detect_architecture() {
    local arch=$(uname -m)
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    
    case $arch in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    case $os in
        linux)
            os="linux"
            ;;
        darwin)
            os="darwin"
            ;;
        *)
            print_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    echo "${os}_${arch}"
}

get_latest_release() {
    print_info "Fetching latest release information..."
    
    local tag_name
    # Try GitHub API first
    if json=$(curl_gh "${GITHUB_API}/releases/latest" 2>/dev/null); then
        tag_name=$(echo "$json" | tr -d '\n\r\t' | sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' | head -1)
    fi
    
    # Fallback: follow the HTML redirect to the latest tag
    if [[ -z "$tag_name" ]]; then
        print_warning "API failed or rate-limited, falling back to HTML redirect..."
        local final_url
        final_url=$(curl -fsSL -H "User-Agent: axonhub-installer" -o /dev/null -w "%{url_effective}" "https://github.com/${REPO}/releases/latest" || true)
        tag_name=$(echo "$final_url" | sed -nE 's#.*/tag/([^/]+).*#\1#p' | head -1)
    fi
    
    if [[ -z "$tag_name" ]]; then
        print_error "Could not determine latest release version"
        exit 1
    fi
    
    echo "$tag_name"
}

# Get asset download url for a given version and platform (e.g., darwin_arm64), prefer .zip
get_asset_download_url() {
    local version=$1
    local platform=$2
    local url=""
    
    print_info "Resolving asset download URL for ${version} (${platform})..."
    if json=$(curl_gh "${GITHUB_API}/releases/tags/${version}" 2>/dev/null); then
        url=$(echo "$json" \
            | tr -d '\n\r\t' \
            | sed -nE 's/.*("browser_download_url"[[:space:]]*:[[:space:]]*"[^"]+").*/\1/p' \
            | sed -nE 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' \
            | grep "$platform" \
            | grep '\.zip$' -m 1)
    fi
    
    # Fallback to patterned URL if API failed or empty
    if [[ -z "$url" ]]; then
        print_warning "API failed or no asset matched; trying patterned URL..."
        local clean_version=${version#v}
        local filename="axonhub_${clean_version}_${platform}.zip"
        local candidate="https://github.com/${REPO}/releases/download/${version}/${filename}"
        if curl -fsI "$candidate" >/dev/null 2>&1; then
            url="$candidate"
        fi
    fi
    
    if [[ -z "$url" ]]; then
        print_error "Could not find a matching .zip asset for platform ${platform} in release ${version}"
        exit 1
    fi
    echo "$url"
}

download_and_extract() {
    local version=$1
    local platform=$2
    local temp_dir=$(mktemp -d)
    
    # Resolve exact asset URL from GitHub API
    local download_url
    download_url=$(get_asset_download_url "$version" "$platform")
    local filename
    filename=$(basename "$download_url")
    
    print_info "Downloading AxonHub ${version} for ${platform}..."
    
    if ! curl -fSL -o "${temp_dir}/${filename}" "$download_url"; then
        print_error "Failed to download AxonHub asset"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    print_info "Extracting archive..."
    
    if ! command -v unzip >/dev/null 2>&1; then
        print_error "unzip command not found. Please install unzip and rerun."
        rm -rf "$temp_dir"
        exit 1
    fi
    
    if ! unzip -q "${temp_dir}/${filename}" -d "$temp_dir"; then
        print_error "Failed to extract archive"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    # Find the extracted binary
    local binary_path
    binary_path=$(find "$temp_dir" -name "axonhub" -type f | head -1)
    
    if [[ -z "$binary_path" ]]; then
        print_error "Could not find axonhub binary in archive"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    echo "$binary_path"
}

create_user() {
    # No system user management per requirements
    print_info "Skipping system user creation"
}

setup_directories() {
    print_info "Setting up directories..."
    
    # Create directories
    mkdir -p "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    
    # Set ownership and permissions to invoking user
    local target_user="${SUDO_USER:-$USER}"
    local target_group
    target_group="$(id -gn "$target_user" 2>/dev/null || echo "$target_user")"
    chown -R "$target_user:$target_group" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR" 2>/dev/null || true
    chmod 755 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
}

install_binary() {
    local binary_path=$1
    
    print_info "Installing AxonHub binary to $INSTALL_DIR..."
    
    # Install binary
    cp "$binary_path" "$INSTALL_DIR/axonhub"
    chmod +x "$INSTALL_DIR/axonhub"
    
    # Clean up temp directory only if it looks like a system temp path
    local dir
    dir="$(dirname "$binary_path")"
    local tmp1="${TMPDIR:-/tmp}"
    if [[ "$dir" == /tmp/* || "$dir" == /var/folders/* || "$dir" == /private/var/folders/* || "$dir" == "$tmp1"* ]]; then
        rm -rf "$dir" 2>/dev/null || true
    fi
}

create_default_config() {
    local config_file="$CONFIG_DIR/config.yml"
    
    if [[ ! -f "$config_file" ]]; then
        print_info "Creating default configuration..."
        
        cat > "$config_file" << EOF
server:
  port: 8090
  name: "AxonHub"
  debug: false

db:
  dialect: "sqlite3"
  dsn: "${BASE_DIR}/axonhub.db?cache=shared&_fk=1&journal_mode=WAL"

log:
  level: "info"
  encoding: "json"
  output: "${BASE_DIR}/axonhub.log"
EOF
        
        local target_user="${SUDO_USER:-$USER}"
        local target_group
        target_group="$(id -gn "$target_user" 2>/dev/null || echo "$target_user")"
        chown "$target_user:$target_group" "$config_file" 2>/dev/null || true
        chmod 644 "$config_file"
        
        print_success "Default configuration created at $config_file"
    else
        print_info "Configuration file already exists at $config_file"
    fi
}

# Note: systemd service installation removed; use deploy/start.sh and deploy/stop.sh to manage AxonHub

main() {
    print_info "Starting AxonHub installation..."
    
    # Check if running as root
    check_root
    
    # Detect system architecture
    local platform
    platform=$(detect_architecture)
    print_info "Detected platform: $platform"
    
    # Determine target version (env AXONHUB_VERSION, positional arg, or latest)
    local version
    version="${AXONHUB_VERSION:-}"
    if [[ -z "$version" && -n "${1:-}" ]]; then
        version="$1"
    fi
    if [[ -z "$version" ]]; then
        version=$(get_latest_release)
    fi
    print_info "Using version: $version"
    
    # Prefer local binary near this script to avoid downloading
    local binary_path
    local script_dir
    script_dir=$(cd "$(dirname "$0")" && pwd)
    if [[ -x "$script_dir/axonhub" ]]; then
        print_info "Found local binary: $script_dir/axonhub"
        binary_path="$script_dir/axonhub"
    else
        # Download and extract
        binary_path=$(download_and_extract "$version" "$platform")
    fi
    
    # Create system user
    create_user
    
    # Setup directories
    setup_directories
    
    # Install binary
    install_binary "$binary_path"
    
    # Create default configuration
    create_default_config
    
    print_success "AxonHub installation completed!"
    echo
    print_info "Next steps:"
    echo "  1. Edit configuration: nano $CONFIG_DIR/config.yml"
    echo "  2. Start AxonHub: ./start.sh"
    echo "  3. Stop AxonHub: ./stop.sh"
    echo "  4. View logs: tail -f $LOG_DIR/axonhub.log"
    echo "  5. Access web interface: http://localhost:8090"
    echo
    print_info "To start AxonHub now, run: ./start.sh"
}

# Run main function
main "$@"
