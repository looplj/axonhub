# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AxonHub is an AI Gateway system built with Go backend and React frontend. It provides a unified OpenAI-compatible API layer that transforms requests to various AI providers (OpenAI, Anthropic, etc.) using a transformer chain pattern.

## Development Commands

### Backend (Go)
```bash
# Run the main server
go run cmd/axonhub/main.go

# Generate GraphQL code
cd internal/server/gql && go generate

# Generate Ent ORM code
cd internal/ent && go run entc.go

# Run tests
go test ./...

# Build the application
go build cmd/axonhub/main.go
```

### Frontend (React)
```bash
# Navigate to frontend directory
cd frontend

# Install dependencies
pnpm install

# Start development server
pnpm dev

# Build for production
pnpm build

# Run linting
pnpm lint

# Format code
pnpm format

# Run tests
pnpm test
pnpm test:ui      # UI mode
pnpm test:headed  # Headed mode
```

## Architecture Overview

### Backend Structure
- **Server Layer** (`internal/server/`): HTTP server and route handling with Gin
- **Business Logic** (`internal/server/biz/`): Core business logic and services
- **API Layer** (`internal/server/api/`): REST and GraphQL API handlers
- **Database** (`internal/ent/`): Ent ORM for database operations with SQLite
- **LLM Integration** (`internal/llm/`): AI provider transformers and decorators
- **Auth & Scopes** (`internal/scopes/`): Permission system with role-based access control

### Frontend Structure
- **React Router** for routing with authenticated layouts
- **TanStack Query** for data fetching and caching
- **TanStack Table** for data tables with pagination/filtering
- **GraphQL** for API communication
- **Shadcn/ui** components with Tailwind CSS
- **Zustand** for state management

### Key Components

#### LLM Transformer System
- **Inbound Transformers**: Convert HTTP requests to internal format
- **Outbound Transformers**: Convert internal format to provider-specific formats
- **Decorators**: Chain-based request modification system
- **Supported Providers**: OpenAI, Anthropic, AI SDK

#### Database Schema
- **Users**: Authentication and role management
- **Roles**: Permission groups with scope-based access
- **Channels**: AI provider configurations
- **API Keys**: Authentication tokens
- **Requests**: Request logging and execution tracking
- **Systems**: System configuration

#### Permission System
- **Scope-based permissions**: read_channels, write_channels, read_users, etc.
- **Owner scope**: Full system access
- **Role-based access control**: Users can have multiple roles
- **Ent privacy policies**: Database-level permission enforcement

## Configuration

### Environment Setup
- Uses SQLite database (axonhub.db)
- Configuration loaded from `conf/conf.go`
- Logging with structured JSON output
- FX dependency injection framework

### Development Workflow
1. Backend: Modify Go code, run `go generate` if schema changes
2. Frontend: Use `pnpm dev` for hot reload
3. Database: Schema changes require Ent ORM code generation
4. GraphQL: Run `go generate` in gql directory after schema changes

## Important Files

- `cmd/axonhub/main.go`: Application entry point
- `internal/server/server.go`: HTTP server configuration
- `internal/llm/transformer/interfaces.go`: Transformer interfaces
- `internal/ent/schema/`: Database schema definitions
- `frontend/src/routes/`: Application routing structure
- `frontend/src/features/`: Feature-based component organization

## Testing

- **Backend**: Go unit tests with testify
- **Frontend**: Playwright E2E tests
- **Integration**: Both layers tested together