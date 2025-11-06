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

# Database configuration
DB_TYPE="sqlite"  # Default: sqlite, mysql, postgres
MYSQL_CONTAINER="axonhub-migration-mysql"
MYSQL_PORT=13306
MYSQL_ROOT_PASSWORD="axonhub_test_root"
MYSQL_DATABASE="axonhub_e2e"
MYSQL_USER="axonhub"
MYSQL_PASSWORD="axonhub_test"

POSTGRES_CONTAINER="axonhub-migration-postgres"
POSTGRES_PORT=15432
POSTGRES_DATABASE="axonhub_e2e"
POSTGRES_USER="axonhub"
POSTGRES_PASSWORD="axonhub_test"

# System initialization defaults (override via AXONHUB_INIT_* env vars)
INIT_OWNER_EMAIL="${AXONHUB_INIT_OWNER_EMAIL:-owner@example.com}"
INIT_OWNER_PASSWORD="${AXONHUB_INIT_OWNER_PASSWORD:-InitPassword123!}"
INIT_OWNER_FIRST_NAME="${AXONHUB_INIT_OWNER_FIRST_NAME:-System}"
INIT_OWNER_LAST_NAME="${AXONHUB_INIT_OWNER_LAST_NAME:-Owner}"
INIT_BRAND_NAME="${AXONHUB_INIT_BRAND_NAME:-AxonHub Migration Test}"

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
  --db-type TYPE   Database type: sqlite, mysql, postgres (default: sqlite)
  --skip-download  Skip downloading binary if cached version exists
  --skip-e2e       Skip running e2e tests after migration
  --skip-init-system
                   Skip system initialization step (reuse existing database state)
  --keep-artifacts Keep work directory after test completion
  --keep-db        Keep database container after test completion
  -h, --help       Show this help and exit

Examples:
  ./migration-test.sh v0.1.0
  ./migration-test.sh v0.1.0 --db-type mysql
  ./migration-test.sh v0.1.0 --db-type postgres --skip-e2e
  ./migration-test.sh v0.2.0 --keep-artifacts

Description:
  This script tests database migration by:
  1. Setting up database (SQLite file or Docker container for MySQL/PostgreSQL)
  2. Downloading the binary for the specified tag from GitHub releases
  3. Initializing a database with the old version
  4. Running migration to the current branch version
  5. Executing e2e tests to verify the migration

  Supported databases:
  - SQLite (default, no Docker required)
  - MySQL (requires Docker, creates temporary container)
  - PostgreSQL (requires Docker, creates temporary container)

  Binaries are cached in: ${CACHE_DIR}
  Test artifacts are in: ${WORK_DIR}
EOF
}

check_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        print_error "Docker is not installed. Please install Docker to use MySQL or PostgreSQL."
        exit 1
    fi
    
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker daemon is not running. Please start Docker."
        exit 1
    fi
}

setup_mysql() {
    print_step "Setting up MySQL database" >&2
    
    check_docker
    
    # Stop and remove existing container if exists
    if docker ps -a --format '{{.Names}}' | grep -q "^${MYSQL_CONTAINER}$"; then
        print_info "Removing existing MySQL container..." >&2
        docker rm -f "$MYSQL_CONTAINER" >/dev/null 2>&1 || true
    fi
    
    # Start MySQL container
    print_info "Starting MySQL container..." >&2
    docker run -d \
        --name "$MYSQL_CONTAINER" \
        -e MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
        -e MYSQL_DATABASE="$MYSQL_DATABASE" \
        -e MYSQL_USER="$MYSQL_USER" \
        -e MYSQL_PASSWORD="$MYSQL_PASSWORD" \
        -p "${MYSQL_PORT}:3306" \
        mysql:8.0 \
        --character-set-server=utf8mb4 \
        --collation-server=utf8mb4_unicode_ci \
        >/dev/null
    
    # Wait for MySQL to be ready
    print_info "Waiting for MySQL to be ready..." >&2
    for i in {1..30}; do
        if docker exec "$MYSQL_CONTAINER" mysqladmin ping -h localhost -u root -p"$MYSQL_ROOT_PASSWORD" >/dev/null 2>&1; then
            print_success "MySQL is ready" >&2
            return 0
        fi
        sleep 1
    done
    
    print_error "MySQL failed to start" >&2
    docker logs "$MYSQL_CONTAINER" >&2
    exit 1
}

