package middleware

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name       string
		clientIPs  []string
		blockedIPs []string
		want       bool
	}{
		{
			name:       "exact match",
			clientIPs:  []string{"203.0.113.10"},
			blockedIPs: []string{"203.0.113.10"},
			want:       true,
		},
		{
			name:       "cidr match",
			clientIPs:  []string{"203.0.113.10"},
			blockedIPs: []string{"203.0.113.0/24"},
			want:       true,
		},
		{
			name:       "trimmed blocked entry match",
			clientIPs:  []string{"203.0.113.10"},
			blockedIPs: []string{" 203.0.113.10 "},
			want:       true,
		},
		{
			name:       "ipv6 exact match",
			clientIPs:  []string{"2001:db8::10"},
			blockedIPs: []string{"2001:db8::10"},
			want:       true,
		},
		{
			name:       "ipv6 cidr match",
			clientIPs:  []string{"2001:db8::10"},
			blockedIPs: []string{"2001:db8::/64"},
			want:       true,
		},
		{
			name:       "later candidate match",
			clientIPs:  []string{"10.0.0.1", "203.0.113.10"},
			blockedIPs: []string{"203.0.113.10"},
			want:       true,
		},
		{
			name:       "invalid client candidate skipped",
			clientIPs:  []string{"bad-ip", "203.0.113.10"},
			blockedIPs: []string{"203.0.113.10"},
			want:       true,
		},
		{
			name:       "invalid blocked entries skipped",
			clientIPs:  []string{"203.0.113.10"},
			blockedIPs: []string{"bad-ip", "bad-prefix/33"},
			want:       false,
		},
		{
			name:       "no match",
			clientIPs:  []string{"203.0.113.10"},
			blockedIPs: []string{"198.51.100.0/24", "192.0.2.1"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBlockedIP(tt.clientIPs, tt.blockedIPs); got != tt.want {
				t.Fatalf("isBlockedIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientIPCandidates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.20")
	req.Header.Set("X-Real-IP", "192.0.2.30")
	ctx.Request = req

	got := clientIPCandidates(ctx)
	want := []string{"10.0.0.1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clientIPCandidates() = %#v, want %#v", got, want)
	}
}

func TestClientIPCandidatesDeduplicates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.20")
	req.Header.Set("X-Real-IP", "203.0.113.10")
	ctx.Request = req

	got := clientIPCandidates(ctx)
	want := []string{"203.0.113.10"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clientIPCandidates() = %#v, want %#v", got, want)
	}
}

func TestClientIPCandidatesUsesForwardedIPForTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(recorder)
	if err := engine.SetTrustedProxies([]string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("failed to set trusted proxies: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.20")
	ctx.Request = req

	got := clientIPCandidates(ctx)
	want := []string{"198.51.100.20"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clientIPCandidates() = %#v, want %#v", got, want)
	}
}

func TestIsAnyAllowedIP(t *testing.T) {
	tests := []struct {
		name       string
		clientIPs  []string
		allowedIPs []string
		want       bool
	}{
		{
			name:       "exact match allowed",
			clientIPs:  []string{"203.0.113.10"},
			allowedIPs: []string{"203.0.113.10"},
			want:       true,
		},
		{
			name:       "cidr match allowed",
			clientIPs:  []string{"203.0.113.10"},
			allowedIPs: []string{"203.0.113.0/24"},
			want:       true,
		},
		{
			name:       "ipv6 exact match allowed",
			clientIPs:  []string{"2001:db8::10"},
			allowedIPs: []string{"2001:db8::10"},
			want:       true,
		},
		{
			name:       "ipv6 cidr match allowed",
			clientIPs:  []string{"2001:db8::10"},
			allowedIPs: []string{"2001:db8::/64"},
			want:       true,
		},
		{
			name:       "multiple allowed entries one matches",
			clientIPs:  []string{"203.0.113.10"},
			allowedIPs: []string{"198.51.100.0/24", "203.0.113.0/24"},
			want:       true,
		},
		{
			name:       "multiple client candidates one matches",
			clientIPs:  []string{"10.0.0.1", "203.0.113.10"},
			allowedIPs: []string{"203.0.113.0/24"},
			want:       true,
		},
		{
			name:       "no match denied",
			clientIPs:  []string{"203.0.113.10"},
			allowedIPs: []string{"198.51.100.0/24", "192.0.2.1"},
			want:       false,
		},
		{
			name:       "invalid client candidates all skipped denied",
			clientIPs:  []string{"bad-ip", "also-bad"},
			allowedIPs: []string{"203.0.113.0/24"},
			want:       false,
		},
		{
			name:       "empty allowed list denied",
			clientIPs:  []string{"203.0.113.10"},
			allowedIPs: []string{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAnyAllowedIP(tt.clientIPs, tt.allowedIPs); got != tt.want {
				t.Fatalf("isAnyAllowedIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIPMatchersMappedIPv4(t *testing.T) {
	tests := []struct {
		name   string
		client string
		entry  string
		want   bool
	}{
		{name: "mapped client matches IPv4", client: "::ffff:203.0.113.10", entry: "203.0.113.10", want: true},
		{name: "mapped client matches IPv4 CIDR", client: "::ffff:203.0.113.10", entry: "203.0.113.0/24", want: true},
		{name: "IPv4 client matches mapped entry", client: "203.0.113.10", entry: "::ffff:203.0.113.10", want: true},
		{name: "IPv4 client matches mapped CIDR", client: "203.0.113.10", entry: "::ffff:203.0.113.0/120", want: true},
		{name: "mapped client matches mapped CIDR", client: "::ffff:203.0.113.10", entry: "::ffff:203.0.113.0/120", want: true},
		{name: "mapped CIDR with host bits", client: "203.0.113.10", entry: "::ffff:203.0.113.99/120", want: true},
		{name: "mapped zero-length IPv4 prefix", client: "198.51.100.10", entry: "::ffff:0.0.0.0/96", want: true},
		{name: "mapped single-address prefix", client: "203.0.113.10", entry: "::ffff:203.0.113.10/128", want: true},
		{name: "mapped single-address prefix rejects other address", client: "203.0.113.11", entry: "::ffff:203.0.113.10/128"},
		{name: "mapped IPv4 outside IPv4 CIDR", client: "::ffff:198.51.100.10", entry: "203.0.113.0/24"},
		{name: "IPv4 outside mapped CIDR", client: "198.51.100.10", entry: "::ffff:203.0.113.0/120"},
		{name: "IPv4 does not match native IPv6 prefix", client: "203.0.113.10", entry: "2001:db8::/64"},
		{name: "native IPv6 does not match mapped prefix", client: "2001:db8::10", entry: "::ffff:0.0.0.0/96"},
		{name: "IPv4 does not match wider IPv6 prefix", client: "203.0.113.10", entry: "::ffff:203.0.113.0/95"},
		{name: "mapped client retains wider IPv6 prefix match", client: "::ffff:203.0.113.10", entry: "::ffff:203.0.113.0/95", want: true},
		{name: "mapped client retains IPv6 all-addresses match", client: "::ffff:203.0.113.10", entry: "::/0", want: true},
		{name: "IPv4 does not match IPv6 all-addresses prefix", client: "203.0.113.10", entry: "::/0"},
	}
	matchers := map[string]func([]string, []string) bool{
		"allowlist": isAnyAllowedIP,
		"blocklist": isBlockedIP,
	}
	for name, match := range matchers {
		t.Run(name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					if got := match([]string{tt.client}, []string{tt.entry}); got != tt.want {
						t.Fatalf("matching %q against %q = %v, want %v", tt.client, tt.entry, got, tt.want)
					}
				})
			}
		})
	}
}

func TestIPMatchersWithTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		remoteAddr string
		header     string
		forwarded  string
		clientIP   string
		entry      string
		matches    bool
	}{
		{name: "forwarded mapped IPv4 matches exact IPv4", remoteAddr: "10.0.0.1:12345", header: "X-Forwarded-For", forwarded: "::ffff:203.0.113.10", clientIP: "::ffff:203.0.113.10", entry: "203.0.113.10", matches: true},
		{name: "forwarded mapped IPv4 matches IPv4 CIDR", remoteAddr: "10.0.0.1:12345", header: "X-Forwarded-For", forwarded: "::ffff:203.0.113.10", clientIP: "::ffff:203.0.113.10", entry: "203.0.113.0/24", matches: true},
		{name: "forwarded IPv4 matches mapped CIDR", remoteAddr: "10.0.0.1:12345", header: "X-Forwarded-For", forwarded: "203.0.113.10", clientIP: "203.0.113.10", entry: "::ffff:203.0.113.0/120", matches: true},
		{name: "real IP mapped IPv4 matches exact IPv4", remoteAddr: "10.0.0.1:12345", header: "X-Real-IP", forwarded: "::ffff:203.0.113.10", clientIP: "::ffff:203.0.113.10", entry: "203.0.113.10", matches: true},
		{name: "direct mapped remote matches mapped entry", remoteAddr: "[::ffff:203.0.113.10]:12345", clientIP: "203.0.113.10", entry: "::ffff:203.0.113.10", matches: true},
		{name: "untrusted peer cannot spoof forwarded address", remoteAddr: "198.51.100.10:12345", header: "X-Forwarded-For", forwarded: "::ffff:203.0.113.10", clientIP: "198.51.100.10", entry: "203.0.113.0/24"},
	}
	for _, blocklist := range []bool{false, true} {
		name := "allowlist"
		if blocklist {
			name = "blocklist"
		}
		t.Run(name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					engine := gin.New()
					if err := engine.SetTrustedProxies([]string{"10.0.0.0/8"}); err != nil {
						t.Fatal(err)
					}
					engine.GET("/", func(c *gin.Context) {
						if got := c.ClientIP(); got != tt.clientIP {
							t.Fatalf("ClientIP() = %q, want %q", got, tt.clientIP)
						}
						clients := clientIPCandidates(c)
						entries := []string{tt.entry}
						denied := !isAnyAllowedIP(clients, entries)
						if blocklist {
							denied = isBlockedIP(clients, entries)
						}
						if denied {
							c.AbortWithStatus(http.StatusForbidden)
							return
						}
						c.Status(http.StatusOK)
					})
					req := httptest.NewRequest(http.MethodGet, "/", nil)
					req.RemoteAddr = tt.remoteAddr
					if tt.header != "" {
						req.Header.Set(tt.header, tt.forwarded)
					}
					recorder := httptest.NewRecorder()
					engine.ServeHTTP(recorder, req)
					want := http.StatusOK
					if (blocklist && tt.matches) || (!blocklist && !tt.matches) {
						want = http.StatusForbidden
					}
					if recorder.Code != want {
						t.Fatalf("status = %d, want %d", recorder.Code, want)
					}
				})
			}
		})
	}
}
