package contexts

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/ent"
)

func TestWithAPIKey(t *testing.T) {
	ctx := context.Background()
	apiKey := &ent.APIKey{
		ID:     1,
		UserID: 123,
		Key:    "sk-1234567890abcdef",
		Name:   "test-key",
	}

	// 测试存储 API key entity
	newCtx := WithAPIKey(ctx, apiKey)
	if newCtx == ctx {
		t.Error("WithAPIKey should return a new context")
	}

	// 测试获取 API key entity
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
	ctx := context.Background()

	// 测试从空 context 获取 API key
	apiKey, ok := GetAPIKey(ctx)
	if ok {
		t.Error("GetAPIKey should return false for empty context")
	}
	if apiKey != nil {
		t.Error("GetAPIKey should return nil for empty context")
	}

	// 测试从包含其他值的 context 获取 API key
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")
	apiKey, ok = GetAPIKey(ctxWithOtherValue)
	if ok {
		t.Error("GetAPIKey should return false for context without API key")
	}
	if apiKey != nil {
		t.Error("GetAPIKey should return nil for context without API key")
	}
}

func TestGetAPIKeyString(t *testing.T) {
	ctx := context.Background()
	apiKey := &ent.APIKey{
		ID:     1,
		UserID: 123,
		Key:    "sk-1234567890abcdef",
		Name:   "test-key",
	}

	// 测试从包含 API key 的 context 获取字符串
	ctxWithKey := WithAPIKey(ctx, apiKey)
	keyString, ok := GetAPIKeyString(ctxWithKey)
	if !ok {
		t.Error("GetAPIKeyString should return true for existing key")
	}
	if keyString != apiKey.Key {
		t.Errorf("expected key string %s, got %s", apiKey.Key, keyString)
	}

	// 测试从空 context 获取字符串
	keyString, ok = GetAPIKeyString(ctx)
	if ok {
		t.Error("GetAPIKeyString should return false for empty context")
	}
	if keyString != "" {
		t.Error("GetAPIKeyString should return empty string for empty context")
	}

	// 测试从包含 nil API key 的 context 获取字符串
	ctxWithNil := WithAPIKey(ctx, nil)
	keyString, ok = GetAPIKeyString(ctxWithNil)
	if ok {
		t.Error("GetAPIKeyString should return false for nil API key")
	}
	if keyString != "" {
		t.Error("GetAPIKeyString should return empty string for nil API key")
	}
}

func TestContextKey(t *testing.T) {
	// 测试 ContextKey 类型
	key1 := ContextKey("test1")
	key2 := ContextKey("test2")
	key3 := ContextKey("test1")

	if key1 == key2 {
		t.Error("Different ContextKey values should not be equal")
	}
	if key1 != key3 {
		t.Error("Same ContextKey values should be equal")
	}

	// 测试 APIKeyContextKey 常量
	if APIKeyContextKey == "" {
		t.Error("APIKeyContextKey should not be empty")
	}
}