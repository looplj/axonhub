# AxonHub - AI Gateway System

AxonHub is a modern AI Gateway system built with Go backend and React frontend. It provides a unified OpenAI-compatible API layer that transforms requests to various AI providers (OpenAI, Anthropic, AI SDK) using an advanced transformer pipeline architecture with enhanced persistence and system management capabilities.

## 🏗️ Architecture Overview

### Core Design Philosophy

AxonHub employs a **dual-transformer pipeline architecture** that separates concerns between user-facing interfaces and provider-specific transformations:

```
User Request → [Inbound Transformer] → Unified Format → [Outbound Transformer] → Provider API
           ←  [Inbound Transformer] ← Unified Format ← [Outbound Transformer] ← Provider Response
```

### Key Components

#### 1. **LLM Pipeline System** (`internal/llm/pipeline/`)
- **Enhanced Pipeline Processing**: Orchestrates the entire request flow with retry capabilities and channel switching
- **Factory Pattern**: Creates configured pipeline instances with decorators and retry policies
- **Stream Processing**: Native support for both streaming and non-streaming responses
- **Channel Retry**: Automatic failover between available channels for high availability

#### 2. **Transformer Architecture** (`internal/llm/transformer/`)

The transformer system implements a **bidirectional transformation pattern**:

**Inbound Transformers**: Convert user requests to unified format
- Transform HTTP requests to unified `llm.Request` format
- Handle response transformation back to user-expected format
- Support streaming response aggregation
- Provider: OpenAI-compatible, AI SDK

**Outbound Transformers**: Convert unified format to provider-specific APIs
- Transform unified requests to provider HTTP format
- Handle provider response normalization
- Provider-specific streaming format handling
- Providers: OpenAI, Anthropic, AI SDK

**Unified Data Model** (`internal/llm/model.go`):
- OpenAI-compatible base structure with extensions
- Support for advanced features: tool calls, function calling, reasoning content
- Flexible content types: text, images, audio
- Comprehensive parameter support for all major providers

#### 3. **Persistent Chat Processing** (`internal/server/chat/`)
- **PersistentInboundTransformer**: Wraps standard transformers with database persistence
- **PersistentOutboundTransformer**: Handles channel management and retry logic
- **Auto-save Functionality**: Configurable persistence of requests and responses
- **Channel Management**: Dynamic channel switching with state preservation

#### 4. **Decorator System** (`internal/llm/decorator/`)
- **Chain Pattern**: Modular request decoration with priority ordering
- **Extensible Design**: Easy addition of new decorators (authentication, rate limiting, etc.)
- **Context-aware**: Conditional decorator application based on request context

#### 5. **Stream Processing** (`internal/pkg/streams/`)
- **Generic Stream Interface**: Type-safe stream processing utilities
- **Transformation Pipeline**: Map, filter, and aggregate operations
- **SSE Support**: Server-sent events for real-time streaming
- **Chunk Aggregation**: Intelligent aggregation of streaming responses

## 🚀 Key Features

### Multi-Provider AI Gateway
- **Unified API**: Single OpenAI-compatible endpoint for all providers
- **Provider Abstraction**: Seamless switching between OpenAI, Anthropic, AI SDK, and more
- **Advanced Features**: Function calling, tool use, streaming, reasoning content
- **Automatic Failover**: Channel-level retry with provider switching

### Enterprise-Ready Backend
- **Database Layer**: Ent ORM with SQLite, comprehensive entity relationships
- **Authentication & Authorization**: Role-based access control with granular permissions
- **Request Persistence**: Complete audit trail with execution tracking
- **System Management**: Web-based configuration and monitoring
- **GraphQL API**: Flexible query interface for complex data operations

### Modern Frontend Stack
- **React Router v7**: File-based routing with nested layouts
- **TanStack Ecosystem**: Query, Table, Router for optimal DX
- **Shadcn/ui Components**: Beautiful, accessible UI components
- **Real-time Updates**: Live configuration and monitoring
- **Responsive Design**: Mobile-first approach with Tailwind CSS

### Developer Experience
- **Type Safety**: Comprehensive TypeScript support
- **Hot Reload**: Fast development iteration
- **Testing Suite**: Playwright E2E tests with multiple browser support
- **Code Quality**: ESLint, Prettier, golangci-lint integration
- **Docker Support**: Containerized deployment ready

## 🛠️ Development Setup

### Backend (Go)
```bash
# Start the server
go run cmd/axonhub/main.go

# Generate GraphQL schema
cd internal/server/gql && go generate

# Generate Ent ORM code
cd internal/ent && go run entc.go

# Run tests
go test ./...

# Lint code
golangci-lint run

# Build binary
go build cmd/axonhub/main.go
```

