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

# CLI options (default: exclude beta/rc)
INCLUDE_BETA="false"
INCLUDE_RC="false"
VERBOSE="false"

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

# Verbose logger
debug() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${YELLOW}[DEBUG]${NC} $1" 1>&2
    fi
}

usage() {
    cat 1>&2 <<EOF
AxonHub Installer

Usage:
  sudo ./install.sh [options] [version]

Options:
  -b, --beta       Consider beta pre-releases when resolving latest version
  -r, --rc         Consider release-candidate (rc) pre-releases when resolving latest version
  -v, --verbose    Print extra debug logs
  -h, --help       Show this help and exit

Notes:
  - By default, beta/rc versions are filtered out and the latest stable release is used.
  - If a version is provided (e.g., v1.2.3), flags are ignored and that version is used.
  - If both --beta and --rc are provided, the newest matching pre-release is selected.
EOF
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
    
    debug "Selected tag: $tag_name"
    echo "$tag_name"
}

# Get the latest version based on flags (default stable; with --beta/--rc select matching pre-releases)
get_latest_version() {
    local include_beta="$1"
    local include_rc="$2"

    # Default path: stable-only
    if [[ "$include_beta" != "true" && "$include_rc" != "true" ]]; then
        get_latest_release
        return
    fi

    print_info "Fetching releases to determine latest version (beta=${include_beta}, rc=${include_rc})..."

    local json tag_name pattern
    tag_name=""
    if [[ "$include_beta" == "true" && "$include_rc" == "true" ]]; then
        pattern='-beta|-rc'
    elif [[ "$include_beta" == "true" ]]; then
        pattern='-beta'
    else
        pattern='-rc'
    fi

    if json=$(curl_gh "${GITHUB_API}/releases?per_page=100" 2>/dev/null); then
        local tags pairs
        if command -v jq >/dev/null 2>&1; then
            tags=$(echo "$json" | jq -r '.[] | select(.draft==false) | .tag_name')
        else
            tags=$(echo "$json" | grep -oE '"tag_name"\s*:\s*"[^"]+"' | sed -E 's/.*"tag_name"\s*:\s*"([^"]+)".*/\1/')
        fi

        debug "Fetched release tags (first 20): $(printf '%s\n' "$tags" | head -n20 | tr '\n' ' ')"

        # Build pairs of cleaned|original, where cleaned strips any prefix before the semantic version
        pairs=$(printf '%s\n' "$tags" | awk '{
            orig=$0;
            cleaned=orig;
            if (match(cleaned,/(v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z\.\-]+)*)$/,m)) {
                cleaned=m[1];
            }
            print cleaned"|"orig
        }')

        # Filter by pattern on the cleaned part and semver-sort to pick the highest; return original tag
        local best
        best=$(printf '%s\n' "$pairs" |
            awk -F'|' -v pat="$pattern" '$1 ~ pat {print $0}' |
            awk -F'|' '{ t=$1; sub(/^v/,"",t); print t"|"$2 }' |
            sort -t '|' -k1,1V | tail -n1 | cut -d'|' -f2)

        tag_name="$best"
    fi

    if [[ -z "$tag_name" ]]; then
        print_warning "No matching pre-release found; falling back to latest stable release."
        tag_name=$(get_latest_release)
    fi

    echo "$tag_name"
}

# Normalize version by removing a leading 'v'
normalize_version() {
    local v="$1"
    v="${v#v}"
    echo "$v"
}

# Return 0 (true) if $1 < $2 in semantic version order, using sort -V
version_lt() {
    local a b first
    a=$(normalize_version "$1")
    b=$(normalize_version "$2")
    first=$(printf '%s\n' "$a" "$b" | sort -V | head -n1)
    [[ "$first" == "$a" && "$a" != "$b" ]]
}

