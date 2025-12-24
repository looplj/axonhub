# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AxonHub is an all-in-one AI development platform that serves as a unified API gateway for multiple AI providers. It provides OpenAI and Anthropic-compatible API interfaces with automatic request transformation, enabling seamless communication between clients and various AI providers through a sophisticated bidirectional data transformation pipeline.

### Core Architecture
- **Transformation Pipeline**: Bidirectional data transformation between clients and AI providers
- **Unified API Layer**: OpenAI/Anthropic-compatible interfaces with automatic translation
- **Channel Management**: Multi-provider support with configurable channels
- **Thread-aware Tracing**: Request tracing with thread linking capabilities
- **Permission System**: RBAC with fine-grained access control
- **System Management**: Web-based configuration interface

## Development Commands

### Backend (Go)
```bash
# Run the main server
go run cmd/axonhub/main.go

# Generate GraphQL and Ent code (run after schema changes)
make generate

# Run tests
go test ./...

# Run linting
golangci-lint run

# Build the application
make build-backend

# Use air for hot reload (development) - server auto-restarts on changes
air
```

### Frontend (React)
```bash
# Navigate to frontend directory
cd frontend

# Install dependencies
pnpm install

# Start development server (port 5173) - already configured with backend proxy
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

### Make Commands
```bash
# Generate GraphQL and Ent code
make generate

# Build backend only
make build-backend

# Build frontend only
make build-frontend

# Build both frontend and backend
make build

# Cleanup test database
make cleanup-db
```

## Architecture Overview

### Technology Stack
- **Backend**: Go 1.25+ with Gin HTTP framework, Ent ORM, gqlgen GraphQL, FX dependency injection
- **Frontend**: React 19 with TypeScript, TanStack Router, TanStack Query, Zustand, Tailwind CSS
- **Database**: SQLite (development), PostgreSQL/MySQL/TiDB (production)
- **Authentication**: JWT with role-based access control

### Backend Structure
- **Server Layer** (`internal/server/`): HTTP server and route handling with Gin
- **Business Logic** (`internal/server/biz/`): Core business logic and services
- **API Layer** (`internal/server/api/`): REST and GraphQL API handlers
- **Database** (`internal/ent/`): Ent ORM for database operations with SQLite
- **LLM Integration** (`internal/llm/`): AI provider transformers and pipeline processing
- **Context Management** (`internal/contexts/`): Context handling utilities
- **Utilities** (`internal/pkg/`): Shared utilities (HTTP client, streams, errors, JSON)
- **Auth & Scopes** (`internal/scopes/`): Permission system with role-based access control

### Frontend Structure
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
- **Supported Providers**: OpenAI, Anthropic, DeepSeek, AI SDK
- **Auto-save**: Configurable persistence of chat requests and responses
- **Load Balancing**: Round-robin and failover strategies

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
- Go version: 1.25+
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
- `internal/ent/schema/`: Database schema definitions
- `internal/pkg/`: Shared utilities and helpers
- `frontend/src/app/`: React Router v7 app directory
- `frontend/src/features/`: Feature-based component organization
- `frontend/src/features/system/`: System management interface

## Key Development Patterns

### Adding a New AI Provider Channel
When introducing a new provider channel, keep backend and frontend changes aligned:

1. **Extend the channel enum in the Ent schema** – add the provider key to the `field.Enum("type")` list in `internal/ent/schema/channel.go` and regenerate Ent artifacts
2. **Wire the outbound transformer** – update the switch in `ChannelService.buildChannel` to construct the correct outbound transformer for the new enum
3. **Sync the frontend schema** – update:
   - Zod schema in `frontend/src/features/channels/data/schema.ts`
   - Channel configuration in `frontend/src/features/channels/data/constants.ts`
   - Internationalization in `frontend/src/locales/en.json` and `frontend/src/locales/zh.json`

### Database Schema Changes
- Always run `make generate` after modifying Ent schemas
- Use `enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")` for testing
- Follow soft delete patterns across all entities

### Error Handling
- Use the unified error response format from `internal/pkg/errors`
- Implement proper error wrapping with context
- Follow middleware-based error recovery patterns

### GraphQL Development
- Never modify `*.resolvers.go` files directly
- Run `make generate` from the project root after schema changes
- Use GraphQL input filtering instead of frontend filtering for data queries

#### Adding New GraphQL Types
When adding new types or inputs to the GraphQL schema, you need to:

