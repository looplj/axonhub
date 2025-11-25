package contexts

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/user"
)

func TestWithAPIKey(t *testing.T) {
	ctx := t.Context()
	apiKey := &ent.APIKey{
		ID:     1,
		UserID: 123,
		Key:    "sk-1234567890abcdef",
		Name:   "test-key",
	}

	// Test storing API key entity
	newCtx := WithAPIKey(ctx, apiKey)
	if newCtx == ctx {
		t.Error("WithAPIKey should return a new context")
	}

	// Test retrieving API key entity
	retrievedKey, ok := GetAPIKey(newCtx)
	if !ok {
		t.Error("GetAPIKey should return true for existing key")
	}

	if retrievedKey == nil {
		t.Error("GetAPIKey should return non-nil API key")
	}

	if retrievedKey.ID != apiKey.ID {
		t.Errorf("expected ID %d, got %d", apiKey.ID, retrievedKey.ID)
	}

	if retrievedKey.UserID != apiKey.UserID {
		t.Errorf("expected UserID %d, got %d", apiKey.UserID, retrievedKey.UserID)
	}

	if retrievedKey.Key != apiKey.Key {
		t.Errorf("expected Key %s, got %s", apiKey.Key, retrievedKey.Key)
	}

	if retrievedKey.Name != apiKey.Name {
		t.Errorf("expected Name %s, got %s", apiKey.Name, retrievedKey.Name)
	}
}

func TestGetAPIKey(t *testing.T) {
	ctx := t.Context()

	// Test retrieving API key from empty context
	apiKey, ok := GetAPIKey(ctx)
	if ok {
		t.Error("GetAPIKey should return false for empty context")
	}

	if apiKey != nil {
		t.Error("GetAPIKey should return nil for empty context")
	}

	// Test retrieving API key from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	apiKey, ok = GetAPIKey(ctxWithOtherValue)
	if ok {
		t.Error("GetAPIKey should return false for context without API key")
	}

	if apiKey != nil {
		t.Error("GetAPIKey should return nil for context without API key")
	}
}

func TestWithUser(t *testing.T) {
	ctx := t.Context()
	user := &ent.User{
		ID:     123,
		Email:  "test@example.com",
		Status: user.Status("active"),
	}

	// Test storing user entity
	newCtx := WithUser(ctx, user)
	if newCtx == ctx {
		t.Error("WithUser should return a new context")
	}

	// Test retrieving user entity
	retrievedUser, ok := GetUser(newCtx)
	if !ok {
		t.Error("GetUser should return true for existing user")
	}

	if retrievedUser == nil {
		t.Error("GetUser should return non-nil user")
	}

	if retrievedUser.ID != user.ID {
		t.Errorf("expected ID %d, got %d", user.ID, retrievedUser.ID)
	}

	if retrievedUser.Email != user.Email {
		t.Errorf("expected Email %s, got %s", user.Email, retrievedUser.Email)
	}

	if retrievedUser.Status != user.Status {
		t.Errorf("expected Status %s, got %s", user.Status, retrievedUser.Status)
	}
}

