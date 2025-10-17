.PHONY: generate build backend frontend cleanup-db

# Generate GraphQL and Ent code
generate:
	@echo "Generating GraphQL and Ent code..."
	cd internal/server/gql && go generate
	@echo "Generation completed!"

# Build the backend application
build-backend:
	@echo "Building axonhub backend..."
	go build -o axonhub ./cmd/axonhub
	@echo "Backend build completed!"

# Build the frontend application
build-frontend:
	@echo "Building axonhub frontend..."
	cd frontend && pnpm vite build
	@echo "Copying frontend dist to server static directory..."
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