1. **Add the type/input definition** in the appropriate `.graphql` file in `internal/server/gql/`
2. **Add type mapping in `gqlgen.yml`** under the `models:` section - map the GraphQL type to the Go type:
   ```yaml
   MyNewType:
     model:
       - github.com/looplj/axonhub/internal/package.MyType
   MyNewInput:
     model:
       - github.com/looplj/axonhub/internal/package.MyInput
   ```
3. **Run `make generate`** to regenerate the code
4. **Implement the resolver** in the appropriate `*_resolvers.go` file

## Testing

- **Backend**: Go unit tests with testify
- **Frontend**: Playwright E2E tests with UI and headed modes
- **Integration**: Both layers tested together
- **Code Quality**: golangci-lint for Go, ESLint for TypeScript
- **E2E Testing**: Use `bash ./scripts/e2e-test.sh` for full integration tests
- **Test Database**: Use in-memory SQLite for isolated tests
- **Test Credentials**: Frontend testing uses `my@example.com` / `pwd123456`

## Key Features in Development
- Enhanced transformer stream aggregation
- Configurable persistence behavior
- System options for controlling data storage
- Improved error handling and recovery mechanisms
- Stream closing when client disconnects
- Real-time request tracing and monitoring

## WindSurf Rules

### General Rules
- All summary files should be stored in `.windsurf/summary` directory if available

### Backend Development Rules
- **CRITICAL**: The server in development is managed by air - it will rebuild and start when code changes, so DO NOT restart manually
- Use `make build-backend` to build the server to ensure successful builds
- When changing any Ent schema or GraphQL schema, run `make generate` to regenerate models and resolvers
- Use `make generate` command to generate GraphQL and Ent code (automatically enters gql directory and runs go generate)
- **FORBIDDEN**: DO NOT ADD ANY NEW METHOD/STRUCTURE/FUNCTION/VARIABLE IN *.resolvers.go files
- Use `enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")` to create a new client for testing
- Always handle errors using the unified error format from `internal/pkg/errors`

### Golang Development Rules
- **REQUIRED**: USE github.com/samber/lo package to handle collection, slice, map, ptr, etc.
- Follow dependency injection patterns using FX framework
- Use structured logging with zap
- Implement proper context propagation

### Frontend Development Rules
- **CRITICAL**: DO NOT restart the development server - it's already started and managed
- Use pnpm as the package manager exclusively
- Use GraphQL input to filter data instead of filtering in the frontend
- Search filters must use debounce to avoid too many requests
- Add sidebar data and route when adding new feature pages
- Use extractNumberID to extract int id from the GUID
- Follow component organization in `frontend/src/features/`

#### Frontend Testing
- Use `my@example.com` as the email and `pwd123456` as the password for login when testing the frontend

#### Frontend i18n Rules
- **REQUIRED**: MUST ADD i18n key in the locales/*.json file if creating a new key in the code
- **REQUIRED**: MUST KEEP THE KEY IN THE CODE AND JSON FILE THE SAME
- Support both English and Chinese translations

## Configuration

### Environment Setup
- Uses SQLite database (axonhub.db) by default
- Configuration loaded from `conf/conf.go` with YAML and env var support
- Logging with structured JSON output using zap
- FX dependency injection framework
- Frontend development server: port 5173 (proxies to backend)
- Backend API: port 8090

### Database Support
- **Development**: SQLite (default, auto-migration)
- **Production**: PostgreSQL 15+, MySQL 8.0+, TiDB V8.0+
- **Cloud**: TiDB Cloud, Neon DB (serverless options)
- All support automatic schema migration

### Key Configuration Files
- `config.yml`: Main configuration file
- `internal/ent/schema/`: Database schema definitions
- `.env`: Environment variables (optional)
- `frontend/vite.config.ts`: Frontend build configuration

## Common Development Tasks

### Running a Single Test
```bash
# Backend: Run specific test
go test -v -run TestFunctionName ./path/to/package

# Frontend: Run specific test file
pnpm test path/to/test.spec.ts
```

### Database Migration
```bash
# Generate new migration after schema changes
make generate

# The migration will be auto-applied on server start
```

### Debugging Stream Processing
- Check `internal/llm/pipeline/` for stream aggregation logic
- Monitor SSE connections in browser DevTools Network tab
- Use request tracing with `AH-Trace-Id` header for debugging

### Performance Optimization
- Use request tracing to identify bottlenecks
- Monitor database queries with Ent's built-in logging
- Check channel load balancing configuration for optimal distribution