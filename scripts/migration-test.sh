#!/bin/bash

# AxonHub Migration Test Script
# Tests database migration from a specified tag to current branch
# Usage: ./migration-test.sh <from-tag> [options]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CACHE_DIR="${SCRIPT_DIR}/migration-test/cache"
WORK_DIR="${SCRIPT_DIR}/migration-test/work"
DB_FILE="${WORK_DIR}/migration-test.db"
LOG_FILE="${WORK_DIR}/migration-test.log"
PLAN_FILE="${WORK_DIR}/migration-plan.json"

# E2E configuration (keep consistent with e2e-test.sh)
E2E_PORT=8099

# GitHub repository
REPO="looplj/axonhub"
GITHUB_API="https://api.github.com/repos/${REPO}"

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

print_step() {
    echo ""
    echo -e "${GREEN}==>${NC} $1"
}

usage() {
    cat <<EOF
AxonHub Migration Test Script

Usage:
  ./migration-test.sh <from-tag> [options]

Arguments:
  from-tag         Git tag to test migration from (e.g., v0.1.0)

Options:
  --skip-download  Skip downloading binary if cached version exists
  --skip-e2e       Skip running e2e tests after migration
  --keep-artifacts Keep work directory after test completion
  -h, --help       Show this help and exit

Examples:
  ./migration-test.sh v0.1.0
  ./migration-test.sh v0.1.0 --skip-e2e
  ./migration-test.sh v0.2.0 --keep-artifacts

Description:
  This script tests database migration by:
  1. Downloading the binary for the specified tag from GitHub releases
  2. Initializing a database with the old version
  3. Running migration to the current branch version
  4. Executing e2e tests to verify the migration

  Binaries are cached in: ${CACHE_DIR}
  Test artifacts are in: ${WORK_DIR}
EOF
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

curl_gh() {
    local url="$1"
    local headers=(
        -H "Accept: application/vnd.github+json"
        -H "X-GitHub-Api-Version: 2022-11-28"
        -H "User-Agent: axonhub-migration-test"
    )
    if [[ -n "$GITHUB_TOKEN" ]]; then
        headers+=( -H "Authorization: Bearer $GITHUB_TOKEN" )
    fi
    curl -fsSL "${headers[@]}" "$url"
}

get_asset_download_url() {
    local version=$1
    local platform=$2
    local url=""
    
    print_info "Resolving asset download URL for ${version} (${platform})..." >&2
    
    if json=$(curl_gh "${GITHUB_API}/releases/tags/${version}" 2>/dev/null); then
        if command -v jq >/dev/null 2>&1; then
            url=$(echo "$json" | jq -r --arg platform "$platform" \
                '.assets[]?.browser_download_url | select(test($platform)) | select(endswith(".zip"))' | head -n1)
        else
            url=$(echo "$json" \
                | tr -d '\n\r\t' \
                | sed -nE 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' \
                | grep "$platform" \
                | grep '\.zip$' -m 1)
        fi
    fi
    
    # Fallback to patterned URL
    if [[ -z "$url" ]]; then
        print_warning "API failed, trying patterned URL..." >&2
        local clean_version="${version#v}"
        local filename="axonhub_${clean_version}_${platform}.zip"
        local candidate="https://github.com/${REPO}/releases/download/${version}/${filename}"
        if curl -fsI "$candidate" >/dev/null 2>&1; then
            url="$candidate"
        fi
    fi
    
    if [[ -z "$url" ]]; then
        print_error "Could not find asset for platform ${platform} in release ${version}" >&2
        exit 1
    fi
    
    echo "$url"
}

download_binary() {
    local version=$1
    local platform=$2
    local cache_path="${CACHE_DIR}/${version}/axonhub"
    
    # Check if cached
    if [[ -f "$cache_path" && "$SKIP_DOWNLOAD" == "true" ]]; then
        print_info "Using cached binary: $cache_path" >&2
        echo "$cache_path"
        return
    fi
    
    # Create cache directory
    mkdir -p "${CACHE_DIR}/${version}"
    
    # Download if not cached
    if [[ ! -f "$cache_path" ]]; then
        print_info "Downloading AxonHub ${version} for ${platform}..." >&2
        
        local download_url
        download_url=$(get_asset_download_url "$version" "$platform")
        local filename=$(basename "$download_url")
        local temp_dir=$(mktemp -d)
        
        if ! curl -fSL -o "${temp_dir}/${filename}" "$download_url"; then
            print_error "Failed to download AxonHub asset" >&2
            rm -rf "$temp_dir"
            exit 1
        fi
        
        print_info "Extracting archive..." >&2
        
        if ! command -v unzip >/dev/null 2>&1; then
            print_error "unzip command not found. Please install unzip." >&2
            rm -rf "$temp_dir"
            exit 1
        fi
        
        if ! unzip -q "${temp_dir}/${filename}" -d "$temp_dir"; then
            print_error "Failed to extract archive" >&2
            rm -rf "$temp_dir"
            exit 1
        fi
        
        # Find and copy binary
        local binary_path
        binary_path=$(find "$temp_dir" -name "axonhub" -type f | head -1)
        
        if [[ -z "$binary_path" ]]; then
            print_error "Could not find axonhub binary in archive" >&2
            rm -rf "$temp_dir"
            exit 1
        fi
        
        cp "$binary_path" "$cache_path"
        chmod +x "$cache_path"
        rm -rf "$temp_dir"
        
        print_success "Binary cached: $cache_path" >&2
    else
        print_info "Using cached binary: $cache_path" >&2
    fi
    
    echo "$cache_path"
}

build_current_binary() {
    local binary_path="${WORK_DIR}/axonhub-current"
    
    print_info "Building current branch binary..." >&2
    cd "$PROJECT_ROOT"
    
    if ! go build -o "$binary_path" ./cmd/axonhub; then
        print_error "Failed to build current branch binary" >&2
        exit 1
    fi
    
    chmod +x "$binary_path"
    print_success "Current binary built: $binary_path" >&2
    
    echo "$binary_path"
}

get_binary_version() {
    local binary_path=$1
    local version
    
    if version=$("$binary_path" version 2>/dev/null | head -n1 | tr -d '\r'); then
        echo "$version"
    else
        echo "unknown"
    fi
}

initialize_database() {
    local binary_path=$1
    local version=$2
    
    print_info "Initializing database with version ${version}..." >&2
    
    # Remove old database
    rm -f "$DB_FILE"
    
    # Start server to initialize database
    AXONHUB_SERVER_PORT=$E2E_PORT \
    AXONHUB_DB_DSN="file:${DB_FILE}?cache=shared&_fk=1" \
    AXONHUB_LOG_OUTPUT="file" \
    AXONHUB_LOG_FILE_PATH="$LOG_FILE" \
    AXONHUB_LOG_LEVEL="info" \
    "$binary_path" > /dev/null 2>&1 &
    
    local pid=$!
    
    # Wait for server to be ready
    print_info "Waiting for server to initialize..." >&2
    for i in {1..30}; do
        if curl -s "http://localhost:$E2E_PORT/health" > /dev/null 2>&1 || \
           curl -s "http://localhost:$E2E_PORT/" > /dev/null 2>&1; then
            print_success "Database initialized with version ${version}" >&2
            kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
            return 0
        fi
        sleep 1
    done
    
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    print_error "Failed to initialize database" >&2
    exit 1
}

run_migration() {
    local binary_path=$1
    local version=$2
    
    print_info "Running migration with version ${version}..." >&2
    
    # Run migration by starting and stopping the server
    AXONHUB_SERVER_PORT=$E2E_PORT \
    AXONHUB_DB_DSN="file:${DB_FILE}?cache=shared&_fk=1" \
    AXONHUB_LOG_OUTPUT="file" \
    AXONHUB_LOG_FILE_PATH="$LOG_FILE" \
    AXONHUB_LOG_LEVEL="debug" \
    "$binary_path" > /dev/null 2>&1 &
    
    local pid=$!
    
    # Wait for server to be ready
    print_info "Waiting for migration to complete..." >&2
    for i in {1..30}; do
        if curl -s "http://localhost:$E2E_PORT/health" > /dev/null 2>&1 || \
           curl -s "http://localhost:$E2E_PORT/" > /dev/null 2>&1; then
            print_success "Migration completed successfully" >&2
            kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
            return 0
        fi
        sleep 1
    done
    
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    print_error "Migration failed or timed out" >&2
    exit 1
}

generate_migration_plan() {
    local from_tag=$1
    local platform=$2
    
    print_info "Generating migration plan..." >&2
    
    # For now, we'll create a simple two-step plan:
    # 1. Initialize with old version
    # 2. Migrate to current version
    
    local old_binary
    old_binary=$(download_binary "$from_tag" "$platform")
    
    local current_binary
    current_binary=$(build_current_binary)
    
    local old_version
    old_version=$(get_binary_version "$old_binary")
    
    local current_version
    current_version=$(get_binary_version "$current_binary")
    
    # Create plan JSON
    cat > "$PLAN_FILE" <<EOF
{
  "from_tag": "$from_tag",
  "from_version": "$old_version",
  "to_version": "$current_version",
  "platform": "$platform",
  "steps": [
    {
      "step": 1,
      "action": "initialize",
      "version": "$from_tag",
      "binary": "$old_binary",
      "description": "Initialize database with version $old_version"
    },
    {
      "step": 2,
      "action": "migrate",
      "version": "current",
      "binary": "$current_binary",
      "description": "Migrate database to version $current_version"
    }
  ]
}
EOF
    
    print_success "Migration plan generated: $PLAN_FILE" >&2
    
    # Display plan
    echo "" >&2
    echo "Migration Plan:" >&2
    echo "  From: $from_tag ($old_version)" >&2
    echo "  To:   current ($current_version)" >&2
    echo "  Steps:" >&2
    echo "    1. Initialize database with $from_tag" >&2
    echo "    2. Migrate to current branch" >&2
    echo "" >&2
}

execute_migration_plan() {
    print_step "Executing migration plan" >&2
    
    if [[ ! -f "$PLAN_FILE" ]]; then
        print_error "Migration plan not found: $PLAN_FILE" >&2
        exit 1
    fi
    
    # Parse plan and execute
    local from_tag from_version to_version
    
    if command -v jq >/dev/null 2>&1; then
        from_tag=$(jq -r '.from_tag' "$PLAN_FILE")
        from_version=$(jq -r '.from_version' "$PLAN_FILE")
        to_version=$(jq -r '.to_version' "$PLAN_FILE")
        
        # Execute step 1: Initialize
        local step1_binary
        step1_binary=$(jq -r '.steps[0].binary' "$PLAN_FILE")
        print_step "Step 1: Initialize database with $from_tag ($from_version)" >&2
        initialize_database "$step1_binary" "$from_version"
        
        # Execute step 2: Migrate
        local step2_binary
        step2_binary=$(jq -r '.steps[1].binary' "$PLAN_FILE")
        print_step "Step 2: Migrate to current ($to_version)" >&2
        run_migration "$step2_binary" "$to_version"
    else
        # Fallback without jq
        from_tag=$(grep '"from_tag"' "$PLAN_FILE" | sed -E 's/.*"from_tag"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        from_version=$(grep '"from_version"' "$PLAN_FILE" | sed -E 's/.*"from_version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        to_version=$(grep '"to_version"' "$PLAN_FILE" | sed -E 's/.*"to_version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        
        # Get binaries from plan
        local step1_binary=$(grep -A 5 '"step": 1' "$PLAN_FILE" | grep '"binary"' | sed -E 's/.*"binary"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        local step2_binary=$(grep -A 5 '"step": 2' "$PLAN_FILE" | grep '"binary"' | sed -E 's/.*"binary"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        
        print_step "Step 1: Initialize database with $from_tag ($from_version)" >&2
        initialize_database "$step1_binary" "$from_version"
        
        print_step "Step 2: Migrate to current ($to_version)" >&2
        run_migration "$step2_binary" "$to_version"
    fi
    
    print_success "Migration plan executed successfully" >&2
}

run_e2e_tests() {
    print_step "Running e2e tests to verify migration" >&2
    
    # Copy migrated database to e2e location
    local e2e_db="${SCRIPT_DIR}/axonhub-e2e.db"
    cp "$DB_FILE" "$e2e_db"
    
    print_info "Database copied to e2e location: $e2e_db" >&2
    
    # Run e2e tests
    cd "$PROJECT_ROOT"
    if ./scripts/e2e-test.sh; then
        print_success "E2E tests passed!" >&2
        return 0
    else
        print_error "E2E tests failed" >&2
        return 1
    fi
}

cleanup() {
    if [[ "$KEEP_ARTIFACTS" != "true" ]]; then
        print_info "Cleaning up work directory..." >&2
        rm -rf "$WORK_DIR"
    else
        print_info "Keeping artifacts in: $WORK_DIR" >&2
    fi
}

main() {
    print_info "AxonHub Migration Test Script" >&2
    echo "" >&2
    
    # Parse arguments
    local from_tag=""
    SKIP_DOWNLOAD="false"
    SKIP_E2E="false"
    KEEP_ARTIFACTS="false"
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-download)
                SKIP_DOWNLOAD="true"
                shift
                ;;
            --skip-e2e)
                SKIP_E2E="true"
                shift
                ;;
            --keep-artifacts)
                KEEP_ARTIFACTS="true"
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            -*)
                print_error "Unknown option: $1" >&2
                usage
                exit 1
                ;;
            *)
                if [[ -z "$from_tag" ]]; then
                    from_tag="$1"
                    shift
                else
                    print_error "Too many arguments" >&2
                    usage
                    exit 1
                fi
                ;;
        esac
    done
    
    if [[ -z "$from_tag" ]]; then
        print_error "Missing required argument: from-tag" >&2
        usage
        exit 1
    fi
    
    print_info "Testing migration from $from_tag to current branch" >&2
    echo "" >&2
    
    # Detect platform
    local platform
    platform=$(detect_architecture)
    print_info "Detected platform: $platform" >&2
    
    # Setup directories
    mkdir -p "$CACHE_DIR" "$WORK_DIR"
    
    # Generate migration plan
    print_step "Step 1: Generate migration plan" >&2
    generate_migration_plan "$from_tag" "$platform"
    
    # Execute migration plan
    print_step "Step 2: Execute migration plan" >&2
    execute_migration_plan
    
    # Run e2e tests
    if [[ "$SKIP_E2E" != "true" ]]; then
        print_step "Step 3: Run e2e tests" >&2
        if ! run_e2e_tests; then
            cleanup
            exit 1
        fi
    else
        print_warning "Skipping e2e tests (--skip-e2e specified)" >&2
    fi
    
    # Cleanup
    cleanup
    
    echo "" >&2
    print_success "Migration test completed successfully!" >&2
    echo "" >&2
    print_info "Summary:" >&2
    echo "  From: $from_tag" >&2
    echo "  To:   current branch" >&2
    echo "  Database: $DB_FILE" >&2
    echo "  Log: $LOG_FILE" >&2
    echo "  Cache: $CACHE_DIR" >&2
    echo "" >&2
}

# Run main function
main "$@"
