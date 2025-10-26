# Anthropic Go SDK Integration Tests Makefile

.PHONY: all test test-all test-qa test-tools test-conversation clean help setup

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

test-tools: setup
	@echo "Running tool calling tests..."
	go test -v ./tool_single
	go test -v ./tool_multiple

test-streaming: setup
	@echo "Running streaming tests..."
	go test -v ./streaming

test-conversation: setup
	@echo "Running conversation tests..."
	go test -v ./conversation

test-trace: setup
	@echo "Running trace tests..."
	go test -v ./trace_multiple_requests

test-thread: setup
	@echo "Running thread tests..."
	go test -v ./tread_multiple_traces

# Run tests with specific verbosity
test-verbose: setup
	@echo "Running tests with verbose output..."
	go test -v -args -test.v=true ./...

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
	@echo "TEST_ANTHROPIC_BASE_URL: $${TEST_ANTHROPIC_BASE_URL:-https://api.anthropic.com}"
	@echo "TEST_MODEL: $${TEST_MODEL:-claude-3-5-sonnet-20241022}"

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
	@echo "  test-tools      - Run tool calling tests only"
	@echo "  test-streaming  - Run streaming tests only"
	@echo "  test-conversation - Run conversation tests only"
	@echo "  test-trace      - Run trace tests only"
	@echo "  test-thread     - Run thread tests only"
	@echo "  test-verbose    - Run tests with verbose output"
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
	@echo "  export TEST_AXONHUB_API_KEY='your-anthropic-api-key'"
	@echo "  export TEST_ANTHROPIC_BASE_URL='https://api.anthropic.com'  # Optional"
	@echo "  export TEST_MODEL='claude-3-5-sonnet-20241022'              # Optional"
	@echo ""
	@echo "You can also create a .env file in the test directory."
	@echo "==================================="