### Frontend (React)
```bash
cd frontend

# Install dependencies
pnpm install

# Development server (port 5173)
pnpm dev

# Production build
pnpm build

# Code quality
pnpm lint
pnpm format
pnpm knip

# Testing
pnpm test
pnpm test:ui      # Interactive UI
pnpm test:headed  # Headed browser mode
```

## 📁 Project Structure

### Backend Architecture
```
internal/
├── llm/                    # Core LLM processing
│   ├── pipeline/           # Request pipeline orchestration
│   ├── transformer/        # Bidirectional transformers
│   │   ├── interfaces.go   # Inbound/Outbound interfaces
│   │   ├── openai/         # OpenAI transformer implementation
│   │   ├── anthropic/      # Anthropic transformer implementation
│   │   └── aisdk/          # AI SDK transformer implementation
│   ├── decorator/          # Request decoration chain
│   └── model.go           # Unified data models
├── server/
│   ├── chat/              # Chat processing with persistence
│   ├── api/               # REST and GraphQL handlers
│   ├── biz/               # Business logic layer
│   └── gql/               # GraphQL schema and resolvers
├── ent/                   # Database ORM and schema
├── pkg/                   # Shared utilities
│   ├── httpclient/        # HTTP client abstraction
│   ├── streams/           # Stream processing utilities
│   └── xerrors/           # Error handling utilities
└── scopes/                # Permission management
```

### Frontend Architecture
```
frontend/src/
├── app/                   # React Router v7 app directory
├── routes/                # File-based routing
├── features/              # Feature-based organization
│   ├── dashboard/         # System overview
│   ├── channels/          # AI provider management
│   ├── requests/          # Request monitoring
│   ├── system/            # System configuration
│   └── chats/             # Chat interface
├── components/            # Shared components
└── lib/                   # Utilities and API client
```

## 🔧 Configuration

### Environment Variables
```bash
# Database
DATABASE_URL=axonhub.db

# Server
PORT=8090
FRONTEND_PORT=5173

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

### Provider Configuration
Configure AI providers through the web interface or directly in the database:

```yaml
# OpenAI Configuration
name: "openai"
type: "openai"
base_url: "https://api.openai.com"
api_key: "your-openai-key"

# Anthropic Configuration  
name: "anthropic"
type: "anthropic"
base_url: "https://api.anthropic.com"
api_key: "your-anthropic-key"
```

## 🔄 API Usage

### Chat Completions
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": false
  }'
```

### Streaming Responses
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Tell me a story"}
    ],
    "stream": true
  }'
```

## 🔒 Security & Permissions

### Role-Based Access Control
- **Granular Scopes**: read_channels, write_channels, read_users, read_settings, write_settings
- **Owner Access**: Full system administration
- **Database Privacy**: Ent-level permission enforcement
- **API Key Management**: Secure token-based authentication

### Data Protection
- **Soft Delete**: Safe data handling across all entities
- **Audit Trail**: Complete request and execution logging
- **Configurable Persistence**: Control what data is stored
- **No Sensitive Logging**: Security-first approach to logging

## 📊 Monitoring & Observability

### Built-in Analytics
- **Request Tracking**: Complete request lifecycle monitoring
- **Performance Metrics**: Response times, token usage, error rates
- **Channel Health**: Provider availability and failover statistics
- **Real-time Dashboard**: Live system monitoring

### Integration Ready
- **Structured Logging**: JSON format with contextual information
- **Metrics Export**: Ready for Prometheus/Grafana integration
- **OpenTelemetry**: Distributed tracing support
- **Health Checks**: Service health endpoints

## 🚀 Deployment

### Development
```bash
# Backend
go run cmd/axonhub/main.go

# Frontend (separate terminal)
cd frontend && pnpm dev
```

### Production
```bash
# Build frontend
cd frontend && pnpm build

# Build and run backend
go build cmd/axonhub/main.go
./main
```

### Docker (Coming Soon)
Full containerization support for easy deployment and scaling.

## 🤝 Contributing

1. **Code Style**: Follow existing patterns and conventions
2. **Testing**: Ensure tests pass before submitting PRs
3. **Documentation**: Update relevant documentation
4. **Type Safety**: Maintain TypeScript and Go type safety
5. **Performance**: Consider performance implications of changes

## 📝 License

MIT License - see LICENSE file for details.

---

**AxonHub** - Bridging the gap between AI providers with a unified, powerful, and developer-friendly gateway solution.