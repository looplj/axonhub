package authz

import (
	"context"
	"testing"

	"github.com/samber/lo"
)

func TestPrincipalType_String(t *testing.T) {
	tests := []struct {
		name string
		p    PrincipalType
		want string
	}{
		{"system", PrincipalTypeSystem, "system"},
		{"user", PrincipalTypeUser, "user"},
		{"apikey", PrincipalTypeAPIKey, "apikey"},
		{"unknown", PrincipalType(999), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("PrincipalType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrincipal_IsSystem(t *testing.T) {
	tests := []struct {
		name string
		p    Principal
		want bool
	}{
		{"system", Principal{Type: PrincipalTypeSystem}, true},
		{"user", Principal{Type: PrincipalTypeUser}, false},
		{"apikey", Principal{Type: PrincipalTypeAPIKey}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.IsSystem(); got != tt.want {
				t.Errorf("Principal.IsSystem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrincipal_IsUser(t *testing.T) {
	tests := []struct {
		name string
		p    Principal
		want bool
	}{
		{"system", Principal{Type: PrincipalTypeSystem}, false},
		{"user", Principal{Type: PrincipalTypeUser}, true},
		{"apikey", Principal{Type: PrincipalTypeAPIKey}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.IsUser(); got != tt.want {
				t.Errorf("Principal.IsUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrincipal_IsAPIKey(t *testing.T) {
	tests := []struct {
		name string
		p    Principal
		want bool
	}{
		{"system", Principal{Type: PrincipalTypeSystem}, false},
		{"user", Principal{Type: PrincipalTypeUser}, false},
		{"apikey", Principal{Type: PrincipalTypeAPIKey}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.IsAPIKey(); got != tt.want {
				t.Errorf("Principal.IsAPIKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrincipal_String(t *testing.T) {
	userID := 123
	apiKeyID := 456

	tests := []struct {
		name string
		p    Principal
		want string
	}{
		{"system", Principal{Type: PrincipalTypeSystem}, "system"},
		{"user with id", Principal{Type: PrincipalTypeUser, UserID: &userID}, "user:123"},
		{"user without id", Principal{Type: PrincipalTypeUser}, "user:unknown"},
		{"apikey with id", Principal{Type: PrincipalTypeAPIKey, APIKeyID: &apiKeyID}, "apikey:456"},
		{"apikey without id", Principal{Type: PrincipalTypeAPIKey}, "apikey:unknown"},
		{"unknown", Principal{Type: PrincipalType(999)}, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("Principal.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithPrincipal_SetOnce(t *testing.T) {
	ctx := context.Background()

	// 第一次设置应该成功
	p1 := Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(123)}

	ctx, err := WithPrincipal(ctx, p1)
	if err != nil {
		t.Fatalf("first WithPrincipal failed: %v", err)
	}

	// 验证设置成功
	got, ok := GetPrincipal(ctx)
	if !ok {
		t.Fatal("GetPrincipal failed after first set")
	}

	if got.Type != p1.Type || *got.UserID != *p1.UserID {
		t.Errorf("GetPrincipal = %v, want %v", got, p1)
	}

	// 设置相同的主体应该幂等成功
	ctx2, err := WithPrincipal(ctx, p1)
	if err != nil {
		t.Fatalf("setting same principal should be idempotent: %v", err)
	}

	if ctx2 != ctx {
		t.Error("setting same principal should return same context")
	}

	// 设置不同的主体应该失败
	p2 := Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(456)}

	_, err = WithPrincipal(ctx, p2)
	if err == nil {
		t.Error("setting different principal should fail")
	}
}

func TestWithPrincipal_ConflictDetection(t *testing.T) {
	ctx := context.Background()

	// 设置 User 主体
	userPrincipal := Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(123)}

	ctx, err := WithPrincipal(ctx, userPrincipal)
	if err != nil {
		t.Fatalf("setting user principal failed: %v", err)
	}

	// 尝试设置 APIKey 主体应该失败
	apiKeyPrincipal := Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(456)}

	_, err = WithPrincipal(ctx, apiKeyPrincipal)
	if err == nil {
		t.Error("mixed principal types should be rejected")
	}

	// 尝试设置不同 UserID 的 User 主体也应该失败
	otherUserPrincipal := Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(789)}

	_, err = WithPrincipal(ctx, otherUserPrincipal)
	if err == nil {
		t.Error("different user id should be rejected")
	}
}

func TestGetPrincipal_NotSet(t *testing.T) {
	ctx := context.Background()

	_, ok := GetPrincipal(ctx)
	if ok {
		t.Error("GetPrincipal should return false when principal not set")
	}
}

func TestMustGetPrincipal(t *testing.T) {
	// 存在主体时不应 panic
	ctx := NewSystemContext(context.Background())

	p := MustGetPrincipal(ctx)
	if !p.IsSystem() {
		t.Error("MustGetPrincipal should return system principal")
	}

	// 不存在主体时应该 panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGetPrincipal should panic when principal not set")
		}
	}()

	MustGetPrincipal(context.Background())
}

func TestNewSystemContext(t *testing.T) {
	ctx := NewSystemContext(context.Background())

	p, ok := GetPrincipal(ctx)
	if !ok {
		t.Fatal("GetPrincipal failed")
	}

	if !p.IsSystem() {
		t.Error("principal should be system type")
	}
}

func TestNewUserContext(t *testing.T) {
	ctx := NewUserContext(context.Background(), 123)

	p, ok := GetPrincipal(ctx)
	if !ok {
		t.Fatal("GetPrincipal failed")
	}

	if !p.IsUser() {
		t.Error("principal should be user type")
	}

	if p.UserID == nil || *p.UserID != 123 {
		t.Error("user id should be 123")
	}
}

func TestNewAPIKeyContext(t *testing.T) {
	ctx := NewAPIKeyContext(context.Background(), 456, 789)

	p, ok := GetPrincipal(ctx)
	if !ok {
		t.Fatal("GetPrincipal failed")
	}

	if !p.IsAPIKey() {
		t.Error("principal should be apikey type")
	}

	if p.APIKeyID == nil || *p.APIKeyID != 456 {
		t.Error("api key id should be 456")
	}

	if p.ProjectID == nil || *p.ProjectID != 789 {
		t.Error("project id should be 789")
	}
}

func TestPrincipalEqual(t *testing.T) {
	tests := []struct {
		name string
		a    Principal
		b    Principal
		want bool
	}{
		{
			name: "same system",
			a:    Principal{Type: PrincipalTypeSystem},
			b:    Principal{Type: PrincipalTypeSystem},
			want: true,
		},
		{
			name: "same user",
			a:    Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(123)},
			b:    Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(123)},
			want: true,
		},
		{
			name: "different user id",
			a:    Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(123)},
			b:    Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(456)},
			want: false,
		},
		{
			name: "different types",
			a:    Principal{Type: PrincipalTypeUser, UserID: lo.ToPtr(123)},
			b:    Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(123)},
			want: false,
		},
		{
			name: "same apikey",
			a:    Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(123), ProjectID: lo.ToPtr(456)},
			b:    Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(123), ProjectID: lo.ToPtr(456)},
			want: true,
		},
		{
			name: "different project id",
			a:    Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(123), ProjectID: lo.ToPtr(456)},
			b:    Principal{Type: PrincipalTypeAPIKey, APIKeyID: lo.ToPtr(123), ProjectID: lo.ToPtr(789)},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := principalEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("principalEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
