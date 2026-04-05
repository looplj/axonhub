#!/bin/bash

# AxonHub Upgrade Script
# This script checks for new versions and upgrades AxonHub

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Resolve non-root user's HOME when running via sudo
if [[ -n "$SUDO_USER" && "$SUDO_USER" != "root" ]]; then
    USER_HOME="$(eval echo ~${SUDO_USER})"
else
    USER_HOME="$HOME"
fi
BASE_DIR="${USER_HOME}/.config/axonhub"
INSTALL_DIR="/usr/local/bin"

# GitHub repository
REPO="looplj/axonhub"
GITHUB_API="https://api.github.com/repos/${REPO}"

# CLI options
INCLUDE_BETA="false"
INCLUDE_RC="false"
VERBOSE="false"
FORCE="false"
YES="false"

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1" 1>&2
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

debug() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${YELLOW}[DEBUG]${NC} $1" 1>&2
    fi
}

show_usage() {
    cat <<EOF
AxonHub Upgrade Script

Usage: $0 [options]

Options:
  -y, --yes        Skip confirmation prompt and upgrade automatically
  -f, --force      Force upgrade even if already on latest version
  -b, --beta       Consider beta pre-releases when checking for updates
  -r, --rc         Consider release-candidate pre-releases when checking for updates
  -v, --verbose    Print extra debug logs
  -h, --help       Show this help message

Examples:
  $0               Check and prompt for upgrade
  $0 -y            Upgrade without confirmation
  $0 --beta        Check beta releases
EOF
}

