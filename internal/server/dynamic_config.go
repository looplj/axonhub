package server

import (
	"github.com/looplj/axonhub/internal/server/middleware"
)

// NewIPAccessControlRuntime creates the single mutable runtime state used by
// the IP access-control middleware. SIGHUP reload handling updates this
// object directly instead of rebuilding routes or introducing a global
// configuration event bus.
func NewIPAccessControlRuntime(cfg Config) (*middleware.IPAccessControlConfig, error) {
	return middleware.NewIPAccessControlConfig(
		cfg.IPAccessControl.Enabled,
		cfg.IPAccessControl.AllowedIPs,
		cfg.IPAccessControl.RedirectURL,
	)
}
