package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

const (
	responsesSessionTTL         = 2 * time.Hour
	responsesSessionMaxRecords  = 2048
	responsesSessionMaxBytes    = 64 << 20
	responsesSessionMaxResponse = 1 << 20
)

type responsesSessionStore struct {
	mu         sync.Mutex
	byResponse map[responsesSessionKey]*responsesSessionRecord
	totalBytes int
	load       responsesSessionLoadFunc
}

type responsesSessionLoadFunc func(ctx context.Context, responseID string) (requestBody, responseBody []byte, found bool, err error)

type responsesSessionKey struct {
	scope      string
	responseID string
}

type responsesSessionRecord struct {
	input     []json.RawMessage
	output    []json.RawMessage
	sessionID string
	updatedAt time.Time
	size      int
}

func newResponsesSessionStore(loaders ...responsesSessionLoadFunc) *responsesSessionStore {
	store := new(responsesSessionStore)
	store.byResponse = make(map[responsesSessionKey]*responsesSessionRecord)
	if len(loaders) > 0 {
		store.load = loaders[0]
	}
	return store
}

func (s *responsesSessionStore) prepare(ctx context.Context, body []byte) ([]byte, string) {
	sessionID, _ := shared.GetSessionID(ctx)
	var current map[string]json.RawMessage
	if json.Unmarshal(body, &current) != nil || current == nil {
		return body, responseSessionID(sessionID)
	}

	normalizedInput, inputChanged := normalizeResponseSessionInput(current["input"])
	if inputChanged {
		current["input"] = normalizedInput
	}

	previousID := responseSessionString(current["previous_response_id"])
	if previousID == "" {
		return encodePreparedResponsesBody(body, current, inputChanged), responseSessionID(sessionID)
	}

	record := s.lookupOrLoad(ctx, previousID)
	if record == nil {
		return encodePreparedResponsesBody(body, current, inputChanged), responseSessionID(sessionID)
	}
	if record.sessionID != "" {
		sessionID = record.sessionID
	}
	sessionID = responseSessionID(sessionID)

	merged := cloneResponseSessionPayload(current)

	baseItems, _ := normalizeResponseSessionItems(record.input)
	normalizedOutput, _ := normalizeResponseSessionItems(record.output)
	baseItems = append(baseItems, normalizedOutput...)
	baseItems = append(baseItems, responseSessionInputItems(current["input"])...)
	if len(baseItems) > 0 {
		encoded, err := json.Marshal(baseItems)
		if err != nil {
			return body, sessionID
		}
		merged["input"] = encoded
	} else {
		delete(merged, "input")
	}
	delete(merged, "previous_response_id")

	encoded, err := json.Marshal(merged)
	if err != nil {
		return body, sessionID
	}

	return encoded, sessionID
}

func (s *responsesSessionStore) lookupOrLoad(ctx context.Context, responseID string) *responsesSessionRecord {
	if scope, ok := shared.GetSessionScope(ctx); !ok || strings.TrimSpace(scope) == "" {
		return nil
	}
	if record := s.lookup(ctx, responseID); record != nil || s.load == nil {
		return record
	}

	requestBody, responseBody, found, err := s.load(ctx, responseID)
	if err != nil {
		log.Warn(ctx, "Failed to restore persisted Responses session", log.Cause(err))
		return nil
	}
	if !found {
		return nil
	}

	s.record(ctx, requestBody, responseBody)
	return s.lookup(ctx, responseID)
}

func encodePreparedResponsesBody(original []byte, payload map[string]json.RawMessage, changed bool) []byte {
	if !changed {
		return original
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return original
	}
	return encoded
}

