#!/bin/bash

# Test script to run migration tests against all supported databases
# Usage: ./test-migration-all-dbs.sh <from-tag>

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo ""
    echo "========================================"
    echo "$1"
    echo "========================================"
    echo ""
}

if [[ -z "$1" ]]; then
    print_error "Usage: $0 <from-tag>"
    echo "Example: $0 v0.1.0"
    exit 1
fi

FROM_TAG="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

print_header "Testing Migration from $FROM_TAG"
print_info "This script will test migration against all supported databases"
echo ""

# Test SQLite
print_header "Test 1/3: SQLite Migration"
if "$SCRIPT_DIR/migration-test.sh" "$FROM_TAG" --db-type sqlite; then
    print_success "SQLite migration test passed"
else
    print_error "SQLite migration test failed"
    exit 1
fi

# Test MySQL
print_header "Test 2/3: MySQL Migration"
if "$SCRIPT_DIR/migration-test.sh" "$FROM_TAG" --db-type mysql; then
    print_success "MySQL migration test passed"
else
    print_error "MySQL migration test failed"
    exit 1
fi

# Test PostgreSQL
print_header "Test 3/3: PostgreSQL Migration"
if "$SCRIPT_DIR/migration-test.sh" "$FROM_TAG" --db-type postgres; then
    print_success "PostgreSQL migration test passed"
else
    print_error "PostgreSQL migration test failed"
    exit 1
fi

# Summary
print_header "All Migration Tests Passed!"
echo "✓ SQLite migration: OK"
echo "✓ MySQL migration: OK"
echo "✓ PostgreSQL migration: OK"
echo ""
print_success "All database types tested successfully!"
