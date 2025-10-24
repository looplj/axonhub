#!/bin/bash

# One-command E2E test script
# Handles backend startup, test execution, and cleanup

set -e

echo "🚀 Starting E2E Test Suite..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

# Prioritize environment variables from migration tests, fallback to command line args
DB_TYPE="${AXONHUB_E2E_DB_TYPE:-$DB_TYPE}"

# Parse command line arguments only if DB_TYPE not set via environment
if [[ -z "$DB_TYPE" ]]; then
  DB_TYPE="sqlite"
fi

# Parse command line arguments
KEEP_DB="${AXONHUB_E2E_KEEP_DB:-false}"
ARGS=()
while [[ $# -gt 0 ]]; do
  case $1 in
    -d|--db-type)
      if [[ -z "$AXONHUB_E2E_DB_TYPE" ]]; then
        # Only override if not set via environment variable
        DB_TYPE="$2"
      fi
      shift 2
      ;;
    --keep-db)
      if [[ -z "$AXONHUB_E2E_KEEP_DB" ]]; then
        # Only override if not set via environment variable
        KEEP_DB=true
      fi
      shift
      ;;
    --help)
      echo "Usage: $0 [OPTIONS] [PLAYWRIGHT_ARGS...]"
      echo ""
      echo "Options:"
      echo "  -d, --db-type TYPE    Database type: sqlite, mysql, postgres (default: sqlite)"
      echo "                       Can also be set via AXONHUB_E2E_DB_TYPE environment variable"
      echo "  --keep-db           Keep database after tests complete (don't cleanup)"
      echo "                       Can also be set via AXONHUB_E2E_KEEP_DB environment variable"
      echo "  --help              Show this help message"
      echo ""
      echo "Environment Variables:"
      echo "  AXONHUB_E2E_DB_TYPE   Database type (takes precedence over --db-type)"
      echo "  AXONHUB_E2E_DB_DSN    Database DSN for MySQL/PostgreSQL"
      echo "  AXONHUB_E2E_USE_EXISTING_DB  Use existing database (don't create new)"
      echo "  AXONHUB_E2E_KEEP_DB   Keep database after tests (takes precedence over --keep-db)"
      echo ""
      echo "Examples:"
      echo "  $0                           # Run tests with sqlite"
      echo "  $0 -d mysql                  # Run tests with MySQL"
      echo "  $0 --keep-db                 # Run tests and keep database"
      echo "  AXONHUB_E2E_DB_TYPE=mysql $0  # Set via environment variable"
      echo "  AXONHUB_E2E_KEEP_DB=true $0   # Keep database via environment variable"
      exit 0
      ;;
    *)
      ARGS+=("$1")
      shift
      ;;
  esac
done

# Validate database type
case "$DB_TYPE" in
  sqlite|mysql|postgres)
    ;;
  *)
    echo "❌ Invalid database type: $DB_TYPE"
    echo "Supported types: sqlite, mysql, postgres"
    exit 1
    ;;
esac

# Function to cleanup on exit
cleanup() {
  echo ""
  echo "🧹 Cleaning up..."
  cd "$PROJECT_ROOT"

  if [ "$KEEP_DB" = true ]; then
    echo "💾 Keeping database (--keep-db flag set)"
    # Only stop the server, don't cleanup database
    ./scripts/e2e-backend.sh stop > /dev/null 2>&1 || true
  else
    echo "🗑️  Removing database and stopping server..."
    ./scripts/e2e-backend.sh stop > /dev/null 2>&1 || true
  fi
}

# Register cleanup function
trap cleanup EXIT

# Start backend server
echo "📦 Starting E2E backend server..."
echo "🗄️  Database type: $DB_TYPE"
cd "$PROJECT_ROOT"

# Clean up any existing database type configuration
rm -f ./scripts/.e2e-backend-db-type

# Pass environment variables to backend script
export AXONHUB_E2E_DB_TYPE="$DB_TYPE"
export AXONHUB_E2E_DB_DSN="${AXONHUB_E2E_DB_DSN:-}"
export AXONHUB_E2E_DB_DIALECT="${AXONHUB_E2E_DB_DIALECT:-}"
export AXONHUB_E2E_USE_EXISTING_DB="${AXONHUB_E2E_USE_EXISTING_DB:-false}"
export AXONHUB_E2E_KEEP_DB="$KEEP_DB"

# Start backend with specified database type
./scripts/e2e-backend.sh start

if [ $? -ne 0 ]; then
  echo "❌ Failed to start E2E backend server"
  exit 1
fi

echo ""
echo "✅ Backend server ready"
echo ""

# Run Playwright tests
cd "$FRONTEND_DIR"
echo "🧪 Running Playwright tests..."
echo ""

# Pass remaining arguments to playwright
pnpm playwright test "${ARGS[@]}"

TEST_EXIT_CODE=$?

echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo "✅ All tests passed!"
  if [ "$KEEP_DB" = true ]; then
    echo "💾 Database preserved (--keep-db flag)"
    echo "📊 Database location:"
    case "$DB_TYPE" in
      sqlite)
        echo "   SQLite: ./scripts/axonhub-e2e.db"
        ;;
      mysql)
        echo "   MySQL container: axonhub-e2e-mysql (port 13306)"
        ;;
      postgres)
        echo "   PostgreSQL container: axonhub-e2e-postgres (port 15432)"
        ;;
    esac
  fi
else
  echo "❌ Some tests failed (exit code: $TEST_EXIT_CODE)"
  if [ "$KEEP_DB" = true ]; then
    echo "💾 Database preserved for debugging (--keep-db flag)"
    echo "📊 Database location:"
    case "$DB_TYPE" in
      sqlite)
        echo "   SQLite: ./scripts/axonhub-e2e.db"
        ;;
      mysql)
        echo "   MySQL container: axonhub-e2e-mysql (port 13306)"
        ;;
      postgres)
        echo "   PostgreSQL container: axonhub-e2e-postgres (port 15432)"
        ;;
    esac
  else
    echo ""
    echo "💡 Tips:"
    echo "  - View report: pnpm test:e2e:report"
    echo "  - Check backend logs: cat ../scripts/e2e-backend.log"
    echo "  - Inspect database: sqlite3 ../scripts/axonhub-e2e.db"
  fi
fi

exit $TEST_EXIT_CODE