setup_postgres() {
    print_step "Setting up PostgreSQL database" >&2
    
    check_docker
    
    # Stop and remove existing container if exists
    if docker ps -a --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
        print_info "Removing existing PostgreSQL container..." >&2
        docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
    fi
    
    # Start PostgreSQL container
    print_info "Starting PostgreSQL container..." >&2
    docker run -d \
        --name "$POSTGRES_CONTAINER" \
        -e POSTGRES_DB="$POSTGRES_DATABASE" \
        -e POSTGRES_USER="$POSTGRES_USER" \
        -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
        -p "${POSTGRES_PORT}:5432" \
        postgres:15-alpine \
        >/dev/null
    
    # Wait for PostgreSQL to be ready
    print_info "Waiting for PostgreSQL to be ready..." >&2
    for i in {1..30}; do
        if docker exec "$POSTGRES_CONTAINER" pg_isready -U "$POSTGRES_USER" >/dev/null 2>&1; then
            print_success "PostgreSQL is ready" >&2
            return 0
        fi
        sleep 1
    done
    
    print_error "PostgreSQL failed to start" >&2
    docker logs "$POSTGRES_CONTAINER" >&2
    exit 1
}

cleanup_database() {
    if [[ "$KEEP_DB" == "true" ]]; then
        print_info "Keeping database container (--keep-db specified)" >&2
        return
    fi
    
    case "$DB_TYPE" in
        mysql)
            if docker ps -a --format '{{.Names}}' | grep -q "^${MYSQL_CONTAINER}$"; then
                print_info "Removing MySQL container..." >&2
                docker rm -f "$MYSQL_CONTAINER" >/dev/null 2>&1 || true
            fi
            ;;
        postgres)
            if docker ps -a --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
                print_info "Removing PostgreSQL container..." >&2
                docker rm -f "$POSTGRES_CONTAINER" >/dev/null 2>&1 || true
            fi
            ;;
        sqlite)
            # SQLite cleanup handled by cleanup() function
            ;;
    esac
}

get_db_dsn() {
    case "$DB_TYPE" in
        sqlite)
            echo "file:${DB_FILE}?cache=shared&_fk=1"
            ;;
        mysql)
            echo "${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(localhost:${MYSQL_PORT})/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=True&loc=Local"
            ;;
        postgres)
            echo "host=localhost port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DATABASE} sslmode=disable"
            ;;
        *)
            print_error "Unknown database type: $DB_TYPE" >&2
            exit 1
            ;;
    esac
}

get_db_dialect() {
    case "$DB_TYPE" in
        sqlite)
            echo "sqlite3"
            ;;
        mysql)
            echo "mysql"
            ;;
        postgres)
            echo "postgres"
            ;;
        *)
            print_error "Unknown database type: $DB_TYPE" >&2
            exit 1
            ;;
    esac
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
    print_info "Binary path: $binary_path" >&2
    print_info "Database type: $DB_TYPE" >&2
    
    # Remove old SQLite database file if using SQLite
    if [[ "$DB_TYPE" == "sqlite" ]]; then
        print_info "Removing old SQLite database file: $DB_FILE" >&2
        rm -f "$DB_FILE"
    fi
    
    local db_dsn
    db_dsn=$(get_db_dsn)
    print_info "Database DSN: $db_dsn" >&2
    
    local db_dialect
    db_dialect=$(get_db_dialect)
    
    # Start server to initialize database
    print_info "Starting server for initialization (PID will be captured)..." >&2
    AXONHUB_SERVER_PORT=$E2E_PORT \
    AXONHUB_DB_DIALECT="$db_dialect" \
    AXONHUB_DB_DSN="$db_dsn" \
    AXONHUB_LOG_OUTPUT="file" \
    AXONHUB_LOG_FILE_PATH="$LOG_FILE" \
    AXONHUB_LOG_LEVEL="info" \
    "$binary_path" > /dev/null 2>&1 &

    local pid=$!
    print_info "Server started with PID: $pid" >&2

    local init_status=0

    if ! wait_for_server_ready "$E2E_PORT" 60; then
        print_error "Server failed to become ready" >&2
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
        print_error "Failed to start server for database initialization" >&2
        exit 1
    fi

    if [[ "$SKIP_INIT_SYSTEM" == "true" ]]; then
        print_warning "Skipping system initialization (--skip-init-system specified)" >&2
    else
        if ! initialize_system_via_api; then
            init_status=1
        fi
    fi

    print_info "Stopping server (PID: $pid)..." >&2
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true

    if [[ $init_status -ne 0 ]]; then
        print_error "Database initialization failed" >&2
        exit 1
    fi

    if [[ "$SKIP_INIT_SYSTEM" == "true" ]]; then
        print_success "Database prepared with existing state (initialization skipped)" >&2
    else
        print_success "Database initialized with version ${version}" >&2
    fi
}

