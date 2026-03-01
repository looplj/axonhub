#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

REPO="looplj/axonhub"
BINARY_NAME="axonclaw"
INSTALL_DIR="${INSTALL_DIR:-.}"

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

    echo "${os}-${arch}"
}

get_latest_version() {
    local api_url="https://api.github.com/repos/${REPO}/releases/latest"
    local version

    if command -v curl >/dev/null 2>&1; then
        version=$(curl -sL "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        version=$(wget -qO- "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        print_error "Neither curl nor wget is available"
        exit 1
    fi

    if [[ -z "$version" ]]; then
        print_warning "Could not determine latest version, using default"
        version="latest"
    fi

    echo "$version"
}

download_binary() {
    local version="$1"
    local platform="$2"
    local os_name="${platform%%-*}"
    local arch="${platform##*-}"

    local download_url="https://github.com/${REPO}/releases/download/${version}/axonclaw-${os_name}-${arch}.tar.gz"
    local temp_dir
    temp_dir=$(mktemp -d)
    local archive_path="${temp_dir}/axonclaw.tar.gz"

    print_info "Downloading axonclaw ${version} for ${platform}..."
    print_info "URL: ${download_url}"

    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL -o "$archive_path" "$download_url"; then
            print_error "Failed to download binary"
            rm -rf "$temp_dir"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -q -O "$archive_path" "$download_url"; then
            print_error "Failed to download binary"
            rm -rf "$temp_dir"
            exit 1
        fi
    else
        print_error "Neither curl nor wget is available"
        rm -rf "$temp_dir"
        exit 1
    fi

    print_success "Download completed"
    echo "$archive_path"
}

extract_archive() {
    local archive_path="$1"
    local target_dir="$2"

    print_info "Extracting to ${target_dir}..."

    mkdir -p "$target_dir"

    if ! tar -xzf "$archive_path" -C "$target_dir"; then
        print_error "Failed to extract archive"
        exit 1
    fi

    local temp_dir
    temp_dir=$(dirname "$archive_path")
    rm -rf "$temp_dir"

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

    local platform
    platform=$(detect_platform)
    print_info "Detected platform: ${platform}"

    local version
    version=$(get_latest_version)
    print_info "Latest version: ${version}"

    local archive_path
    archive_path=$(download_binary "$version" "$platform")

    extract_archive "$archive_path" "$INSTALL_DIR"

    make_scripts_executable "$INSTALL_DIR"

    print_success "AxonClaw ${version} installed successfully to ${INSTALL_DIR}"

    if [[ -n "$AXONCLAW_NAME" ]] || [[ -n "$AXONCLAW_BASE_URL" ]] || [[ -n "$AXONCLAW_API_KEY" ]]; then
        start_axonclaw "$INSTALL_DIR"
    else
        print_info "Environment variables not set, skipping auto-start"
        print_info "To start axonclaw manually:"
        print_info "  cd ${INSTALL_DIR} && ./start.sh"
        print_info ""
        print_info "Or with environment variables:"
        print_info "  AXONCLAW_NAME=my-agent AXONCLAW_BASE_URL=http://localhost:8090 AXONCLAW_API_KEY=your-key ./start.sh"
    fi
}

main "$@"