func (s *responsesSessionStore) record(ctx context.Context, requestBody, responseBody []byte) {
	if len(responseBody) == 0 || len(responseBody) > responsesSessionMaxResponse {
		return
	}

	var requestPayload map[string]json.RawMessage
	if json.Unmarshal(requestBody, &requestPayload) != nil || requestPayload == nil {
		return
	}

	var response struct {
		ID     string            `json:"id"`
		Status string            `json:"status"`
		Output []json.RawMessage `json:"output"`
	}
	if json.Unmarshal(responseBody, &response) != nil || response.ID == "" {
		return
	}
	// Both spellings have appeared in Responses-compatible implementations.
	//nolint:misspell
	if response.Status == "failed" || response.Status == "cancelled" || response.Status == "canceled" {
		return
	}

	input := responseSessionInputItems(requestPayload["input"])
	output := cloneResponseSessionValues(response.Output)
	size := rawResponseSessionValuesSize(input) + rawResponseSessionValuesSize(output)
	if size > responsesSessionMaxResponse {
		return
	}

	now := time.Now()
	scope, ok := shared.GetSessionScope(ctx)
	if !ok || strings.TrimSpace(scope) == "" {
		return
	}
	sessionID, _ := shared.GetSessionID(ctx)
	key := responsesSessionKey{scope: scope, responseID: response.ID}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked(now)
	if previous := s.byResponse[key]; previous != nil {
		s.totalBytes -= previous.size
	}
	s.byResponse[key] = &responsesSessionRecord{
		input:     cloneResponseSessionValues(input),
		output:    output,
		sessionID: sessionID,
		updatedAt: now,
		size:      size,
	}
	s.totalBytes += size
	for len(s.byResponse) > responsesSessionMaxRecords || s.totalBytes > responsesSessionMaxBytes {
		if !s.evictOldestLocked() {
			s.totalBytes = 0
			break
		}
	}
}

func (s *responsesSessionStore) lookup(ctx context.Context, responseID string) *responsesSessionRecord {
	now := time.Now()
	scope, ok := shared.GetSessionScope(ctx)
	if !ok || strings.TrimSpace(scope) == "" {
		return nil
	}
	key := responsesSessionKey{scope: scope, responseID: responseID}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked(now)
	record := s.byResponse[key]
	if record == nil {
		return nil
	}

	return &responsesSessionRecord{
		input:     cloneResponseSessionValues(record.input),
		output:    cloneResponseSessionValues(record.output),
		sessionID: record.sessionID,
		updatedAt: record.updatedAt,
		size:      record.size,
	}
}

func (s *responsesSessionStore) evictExpiredLocked(now time.Time) {
	for key, record := range s.byResponse {
		if record == nil || now.Sub(record.updatedAt) > responsesSessionTTL {
			if record != nil {
				s.totalBytes -= record.size
			}
			delete(s.byResponse, key)
		}
	}
}

func (s *responsesSessionStore) evictOldestLocked() bool {
	var oldestKey responsesSessionKey
	var oldest time.Time
	found := false
	for key, record := range s.byResponse {
		if record == nil {
			oldestKey = key
			found = true
			break
		}
		if !found || record.updatedAt.Before(oldest) {
			oldestKey = key
			oldest = record.updatedAt
			found = true
		}
	}
	if !found {
		return false
	}
	if record := s.byResponse[oldestKey]; record != nil {
		s.totalBytes -= record.size
	}
	delete(s.byResponse, oldestKey)
	return true
}

func (s *responsesSessionStore) wrapStream(
	ctx context.Context,
	requestBody []byte,
	stream streams.Stream[*httpclient.StreamEvent],
) streams.Stream[*httpclient.StreamEvent] {
	wrapped := new(responsesSessionStream)
	wrapped.ctx = ctx
	wrapped.store = s
	wrapped.requestBody = append([]byte(nil), requestBody...)
	wrapped.inner = stream

	return wrapped
}

type responsesSessionStream struct {
	ctx         context.Context
	store       *responsesSessionStore
	requestBody []byte
	inner       streams.Stream[*httpclient.StreamEvent]
	chunks      []*httpclient.StreamEvent
	chunksBytes int
	terminal    bool
	recorded    bool
	overflow    bool
}

