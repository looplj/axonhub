# Real-Time Request Updates via Server-Sent Events (SSE)

## Overview

Replace the current 10-second polling mechanism on the requests page with Server-Sent Events (SSE) for real-time updates when Auto Refresh is enabled. This will provide instant feedback when new requests are created or existing requests are updated.

## Current State

### Frontend
- **Location**: `frontend/src/features/requests/index.tsx`
- **Mechanism**: `useInterval` hook polls every 10 seconds
- **Trigger**: Only when `autoRefresh === true` AND `isFirstPage === true`
- **Action**: Calls `refetch()` from React Query to reload GraphQL data

### Backend
- **Request Creation**: `internal/server/biz/request.go:113` - `CreateRequest()`
- **Request Updates**: 
  - `UpdateRequestCompleted()` (line 352) - when response received
  - `UpdateRequestStatus()` (line 765) - status changes (failed, canceled)
  - `UpdateRequestChannelID()` (line 788) - channel assignment
- **No Event System**: Currently no pub/sub or notification mechanism

## Architecture Decision: Custom Event Broker

After evaluating libraries (NATS, Watermill, Redis Pub/Sub), we're implementing a custom in-memory event broker because:

1. **Simple Requirements**: In-process, in-memory pub/sub for single-instance deployment
2. **Existing Infrastructure**: Already have SSE working (`chat.go:104`)
3. **Minimal Code**: ~250-300 lines total vs thousands with libraries
4. **Zero Dependencies**: No new servers or infrastructure
5. **Extensible**: Easy to add trace/channel events later

---

## Implementation Plan

### Phase 1: Backend Event Broker (Core Infrastructure)

#### 1.1 Event Type Definitions

**File**: `internal/server/events/types.go` (~50 lines)

**Purpose**: Define strongly-typed event structures for type safety and extensibility.

**Implementation**:
```go
package events

import "time"

// EventType represents the type of event being published
type EventType string

const (
    EventTypeRequestCreated   EventType = "request.created"
    EventTypeRequestUpdated   EventType = "request.updated"
    EventTypeRequestCompleted EventType = "request.completed"
    // Future: EventTypeTraceCreated, EventTypeChannelUpdated, etc.
)

// Topic represents the event topic/channel
type Topic string

const (
    TopicRequests Topic = "requests"
    // Future: TopicTraces, TopicChannels
)

// Event is the base event structure
type Event struct {
    Type      EventType   `json:"type"`
    Topic     Topic       `json:"topic"`
    Timestamp time.Time   `json:"timestamp"`
    Payload   interface{} `json:"payload"`
}

// RequestEventPayload contains request-specific event data
type RequestEventPayload struct {
    RequestID int    `json:"request_id"`
    ProjectID int    `json:"project_id"`
    Status    string `json:"status"`
    ModelID   string `json:"model_id,omitempty"`
    Source    string `json:"source,omitempty"`
    Stream    bool   `json:"stream"`
    
    // Include enough data for UI to update without additional fetch
    APIKeyID   *int       `json:"api_key_id,omitempty"`
    ChannelID  *int       `json:"channel_id,omitempty"`
    CreatedAt  time.Time  `json:"created_at"`
}
```

**Testing**:
- Unit tests for JSON serialization/deserialization
- Validate event type constants are unique
- Test payload structure completeness

---

#### 1.2 Event Broker Implementation

**File**: `internal/server/events/broker.go` (~150 lines)

**Purpose**: Thread-safe in-memory pub/sub broker with topic-based filtering.

**Key Features**:
- Subscribe to topics with optional project ID filtering
- Automatic cleanup of disconnected subscribers
- Non-blocking publish (buffered channels)
- Graceful shutdown support