func TestGetUser(t *testing.T) {
	ctx := t.Context()

	// Test retrieving user from empty context
	user, ok := GetUser(ctx)
	if ok {
		t.Error("GetUser should return false for empty context")
	}

	if user != nil {
		t.Error("GetUser should return nil for empty context")
	}

	// Test retrieving user from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	user, ok = GetUser(ctxWithOtherValue)
	if ok {
		t.Error("GetUser should return false for context without user")
	}

	if user != nil {
		t.Error("GetUser should return nil for context without user")
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := t.Context()
	traceID := "trace-12345-abcdef"

	// Test storing trace ID
	newCtx := WithTraceID(ctx, traceID)
	if newCtx == ctx {
		t.Error("WithTraceID should return a new context")
	}

	// Test retrieving trace ID
	retrievedTraceID, ok := GetTraceID(newCtx)
	if !ok {
		t.Error("GetTraceID should return true for existing trace ID")
	}

	if retrievedTraceID != traceID {
		t.Errorf("expected trace ID %s, got %s", traceID, retrievedTraceID)
	}
}

func TestGetTraceID(t *testing.T) {
	ctx := t.Context()

	// Test retrieving trace ID from empty context
	traceID, ok := GetTraceID(ctx)
	if ok {
		t.Error("GetTraceID should return false for empty context")
	}

	if traceID != "" {
		t.Error("GetTraceID should return empty string for empty context")
	}

	// Test retrieving trace ID from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	traceID, ok = GetTraceID(ctxWithOtherValue)
	if ok {
		t.Error("GetTraceID should return false for context without trace ID")
	}

	if traceID != "" {
		t.Error("GetTraceID should return empty string for context without trace ID")
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := t.Context()
	requestID := "req-12345-abcdef"

	// Test storing request ID
	newCtx := WithRequestID(ctx, requestID)
	if newCtx == ctx {
		t.Error("WithRequestID should return a new context")
	}

	// Test retrieving request ID
	retrievedRequestID, ok := GetRequestID(newCtx)
	if !ok {
		t.Error("GetRequestID should return true for existing request ID")
	}

	if retrievedRequestID != requestID {
		t.Errorf("expected request ID %s, got %s", requestID, retrievedRequestID)
	}
}

func TestGetRequestID(t *testing.T) {
	ctx := t.Context()

	// Test retrieving request ID from empty context
	requestID, ok := GetRequestID(ctx)
	if ok {
		t.Error("GetRequestID should return false for empty context")
	}

	if requestID != "" {
		t.Error("GetRequestID should return empty string for empty context")
	}

	// Test retrieving request ID from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	requestID, ok = GetRequestID(ctxWithOtherValue)
	if ok {
		t.Error("GetRequestID should return false for context without request ID")
	}

	if requestID != "" {
		t.Error("GetRequestID should return empty string for context without request ID")
	}
}

func TestWithOperationName(t *testing.T) {
	ctx := t.Context()
	operationName := "user.create"

	// Test storing operation name
	newCtx := WithOperationName(ctx, operationName)
	if newCtx == ctx {
		t.Error("WithOperationName should return a new context")
	}

	// Test retrieving operation name
	retrievedOperationName, ok := GetOperationName(newCtx)
	if !ok {
		t.Error("GetOperationName should return true for existing operation name")
	}

	if retrievedOperationName != operationName {
		t.Errorf("expected operation name %s, got %s", operationName, retrievedOperationName)
	}
}

func TestGetOperationName(t *testing.T) {
	ctx := t.Context()

	// Test retrieving operation name from empty context
	operationName, ok := GetOperationName(ctx)
	if ok {
		t.Error("GetOperationName should return false for empty context")
	}

	if operationName != "" {
		t.Error("GetOperationName should return empty string for empty context")
	}

	// Test retrieving operation name from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	operationName, ok = GetOperationName(ctxWithOtherValue)
	if ok {
		t.Error("GetOperationName should return false for context without operation name")
	}

	if operationName != "" {
		t.Error("GetOperationName should return empty string for context without operation name")
	}
}

func TestWithProjectID(t *testing.T) {
	ctx := t.Context()
	projectID := 123

	// Test storing project ID
	newCtx := WithProjectID(ctx, projectID)
	if newCtx == ctx {
		t.Error("WithProjectID should return a new context")
	}

	// Test retrieving project ID
	retrievedProjectID, ok := GetProjectID(newCtx)
	if !ok {
		t.Error("GetProjectID should return true for existing project ID")
	}

	if retrievedProjectID != projectID {
		t.Errorf("expected project ID %d, got %d", projectID, retrievedProjectID)
	}
}

func TestGetProjectID(t *testing.T) {
	ctx := t.Context()

	// Test retrieving project ID from empty context
	projectID, ok := GetProjectID(ctx)
	if ok {
		t.Error("GetProjectID should return false for empty context")
	}

	if projectID != 0 {
		t.Error("GetProjectID should return 0 for empty context")
	}

	// Test retrieving project ID from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	projectID, ok = GetProjectID(ctxWithOtherValue)
	if ok {
		t.Error("GetProjectID should return false for context without project ID")
	}

	if projectID != 0 {
		t.Error("GetProjectID should return 0 for context without project ID")
	}
}

func TestWithSource(t *testing.T) {
	ctx := t.Context()
	source := request.SourcePlayground

	// Test storing request source
	newCtx := WithSource(ctx, source)
	if newCtx == ctx {
		t.Error("WithSource should return a new context")
	}

	// Test retrieving request source
	retrievedSource, ok := GetSource(newCtx)
	if !ok {
		t.Error("GetSource should return true for existing source")
	}

	if retrievedSource != source {
		t.Errorf("expected source %s, got %s", source, retrievedSource)
	}
}

func TestGetSource(t *testing.T) {
	ctx := t.Context()

	// Test retrieving request source from empty context
	source, ok := GetSource(ctx)
	if ok {
		t.Error("GetSource should return false for empty context")
	}

	if source != request.SourceAPI {
		t.Error("GetSource should return default source for empty context")
	}

	// Test retrieving request source from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	source, ok = GetSource(ctxWithOtherValue)
	if ok {
		t.Error("GetSource should return false for context without source")
	}

	if source != request.SourceAPI {
		t.Error("GetSource should return default source for context without source")
	}
}

func TestGetSourceOrDefault(t *testing.T) {
	ctx := t.Context()

	// Test retrieving request source from empty context (using default value)
	source := GetSourceOrDefault(ctx, request.SourceTest)
	if source != request.SourceTest {
		t.Errorf("expected default source %s, got %s", request.SourceTest, source)
	}

	// Test retrieving request source from context containing source
	ctxWithSource := WithSource(ctx, request.SourcePlayground)

	source = GetSourceOrDefault(ctxWithSource, request.SourceTest)
	if source != request.SourcePlayground {
		t.Errorf("expected source %s, got %s", request.SourcePlayground, source)
	}

	// Test using different default values
	source = GetSourceOrDefault(ctx, request.SourceAPI)
	if source != request.SourceAPI {
		t.Errorf("expected default source %s, got %s", request.SourceAPI, source)
	}
}

func TestContextContainerMultipleValues(t *testing.T) {
	ctx := t.Context()

	// Test storing multiple different values
	ctx = WithAPIKey(ctx, &ent.APIKey{ID: 1, Key: "test-key"})
	ctx = WithUser(ctx, &ent.User{ID: 123, Email: "test@example.com"})
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithRequestID(ctx, "req-456")
	ctx = WithOperationName(ctx, "test.operation")
	ctx = WithProjectID(ctx, 456)
	ctx = WithSource(ctx, request.SourcePlayground)

	// Test retrieving all values
	apiKey, ok := GetAPIKey(ctx)
	if !ok || apiKey.ID != 1 {
		t.Error("API key should be stored and retrievable")
	}

	user, ok := GetUser(ctx)
	if !ok || user.ID != 123 {
		t.Error("User should be stored and retrievable")
	}

	traceID, ok := GetTraceID(ctx)
	if !ok || traceID != "trace-123" {
		t.Error("Trace ID should be stored and retrievable")
	}

	requestID, ok := GetRequestID(ctx)
	if !ok || requestID != "req-456" {
		t.Error("Request ID should be stored and retrievable")
	}

	operationName, ok := GetOperationName(ctx)
	if !ok || operationName != "test.operation" {
		t.Error("Operation name should be stored and retrievable")
	}

	projectID, ok := GetProjectID(ctx)
	if !ok || projectID != 456 {
		t.Error("Project ID should be stored and retrievable")
	}

	source, ok := GetSource(ctx)
	if !ok || source != request.SourcePlayground {
		t.Error("Source should be stored and retrievable")
	}
}

func TestContextContainerOverwrite(t *testing.T) {
	ctx := t.Context()

	// Test overwriting existing values
	ctx = WithAPIKey(ctx, &ent.APIKey{ID: 1, Key: "key-1"})
	ctx = WithAPIKey(ctx, &ent.APIKey{ID: 2, Key: "key-2"})

	apiKey, ok := GetAPIKey(ctx)
	if !ok {
		t.Error("API key should exist")
	}

	if apiKey.ID != 2 || apiKey.Key != "key-2" {
		t.Error("API key should be the overwritten value")
	}

	// Test overwriting trace ID
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithTraceID(ctx, "trace-2")

	traceID, ok := GetTraceID(ctx)
	if !ok || traceID != "trace-2" {
		t.Error("Trace ID should be the overwritten value")
	}
}

func TestContextContainerIsolation(t *testing.T) {
	ctx := t.Context()

	// Create a context with values
	ctx1 := WithAPIKey(ctx, &ent.APIKey{ID: 1, Key: "key-1"})
	ctx1 = WithTraceID(ctx1, "trace-1")

	// Create another context with different values
	ctx2 := WithAPIKey(ctx, &ent.APIKey{ID: 2, Key: "key-2"})
	ctx2 = WithTraceID(ctx2, "trace-2")

	// Test that the two contexts are isolated from each other
	apiKey1, ok1 := GetAPIKey(ctx1)
	apiKey2, ok2 := GetAPIKey(ctx2)

	if !ok1 || !ok2 {
		t.Error("Both contexts should have API keys")
	}

	if apiKey1.ID == apiKey2.ID {
		t.Error("API keys should be different")
	}

	traceID1, ok1 := GetTraceID(ctx1)
	traceID2, ok2 := GetTraceID(ctx2)

	if !ok1 || !ok2 {
		t.Error("Both contexts should have trace IDs")
	}

	if traceID1 == traceID2 {
		t.Error("Trace IDs should be different")
	}
}

func TestContextContainerWithOtherValues(t *testing.T) {
	ctx := t.Context()

	// Create a context containing other values
	ctxWithOther := context.WithValue(ctx, "other_key", "other_value")
	ctxWithOther = context.WithValue(ctxWithOther, "another_key", 123)

	// Store our values in this context
	ctxWithOurs := WithAPIKey(ctxWithOther, &ent.APIKey{ID: 1, Key: "test-key"})

	// Test that other values are still present
	if ctxWithOurs.Value("other_key") != "other_value" {
		t.Error("Other context values should be preserved")
	}

	if ctxWithOurs.Value("another_key") != 123 {
		t.Error("Other context values should be preserved")
	}

	// Test that our values are also accessible
	apiKey, ok := GetAPIKey(ctxWithOurs)
	if !ok || apiKey.ID != 1 {
		t.Error("Our context values should also be accessible")
	}
}
