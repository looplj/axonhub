# OpenAI Go SDK Integration Tests Makefile

.PHONY: all test test-all test-qa test-tools test-streaming test-conversation test-exclude-responses-extra-body clean help setup

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
	go test --count=1 -v ./...

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
	go test --count=1 -v ./tool_single
	go test --count=1 -v ./tool_multiple

test-streaming: setup
	@echo "Running streaming tests..."
	go test -v ./streaming

test-conversation: setup
	@echo "Running conversation tests..."
	go test -v ./conversation

# Run all tests except responses and extra_body
test-exclude-responses-extra-body: setup
	@echo "Running all tests except responses and extra_body..."
	go test --count=1 -parallel=1 -v ./qa_simple ./tool_single ./tool_multiple ./streaming ./conversation ./thread_multiple_traces ./trace_multiple_requests

# Run specific test patterns
test-simple: setup
	@echo "Running simple tests..."
	go test -v -run "TestSimple" ./qa_simple

test-weather: setup
	@echo "Running weather tool tests..."
	go test -v -run "TestSingleToolCall" ./tool_single

test-calculator: setup
	@echo "Running calculator tool tests..."
	go test -v -run "TestSingleToolCallMath" ./tool_single

test-multi: setup
	@echo "Running multiple tool tests..."
	go test --count=1 -v ./tool_multiple

test-stream: setup
	@echo "Running streaming tests..."
	go test -v ./streaming

test-convo: setup
	@echo "Running conversation tests..."
	go test -v ./conversation

# Run tests with specific verbosity
test-verbose: setup
	@echo "Running tests with verbose output..."
	go test -v -args -test.v=true ./...

# Run tests in parallel (be careful with API rate limits)
test-parallel: setup
	@echo "Running tests in parallel..."
	go test --count=1 -parallel 3 -v ./...

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
	@if [ -z "$$OPENAI_API_KEY" ]; then \
		echo "WARNING: OPENAI_API_KEY not set"; \
	else \
		echo "✓ OPENAI_API_KEY is set"; \
	fi
	@echo "OPENAI_BASE_URL: $${OPENAI_BASE_URL:-https://api.openai.com/v1}"
	@echo "TEST_TRACE_ID: $${TEST_TRACE_ID:-test-trace-123}"
	@echo "TEST_THREAD_ID: $${TEST_THREAD_ID:-test-thread-456}"
	@echo "TEST_PROJECT_ID: $${TEST_PROJECT_ID:-test-project}"

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
	@echo "  test-exclude-responses-extra-body - Run all tests except responses and extra_body"
	@echo "  test-simple     - Run simple tests"
	@echo "  test-weather    - Run weather tool tests"
	@echo "  test-calculator - Run calculator tool tests"
	@echo "  test-multi      - Run multiple tool tests"
	@echo "  test-stream     - Run streaming tests"
	@echo "  test-convo      - Run conversation tests"
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
	@echo "  export OPENAI_API_KEY='your-openai-api-key'"
	@echo "  export OPENAI_BASE_URL='https://api.openai.com/v1'  # Optional"
	@echo "  export TEST_TRACE_ID='test-trace-123'              # Optional"
	@echo "  export TEST_THREAD_ID='test-thread-456'            # Optional"
	@echo "  export TEST_PROJECT_ID='test-project'              # Optional"
	@echo ""
	@echo "You can also create a .env file in the test directory."
	@echo "==================================="
