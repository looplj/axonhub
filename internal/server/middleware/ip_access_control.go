package middleware

import (
	"context"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/log"
)

// WithIPAccessControl restricts all access to whitelisted IPs/CIDRs.
// When enabled, requests from non-allowed IPs are redirected to redirectURL.
func WithIPAccessControl(enabled bool, allowedIPs []string, redirectURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		clientIP := strings.TrimSpace(c.ClientIP())
		if clientIP == "" {
			denyRequest(c, redirectURL)
			return
		}

		clientAddr, err := netip.ParseAddr(clientIP)
		if err != nil {
			log.Warn(context.Background(), "failed to parse client IP", log.String("client_ip", clientIP), log.Cause(err))
			denyRequest(c, redirectURL)
			return
		}

		if matchIP(clientAddr, allowedIPs) {
			c.Next()
			return
		}

		denyRequest(c, redirectURL)
	}
}

func denyRequest(c *gin.Context, redirectURL string) {
	if redirectURL != "" {
		c.Redirect(http.StatusFound, redirectURL)
		c.Abort()
		return
	}

	c.AbortWithStatus(http.StatusNotFound)
}

func matchIP(clientAddr netip.Addr, ips []string) bool {
	for _, item := range ips {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if strings.Contains(item, "/") {
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				log.Warn(context.Background(), "failed to parse IP prefix", log.String("ip", item), log.Cause(err))
				continue
			}

			if prefix.Contains(clientAddr) {
				return true
			}

			continue
		}

		addr, err := netip.ParseAddr(item)
		if err != nil {
			log.Warn(context.Background(), "failed to parse IP", log.String("ip", item), log.Cause(err))
			continue
		}

		if addr == clientAddr {
			return true
		}
	}

	return false
}