func (s *responsesSessionStream) Next() bool {
	if !s.inner.Next() {
		return false
	}

	event := s.inner.Current()
	if event != nil {
		if !s.overflow {
			s.chunksBytes += len(event.Type) + len(event.LastEventID) + len(event.Data)
			if s.chunksBytes > responsesSessionMaxResponse {
				s.chunks = nil
				s.overflow = true
			} else {
				s.chunks = append(s.chunks, &httpclient.StreamEvent{
					Type:        event.Type,
					LastEventID: event.LastEventID,
					Data:        append([]byte(nil), event.Data...),
					Size:        event.Size,
				})
			}
		}
		if IsTerminalStreamEvent(event) {
			s.terminal = true
			s.recordCompleted()
		}
	}

	return true
}

func (s *responsesSessionStream) Current() *httpclient.StreamEvent { return s.inner.Current() }

func (s *responsesSessionStream) Err() error { return s.inner.Err() }

func (s *responsesSessionStream) Close() error {
	s.recordCompleted()

	return s.inner.Close()
}

func (s *responsesSessionStream) recordCompleted() {
	if s.recorded || s.overflow || !s.terminal {
		return
	}

	body, _, err := responses.AggregateStreamChunks(s.ctx, s.chunks)
	if err != nil {
		return
	}

	s.store.record(s.ctx, s.requestBody, body)
	s.recorded = true
}

func cloneResponseSessionPayload(payload map[string]json.RawMessage) map[string]json.RawMessage {
	cloned := make(map[string]json.RawMessage, len(payload))
	for key, value := range payload {
		cloned[key] = cloneResponseSessionValue(value)
	}
	return cloned
}

func cloneResponseSessionValue(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}

func cloneResponseSessionValues(values []json.RawMessage) []json.RawMessage {
	cloned := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneResponseSessionValue(value))
	}
	return cloned
}

func rawResponseSessionValuesSize(values []json.RawMessage) int {
	total := 0
	for _, value := range values {
		total += len(value)
	}
	return total
}

func responseSessionID(sessionID string) string {
	if sessionID != "" {
		return sessionID
	}
	return "responses-" + uuid.NewString()
}

func responseSessionString(value json.RawMessage) string {
	var result string
	_ = json.Unmarshal(value, &result)
	return result
}

func responseSessionInputItems(value json.RawMessage) []json.RawMessage {
	if len(value) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil
	}

	var items []json.RawMessage
	if json.Unmarshal(value, &items) == nil {
		return cloneResponseSessionValues(items)
	}

	var text string
	if json.Unmarshal(value, &text) != nil {
		return nil
	}
	item, err := json.Marshal(map[string]any{
		"type":    "message",
		"role":    "user",
		"content": text,
	})
	if err != nil {
		return nil
	}
	return []json.RawMessage{item}
}

func normalizeResponseSessionInput(value json.RawMessage) (json.RawMessage, bool) {
	var items []json.RawMessage
	if json.Unmarshal(value, &items) != nil {
		return value, false
	}

	normalized, changed := normalizeResponseSessionItems(items)
	if !changed {
		return value, false
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return value, false
	}
	return encoded, true
}

func normalizeResponseSessionItems(items []json.RawMessage) ([]json.RawMessage, bool) {
	normalized := make([]json.RawMessage, 0, len(items))
	changed := false
	for _, item := range items {
		var object map[string]json.RawMessage
		if json.Unmarshal(item, &object) != nil || object == nil {
			normalized = append(normalized, cloneResponseSessionValue(item))
			continue
		}
		if _, ok := object["status"]; !ok {
			normalized = append(normalized, cloneResponseSessionValue(item))
			continue
		}

		delete(object, "status")
		encoded, err := json.Marshal(object)
		if err != nil {
			normalized = append(normalized, cloneResponseSessionValue(item))
			continue
		}
		normalized = append(normalized, encoded)
		changed = true
	}
	return normalized, changed
}
