package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/log"
)

// ipAccessControlState holds the IP access control configuration values.
// It is stored atomically via atomic.Pointer in IPAccessControlConfig.
type ipAccessControlState struct {
	enabled     bool
	exact       map[netip.Addr]struct{}
	prefixes    []netip.Prefix
	redirectURL string
}

// IPAccessControlConfig holds reloadable IP access control configuration.
// It atomically replaces validated, immutable snapshots at runtime.
type IPAccessControlConfig struct {
	ptr atomic.Pointer[ipAccessControlState]
}

// NewIPAccessControlConfig creates a new reloadable IP access control config.
func NewIPAccessControlConfig(enabled bool, allowedIPs []string, redirectURL string) (*IPAccessControlConfig, error) {
	state, err := newIPAccessControlState(enabled, allowedIPs, redirectURL)
	if err != nil {
		return nil, err
	}

	cfg := &IPAccessControlConfig{}
	cfg.ptr.Store(state)
	return cfg, nil
}

// Apply validates a replacement state before atomically making it live.
// A rejected reload leaves the previous snapshot active for in-flight and
// future requests.
func (c *IPAccessControlConfig) Apply(enabled bool, allowedIPs []string, redirectURL string) error {
	state, err := newIPAccessControlState(enabled, allowedIPs, redirectURL)
	if err != nil {
		return err
	}

	c.ptr.Store(state)
	return nil
}

// WithIPAccessControl restricts all access to whitelisted IPs/CIDRs.
// It reads the current immutable snapshot on every request, so committed
// configuration changes take effect without rebuilding routes.
// When enabled, requests from non-allowed IPs are redirected to redirectURL.
func WithIPAccessControl(cfg *IPAccessControlConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := cfg.ptr.Load()
		if !state.enabled {
			c.Next()
			return
		}

		clientIP := strings.TrimSpace(c.ClientIP())
		if clientIP == "" {
			denyRequest(c, state.redirectURL)
			return
		}

		clientAddr, err := netip.ParseAddr(clientIP)
		if err != nil {
			log.Warn(context.Background(), "failed to parse client IP", log.String("client_ip", clientIP), log.Cause(err))
			denyRequest(c, state.redirectURL)
			return
		}

		if state.allows(clientAddr.Unmap()) {
			c.Next()
			return
		}

		denyRequest(c, state.redirectURL)
	}
}

func newIPAccessControlState(enabled bool, allowedIPs []string, redirectURL string) (*ipAccessControlState, error) {
	state := &ipAccessControlState{
		enabled:     enabled,
		exact:       make(map[netip.Addr]struct{}, len(allowedIPs)),
		prefixes:    make([]netip.Prefix, 0, len(allowedIPs)),
		redirectURL: redirectURL,
	}

	for _, raw := range allowedIPs {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}

		if strings.Contains(item, "/") {
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				return nil, fmt.Errorf("invalid allowed IP prefix %q: %w", item, err)
			}
			state.prefixes = append(state.prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(item)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed IP %q: %w", item, err)
		}
		state.exact[addr.Unmap()] = struct{}{}
	}

	return state, nil
}

func (s *ipAccessControlState) allows(clientAddr netip.Addr) bool {
	if _, ok := s.exact[clientAddr]; ok {
		return true
	}

	for _, prefix := range s.prefixes {
		if prefix.Contains(clientAddr) {
			return true
		}
	}

	return false
}

func denyRequest(c *gin.Context, redirectURL string) {
	if redirectURL != "" {
		c.Redirect(http.StatusFound, redirectURL)
		c.Abort()
		return
	}

	c.AbortWithStatus(http.StatusNotFound)
}
