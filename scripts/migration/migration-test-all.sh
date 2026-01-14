#!/bin/bash

# AxonHub Migration Test - Test All Versions
# Tests migration from multiple tags to current branch

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat <<EOF
AxonHub Migration Test - Test All Versions

Usage:
  ./migration-test-all.sh [options]

Options:
  --tags <tags>    Comma-separated list of tags to test (default: auto-detect recent tags)
  --db-type TYPE   Database type: sqlite, mysql, postgres (default: all)
  --skip-e2e       Skip e2e tests for all migrations
  --keep-artifacts Keep artifacts for all tests
  -h, --help       Show this help and exit

Examples:
  # Test migration from last 3 stable releases on all databases
  ./migration-test-all.sh

  # Test specific versions on SQLite only
  ./migration-test-all.sh --tags v0.1.0,v0.2.0 --db-type sqlite

  # Test on MySQL only
  ./migration-test-all.sh --db-type mysql

  # Test without e2e
  ./migration-test-all.sh --skip-e2e

Description:
  This script runs migration tests from multiple tags to the current branch
  across all supported database types. By default, it tests the last 3 stable
  (non-beta, non-rc) releases on all database types (SQLite, MySQL, PostgreSQL).
EOF
}

get_recent_stable_tags() {
    local count=${1:-3}
    git tag --sort=-version:refname | grep -v -E '(beta|rc|alpha)' | head -n "$count"
}

main() {
    local tags=""
    local db_type=""
    local skip_e2e=""
    local keep_artifacts=""
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --tags)
                if [[ -z "$2" || "$2" == -* ]]; then
                    print_error "--tags requires an argument"
                    usage
                    exit 1
                fi
                tags="$2"
                shift 2
                ;;
            --db-type)
                if [[ -z "$2" || "$2" == -* ]]; then
                    print_error "--db-type requires an argument"
                    usage
                    exit 1
                fi
                db_type="$2"
                shift 2
                ;;
            --skip-e2e)
                skip_e2e="--skip-e2e"
                shift
                ;;
            --keep-artifacts)
                keep_artifacts="--keep-artifacts"
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
    
    # Validate database type
    if [[ -n "$db_type" ]]; then
        case "$db_type" in
            sqlite|mysql|postgres)
                ;;
            *)
                print_error "Invalid database type: $db_type (must be sqlite, mysql, or postgres)"
                exit 1
                ;;
        esac
    fi

    # Set default database types if not specified
    if [[ -z "$db_type" ]]; then
        local db_types=("sqlite" "mysql" "postgres")
    else
        local db_types=("$db_type")
    fi

    # Get tags to test
    if [[ -z "$tags" ]]; then
        print_info "Auto-detecting recent stable tags..."
        tags=$(get_recent_stable_tags 3 | tr '\n' ',')
        tags="${tags%,}"  # Remove trailing comma
    fi

    if [[ -z "$tags" ]]; then
        print_error "No tags found to test"
        exit 1
    fi

    print_info "Testing migration from tags: $tags"
    print_info "Database types: ${db_types[*]}"
    echo ""

    # Convert comma-separated tags to array
    IFS=',' read -ra tag_array <<< "$tags"

    local total_tags=${#tag_array[@]}
    local total_dbs=${#db_types[@]}
    local total_tests=$((total_tags * total_dbs))
    local test_count=0
    local passed=0
    local failed=0
    local failed_tests=()

    # Test each tag on each database type
    for tag in "${tag_array[@]}"; do
        tag=$(echo "$tag" | xargs)  # Trim whitespace

        for db in "${db_types[@]}"; do
            ((test_count++))

            echo ""
            echo "========================================"
            print_info "Testing migration from $tag on $db ($test_count/$total_tests)"
            echo "========================================"
            echo ""

            if "$SCRIPT_DIR/migration-test.sh" "$tag" --db-type "$db" $skip_e2e $keep_artifacts; then
                ((passed++))
                print_success "Migration test passed: $tag on $db"
            else
                ((failed++))
                failed_tests+=("$tag on $db")
                print_error "Migration test failed: $tag on $db"
            fi
        done
    done
    
    # Summary
    echo ""
    echo "========================================"
    echo "Migration Test Summary"
    echo "========================================"
    echo "Tags tested: $total_tags"
    echo "Database types: ${db_types[*]}"
    echo "Total tests: $total_tests"
    echo "Passed: $passed"
    echo "Failed: $failed"

    if [[ $failed -gt 0 ]]; then
        echo ""
        print_error "Failed tests:"
        for test in "${failed_tests[@]}"; do
            echo "  - $test"
        done
        echo ""
        exit 1
    else
        echo ""
        print_success "All migration tests passed!"
        echo ""
        exit 0
    fi
}

main "$@"
