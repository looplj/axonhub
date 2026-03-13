#!/bin/bash

# AxonClaw Installer
# This script downloads and installs the latest AxonClaw release.
# It specifically targets the 'axonclaw' binary from the AxonHub release.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="looplj/axonhub"
GITHUB_API="https://api.github.com/repos/${REPO}"
INSTALL_DIR="${INSTALL_DIR:-.}"
BINARY_NAME="axonclaw"

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

curl_gh() {
    # Curl helper for GitHub with proper headers and optional token
    local url="$1"
    local headers=(
        -H "Accept: application/vnd.github+json"
        -H "X-GitHub-Api-Version: 2022-11-28"
        -H "User-Agent: axonclaw-installer"
    )
    if [[ -n "$GITHUB_TOKEN" ]]; then
        headers+=( -H "Authorization: Bearer $GITHUB_TOKEN" )
    fi
    curl -fsSL "${headers[@]}" "$url"
}

detect_platform() {
    local os=""
    local arch=""

    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)          os="linux" ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        arm64|aarch64)  arch="arm64" ;;
        armv7l|armv6l)  arch="arm" ;;
        i386|i686)      arch="386" ;;
        *)              arch="amd64" ;;
    esac

    DETECTED_OS="$os"
    DETECTED_ARCH="$arch"
    echo "${os}_${arch}"
}