curl_gh() {
    local url="$1"
    local headers=(
        -H "Accept: application/vnd.github+json"
        -H "X-GitHub-Api-Version: 2022-11-28"
        -H "User-Agent: axonhub-upgrader"
    )
    if [[ -n "$GITHUB_TOKEN" ]]; then
        headers+=( -H "Authorization: Bearer $GITHUB_TOKEN" )
    fi
    curl -fsSL "${headers[@]}" "$url"
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

normalize_version() {
    local v="$1"
    v="${v#v}"
    echo "$v"
}

version_lt() {
    local a b first
    a=$(normalize_version "$1")
    b=$(normalize_version "$2")
    first=$(printf '%s\n' "$a" "$b" | sort -V | head -n1)
    [[ "$first" == "$a" && "$a" != "$b" ]]
}

get_latest_release() {
    print_info "Fetching latest release information..."
    
    local tag_name
    if json=$(curl_gh "${GITHUB_API}/releases/latest" 2>/dev/null); then
        tag_name=$(echo "$json" | tr -d '\n\r\t' | sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' | head -1)
    fi
    
    if [[ -z "$tag_name" ]]; then
        print_warning "API failed or rate-limited, falling back to HTML redirect..."
        local final_url
        final_url=$(curl -fsSL -H "User-Agent: axonhub-upgrader" -o /dev/null -w "%{url_effective}" "https://github.com/${REPO}/releases/latest" || true)
        tag_name=$(echo "$final_url" | sed -nE 's#.*/tag/([^/]+).*#\1#p' | head -1)
    fi
    
    if [[ -z "$tag_name" ]]; then
        print_error "Could not determine latest release version"
        exit 1
    fi
    
    debug "Selected tag: $tag_name"
    echo "$tag_name"
}

get_latest_version() {
    local include_beta="$1"
    local include_rc="$2"

    if [[ "$include_beta" != "true" && "$include_rc" != "true" ]]; then
        get_latest_release
        return
    fi

    print_info "Fetching releases (beta=${include_beta}, rc=${include_rc})..."

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

        debug "Fetched release tags: $(printf '%s\n' "$tags" | head -n20 | tr '\n' ' ')"

        pairs=$(printf '%s\n' "$tags" | awk '{
            orig=$0;
            cleaned=orig;
            if (match(cleaned,/(v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z\.\-]+)*)$/,m)) {
                cleaned=m[1];
            }
            print cleaned"|"orig
        }')

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

get_current_version() {
    local binary="$1"
    if [[ -x "$binary" ]]; then
        "$binary" version 2>/dev/null | head -n1 | tr -d '\r'
    else
        echo ""
    fi
}

download_and_extract() {
    local version=$1
    local platform=$2
    local temp_dir=$(mktemp -d)
    
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
    
    local binary_path
    binary_path=$(find "$temp_dir" -name "axonhub" -type f | head -1)
    
    if [[ -z "$binary_path" ]]; then
        print_error "Could not find axonhub binary in archive"
        rm -rf "$temp_dir"
        exit 1
    fi
    
    echo "$binary_path"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

main() {
    print_info "Checking for AxonHub updates..."
    
    check_root
    
    local binary_path="$INSTALL_DIR/axonhub"
    
    if [[ ! -x "$binary_path" ]]; then
        print_error "AxonHub is not installed at $binary_path"
        print_info "Please run install.sh first"
        exit 1
    fi
    
    local current_version
    current_version=$(get_current_version "$binary_path")
    
    if [[ -z "$current_version" ]]; then
        print_warning "Could not determine current version"
        current_version="unknown"
    else
        print_info "Current version: $current_version"
    fi
    
    local latest_version
    latest_version=$(get_latest_version "$INCLUDE_BETA" "$INCLUDE_RC")
    print_info "Latest version: $latest_version"
    
    local norm_current norm_latest
    norm_current=$(normalize_version "$current_version")
    norm_latest=$(normalize_version "$latest_version")
    
    if [[ "$current_version" != "unknown" ]] && ! version_lt "$norm_current" "$norm_latest" && [[ "$FORCE" != "true" ]]; then
        print_success "AxonHub is already up to date ($current_version)"
        exit 0
    fi
    
    if [[ "$FORCE" == "true" && "$current_version" != "unknown" ]]; then
        print_warning "Force upgrade requested (current: $current_version, target: $latest_version)"
    fi
    
    if [[ "$YES" != "true" ]]; then
        echo -n "Upgrade AxonHub from ${current_version} to ${latest_version}? [y/N]: "
        read -r reply
        if [[ ! "$reply" =~ ^[Yy]$ ]]; then
            print_info "Upgrade cancelled"
            exit 0
        fi
    fi
    
    local platform
    platform=$(detect_architecture)
    print_info "Detected platform: $platform"
    
    local new_binary
    new_binary=$(download_and_extract "$latest_version" "$platform")
    
    print_info "Installing new binary..."
    cp "$new_binary" "$INSTALL_DIR/axonhub"
    chmod +x "$INSTALL_DIR/axonhub"
    
    local temp_dir
    temp_dir="$(dirname "$new_binary")"
    local tmp1="${TMPDIR:-/tmp}"
    if [[ "$temp_dir" == /tmp/* || "$temp_dir" == /var/folders/* || "$temp_dir" == /private/var/folders/* || "$temp_dir" == "$tmp1"* ]]; then
        rm -rf "$temp_dir" 2>/dev/null || true
    fi
    
    print_success "AxonHub upgraded to ${latest_version}"
    
    print_info "Restarting AxonHub..."
    if [[ -x "$SCRIPT_DIR/restart.sh" ]]; then
        "$SCRIPT_DIR/restart.sh"
    else
        print_warning "restart.sh not found, please restart AxonHub manually"
    fi
    
    print_success "Upgrade completed!"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -y|--yes)
            YES="true"; shift ;;
        -f|--force)
            FORCE="true"; shift ;;
        -b|--beta)
            INCLUDE_BETA="true"; shift ;;
        -r|--rc)
            INCLUDE_RC="true"; shift ;;
        -v|--verbose)
            VERBOSE="true"; shift ;;
        -h|--help)
            show_usage; exit 0 ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1 ;;
    esac
done

main
