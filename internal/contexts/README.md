# Context Management for Thread and Trace Entities

This document describes the implementation of thread and trace entity management in the context system.

## Overview

Thread and Trace entities have been added to the context management system, allowing them to be stored and retrieved throughout the request lifecycle.

## Components

### 1. Context Methods

#### Thread Context (`thread.go`)
- `WithThread(ctx, thread)` - Stores a thread entity in the context
- `GetThread(ctx)` - Retrieves the thread entity from the context

#### Trace Context (`trace.go`)
- `WithTrace(ctx, trace)` - Stores a trace entity in the context
- `GetTrace(ctx)` - Retrieves the trace entity from the context

### 2. Business Logic Layer

#### ThreadService (`internal/server/biz/thread.go`)
- `GetOrCreateThread(ctx, threadID, projectID)` - Gets an existing thread or creates a new one
- `GetThreadByID(ctx, threadID, projectID)` - Retrieves a thread by its ID

#### TraceService (`internal/server/biz/trace.go`)
- `GetOrCreateTrace(ctx, traceID, projectID, threadID)` - Gets an existing trace or creates a new one
- `GetTraceByID(ctx, traceID, projectID)` - Retrieves a trace by its ID

### 3. Middleware

#### Thread Middleware (`internal/server/middleware/thread.go`)
- `WithThreadID(threadService)` - Extracts `X-Thread-ID` header and gets/creates the thread entity
- Requires project ID in context
- Non-blocking: continues on error

#### Trace Middleware (`internal/server/middleware/trace.go`)
- `WithTraceID(traceService)` - Extracts `X-Trace-ID` header and gets/creates the trace entity
- Requires project ID in context
- Optionally links to thread if available in context
- Non-blocking: continues on error

## Usage

### Setting up Middleware

```go
// Initialize services
threadService := biz.NewThreadService()
traceService := biz.NewTraceService()

// Add middleware to router (order matters)
router.Use(middleware.WithProjectID())      // Must come first
router.Use(middleware.WithThreadID(threadService))
router.Use(middleware.WithTraceID(traceService))
```

### Using in Handlers

```go
func MyHandler(c *gin.Context) {
    // Get thread from context
    if thread, ok := contexts.GetThread(c.Request.Context()); ok {
        // Use thread
        fmt.Printf("Thread ID: %s\n", thread.ThreadID)
    }
    
    // Get trace from context
    if trace, ok := contexts.GetTrace(c.Request.Context()); ok {
        // Use trace
        fmt.Printf("Trace ID: %s\n", trace.TraceID)
        if trace.ThreadID != 0 {
            fmt.Printf("Linked to Thread: %d\n", trace.ThreadID)
        }
    }
}
```

### Client Usage

Clients can send thread and trace IDs via headers:

```bash
curl -H "X-Project-ID: proj_123" \
     -H "X-Thread-ID: thread-abc-123" \
     -H "X-Trace-ID: trace-xyz-456" \
     https://api.example.com/endpoint
```

## Important Notes

1. **Global Uniqueness**: Both `thread_id` and `trace_id` are globally unique across all projects (enforced by database schema)

2. **Project Requirement**: Both middleware require a project ID in the context. If no project ID is present, they skip entity creation.

3. **Non-blocking**: Both middleware are non-blocking - if entity creation fails, the request continues without the entity in context.

4. **Thread-Trace Relationship**: Traces can optionally be linked to threads. If a thread is in the context when a trace is created, the trace will be automatically linked to it.

5. **Idempotency**: The `GetOrCreate` methods are idempotent - calling them multiple times with the same ID returns the same entity.

## Testing

Comprehensive test coverage includes:
- Context storage and retrieval tests
- Business logic tests with database operations
- Middleware integration tests
- Idempotency tests
- Error handling tests

Run tests:
```bash
go test ./internal/contexts/
go test ./internal/server/biz/thread_test.go ./internal/server/biz/thread.go
go test ./internal/server/biz/trace_test.go ./internal/server/biz/trace.go
go test ./internal/server/middleware/thread_test.go ./internal/server/middleware/thread.go
go test ./internal/server/middleware/trace_test.go ./internal/server/middleware/trace.go
```
