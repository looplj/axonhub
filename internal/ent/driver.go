package ent

import "entgo.io/ent/dialect"

// Driver returns the underlying dialect.Driver.
// This is useful for executing raw SQL queries when needed.
func (c *Client) Driver() dialect.Driver {
	return c.config.driver
}
