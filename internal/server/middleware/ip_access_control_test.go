package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMatchIP(t *testing.T) {
	tests := []struct {
		name      string
		clientIP  string
		ips       []string
		wantMatch bool
	}{
		{
			name:      "exact match",
			clientIP:  "203.0.113.10",
			ips:       []string{"203.0.113.10"},
			wantMatch: true,
		},
		{
			name:      "cidr match",
			clientIP:  "203.0.113.10",
			ips:       []string{"203.0.113.0/24"},
			wantMatch: true,
		},
		{
			name:      "trimmed entry match",
			clientIP:  "203.0.113.10",
			ips:       []string{" 203.0.113.10 "},
			wantMatch: true,
		},
		{
			name:      "ipv6 exact match",
			clientIP:  "2001:db8::10",
			ips:       []string{"2001:db8::10"},
			wantMatch: true,
		},
		{
			name:      "ipv6 cidr match",
			clientIP:  "2001:db8::10",
			ips:       []string{"2001:db8::/64"},
			wantMatch: true,
		},
		{
			name:      "invalid entries skipped",
			clientIP:  "203.0.113.10",
			ips:       []string{"bad-ip", "bad-prefix/33"},
			wantMatch: false,
		},
		{
			name:      "no match",
			clientIP:  "203.0.113.10",
			ips:       []string{"198.51.100.0/24", "192.0.2.1"},
			wantMatch: false,
		},
		{
			name:      "empty string in list skipped",
			clientIP:  "203.0.113.10",
			ips:       []string{"", "203.0.113.10"},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientAddr, err := netip.ParseAddr(tt.clientIP)
			if err != nil {
				t.Fatalf("failed to parse client IP %q: %v", tt.clientIP, err)
			}
			if got := matchIP(clientAddr, tt.ips); got != tt.wantMatch {
				t.Fatalf("matchIP() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestDenyRequestWithRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	denyRequest(ctx, "https://example.com/blocked")

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusFound)
	}
	if loc := recorder.Header().Get("Location"); loc != "https://example.com/blocked" {
		t.Fatalf("Location = %q, want %q", loc, "https://example.com/blocked")
	}
}

func TestDenyRequestWithoutRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	denyRequest(ctx, "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestWithIPAccessControlDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler := WithIPAccessControl(false, nil, "")
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (pass through)", recorder.Code, http.StatusOK)
	}
}

func TestWithIPAccessControlAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	ctx.Request = req

	handler := WithIPAccessControl(true, []string{"10.0.0.0/8"}, "")
	handler(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (pass through)", recorder.Code, http.StatusOK)
	}
}

func TestWithIPAccessControlDeniedRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	ctx.Request = req

	handler := WithIPAccessControl(true, []string{"192.168.0.0/16"}, "https://example.com/blocked")
	handler(ctx)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusFound)
	}
	if loc := recorder.Header().Get("Location"); loc != "https://example.com/blocked" {
		t.Fatalf("Location = %q, want %q", loc, "https://example.com/blocked")
	}
}

func TestWithIPAccessControlDeniedNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	ctx.Request = req

	handler := WithIPAccessControl(true, []string{"192.168.0.0/16"}, "")
	handler(ctx)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", recorder.Body.String())
	}
}

func TestWithIPAccessControlEmptyClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ""
	ctx.Request = req

	handler := WithIPAccessControl(true, []string{"10.0.0.0/8"}, "")
	handler(ctx)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (fail closed)", recorder.Code, http.StatusNotFound)
	}
}

func TestWithIPAccessControlSpoofedHeaderNotTrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	ctx.Request = req

	handler := WithIPAccessControl(true, []string{"192.168.0.0/16"}, "https://example.com/blocked")
	handler(ctx)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d (spoofed header should not bypass)", recorder.Code, http.StatusFound)
	}
}
