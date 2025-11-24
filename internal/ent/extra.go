package ent

import (
	"github.com/looplj/axonhub/internal/ent/channel"
)

func (r *Role) IsSystemRole() bool {
	return r.ProjectID == nil || *r.ProjectID == 0
}

func (c *ChannelOrder) ToOrderOption() channel.OrderOption {
	return c.Field.toTerm(c.Direction.OrderTermOption())
}