**Implementation**:
```go
package events

import (
    "context"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/looplj/axonhub/internal/log"
)

// EventBroker manages event publication and subscription
type EventBroker struct {
    mu          sync.RWMutex
    subscribers map[string]*Subscriber
    logger      log.Logger
}

// Subscriber represents a client subscribed to events
type Subscriber struct {
    ID        string
    Topic     Topic
    ProjectID *int              // nil = all projects
    Events    chan *Event       // buffered channel
    ctx       context.Context
    cancel    context.CancelFunc
}

// NewEventBroker creates a new event broker
func NewEventBroker(logger log.Logger) *EventBroker {
    return &EventBroker{
        subscribers: make(map[string]*Subscriber),
        logger:      logger,
    }
}

// Subscribe creates a new subscription to a topic
// projectID == nil subscribes to all projects (admin use case)
func (b *EventBroker) Subscribe(ctx context.Context, topic Topic, projectID *int) *Subscriber {
    subscriberID := uuid.New().String()
    
    subCtx, cancel := context.WithCancel(ctx)
    subscriber := &Subscriber{
        ID:        subscriberID,
        Topic:     topic,
        ProjectID: projectID,
        Events:    make(chan *Event, 100), // buffer 100 events
        ctx:       subCtx,
        cancel:    cancel,
    }
    
    b.mu.Lock()
    b.subscribers[subscriberID] = subscriber
    b.mu.Unlock()
    
    b.logger.Debug(ctx, "New subscriber", 
        log.String("id", subscriberID),
        log.String("topic", string(topic)),
        log.Any("project_id", projectID))
    
    return subscriber
}

// Unsubscribe removes a subscriber and closes its channel
func (b *EventBroker) Unsubscribe(subscriberID string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    if subscriber, exists := b.subscribers[subscriberID]; exists {
        subscriber.cancel()
        close(subscriber.Events)
        delete(b.subscribers, subscriberID)
        
        b.logger.Debug(context.Background(), "Subscriber removed", 
            log.String("id", subscriberID))
    }
}

// Publish sends an event to all matching subscribers
// Non-blocking: uses select with default to avoid slow subscriber blocking
func (b *EventBroker) Publish(ctx context.Context, event *Event) {
    if event == nil {
        return
    }
    
    // Set timestamp if not already set
    if event.Timestamp.IsZero() {
        event.Timestamp = time.Now()
    }
    
    b.mu.RLock()
    defer b.mu.RUnlock()
    
    sent := 0
    dropped := 0
    
    for _, subscriber := range b.subscribers {
        // Filter by topic
        if subscriber.Topic != event.Topic {
            continue
        }
        
        // Filter by project ID if subscriber has project filter
        if subscriber.ProjectID != nil {
            // Extract project ID from payload (type assertion based on topic)
            if !matchesProjectFilter(event, *subscriber.ProjectID) {
                continue
            }
        }
        
        // Non-blocking send to avoid slow subscriber blocking others
        select {
        case subscriber.Events <- event:
            sent++
        default:
            // Channel full, drop event (subscriber too slow)
            dropped++
            b.logger.Warn(ctx, "Dropped event due to full subscriber buffer",
                log.String("subscriber_id", subscriber.ID),
                log.String("event_type", string(event.Type)))
        }
    }
    
    if sent > 0 || dropped > 0 {
        b.logger.Debug(ctx, "Event published",
            log.String("type", string(event.Type)),
            log.Int("sent", sent),
            log.Int("dropped", dropped))
    }
}

// matchesProjectFilter checks if event matches subscriber's project filter
func matchesProjectFilter(event *Event, projectID int) bool {
    switch event.Topic {
    case TopicRequests:
        if payload, ok := event.Payload.(*RequestEventPayload); ok {
            return payload.ProjectID == projectID
        }
    // Future: case TopicTraces, TopicChannels
    }
    return false
}

// Shutdown gracefully shuts down the broker
func (b *EventBroker) Shutdown() {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    for id := range b.subscribers {
        b.Unsubscribe(id)
    }
}

// SubscriberCount returns the current number of subscribers
func (b *EventBroker) SubscriberCount() int {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return len(b.subscribers)
}
```

**Testing**:
- Test concurrent Subscribe/Unsubscribe operations
- Test Publish to multiple subscribers
- Test project ID filtering
- Test buffer overflow behavior (dropped events)
- Test graceful shutdown
- Benchmark: 10k events/sec with 100 concurrent subscribers

---

#### 1.3 Integrate Event Emission in RequestService

**File**: `internal/server/biz/request.go` (modifications)

**Purpose**: Emit events when requests are created or updated.