get_latest_release() {
    print_info "Fetching latest axonclaw release..."
    
    local tag_name=""

    # List recent releases and find the latest axonclaw-v* tag
    if json=$(curl_gh "${GITHUB_API}/releases?per_page=30" 2>/dev/null); then
        if command -v jq >/dev/null 2>&1; then
            tag_name=$(echo "$json" | jq -r '[.[] | select(.tag_name | startswith("axonclaw-v")) | select(.draft | not)][0].tag_name // empty')
        else
            tag_name=$(echo "$json" \
                | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"axonclaw-v[^"]*"' \
                | head -1 \
                | sed -E 's/.*"(axonclaw-v[^"]+)".*/\1/')
        fi
    fi

    # Fallback: list tags via API
    if [[ -z "$tag_name" || "$tag_name" == "null" ]]; then
        print_warning "Release list failed, falling back to tags API..."
        if json=$(curl_gh "${GITHUB_API}/tags?per_page=30" 2>/dev/null); then
            if command -v jq >/dev/null 2>&1; then
                tag_name=$(echo "$json" | jq -r '[.[].name | select(startswith("axonclaw-v"))][0] // empty')
            else
                tag_name=$(echo "$json" \
                    | grep -o '"name"[[:space:]]*:[[:space:]]*"axonclaw-v[^"]*"' \
                    | head -1 \
                    | sed -E 's/.*"(axonclaw-v[^"]+)".*/\1/')
            fi
        fi
    fi
    
    if [[ -z "$tag_name" || "$tag_name" == "null" ]]; then
        print_error "Could not determine latest axonclaw release version"
        exit 1
    fi
    
    echo "$tag_name"
}

get_download_url() {
    local version="$1"
    local os_name="$DETECTED_OS"
    local arch="$DETECTED_ARCH"
    local url=""
    local asset_name=""
    
    # Use tar.gz for Linux/Darwin, zip for Windows
    if [[ "$os_name" == "windows" ]]; then
        asset_name="axonclaw_${os_name}_${arch}.zip"
    else
        asset_name="axonclaw_${os_name}_${arch}.tar.gz"
    fi
    
    print_info "Resolving download URL for ${version} (${os_name}/${arch})..."
    
    # Try GitHub API to get the exact asset URL
    if json=$(curl_gh "${GITHUB_API}/releases/tags/${version}" 2>/dev/null); then
        if command -v jq >/dev/null 2>&1; then
            url=$(echo "$json" | jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .browser_download_url' | head -n1)
        else
            url=$(echo "$json" \
                | grep "browser_download_url" \
                | grep "$asset_name" \
                | head -1 \
                | sed -E 's/.*"([^"]+)".*/\1/')
        fi
    fi
    
    # Fallback: construct the URL directly
    if [[ -z "$url" || "$url" == "null" ]]; then
        print_warning "API lookup failed, using direct download URL..."
        url="https://github.com/${REPO}/releases/download/${version}/${asset_name}"
    fi
    
    echo "$url"
}

download_and_extract() {
    local download_url="$1"
    
    local temp_dir
    temp_dir=$(mktemp -d)
    local filename
    filename=$(basename "$download_url")
    local archive_path="${temp_dir}/${filename}"

    print_info "Downloading ${filename}..."
    print_info "URL: ${download_url}"

    local max_retries=3
    local attempt=1
    while (( attempt <= max_retries )); do
        if curl -fSL --retry 3 --retry-delay 2 -o "$archive_path" "$download_url"; then
            break
        fi
        print_warning "Download attempt ${attempt}/${max_retries} failed, retrying..."
        attempt=$((attempt + 1))
        sleep 2
    done

    if (( attempt > max_retries )); then
        print_error "Failed to download after ${max_retries} attempts"
        rm -rf "$temp_dir"
        exit 1
    fi

    print_success "Download completed"

    print_info "Extracting to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"

    # Extract based on archive type
    if [[ "$filename" == *.tar.gz ]]; then
        if ! tar -xzf "$archive_path" -C "$INSTALL_DIR"; then
            print_error "Failed to extract tar.gz archive"
            rm -rf "$temp_dir"
            exit 1
        fi
    else
        if ! command -v unzip >/dev/null 2>&1; then
            print_warning "unzip command not found. Attempting to install..."
            if command -v apt-get >/dev/null 2>&1; then
                sudo apt-get update && sudo apt-get install -y unzip
            elif command -v yum >/dev/null 2>&1; then
                sudo yum install -y unzip
            elif command -v apk >/dev/null 2>&1; then
                sudo apk add unzip
            else
                print_error "Could not install unzip automatically. Please install unzip manually."
                rm -rf "$temp_dir"
                exit 1
            fi
        fi
        if ! unzip -o -q "$archive_path" -d "$INSTALL_DIR"; then
            print_error "Failed to extract zip archive"
            rm -rf "$temp_dir"
            exit 1
        fi
    fi

    rm -rf "$temp_dir"

    # Rename platform-specific binary to generic name
    local extracted_binary="${INSTALL_DIR}/axonclaw_${DETECTED_OS}_${DETECTED_ARCH}"
    if [[ -f "$extracted_binary" ]]; then
        mv "$extracted_binary" "${INSTALL_DIR}/${BINARY_NAME}"
        print_info "Renamed binary to ${BINARY_NAME}"
    fi

    print_success "Extraction completed"
}

make_scripts_executable() {
    local target_dir="$1"

    for script in start.sh stop.sh restart.sh; do
        if [[ -f "${target_dir}/${script}" ]]; then
            chmod +x "${target_dir}/${script}"
            print_info "Made ${script} executable"
        fi
    done

    if [[ -f "${target_dir}/${BINARY_NAME}" ]]; then
        chmod +x "${target_dir}/${BINARY_NAME}"
        print_info "Made ${BINARY_NAME} executable"
    fi
}

start_axonclaw() {
    local target_dir="$1"

    if [[ -f "${target_dir}/start.sh" ]]; then
        print_info "Starting axonclaw..."
        cd "$target_dir" && ./start.sh
    else
        print_warning "start.sh not found, axonclaw not started automatically"
        print_info "To start manually, run: cd ${target_dir} && ./start.sh"
    fi
}

main() {
    print_info "AxonClaw Installer"
    print_info "=================="
    print_info "Note: This installer targets the 'axonclaw' component from the AxonHub project."

    local platform
    platform=$(detect_platform)
    DETECTED_OS="${platform%%_*}"
    DETECTED_ARCH="${platform#*_}"
    print_info "Detected platform: ${DETECTED_OS}/${DETECTED_ARCH}"

    local version
    version=$(get_latest_release)
    print_info "Latest version: ${version}"

    local download_url
    download_url=$(get_download_url "$version")
    
    download_and_extract "$download_url"

    make_scripts_executable "$INSTALL_DIR"

    print_success "AxonClaw ${version} installed successfully to ${INSTALL_DIR}"

    if [[ -n "$AXONCLAW_AUTO_SYNC_CONFIG" ]] || [[ -n "$AXONCLAW_BASE_URL" ]] || [[ -n "$AXONCLAW_API_KEY" ]]; then
        start_axonclaw "$INSTALL_DIR"
    else
        print_info "Environment variables not set, skipping auto-start"
        print_info "To start axonclaw manually:"
        print_info "  cd ${INSTALL_DIR} && ./start.sh"
        print_info ""
        print_info "Or with environment variables:"
        print_info "  AXONCLAW_BASE_URL=http://localhost:8090 AXONCLAW_API_KEY=your-key AXONCLAW_AUTO_SYNC_CONFIG=true ./start.sh"
    fi
}

main "$@"