run_migration() {
    local binary_path=$1
    local version=$2
    
    print_info "Running migration with version ${version}..." >&2
    print_info "Binary path: $binary_path" >&2
    print_info "Database type: $DB_TYPE" >&2
    
    local db_dsn
    db_dsn=$(get_db_dsn)
    print_info "Database DSN: $db_dsn" >&2
    
    local db_dialect
    db_dialect=$(get_db_dialect)
    
    # Run migration by starting and stopping the server
    print_info "Starting server for migration (PID will be captured)..." >&2
    AXONHUB_SERVER_PORT=$E2E_PORT \
    AXONHUB_DB_DIALECT="$db_dialect" \
    AXONHUB_DB_DSN="$db_dsn" \
    AXONHUB_LOG_OUTPUT="file" \
    AXONHUB_LOG_FILE_PATH="$LOG_FILE" \
    AXONHUB_LOG_LEVEL="debug" \
    "$binary_path" > /dev/null 2>&1 &
    
    local pid=$!
    print_info "Server started with PID: $pid" >&2
    
    # Wait for server to be ready
    print_info "Waiting for migration to complete..." >&2
    if wait_for_server_ready "$E2E_PORT" 60; then
        # Check if the server process is still running after becoming ready
        if ! kill -0 "$pid" 2>/dev/null; then
            # Server process has already exited, check its exit status
            local exit_status=0
            wait "$pid" 2>/dev/null || exit_status=$?
            print_error "Server process exited unexpectedly with status: $exit_status" >&2
            print_error "Check log file for details: $LOG_FILE" >&2
            exit 1
        fi
        
        # Give the server a bit more time to ensure migration completes and server stays stable
        print_info "Server is ready, verifying stability..." >&2
        sleep 3
        
        # Check again if server is still running
        if ! kill -0 "$pid" 2>/dev/null; then
            local exit_status=0
            wait "$pid" 2>/dev/null || exit_status=$?
            print_error "Server process crashed after startup with status: $exit_status" >&2
            print_error "Check log file for details: $LOG_FILE" >&2
            exit 1
        fi
        
        print_success "Migration completed successfully" >&2
        print_info "Stopping server (PID: $pid)..." >&2
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
        return 0
    fi

    print_error "Server failed to become ready, stopping..." >&2
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    print_error "Migration failed or timed out" >&2
    exit 1
}

wait_for_server_ready() {
    local port=$1
    local max_attempts=${2:-60}

    print_info "Waiting for server to be ready on port ${port}..." >&2

    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
        if curl -s "http://localhost:${port}/health" > /dev/null 2>&1 || \
           curl -s "http://localhost:${port}/" > /dev/null 2>&1; then
            return 0
        fi

        sleep 1
    done

    return 1
}

