package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// WithIPBlocklist blocks external API requests from configured IP addresses or CIDR ranges.
func WithIPBlocklist(systemService *biz.SystemService) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIPs := clientIPCandidates(c)
		if len(clientIPs) == 0 {
			c.Next()
			return
		}

		ctx := authz.WithSystemBypass(c.Request.Context(), "ip-blocklist-middleware")
		settings := systemService.SecuritySettingsOrDefault(ctx)
		if !isBlockedIP(clientIPs, settings.BlockedIPs) {
			c.Next()
			return
		}

		AbortWithError(c, http.StatusForbidden, errors.New("IP address is blocked"))
	}
}

func clientIPCandidates(c *gin.Context) []string {
	candidates := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}

		if _, ok := seen[value]; ok {
			return
		}

		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	add(c.ClientIP())

	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		before, _, _ := strings.Cut(xff, ",")
		add(before)
	}

	add(c.Request.Header.Get("X-Real-IP"))

	return candidates
}

func isBlockedIP(clientIPs []string, blockedIPs []string) bool {
	for _, clientIP := range clientIPs {
		clientAddr, err := netip.ParseAddr(clientIP)
		if err != nil {
			log.Warn(context.Background(), "failed to parse client IP", log.String("client_ip", clientIP), log.Cause(err))
			continue
		}

		if isBlockedAddr(clientAddr, blockedIPs) {
			return true
		}
	}

	return false
}

func isBlockedAddr(clientAddr netip.Addr, blockedIPs []string) bool {
	for _, item := range blockedIPs {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if strings.Contains(item, "/") {
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				log.Warn(context.Background(), "failed to parse blocked IP prefix", log.String("blocked_ip", item), log.Cause(err))
				continue
			}

			if prefix.Contains(clientAddr) {
				return true
			}

			continue
		}

		blockedAddr, err := netip.ParseAddr(item)
		if err != nil {
			log.Warn(context.Background(), "failed to parse blocked IP", log.String("blocked_ip", item), log.Cause(err))
			continue
		}

		if blockedAddr == clientAddr {
			return true
		}
	}

	return false
}
