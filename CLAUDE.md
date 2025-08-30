# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AxonHub is an AI Gateway system built with Go backend and React frontend. It provides a unified OpenAI-compatible API layer that transforms requests to various AI providers (OpenAI, Anthropic, etc.) using a transformer chain pattern with enhanced persistence and system management capabilities.

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

# Run linting
golangci-lint run

# Build the application
go build cmd/axonhub/main.go
```

### Frontend (React)
```bash
# Navigate to frontend directory
cd frontend

# Install dependencies
pnpm install

# Start development server (port 5173)
pnpm dev

# Build for production
pnpm build

# Run linting
pnpm lint

# Format code
pnpm format

# Check for unused dependencies
pnpm knip

# Run tests
pnpm test
pnpm test:ui      # UI mode
pnpm test:headed  # Headed mode
```

## Architecture Overview

### Backend Structure
- **Server Layer** (`internal/server/`): HTTP server and route handling with Gin
- **Business Logic** (`internal/server/biz/`): Core business logic and services
- **Chat Processing** (`internal/server/chat/`): Dedicated chat handling with persistence
- **API Layer** (`internal/server/api/`): REST and GraphQL API handlers
- **Database** (`internal/ent/`): Ent ORM for database operations with SQLite
- **LLM Integration** (`internal/llm/`): AI provider transformers and pipeline processing
- **Context Management** (`internal/contexts/`): Context handling utilities
- **Utilities** (`internal/pkg/`): Shared utilities (HTTP client, streams, errors, JSON)
- **Auth & Scopes** (`internal/scopes/`): Permission system with role-based access control

### Frontend Structure
- **React Router v7** with app directory structure
- **TanStack Query** for data fetching and caching
- **TanStack Table** for data tables with pagination/filtering
- **TanStack Router** for file-based routing
- **GraphQL** for API communication
- **Shadcn/ui** components with Tailwind CSS
- **Zustand** for state management
- **AI SDK** integration for enhanced AI capabilities

### Key Components

#### LLM Transformer System
- **Pipeline Architecture**: Enhanced request processing with retry capabilities
- **Persistent Transformers**: `PersistentInboundTransformer` and `PersistentOutboundTransformer`
- **Stream Processing**: Enhanced SSE support with chunk aggregation
- **Supported Providers**: OpenAI, Anthropic, AI SDK
- **Auto-save**: Configurable persistence of chat requests and responses

#### Database Schema
- **Users**: Authentication and role management with soft delete
- **Roles**: Permission groups with scope-based access
- **Channels**: AI provider configurations
- **API Keys**: Authentication tokens
- **Requests**: Request logging and execution tracking
- **Systems**: System-wide configuration (storeChunks, etc.)
- **Soft Delete**: Data safety across all entities

#### Permission System
- **Enhanced Scopes**: read_channels, write_channels, read_users, read_settings, write_settings
- **Owner scope**: Full system access
- **Role-based access control**: Users can have multiple roles
- **Ent privacy policies**: Database-level permission enforcement
- **Granular permissions**: Fine-grained access control

#### System Management
- **Web Interface**: Complete system settings management
- **Configuration Options**: Controllable persistence and system behavior
- **Real-time Updates**: Live configuration changes
- **GraphQL API**: System configuration endpoints

## Configuration

### Environment Setup
- Uses SQLite database (axonhub.db)
- Configuration loaded from `conf/conf.go`
- Logging with structured JSON output using zap
- FX dependency injection framework
- Go version: 1.24.4
- Frontend development server: port 5173
- Backend API: port 8090

### Development Workflow
1. Backend: Modify Go code, run `go generate` if schema changes
2. Frontend: Use `pnpm dev` for hot reload with proxy to backend
3. Database: Schema changes require Ent ORM code generation
4. GraphQL: Run `go generate` in gql directory after schema changes
5. Linting: Run `golangci-lint run` for Go, `pnpm lint` for frontend

## Important Files

- `cmd/axonhub/main.go`: Application entry point
- `internal/server/server.go`: HTTP server configuration
- `internal/llm/pipeline/`: Pipeline processing architecture
- `internal/server/chat/`: Chat processing with persistence
- `internal/ent/schema/`: Database schema definitions
- `internal/pkg/`: Shared utilities and helpers
- `frontend/src/app/`: React Router v7 app directory
- `frontend/src/features/`: Feature-based component organization
- `frontend/src/features/system/`: System management interface

## Testing

- **Backend**: Go unit tests with testify
- **Frontend**: Playwright E2E tests with UI and headed modes
- **Integration**: Both layers tested together
- **Code Quality**: golangci-lint for Go, ESLint for TypeScript


### Key Features in Development
- Enhanced transformer stream aggregation
- Configurable persistence behavior
- System options for controlling data storage
- Improved error handling and recovery mechanisms
- @internal/server/api/  Fix the stream not close when client closed
- @internal/server/api/  Fix the stream not close when client closed