# Get asset download url for a given version and platform (e.g., darwin_arm64), prefer .zip
get_asset_download_url() {
    local version=$1
    local platform=$2
    local url=""
    
    print_info "Resolving asset download URL for ${version} (${platform})..."
    debug "Querying ${GITHUB_API}/releases/tags/${version}"
    if json=$(curl_gh "${GITHUB_API}/releases/tags/${version}" 2>/dev/null); then
        if command -v jq >/dev/null 2>&1; then
            debug "Assets on tag (names): $(echo "$json" | jq -r '.assets[]?.name' | tr '\n' ' ')"
            url=$(echo "$json" | jq -r --arg platform "$platform" '.assets[]?.browser_download_url | select(test($platform)) | select(endswith(".zip"))' | head -n1)
        else
            url=$(echo "$json" \
                | tr -d '\n\r\t' \
                | sed -nE 's/.*("browser_download_url"[[[:space:]]]*:[[:space:]]*"[^"]+").*/\1/p' \
                | sed -nE 's/.*"browser_download_url"[[[:space:]]]*:[[:space:]]*"([^"]+)".*/\1/p' \
                | grep "$platform" \
                | grep '\.zip$' -m 1)
        fi
    fi
    debug "Matched asset URL from tag endpoint: ${url:-<none>}"

    # Fallback to patterned URL if API failed or empty
    if [[ -z "$url" ]]; then
        print_warning "API failed or no asset matched; trying list endpoint..."
        if json2=$(curl_gh "${GITHUB_API}/releases?per_page=100" 2>/dev/null); then
            if command -v jq >/dev/null 2>&1; then
                url=$(echo "$json2" | jq -r --arg tag "$version" --arg platform "$platform" '.[] | select(.tag_name==$tag) | .assets[]?.browser_download_url | select(test($platform)) | select(endswith(".zip"))' | head -n1)
            else
                url=$(echo "$json2" \
                    | tr -d '\n\r\t' \
                    | sed -nE 's/.*\{([^}]*)\}.*/\{\1\}/gp' \
                    | grep -E '"tag_name"[[:space:]]*:[[:space:]]*"'"$version"'"' \
                    | sed -nE 's/.*("browser_download_url"[[[:space:]]]*:[[:space:]]*"[^"]+").*/\1/p' \
                    | sed -nE 's/.*"browser_download_url"[[[:space:]]]*:[[:space:]]*"([^"]+)".*/\1/p' \
                    | grep "$platform" \
                    | grep '\.zip$' -m 1)
            fi
        fi
        debug "Matched asset URL from list endpoint: ${url:-<none>}"
    fi

    if [[ -z "$url" ]]; then
        print_warning "API failed or no asset matched; trying patterned URL..."
        local clean_version="$version"
        clean_version="${clean_version##*:}"
        clean_version="${clean_version#v}"
        local filename="axonhub_${clean_version}_${platform}.zip"
        local candidate="https://github.com/${REPO}/releases/download/${version}/${filename}"
        debug "Trying candidate URL: $candidate"
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
  output: "file"
  file:
    path: "${BASE_DIR}/logs/axonhub.log"
    max_size: 100
    max_age: 30
    max_backups: 10
    local_time: true
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
    local version version_arg
    version="${AXONHUB_VERSION:-}"

    # Parse CLI flags and optional version argument
    if [[ -z "$version" ]]; then
        while [[ $# -gt 0 ]]; do
            case "$1" in
                -b|--beta)
                    INCLUDE_BETA="true"; shift ;;
                -r|--rc)
                    INCLUDE_RC="true"; shift ;;
                -v|--verbose)
                    VERBOSE="true"; shift ;;
                -h|--help)
                    usage; exit 0 ;;
                --)
                    shift; break ;;
                -*)
                    print_error "Unknown option: $1"; usage; exit 1 ;;
                *)
                    if [[ -z "${version_arg:-}" ]]; then
                        version_arg="$1"; shift
                    else
                        break
                    fi ;;
            esac
        done
        version="${version_arg:-}"
    fi

    if [[ -z "$version" ]]; then
        version=$(get_latest_version "$INCLUDE_BETA" "$INCLUDE_RC")
    fi
    print_info "Using version: $version"
    
    # Prefer local binary near this script; offer to update if newer is available
    local binary_path
    local script_dir
    script_dir=$(cd "$(dirname "$0")" && pwd)
    if [[ -x "$script_dir/axonhub" ]]; then
        print_info "Found local binary: $script_dir/axonhub"
        # Try to read local version from the binary
        local local_version norm_local norm_target
        if local_version=$("$script_dir/axonhub" version 2>/dev/null | head -n1 | tr -d '\r'); then
            print_info "Local binary version: $local_version"
            norm_local=$(normalize_version "$local_version")
        else
            print_warning "Could not determine local binary version"
            norm_local=""
        fi
        norm_target=$(normalize_version "$version")

        if [[ -n "$norm_local" && "$norm_local" != "dev" ]] && version_lt "$norm_local" "$norm_target"; then
            echo -n "A newer version is available (local ${local_version}, latest ${version}). Download the latest now? [Y/n]: " 1>&2
            read -r reply
            if [[ -z "$reply" || "$reply" =~ ^[Yy]$ ]]; then
                binary_path=$(download_and_extract "$version" "$platform")
            else
                print_info "Using existing local binary as requested."
                binary_path="$script_dir/axonhub"
            fi
        else
            print_info "Local binary is up-to-date. Using existing local binary."
            binary_path="$script_dir/axonhub"
        fi
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
