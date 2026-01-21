.PHONY: generate build backend frontend cleanup-db \
	e2e-test e2e-backend-start e2e-backend-stop e2e-backend-status e2e-backend-restart e2e-backend-clean \
	migration-test migration-test-all migration-test-all-dbs \
	sync-faq sync-models filter-logs

# Generate GraphQL and Ent code
generate:
	@echo "Generating GraphQL and Ent code..."
	cd internal/server/gql && go generate
	@echo "Generation completed!"

generate-openapi:
	@echo "Generating GraphQL and Ent code..."
	cd internal/server/gql/openapi && go generate
	@echo "Generation completed!"

# Build the backend application
build-backend:
	@echo "Building axonhub backend..."
	go build -ldflags "-s -w" -tags=nomsgpack -o axonhub ./cmd/axonhub
	@echo "Backend build completed!"

# Build the frontend application
build-frontend:
	@echo "Building axonhub frontend..."
	cd frontend && pnpm vite build
	@echo "Copying frontend dist to server static directory..."
	rm -rf internal/server/static/dist/assets
	mkdir -p internal/server/static/dist
	cp -r frontend/dist/* internal/server/static/dist/
	@echo "Frontend build completed!"

# Build both frontend and backend
build: build-frontend build-backend
	@echo "Full build completed!"

# Cleanup test database - remove all playwright test data
cleanup-db:
	@echo "Cleaning up playwright test data from database..."
	@sqlite3 axonhub.db "DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'pw-test-%' OR first_name LIKE 'pw-test%');"
	@sqlite3 axonhub.db "DELETE FROM user_projects WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'pw-test-%' OR first_name LIKE 'pw-test%');"
	@sqlite3 axonhub.db "DELETE FROM user_projects WHERE project_id IN (SELECT id FROM projects WHERE slug LIKE 'pw-test-%' OR name LIKE 'pw-test-%');"
	@sqlite3 axonhub.db "DELETE FROM api_keys WHERE name LIKE 'pw-test-%';"
	@sqlite3 axonhub.db "DELETE FROM api_keys WHERE user_id IN (SELECT id FROM users WHERE email LIKE 'pw-test-%' OR first_name LIKE 'pw-test%');"
	@sqlite3 axonhub.db "DELETE FROM api_keys WHERE project_id IN (SELECT id FROM projects WHERE slug LIKE 'pw-test-%' OR name LIKE 'pw-test-%');"
	@sqlite3 axonhub.db "DELETE FROM roles WHERE code LIKE 'pw-test-%' OR name LIKE 'pw-test-%';"
	@sqlite3 axonhub.db "DELETE FROM roles WHERE project_id IN (SELECT id FROM projects WHERE slug LIKE 'pw-test-%' OR name LIKE 'pw-test-%');"
	@sqlite3 axonhub.db "DELETE FROM usage_logs WHERE project_id IN (SELECT id FROM projects WHERE slug LIKE 'pw-test-%' OR name LIKE 'pw-test-%');"
	@sqlite3 axonhub.db "DELETE FROM requests WHERE project_id IN (SELECT id FROM projects WHERE slug LIKE 'pw-test-%' OR name LIKE 'pw-test-%');"
	@sqlite3 axonhub.db "DELETE FROM users WHERE email LIKE 'pw-test-%' OR first_name LIKE 'pw-test%';"
	@sqlite3 axonhub.db "DELETE FROM projects WHERE slug LIKE 'pw-test-%' OR name LIKE 'pw-test-%';"
	@echo "Cleanup completed!"

# --- E2E Testing ---

# Run the full E2E test suite
e2e-test:
	@echo "Running E2E tests..."
	@./scripts/e2e/e2e-test.sh

# Start the E2E backend service
e2e-backend-start:
	@echo "Starting E2E backend..."
	@./scripts/e2e/e2e-backend.sh start

# Stop the E2E backend service
e2e-backend-stop:
	@echo "Stopping E2E backend..."
	@./scripts/e2e/e2e-backend.sh stop

# Check E2E backend status
e2e-backend-status:
	@./scripts/e2e/e2e-backend.sh status

# Restart the E2E backend service
e2e-backend-restart:
	@echo "Restarting E2E backend..."
	@./scripts/e2e/e2e-backend.sh restart

# Clean up E2E test files
e2e-backend-clean:
	@echo "Cleaning up E2E test files..."
	@./scripts/e2e/e2e-backend.sh clean

# --- Migration Testing ---

# Test database migration from a specific tag
# Usage: make migration-test TAG=v0.1.0
migration-test:
	@if [ -z "$(TAG)" ]; then echo "Error: TAG is required. Usage: make migration-test TAG=v0.1.0"; exit 1; fi
	@echo "Running migration test from $(TAG)..."
	@./scripts/migration/migration-test.sh $(TAG)

# Run migration tests for all recent stable versions
migration-test-all:
	@echo "Running migration tests for all versions..."
	@./scripts/migration/migration-test-all.sh

# Test migration across all supported database types
# Usage: make migration-test-all-dbs TAG=v0.1.0
migration-test-all-dbs:
	@if [ -z "$(TAG)" ]; then echo "Error: TAG is required. Usage: make migration-test-all-dbs TAG=v0.1.0"; exit 1; fi
	@echo "Running migration tests across all DBs from $(TAG)..."
	@./scripts/migration/test-migration-all-dbs.sh $(TAG)

# --- Data Syncing ---

# Sync FAQ from GitHub issues
sync-faq:
	@echo "Syncing FAQ from GitHub..."
	@node ./scripts/sync/sync-github-faq.js

# Sync model developers data
sync-models:
	@echo "Syncing model developers..."
	@node ./scripts/sync/sync-model-developers.js

# --- Utilities ---

# Filter and analyze load balance logs
filter-logs:
	@echo "Filtering load balance logs..."
	@./scripts/utils/filter-load-balance-logs.sh
