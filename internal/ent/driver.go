package ent

import (
	"entgo.io/ent/dialect"
)

// Driver returns the underlying dialect.Driver.
// This is useful for executing raw SQL queries when needed.
// It automatically unwraps any dialect.DebugDriver layers
// to return the actual driver (e.g., *sql.Driver).
func (c *Client) Driver() dialect.Driver {
	drv := c.config.driver
	for {
		if dd, ok := drv.(*dialect.DebugDriver); ok {
			drv = dd.Driver
			continue
		}
		return drv
	}
}
