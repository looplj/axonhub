.PHONY: generate build

# Generate GraphQL and Ent code
generate:
	@echo "Generating GraphQL and Ent code..."
	cd internal/server/gql && go generate
	@echo "Generation completed!"

# Build the application
build:
	@echo "Building axonhub..."
	go build -o axonhub ./cmd/axonhub
	@echo "Build completed!"
