# Gemini Go SDK Integration Tests Makefile

.PHONY: all test test-all test-qa clean help setup

# Default target
all: test

# Setup dependencies
setup:
	@echo "Setting up test dependencies..."
	go mod tidy
	@echo "Setup complete!"

# Run all tests
test-all: setup
	@echo "Running all integration tests..."
	go test -v ./...

# Run tests with coverage
test-coverage: setup
	@echo "Running tests with coverage..."
	go test -cover ./...

# Run all tests (alias for test-all)
test: test-all

# Run specific test suites
test-qa: setup
	@echo "Running Q&A tests..."
	go test -v ./qa_simple

# Run specific test patterns
test-simple: setup
	@echo "Running simple tests..."
	go test -v -run "TestSimple" ./qa_simple

test-conversation: setup
	@echo "Running conversation history tests..."
	go test -v -run "TestConversation" ./qa_simple

test-multiple: setup
	@echo "Running multiple questions tests..."
	go test -v -run "TestMultiple" ./qa_simple

# Run tests with specific verbosity
test-verbose: setup
	@echo "Running tests with verbose output..."
	go test -v -args -test.v=true ./...

# Run tests in parallel (be careful with API rate limits)
test-parallel: setup
	@echo "Running tests in parallel..."
	go test -parallel 3 -v ./...

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	go clean ./...
	@echo "Clean complete!"

# Validate code quality
lint: setup
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Check if environment is properly configured
check-env:
	@echo "Checking environment configuration..."
	@if [ -z "$$TEST_AXONHUB_API_KEY" ]; then \
		echo "WARNING: TEST_AXONHUB_API_KEY not set"; \
	else \
		echo "✓ TEST_AXONHUB_API_KEY is set"; \
	fi
	@echo "TEST_GEMINI_BASE_URL: $${TEST_GEMINI_BASE_URL:-http://localhost:8090/gemini}"
	@echo "TEST_TRACE_ID: $${TEST_TRACE_ID:-test-trace-123}"
	@echo "TEST_THREAD_ID: $${TEST_THREAD_ID:-test-thread-456}"
	@echo "TEST_MODEL: $${TEST_MODEL:-gemini-1.5-flash}"

# Quick validation (fast tests only)
quick: setup check-env
	@echo "Running quick validation..."
	go test -short -v ./qa_simple

# Full validation with all features
validate: setup check-env lint test-coverage
	@echo "Full validation complete!"

# Show help
help:
	@echo "Available targets:"
	@echo "  all              - Run all tests (default)"
	@echo "  setup           - Install dependencies"
	@echo "  test            - Run all tests"
	@echo "  test-all        - Run all tests with verbose output"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  test-qa         - Run Q&A tests only"
	@echo "  test-simple     - Run simple tests"
	@echo "  test-conversation - Run conversation history tests"
	@echo "  test-multiple   - Run multiple questions tests"
	@echo "  test-verbose    - Run tests with verbose output"
	@echo "  test-parallel   - Run tests in parallel"
	@echo "  clean           - Clean test artifacts"
	@echo "  lint            - Run linter"
	@echo "  fmt             - Format code"
	@echo "  check-env       - Check environment configuration"
	@echo "  quick           - Quick validation (fast tests)"
	@echo "  validate        - Full validation (lint + tests + coverage)"
	@echo "  help            - Show this help message"

# Environment setup reminder
env-reminder:
	@echo "=== Environment Setup Reminder ==="
	@echo "Make sure you have set the following environment variables:"
	@echo "  export TEST_AXONHUB_API_KEY='your-axonhub-api-key'"
	@echo "  export TEST_GEMINI_BASE_URL='http://localhost:8090/gemini'  # Optional"
	@echo "  export TEST_TRACE_ID='test-trace-123'                       # Optional"
	@echo "  export TEST_THREAD_ID='test-thread-456'                     # Optional"
	@echo "  export TEST_MODEL='gemini-1.5-flash'                        # Optional"
	@echo ""
	@echo "You can also create a .env file in the test directory."
	@echo "==================================="
