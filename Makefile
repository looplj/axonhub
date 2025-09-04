.PHONY: generate build backend frontend

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
