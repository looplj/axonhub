#!/bin/bash

# One-command E2E test script
# Handles backend startup, test execution, and cleanup

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

cd "$FRONTEND_DIR"

echo "🚀 Starting E2E Test Suite..."
echo ""

# Function to cleanup on exit
cleanup() {
  echo ""
  echo "🧹 Cleaning up..."
  cd "$PROJECT_ROOT"
  ./scripts/e2e-backend.sh stop > /dev/null 2>&1 || true
}

# Register cleanup function
trap cleanup EXIT

# Start backend server
echo "📦 Starting E2E backend server..."
cd "$PROJECT_ROOT"
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

# Pass all arguments to playwright
pnpm playwright test "$@"

TEST_EXIT_CODE=$?

echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo "✅ All tests passed!"
else
  echo "❌ Some tests failed (exit code: $TEST_EXIT_CODE)"
  echo ""
  echo "💡 Tips:"
  echo "  - View report: pnpm test:e2e:report"
  echo "  - Check backend logs: cat ../scripts/e2e-backend.log"
  echo "  - Inspect database: sqlite3 ../scripts/axonhub-e2e.db"
fi

exit $TEST_EXIT_CODE