initialize_system_via_api() {
    local url="http://localhost:${E2E_PORT}/admin/system/initialize"
    local response_file="${WORK_DIR}/initialize-response.json"
    local payload

    payload=$(cat <<EOF
{
  "ownerEmail": "${INIT_OWNER_EMAIL}",
  "ownerPassword": "${INIT_OWNER_PASSWORD}",
  "ownerFirstName": "${INIT_OWNER_FIRST_NAME}",
  "ownerLastName": "${INIT_OWNER_LAST_NAME}",
  "brandName": "${INIT_BRAND_NAME}"
}
EOF
    )

    print_info "Initializing system via API (${url})..." >&2

    for attempt in {1..5}; do
        local tmp_file="${response_file}.tmp"
        local status

        status=$(curl -sS -o "$tmp_file" -w "%{http_code}" \
            -H "Content-Type: application/json" \
            -X POST \
            -d "$payload" \
            "$url" || echo "000")
        status=$(echo "$status" | tr -d '\n\r')

        if [[ -f "$tmp_file" ]]; then
            mv "$tmp_file" "$response_file"
        fi

        case "$status" in
            200)
                print_success "System initialized successfully via API" >&2
                return 0
                ;;
            400)
                if [[ -f "$response_file" ]] && grep -qi "already initialized" "$response_file"; then
                    print_warning "System initialization API returned already initialized" >&2
                    return 0
                fi
                ;;
        esac

        if [[ "$status" == "000" ]]; then
            print_warning "Initialization attempt ${attempt} failed: unable to reach server" >&2
        else
            local body=""
            if [[ -f "$response_file" ]]; then
                body=$(cat "$response_file")
            fi
            print_warning "Initialization attempt ${attempt} failed (status: ${status}): ${body}" >&2
        fi

        sleep 2
    done

    print_error "System initialization via API failed after multiple attempts" >&2
    if [[ -f "$response_file" ]]; then
        print_error "Last response: $(cat "$response_file")" >&2
    fi

    return 1
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
        if [[ "$SKIP_INIT_SYSTEM" == "true" ]]; then
            print_step "Step 2.1: Initialize database with $from_tag ($from_version) [skipped]" >&2
            print_warning "System initialization skipped by option" >&2
        else
            print_step "Step 2.1: Initialize database with $from_tag ($from_version)" >&2
            print_info "Using binary: $step1_binary" >&2
            initialize_database "$step1_binary" "$from_version"
            print_success "Step 2.1 completed" >&2
        fi
        
        # Execute step 2: Migrate
        local step2_binary
        step2_binary=$(jq -r '.steps[1].binary' "$PLAN_FILE")
        print_step "Step 2.2: Migrate to current ($to_version)" >&2
        print_info "Using binary: $step2_binary" >&2
        run_migration "$step2_binary" "$to_version"
        print_success "Step 2.2 completed" >&2
    else
        # Fallback without jq
        from_tag=$(grep '"from_tag"' "$PLAN_FILE" | sed -E 's/.*"from_tag"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        from_version=$(grep '"from_version"' "$PLAN_FILE" | sed -E 's/.*"from_version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        to_version=$(grep '"to_version"' "$PLAN_FILE" | sed -E 's/.*"to_version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        
        # Get binaries from plan
        local step1_binary=$(grep -A 5 '"step": 1' "$PLAN_FILE" | grep '"binary"' | sed -E 's/.*"binary"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        local step2_binary=$(grep -A 5 '"step": 2' "$PLAN_FILE" | grep '"binary"' | sed -E 's/.*"binary"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
        
        if [[ "$SKIP_INIT_SYSTEM" == "true" ]]; then
            print_step "Step 2.1: Initialize database with $from_tag ($from_version) [skipped]" >&2
            print_warning "System initialization skipped by option" >&2
        else
            print_step "Step 2.1: Initialize database with $from_tag ($from_version)" >&2
            print_info "Using binary: $step1_binary" >&2
            initialize_database "$step1_binary" "$from_version"
            print_success "Step 2.1 completed" >&2
        fi

        print_step "Step 2.2: Migrate to current ($to_version)" >&2
        print_info "Using binary: $step2_binary" >&2
        run_migration "$step2_binary" "$to_version"
        print_success "Step 2.2 completed" >&2
    fi
    
    print_success "Migration plan executed successfully" >&2
}

run_e2e_tests() {
    print_step "Running e2e tests to verify migration" >&2
    
    local db_dsn
    db_dsn=$(get_db_dsn)
    local db_dialect
    db_dialect=$(get_db_dialect)

    if [[ "$DB_TYPE" == "sqlite" ]]; then
        local e2e_db="${SCRIPT_DIR}/axonhub-e2e.db"
        cp "$DB_FILE" "$e2e_db"
        print_info "Database copied to e2e location: $e2e_db" >&2
        cd "$PROJECT_ROOT"
        if env \
            AXONHUB_E2E_DB_TYPE="$DB_TYPE" \
            AXONHUB_E2E_DB_DIALECT="$db_dialect" \
            ./scripts/e2e-test.sh; then
            print_success "E2E tests passed!" >&2
            return 0
        else
            print_error "E2E tests failed" >&2
            return 1
        fi
    fi

    print_info "Reusing migrated $DB_TYPE database for e2e tests" >&2

    cd "$PROJECT_ROOT"
    if env \
        AXONHUB_E2E_DB_TYPE="$DB_TYPE" \
        AXONHUB_E2E_DB_DIALECT="$db_dialect" \
        AXONHUB_E2E_DB_DSN="$db_dsn" \
        AXONHUB_E2E_USE_EXISTING_DB="true" \
        ./scripts/e2e-test.sh; then
        print_success "E2E tests passed!" >&2
        return 0
    else
        print_error "E2E tests failed" >&2
        return 1
    fi
}

cleanup() {
    # Cleanup database containers
    cleanup_database
    
    # Cleanup work directory
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
    SKIP_INIT_SYSTEM="false"
    KEEP_ARTIFACTS="false"
    KEEP_DB="false"
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --db-type)
                if [[ -z "$2" || "$2" == -* ]]; then
                    print_error "--db-type requires an argument" >&2
                    usage
                    exit 1
                fi
                DB_TYPE="$2"
                shift 2
                ;;
            --skip-download)
                SKIP_DOWNLOAD="true"
                shift
                ;;
            --skip-e2e)
                SKIP_E2E="true"
                shift
                ;;
            --skip-init-system)
                SKIP_INIT_SYSTEM="true"
                shift
                ;;
            --keep-artifacts)
                KEEP_ARTIFACTS="true"
                shift
                ;;
            --keep-db)
                KEEP_DB="true"
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
    
    # Validate database type
    case "$DB_TYPE" in
        sqlite|mysql|postgres)
            ;;
        *)
            print_error "Invalid database type: $DB_TYPE (must be sqlite, mysql, or postgres)" >&2
            exit 1
            ;;
    esac
    
    print_info "Testing migration from $from_tag to current branch" >&2
    print_info "Database type: $DB_TYPE" >&2
    echo "" >&2
    
    # Detect platform
    local platform
    platform=$(detect_architecture)
    print_info "Detected platform: $platform" >&2
    
    # Setup directories
    mkdir -p "$CACHE_DIR" "$WORK_DIR"
    
    # Setup database
    case "$DB_TYPE" in
        mysql)
            setup_mysql
            ;;
        postgres)
            setup_postgres
            ;;
        sqlite)
            print_info "Using SQLite database: $DB_FILE" >&2
            ;;
    esac
    
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
    echo "  Database Type: $DB_TYPE" >&2
    case "$DB_TYPE" in
        sqlite)
            echo "  Database File: $DB_FILE" >&2
            ;;
        mysql)
            echo "  MySQL Container: $MYSQL_CONTAINER" >&2
            echo "  MySQL Port: $MYSQL_PORT" >&2
            echo "  MySQL Database: $MYSQL_DATABASE" >&2
            if [[ "$KEEP_DB" == "true" ]]; then
                echo "  MySQL DSN: ${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(localhost:${MYSQL_PORT})/${MYSQL_DATABASE}" >&2
            fi
            ;;
        postgres)
            echo "  PostgreSQL Container: $POSTGRES_CONTAINER" >&2
            echo "  PostgreSQL Port: $POSTGRES_PORT" >&2
            echo "  PostgreSQL Database: $POSTGRES_DATABASE" >&2
            if [[ "$KEEP_DB" == "true" ]]; then
                echo "  PostgreSQL DSN: host=localhost port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DATABASE}" >&2
            fi
            ;;
    esac
    echo "  Log: $LOG_FILE" >&2
    echo "  Cache: $CACHE_DIR" >&2
    echo "" >&2
}

# Run main function
main "$@"
