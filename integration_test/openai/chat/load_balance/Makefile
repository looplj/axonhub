.PHONY: test test-trace test-weight test-advanced test-all clean help

# Default target
.DEFAULT_GOAL := help

# Test configuration
TEST_TIMEOUT ?= 5m
TEST_FLAGS ?= -v
GO_TEST := go test --count=1 $(TEST_FLAGS) -timeout $(TEST_TIMEOUT)

help: ## Show this help message
	@echo "Load Balance Integration Tests"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
	@echo ""
	@echo "Environment Variables:"
	@echo "  TEST_AXONHUB_API_KEY      API key for AxonHub (required)"
	@echo "  TEST_OPENAI_BASE_URL      Base URL for AxonHub (default: http://localhost:8090/v1)"
	@echo "  TEST_MODEL                Model to use for tests (default: deepseek-chat)"
	@echo "  TEST_DISABLE_TRACE        Disable trace ID generation (default: false)"
	@echo "  TEST_DISABLE_THREAD       Disable thread ID generation (default: false)"
	@echo "  TEST_TIMEOUT              Test timeout (default: 5m)"
	@echo "  TEST_FLAGS                Additional go test flags (default: -v)"

test: ## Run all load balance tests
	@echo "Running all load balance tests..."
	$(GO_TEST) ./...

test-trace: ## Run trace-based load balancing tests
	@echo "Running trace-based load balancing tests..."
	$(GO_TEST) -run TestTrace ./...

test-weight: ## Run weight-based load balancing tests
	@echo "Running weight-based load balancing tests..."
	$(GO_TEST) -run TestWeight ./...

test-advanced: ## Run advanced load balancing tests
	@echo "Running advanced load balancing tests..."
	$(GO_TEST) -run "TestConnection|TestError|TestInactivity|TestScaling|TestPriority|TestComposite" ./...

test-failover: ## Run failover and retry tests
	@echo "Running failover and retry tests..."
	$(GO_TEST) -run "TestFailover|TestRetry" ./...

test-debug: ## Run tests with debug output
	@echo "Running tests with debug output..."
	@echo "Note: Set AXONHUB_DEBUG_LOAD_BALANCER_ENABLED=true on server for detailed logs"
	$(GO_TEST) -run TestLoadBalancingDebugMode ./...

test-quick: ## Run quick smoke tests
	@echo "Running quick smoke tests..."
	$(GO_TEST) -run "TestTraceBasedLoadBalancing$$|TestWeightBasedLoadBalancing$$" -short ./...

test-concurrent: ## Run concurrent load tests
	@echo "Running concurrent load tests..."
	$(GO_TEST) -run "TestWeightRoundRobinLoadBalancing|TestConnectionAwareLoadBalancing" ./...

test-all: test ## Run all tests (alias for 'test')

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GO_TEST) -coverprofile=coverage.out ./...
	@echo ""
	@echo "Coverage report:"
	@go tool cover -func=coverage.out | tail -n 1

test-coverage-html: test-coverage ## Generate HTML coverage report
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test --count=1 -bench=. -benchmem ./...

clean: ## Clean test artifacts
	@echo "Cleaning test artifacts..."
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

check-env: ## Check required environment variables
	@echo "Checking environment variables..."
	@if [ -z "$$TEST_AXONHUB_API_KEY" ]; then \
		echo "❌ ERROR: TEST_AXONHUB_API_KEY is not set"; \
		echo "   Please set it with: export TEST_AXONHUB_API_KEY=your-api-key"; \
		exit 1; \
	else \
		echo "✅ TEST_AXONHUB_API_KEY is set"; \
	fi
	@echo "✅ TEST_OPENAI_BASE_URL: $${TEST_OPENAI_BASE_URL:-http://localhost:8090/v1 (default)}"
	@echo "✅ TEST_MODEL: $${TEST_MODEL:-deepseek-chat (default)}"
	@echo "✅ TEST_DISABLE_TRACE: $${TEST_DISABLE_TRACE:-false (default)}"
	@echo "✅ TEST_DISABLE_THREAD: $${TEST_DISABLE_THREAD:-false (default)}"

verify: check-env ## Verify test environment is ready
	@echo ""
	@echo "Verifying server connectivity..."
	@BASE_URL=$${TEST_OPENAI_BASE_URL:-http://localhost:8090/v1}; \
	if curl -s -f -o /dev/null "$$BASE_URL/../health" 2>/dev/null; then \
		echo "✅ Server is reachable at $$BASE_URL"; \
	else \
		echo "❌ WARNING: Cannot reach server at $$BASE_URL"; \
		echo "   Make sure AxonHub server is running"; \
	fi
	@echo ""
	@echo "Environment is ready for testing!"

list: ## List all available tests
	@echo "Available tests:"
	@grep -r "^func Test" . --include="*.go" | sed 's/.*func /  - /' | sed 's/(.*$$//' | sort

watch: ## Watch for changes and run tests automatically
	@echo "Watching for changes..."
	@which fswatch > /dev/null || (echo "fswatch not found. Install with: brew install fswatch" && exit 1)
	@fswatch -o . | xargs -n1 -I{} make test

# Development helpers
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./... || echo "Note: Install golangci-lint for linting"

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

# Quick test combinations
test-basic: test-trace test-weight ## Run basic load balancing tests

test-full: test-basic test-advanced ## Run comprehensive test suite

# CI/CD targets
ci: check-env test-coverage ## Run tests for CI/CD pipeline
	@echo "CI tests completed successfully"

ci-quick: check-env test-quick ## Run quick tests for CI/CD pipeline
	@echo "Quick CI tests completed successfully"