**Implementation Strategy**:
1. Add `EventBroker` field to `RequestService`
2. Emit events asynchronously (don't block request processing)
3. Include only essential data in events (not full request body)

**Code Changes**:

```go
// Add to RequestService struct (around line 28)
type RequestService struct {
    *AbstractService
    
    SystemService      *SystemService
    UsageLogService    *UsageLogService
    DataStorageService *DataStorageService
    channelCache       xcache.Cache[int]
    eventBroker        *events.EventBroker  // NEW
}

// Update NewRequestService constructor (around line 38)
func NewRequestService(
    ent *ent.Client, 
    systemService *SystemService, 
    usageLogService *UsageLogService, 
    dataStorageService *DataStorageService,
    eventBroker *events.EventBroker,  // NEW parameter
) *RequestService {
    return &RequestService{
        AbstractService: &AbstractService{db: ent},
        SystemService:      systemService,
        UsageLogService:    usageLogService,
        DataStorageService: dataStorageService,
        eventBroker:        eventBroker,
        channelCache: xcache.NewFromConfig[int](/* ... */),
    }
}

// Helper method to emit request events
func (s *RequestService) emitRequestEvent(
    ctx context.Context, 
    eventType events.EventType, 
    req *ent.Request,
) {
    if s.eventBroker == nil {
        return
    }
    
    // Emit asynchronously to avoid blocking database operations
    go func() {
        event := &events.Event{
            Type:  eventType,
            Topic: events.TopicRequests,
            Payload: &events.RequestEventPayload{
                RequestID:  req.ID,
                ProjectID:  req.ProjectID,
                Status:     string(req.Status),
                ModelID:    req.ModelID,
                Source:     string(req.Source),
                Stream:     req.Stream,
                APIKeyID:   ptrOrNil(req.APIKeyID),
                ChannelID:  ptrOrNil(req.ChannelID),
                CreatedAt:  req.CreatedAt,
            },
        }
        
        s.eventBroker.Publish(context.Background(), event)
    }()
}

func ptrOrNil(id int) *int {
    if id == 0 {
        return nil
    }
    return &id
}
```

**Emit Events in Key Functions**:

1. **CreateRequest()** (line 113):
```go
// After successful request creation (line 229)
s.emitRequestEvent(ctx, events.EventTypeRequestCreated, req)
return req, nil
```

2. **UpdateRequestCompleted()** (line 352):
```go
// After successful update (line 428, before return nil)
// Fetch the updated request to emit with latest data
req, _ = client.Request.Get(ctx, requestID)
if req != nil {
    s.emitRequestEvent(ctx, events.EventTypeRequestCompleted, req)
}
return nil
```

3. **UpdateRequestStatus()** (line 765):
```go
// After successful status update (line 773, before return nil)
req, _ := client.Request.Get(ctx, requestID)
if req != nil {
    s.emitRequestEvent(ctx, events.EventTypeRequestUpdated, req)
}
return nil
```

**Testing**:
- Mock event broker, verify events emitted
- Test event emission doesn't block request operations
- Test graceful handling when event broker is nil
- Verify event payloads contain correct data

---

#### 1.4 SSE HTTP Handler for Request Events

**File**: `internal/server/api/events.go` (~100 lines)

**Purpose**: Expose SSE endpoint for clients to receive real-time request events.

**Endpoint**: `GET /admin/events/requests?project_id={id}`

**Implementation**:
```go
package api

import (
    "context"
    "net/http"
    "strconv"
    "time"
    
    "github.com/gin-contrib/sse"
    "github.com/gin-gonic/gin"
    
    "github.com/looplj/axonhub/internal/log"
    "github.com/looplj/axonhub/internal/server/events"
)

type EventHandlers struct {
    broker *events.EventBroker
}

func NewEventHandlers(broker *events.EventBroker) *EventHandlers {
    return &EventHandlers{
        broker: broker,
    }
}

// StreamRequestEvents handles SSE connections for request events
func (h *EventHandlers) StreamRequestEvents(c *gin.Context) {
    ctx := c.Request.Context()
    
    // Parse project ID from query params
    projectIDStr := c.Query("project_id")
    if projectIDStr == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "project_id query parameter required",
        })
        return
    }
    
    projectID, err := strconv.Atoi(projectIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "invalid project_id",
        })
        return
    }
    
    // TODO: Verify user has access to this project
    // This should use the same auth middleware as GraphQL
    
    // Subscribe to request events for this project
    subscriber := h.broker.Subscribe(ctx, events.TopicRequests, &projectID)
    defer h.broker.Unsubscribe(subscriber.ID)
    
    log.Info(ctx, "SSE client connected",
        log.Int("project_id", projectID),
        log.String("subscriber_id", subscriber.ID))
    
    // Set SSE headers
    c.Header("Content-Type", sse.ContentType)
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no") // Disable nginx buffering
    
    // Send initial connection confirmation
    c.SSEvent("connected", gin.H{
        "subscriber_id": subscriber.ID,
        "timestamp":     time.Now().Unix(),
    })
    c.Writer.Flush()
    
    // Heartbeat ticker to keep connection alive
    heartbeatTicker := time.NewTicker(30 * time.Second)
    defer heartbeatTicker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            // Client disconnected
            log.Info(ctx, "SSE client disconnected",
                log.String("subscriber_id", subscriber.ID))
            return
            
        case <-heartbeatTicker.C:
            // Send heartbeat to keep connection alive
            c.SSEvent("heartbeat", gin.H{
                "timestamp": time.Now().Unix(),
            })
            c.Writer.Flush()
            
        case event, ok := <-subscriber.Events:
            if !ok {
                // Channel closed (broker shutdown)
                log.Warn(ctx, "Event channel closed",
                    log.String("subscriber_id", subscriber.ID))
                return
            }
            
            // Send event to client
            c.SSEvent(string(event.Type), event.Payload)
            c.Writer.Flush()
            
            log.Debug(ctx, "Event sent to client",
                log.String("event_type", string(event.Type)),
                log.String("subscriber_id", subscriber.ID))
        }
    }
}
```

**Route Registration** (add to router setup):
```go
// In internal/server/server.go or wherever routes are registered
eventHandlers := api.NewEventHandlers(eventBroker)
adminAPI.GET("/events/requests", eventHandlers.StreamRequestEvents)
```

**Testing**:
- Integration test: Connect SSE client, create request, verify event received
- Test heartbeat delivery every 30s
- Test client disconnect cleanup
- Test multiple concurrent SSE connections
- Test project_id filtering (ensure users only get their project's events)

---

### Phase 2: Frontend SSE Infrastructure

#### 2.1 Generic SSE Hook

**File**: `frontend/src/hooks/useSSE.ts` (~100 lines)

**Purpose**: Reusable hook for any SSE endpoint with automatic reconnection.

**Features**:
- Automatic reconnection with exponential backoff
- Proper cleanup on unmount
- Event type filtering
- Connection state tracking

**Implementation**:
```typescript
import { useEffect, useRef, useState } from 'react'

export interface UseSSEOptions {
  url: string
  enabled: boolean
  onMessage?: (event: MessageEvent) => void
  onOpen?: () => void
  onError?: (error: Event) => void
  reconnectInterval?: number
  maxReconnectAttempts?: number
}

export interface SSEState {
  isConnected: boolean
  isConnecting: boolean
  error: Error | null
  reconnectAttempt: number
}

export function useSSE(options: UseSSEOptions): SSEState {
  const {
    url,
    enabled,
    onMessage,
    onOpen,
    onError,
    reconnectInterval = 3000,
    maxReconnectAttempts = 10,
  } = options

  const [state, setState] = useState<SSEState>({
    isConnected: false,
    isConnecting: false,
    error: null,
    reconnectAttempt: 0,
  })

  const eventSourceRef = useRef<EventSource | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const isMountedRef = useRef(true)

  useEffect(() => {
    isMountedRef.current = true
    return () => {
      isMountedRef.current = false
    }
  }, [])

  const cleanup = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close()
      eventSourceRef.current = null
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }
  }

  const connect = () => {
    if (!enabled || !isMountedRef.current) {
      return
    }

    cleanup()

    setState((prev) => ({ ...prev, isConnecting: true, error: null }))

    const eventSource = new EventSource(url)
    eventSourceRef.current = eventSource

    eventSource.addEventListener('open', () => {
      if (!isMountedRef.current) return
      setState({
        isConnected: true,
        isConnecting: false,
        error: null,
        reconnectAttempt: 0,
      })
      onOpen?.()
    })

    eventSource.addEventListener('message', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event)
    })

    eventSource.addEventListener('error', (event) => {
      if (!isMountedRef.current) return

      const error = new Error('SSE connection error')
      setState((prev) => ({
        ...prev,
        isConnected: false,
        isConnecting: false,
        error,
      }))

      onError?.(event)
      eventSource.close()

      // Attempt reconnection with exponential backoff
      if (state.reconnectAttempt < maxReconnectAttempts) {
        const backoff = Math.min(
          reconnectInterval * Math.pow(2, state.reconnectAttempt),
          30000 // max 30 seconds
        )

        reconnectTimeoutRef.current = setTimeout(() => {
          if (!isMountedRef.current) return
          setState((prev) => ({
            ...prev,
            reconnectAttempt: prev.reconnectAttempt + 1,
          }))
          connect()
        }, backoff)
      }
    })

    // Listen for custom event types
    eventSource.addEventListener('request.created', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event as MessageEvent)
    })

    eventSource.addEventListener('request.updated', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event as MessageEvent)
    })

    eventSource.addEventListener('request.completed', (event) => {
      if (!isMountedRef.current) return
      onMessage?.(event as MessageEvent)
    })
  }

  useEffect(() => {
    if (enabled) {
      connect()
    } else {
      cleanup()
      setState({
        isConnected: false,
        isConnecting: false,
        error: null,
        reconnectAttempt: 0,
      })
    }

    return cleanup
  }, [url, enabled])

  return state
}
```

**Testing**:
- Unit tests with mock EventSource
- Test reconnection logic with fake timers
- Test cleanup on unmount
- Test enabled/disabled toggling

---

#### 2.2 Request-Specific SSE Hook

**File**: `frontend/src/features/requests/hooks/useRequestsSSE.ts` (~80 lines)

**Purpose**: Typed wrapper for request events with React Query integration.

**Implementation**:
```typescript
import { useQueryClient } from '@tanstack/react-query'
import { useSSE } from '@/hooks/useSSE'

interface RequestEventPayload {
  request_id: number
  project_id: number
  status: string
  model_id?: string
  source?: string
  stream: boolean
  api_key_id?: number
  channel_id?: number
  created_at: string
}

interface UseRequestsSSEOptions {
  enabled: boolean
  projectId: number
  onRequestCreated?: (payload: RequestEventPayload) => void
  onRequestUpdated?: (payload: RequestEventPayload) => void
  onRequestCompleted?: (payload: RequestEventPayload) => void
}

export function useRequestsSSE(options: UseRequestsSSEOptions) {
  const { enabled, projectId, onRequestCreated, onRequestUpdated, onRequestCompleted } = options
  const queryClient = useQueryClient()

  const handleMessage = (event: MessageEvent) => {
    try {
      const payload: RequestEventPayload = JSON.parse(event.data)

      // Route to appropriate handler based on event type
      if (event.type === 'request.created') {
        onRequestCreated?.(payload)
        
        // Invalidate queries to refresh the list
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      } else if (event.type === 'request.updated') {
        onRequestUpdated?.(payload)
        
        // Update specific request in cache if we have it
        // Otherwise invalidate to refresh
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      } else if (event.type === 'request.completed') {
        onRequestCompleted?.(payload)
        
        // Update the request in cache
        queryClient.invalidateQueries({ queryKey: ['requests'] })
      }
    } catch (error) {
      console.error('Failed to parse SSE event:', error)
    }
  }

  const sseState = useSSE({
    url: `/admin/events/requests?project_id=${projectId}`,
    enabled,
    onMessage: handleMessage,
    onOpen: () => {
      console.log('SSE connection established for requests')
    },
    onError: (error) => {
      console.error('SSE connection error:', error)
    },
  })

  return sseState
}
```

**Testing**:
- Test event parsing
- Test React Query invalidation calls
- Test different event types routed correctly
- Test error handling for malformed events

---

### Phase 3: Frontend Integration

#### 3.1 Integrate SSE into Requests Page

**File**: `frontend/src/features/requests/index.tsx` (modifications)

**Changes**:
1. Import `useRequestsSSE` hook
2. Replace `useInterval` with SSE when enabled
3. Keep polling as fallback

**Implementation**:
```typescript
import { useState, useCallback } from 'react';
import { DateRange } from 'react-day-picker';
import { useTranslation } from 'react-i18next';
import { buildDateRangeWhereClause } from '@/utils/date-range';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import useInterval from '@/hooks/useInterval';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { RequestsTable } from './components';
import { RequestsProvider } from './context';
import { useRequests } from './data';
import { useRequestsSSE } from './hooks/useRequestsSSE'; // NEW

function RequestsContent() {
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs, cursorHistory } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'requests-table-page-size',
  });
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [sourceFilter, setSourceFilter] = useState<string[]>([]);
  const [channelFilter, setChannelFilter] = useState<string[]>([]);
  const [apiKeyFilter, setApiKeyFilter] = useState<string[]>([]);
  const [dateRange, setDateRange] = useState<DateRange | undefined>();
  const [autoRefresh, setAutoRefresh] = useState(false);

  // Build where clause with filters
  const whereClause = (() => {
    const where: { [key: string]: any } = {
      ...buildDateRangeWhereClause(dateRange),
    };
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    }
    if (sourceFilter.length > 0) {
      where.sourceIn = sourceFilter;
    }
    if (channelFilter.length > 0) {
      where.channelIDIn = channelFilter;
    }
    if (apiKeyFilter.length > 0) {
      where.apiKeyIDIn = apiKeyFilter;
    }
    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const { data, isLoading, refetch } = useRequests({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  });

  const requests = data?.edges?.map((edge) => edge.node) || [];
  const pageInfo = data?.pageInfo;

  const isFirstPage = !paginationArgs.after && cursorHistory.length === 0;

  // NEW: Use SSE for real-time updates when enabled and on first page
  // TODO: Get actual project ID from context or props
  const projectId = 1; // Replace with actual project ID
  
  const sseState = useRequestsSSE({
    enabled: autoRefresh && isFirstPage,
    projectId,
    onRequestCreated: (payload) => {
      console.log('New request created:', payload.request_id);
      // Query will be invalidated automatically by the hook
    },
    onRequestUpdated: (payload) => {
      console.log('Request updated:', payload.request_id);
    },
    onRequestCompleted: (payload) => {
      console.log('Request completed:', payload.request_id);
    },
  });

  // MODIFIED: Fallback polling only if SSE is not connected
  // This provides graceful degradation if SSE fails
  const shouldPoll = autoRefresh && isFirstPage && !sseState.isConnected;
  
  useInterval(
    () => {
      refetch();
    },
    shouldPoll ? 10000 : null
  );

  // Rest of the component remains the same...
  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after');
    }
  };

  // ... (other handlers unchanged)

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <RequestsTable
        data={requests}
        loading={isLoading}
        pageInfo={pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        statusFilter={statusFilter}
        sourceFilter={sourceFilter}
        channelFilter={channelFilter}
        apiKeyFilter={apiKeyFilter}
        dateRange={dateRange}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onStatusFilterChange={handleStatusFilterChange}
        onSourceFilterChange={handleSourceFilterChange}
        onChannelFilterChange={handleChannelFilterChange}
        onApiKeyFilterChange={handleApiKeyFilterChange}
        onDateRangeChange={handleDateRangeChange}
        onRefresh={refetch}
        showRefresh={isFirstPage}
        autoRefresh={autoRefresh}
        onAutoRefreshChange={setAutoRefresh}
      />
    </div>
  );
}

// ... (rest of file unchanged)
```

**Testing**:
- E2E test: Enable auto-refresh, create request via API, verify appears in UI without polling
- Test SSE connection indicator (optional visual feedback)
- Test fallback to polling when SSE fails
- Test switching pages disables SSE
- Test re-enabling SSE when returning to first page

---

### Phase 4: Dependency Injection & Wiring

#### 4.1 Update Dependency Injection

**File**: `cmd/axonhub/main.go` or DI configuration file

**Changes**:
1. Create EventBroker instance
2. Inject into RequestService
3. Provide to EventHandlers

**Implementation**:
```go
// Create event broker
eventBroker := events.NewEventBroker(logger)

// Provide to RequestService
requestService := biz.NewRequestService(
    entClient,
    systemService,
    usageLogService,
    dataStorageService,
    eventBroker, // NEW
)

// Create event handlers
eventHandlers := api.NewEventHandlers(eventBroker)

// Register routes
adminAPI.GET("/events/requests", eventHandlers.StreamRequestEvents)

// Ensure graceful shutdown
// In shutdown handler:
eventBroker.Shutdown()
```

#### 4.2 Update All RequestService Instantiations

**Files to Update**:
- `internal/server/orchestrator/*.go` - anywhere RequestService is created
- Test files creating RequestService mocks

**Migration Strategy**:
1. Add `eventBroker` parameter to constructor
2. Pass `nil` for tests that don't need events
3. Update production code to pass real broker

---

### Phase 5: Testing Strategy

#### 5.1 Backend Unit Tests

**File**: `internal/server/events/broker_test.go`

**Tests**:
- `TestEventBroker_PublishSubscribe` - basic pub/sub
- `TestEventBroker_ProjectFiltering` - verify project isolation
- `TestEventBroker_SlowSubscriber` - verify events dropped when buffer full
- `TestEventBroker_ConcurrentOperations` - thread safety
- `TestEventBroker_Shutdown` - graceful cleanup

**File**: `internal/server/biz/request_test.go` (additions)

**Tests**:
- `TestRequestService_EmitsEventOnCreate`
- `TestRequestService_EmitsEventOnUpdate`
- `TestRequestService_EmitsEventOnComplete`
- `TestRequestService_EventEmissionDoesNotBlockRequest`

#### 5.2 Backend Integration Tests

**File**: `integration_test/events_test.go`

**Tests**:
- `TestSSERequestEventsEndpoint` - connect, verify events received
- `TestSSEMultipleClients` - multiple concurrent connections
- `TestSSEProjectFiltering` - verify isolation between projects
- `TestSSEHeartbeat` - verify heartbeat sent every 30s
- `TestSSEClientDisconnect` - verify cleanup

#### 5.3 Frontend Unit Tests

**File**: `frontend/src/hooks/__tests__/useSSE.test.ts`

**Tests**:
- Test connection lifecycle
- Test reconnection with exponential backoff
- Test cleanup on unmount
- Test enabled/disabled toggle

**File**: `frontend/src/features/requests/hooks/__tests__/useRequestsSSE.test.ts`

**Tests**:
- Test event parsing
- Test React Query invalidation
- Test error handling

#### 5.4 End-to-End Tests

**File**: `frontend/e2e/requests-realtime.spec.ts`

**Tests**:
```typescript
test('requests page receives real-time updates via SSE', async ({ page, request }) => {
  // 1. Navigate to requests page
  await page.goto('/requests');
  
  // 2. Enable auto-refresh
  await page.getByLabel('Auto Refresh').check();
  
  // 3. Wait for SSE connection
  await page.waitForTimeout(1000);
  
  // 4. Create a new request via API
  const response = await request.post('/api/v1/chat/completions', {
    data: {
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'test' }],
    },
  });
  
  // 5. Verify new request appears in table within 2 seconds (not 10s polling)
  await expect(page.getByText('test')).toBeVisible({ timeout: 2000 });
});

test('requests page falls back to polling when SSE fails', async ({ page }) => {
  // Mock SSE endpoint to fail
  await page.route('/admin/events/requests*', (route) => route.abort());
  
  // Enable auto-refresh
  await page.goto('/requests');
  await page.getByLabel('Auto Refresh').check();
  
  // Verify polling still works (wait for refresh interval)
  await page.waitForTimeout(11000);
  await expect(page.getByTestId('requests-table')).toBeVisible();
});
```

---

### Phase 6: Error Handling & Edge Cases

#### 6.1 Backend Error Scenarios

**Scenario**: Event broker is nil
- **Handling**: `emitRequestEvent()` checks for nil, returns early
- **Impact**: No events emitted, but requests still processed normally

**Scenario**: Subscriber buffer full (slow client)
- **Handling**: Events dropped with warning log
- **Impact**: Client may miss some events, will resync on next poll/page refresh

**Scenario**: Database update succeeds but event publish fails
- **Handling**: Event publish is async and best-effort
- **Impact**: Minimal - client will get update on next poll

**Scenario**: EventSource closes unexpectedly
- **Handling**: Client automatically reconnects with backoff
- **Impact**: Brief interruption, then resumes

#### 6.2 Frontend Error Scenarios

**Scenario**: SSE connection fails to establish
- **Handling**: Fallback to polling after max reconnect attempts
- **Impact**: User gets updates via polling instead of real-time

**Scenario**: Malformed SSE event data
- **Handling**: Try-catch in `handleMessage`, log error, continue
- **Impact**: Single event ignored, rest continue processing

**Scenario**: User navigates away during SSE connection
- **Handling**: `useEffect` cleanup closes EventSource
- **Impact**: Clean disconnect, no resource leak

**Scenario**: Network interruption mid-stream
- **Handling**: EventSource fires error, automatic reconnect
- **Impact**: Brief gap in events, then resumes

---

### Phase 7: Monitoring & Observability

#### 7.1 Metrics to Track

**Backend Metrics**:
- Active SSE connections count (`eventBroker.SubscriberCount()`)
- Events published per second (by type)
- Events dropped due to buffer overflow
- Average subscriber buffer usage
- SSE connection duration histogram

**Frontend Metrics** (optional):
- SSE connection state changes
- Reconnection attempts
- Events received per second
- Time to receive event after creation

#### 7.2 Logging Strategy

**Backend Logs**:
- Info: SSE client connect/disconnect with subscriber ID
- Debug: Event published (type, sent count, dropped count)
- Warn: Event dropped due to full buffer
- Error: Unexpected EventSource errors

**Frontend Logs**:
- Console.log: SSE connection state changes (dev only)
- Console.error: Event parsing errors
- Analytics: Track SSE usage vs polling usage

#### 7.3 Health Check

Add endpoint to monitor event broker health:

```go
// GET /admin/health/events
func (h *EventHandlers) HealthCheck(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "active_subscribers": h.broker.SubscriberCount(),
        "status": "healthy",
    })
}
```

---

### Phase 8: Configuration & Feature Flags

#### 8.1 Configuration Options

**Backend Config** (`config.yml`):
```yaml
events:
  enabled: true
  subscriber_buffer_size: 100
  heartbeat_interval: 30s
  max_subscribers: 1000  # prevent resource exhaustion
```

#### 8.2 Feature Flag (Optional)

For gradual rollout, add feature flag:

```go
// In system settings or config
if !config.Events.Enabled {
    eventBroker = nil  // Disables event emission
}
```

Frontend can check if SSE endpoint exists before attempting connection.

---

### Phase 9: Performance Considerations

#### 9.1 Backend Performance

**Memory Usage**:
- Each subscriber: ~1KB (struct) + 100 events × ~500 bytes = ~51KB
- 100 concurrent subscribers: ~5MB memory (negligible)

**CPU Usage**:
- Event publish: O(n) where n = subscriber count
- Read lock contention minimal (RWMutex)
- Async emission prevents blocking request processing

**Optimization**: If >1000 subscribers, consider:
- Topic-based filtering before iterating subscribers
- Fan-out goroutines for parallel delivery
- Connection throttling

#### 9.2 Frontend Performance

**EventSource Connections**:
- 1 connection per browser tab
- Minimal overhead (~1KB/sec for heartbeats)
- Automatic browser-level connection pooling

**React Re-renders**:
- Query invalidation triggers minimal re-renders (React Query optimization)
- Event handlers are memoized to prevent unnecessary subscriptions

---

### Phase 10: Future Enhancements

#### 10.1 Short-Term Enhancements

1. **Optimistic Updates**: Instead of just invalidating, update React Query cache directly
2. **Event Filtering**: Allow clients to subscribe to specific statuses or sources
3. **Visual Indicators**: Show "Live" badge when SSE connected
4. **Event History**: Send last N events on connection for immediate sync

#### 10.2 Long-Term Enhancements

1. **GraphQL Subscriptions**: Migrate to GraphQL subscriptions for consistency
2. **Redis Pub/Sub**: For multi-instance deployments, use Redis as event backbone
3. **Trace Events**: Real-time updates for trace page
4. **Channel Events**: Real-time health status updates
5. **User Notifications**: Toast notifications for important events

#### 10.3 Scalability Path

**Single Instance** (Current):
- In-memory event broker
- Works perfectly for 1-1000 concurrent users

**Multi-Instance** (Future):
- Replace in-memory broker with Redis Pub/Sub
- Each instance subscribes to Redis, publishes to local SSE clients
- Minimal code changes (implement EventBroker interface with Redis)

**High Scale** (Future):
- Dedicated event service (separate microservice)
- NATS or Kafka for durable event log
- Event sourcing and CQRS patterns

---

## Implementation Checklist

### Phase 1: Backend Event Infrastructure
- [ ] Create `internal/server/events/types.go`
- [ ] Create `internal/server/events/broker.go`
- [ ] Write tests for `broker.go`
- [ ] Update `RequestService` to accept `EventBroker`
- [ ] Add `emitRequestEvent()` helper method
- [ ] Emit events in `CreateRequest()`
- [ ] Emit events in `UpdateRequestCompleted()`
- [ ] Emit events in `UpdateRequestStatus()`
- [ ] Write tests for event emission
- [ ] Create `internal/server/api/events.go`
- [ ] Implement `StreamRequestEvents()` handler
- [ ] Register SSE route in router
- [ ] Write integration tests for SSE endpoint

### Phase 2: Frontend SSE Infrastructure
- [ ] Create `frontend/src/hooks/useSSE.ts`
- [ ] Write tests for `useSSE`
- [ ] Create `frontend/src/features/requests/hooks/useRequestsSSE.ts`
- [ ] Write tests for `useRequestsSSE`

### Phase 3: Frontend Integration
- [ ] Import `useRequestsSSE` in requests page
- [ ] Replace polling with SSE when enabled
- [ ] Add fallback polling logic
- [ ] Handle project ID correctly
- [ ] Test SSE connection toggling

### Phase 4: Dependency Injection
- [ ] Update DI container to create EventBroker
- [ ] Inject EventBroker into RequestService
- [ ] Update all RequestService instantiations
- [ ] Update test mocks

### Phase 5: Testing
- [ ] Run all backend unit tests
- [ ] Run backend integration tests
- [ ] Run frontend unit tests
- [ ] Run E2E tests
- [ ] Manual testing with browser DevTools

### Phase 6: Documentation & Deployment
- [ ] Update API documentation
- [ ] Add monitoring/logging
- [ ] Update deployment docs
- [ ] Feature flag configuration
- [ ] Production deployment plan

---

## Success Criteria

1. **Functionality**: Requests appear in UI within 1 second of creation (not 10s)
2. **Reliability**: Graceful degradation to polling if SSE fails
3. **Performance**: No noticeable performance impact on request processing
4. **Scalability**: Supports 100+ concurrent SSE connections without issues
5. **User Experience**: Seamless - users don't notice the change except faster updates
6. **Testing**: >90% test coverage on new code
7. **Monitoring**: Can track SSE connection health in production

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| SSE not supported by proxy/firewall | Medium | High | Fallback to polling always available |
| Memory leak from unclosed connections | Low | Medium | Proper cleanup in useEffect, timeout on backend |
| Event broker becomes bottleneck | Low | Medium | Performance tests, monitoring, profiling |
| Browser tab limits SSE connections | Medium | Low | Only 1 connection per tab, documented limit |
| Race condition in event delivery | Low | High | Careful testing, proper locking in broker |
| Breaking change in API | Low | High | Integration tests, gradual rollout |

---

## Timeline Estimate

| Phase | Estimated Time | Dependencies |
|-------|----------------|--------------|
| Phase 1: Backend Event Infrastructure | 6-8 hours | None |
| Phase 2: Frontend SSE Infrastructure | 3-4 hours | None |
| Phase 3: Frontend Integration | 2-3 hours | Phase 2 |
| Phase 4: Dependency Injection | 1-2 hours | Phase 1 |
| Phase 5: Testing | 4-6 hours | Phases 1-4 |
| Phase 6: Documentation & Deployment | 2-3 hours | Phase 5 |
| **Total** | **18-26 hours** | |

---

## Rollout Plan

### Stage 1: Development & Testing
- Implement in feature branch
- Local testing with manual triggers
- Automated test suite passes

### Stage 2: Staging Deployment
- Deploy to staging environment
- Monitor event broker metrics
- Load testing with realistic traffic

### Stage 3: Production Rollout (Gradual)
- Deploy backend with events disabled
- Enable events for internal testing
- Enable for 10% of users (feature flag)
- Enable for 50% of users
- Enable for 100% of users

### Stage 4: Cleanup
- Remove polling code after 2 weeks of stable SSE
- Or keep polling as permanent fallback

---

## Open Questions

1. **Project ID**: How do we get the current project ID in the frontend? From context? Auth token?
2. **Authentication**: How do we authenticate SSE connections? Reuse existing session?
3. **Multi-tenancy**: Do we need tenant-level isolation or is project-level sufficient?
4. **Event Retention**: Should we buffer last N events for new subscribers? Or just send new events?
5. **Monitoring**: What's the preferred monitoring solution? Prometheus? DataDog?

---

## References

- [MDN: Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [EventSource API](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)
- [React Query Invalidation](https://tanstack.com/query/latest/docs/framework/react/guides/invalidations-from-mutations)
- Existing SSE implementation: `internal/server/api/chat.go:104`
