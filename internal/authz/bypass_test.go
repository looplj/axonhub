package authz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/samber/lo"
)

func TestWithBypassPrivacy(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	// 测试 WithBypassPrivacy
	bypassCtx, err := WithBypassPrivacy(ctx, "test-reason")
	if err != nil {
		t.Fatalf("WithBypassPrivacy failed: %v", err)
	}

	// 验证 bypass 信息是否正确设置
	info, ok := GetBypassInfo(bypassCtx)
	if !ok {
		t.Fatal("GetBypassInfo should return true after WithBypassPrivacy")
	}

	if info.Reason != "test-reason" {
		t.Errorf("Reason = %v, want %v", info.Reason, "test-reason")
	}

	if !info.Principal.IsSystem() {
		t.Error("Principal should be system type")
	}

	if info.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestRunWithBypass(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	// 测试 RunWithBypass
	executed := false

	result, err := RunWithBypass(ctx, "test-closure", func(bypassCtx context.Context) (string, error) {
		executed = true

		// 验证 bypass 在闭包内生效
		if !IsBypassActive(bypassCtx) {
			t.Error("Bypass should be active inside closure")
		}

		return "success", nil
	})
	if err != nil {
		t.Fatalf("RunWithBypass failed: %v", err)
	}

	if !executed {
		t.Error("Closure should be executed")
	}

	if result != "success" {
		t.Errorf("Result = %v, want %v", result, "success")
	}

	// 验证 bypass 在闭包外不生效
	if IsBypassActive(ctx) {
		t.Error("Bypass should not be active outside closure")
	}
}

func TestRunWithBypass_ErrorPropagation(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	// 测试错误传播
	expectedErr := context.Canceled
	_, err := RunWithBypass(ctx, "test-error", func(bypassCtx context.Context) (string, error) {
		return "", expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("Error should be propagated: got %v, want %v", err, expectedErr)
	}
}

func TestIsBypassActive(t *testing.T) {
	ctx := context.Background()

	// 未设置 bypass 时应该返回 false
	if IsBypassActive(ctx) {
		t.Error("IsBypassActive should return false when not set")
	}

	// 设置 bypass 后应该返回 true
	bypassCtx, err := WithBypassPrivacy(NewSystemContext(ctx), "test")
	if err != nil {
		t.Fatalf("WithBypassPrivacy failed: %v", err)
	}

	if !IsBypassActive(bypassCtx) {
		t.Error("IsBypassActive should return true after WithBypassPrivacy")
	}
}

func TestGetBypassInfo_NotSet(t *testing.T) {
	ctx := context.Background()

	_, ok := GetBypassInfo(ctx)
	if ok {
		t.Error("GetBypassInfo should return false when not set")
	}
}

func TestRequireSystemPrincipal(t *testing.T) {
	// System 主体应该通过
	systemCtx := NewSystemContext(context.Background())
	if err := RequireSystemPrincipal(systemCtx); err != nil {
		t.Errorf("RequireSystemPrincipal should pass for system principal: %v", err)
	}

	// User 主体应该失败
	userCtx := NewUserContext(context.Background(), 123)
	if err := RequireSystemPrincipal(userCtx); err == nil {
		t.Error("RequireSystemPrincipal should fail for user principal")
	}

	// 无主体应该失败
	emptyCtx := context.Background()
	if err := RequireSystemPrincipal(emptyCtx); err == nil {
		t.Error("RequireSystemPrincipal should fail when no principal")
	}
}

func TestRequirePrincipal(t *testing.T) {
	// 有主体应该通过
	systemCtx := NewSystemContext(context.Background())
	if err := RequirePrincipal(systemCtx); err != nil {
		t.Errorf("RequirePrincipal should pass when principal exists: %v", err)
	}

	// 无主体应该失败
	emptyCtx := context.Background()
	if err := RequirePrincipal(emptyCtx); err == nil {
		t.Error("RequirePrincipal should fail when no principal")
	}
}

func TestSetAuditLogger(t *testing.T) {
	// 保存原始 logger
	originalLogger := auditLogger

	defer func() {
		auditLogger = originalLogger
	}()

	// 测试自定义 logger
	var capturedRecord bypassAuditRecord

	customLogger := func(ctx context.Context, record bypassAuditRecord) {
		capturedRecord = record
	}

	SetAuditLogger(customLogger)

	ctx := NewSystemContext(context.Background())

	_, err := WithBypassPrivacy(ctx, "custom-audit-test")
	if err != nil {
		t.Fatalf("WithBypassPrivacy failed: %v", err)
	}

	// 验证自定义 logger 被调用
	if capturedRecord.Reason != "custom-audit-test" {
		t.Errorf("Custom logger should be called with reason: got %v", capturedRecord.Reason)
	}

	if capturedRecord.Principal != "system" {
		t.Errorf("Custom logger should capture principal: got %v", capturedRecord.Principal)
	}

	if capturedRecord.Operation != "bypass" {
		t.Errorf("Operation should be 'bypass': got %v", capturedRecord.Operation)
	}

	if capturedRecord.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestBypassScopeIsolation(t *testing.T) {
	// 测试 bypass 作用域隔离
	ctx := NewSystemContext(context.Background())

	// 在外部设置一个值
	type testKey struct{}

	ctx = context.WithValue(ctx, testKey{}, "outer")

	// 在 RunWithBypass 内部修改 context
	var innerValue string

	_, _ = RunWithBypass(ctx, "scope-test", func(bypassCtx context.Context) (string, error) {
		// bypassCtx 应该继承父 context 的值
		if v, ok := bypassCtx.Value(testKey{}).(string); ok {
			innerValue = v
		}

		return "done", nil
	})

	if innerValue != "outer" {
		t.Errorf("Inner context should inherit outer values: got %v", innerValue)
	}
}

func TestBypassWithSystemPrincipal(t *testing.T) {
	// System principal should succeed
	ctx := context.WithValue(context.Background(), principalKey{}, Principal{Type: PrincipalTypeSystem})

	bypassCtx, err := WithBypassPrivacy(ctx, "system-operation")
	if err != nil {
		t.Fatalf("WithBypassPrivacy failed: %v", err)
	}

	info, ok := GetBypassInfo(bypassCtx)
	if !ok {
		t.Fatal("GetBypassInfo failed")
	}

	if info.Reason != "system-operation" {
		t.Errorf("Reason = %v, want %v", info.Reason, "system-operation")
	}

	if !info.Principal.IsSystem() {
		t.Errorf("Principal should be system type")
	}

	if !info.Timestamp.Before(time.Now().Add(time.Second)) {
		t.Error("Timestamp should be set to a reasonable time")
	}
}

func TestBypassWithNonSystemPrincipal(t *testing.T) {
	// User principal should fail
	t.Run("user principal", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), principalKey{}, Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(123)})

		_, err := WithBypassPrivacy(ctx, "user-operation")
		if err == nil {
			t.Error("WithBypassPrivacy should fail for user principal")
		}
	})

	// APIKey principal should fail
	t.Run("apikey principal", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), principalKey{}, Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(456), ProjectID: lo.ToPtr(789)})

		_, err := WithBypassPrivacy(ctx, "apikey-operation")
		if err == nil {
			t.Error("WithBypassPrivacy should fail for apikey principal")
		}
	})
}

func TestBypassAuditRecordStructure(t *testing.T) {
	now := time.Now()
	record := bypassAuditRecord{
		Timestamp:   now,
		Principal:   "user:123",
		Reason:      "test-reason",
		Operation:   "bypass",
		Entity:      "privacy",
		Description: "test description",
	}

	if record.Timestamp != now {
		t.Error("Timestamp mismatch")
	}

	if record.Principal != "user:123" {
		t.Error("Principal mismatch")
	}

	if record.Reason != "test-reason" {
		t.Error("Reason mismatch")
	}

	if record.Operation != "bypass" {
		t.Error("Operation mismatch")
	}

	if record.Entity != "privacy" {
		t.Error("Entity mismatch")
	}

	if record.Description != "test description" {
		t.Error("Description mismatch")
	}
}
