package ent

import (
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func (r *Role) IsSystemRole() bool {
	return r.ProjectID == nil || *r.ProjectID == 0
}

func (c *ChannelOrder) ToOrderOption() channel.OrderOption {
	return c.Field.toTerm(c.Direction.OrderTermOption())
}

func (a *APIKey) GetActiveProfile() *objects.APIKeyProfile {
	if a == nil || a.Profiles == nil || a.Profiles.ActiveProfile == "" {
		return nil
	}

	for i := range a.Profiles.Profiles {
		if a.Profiles.Profiles[i].Name == a.Profiles.ActiveProfile {
			return &a.Profiles.Profiles[i]
		}
	}

	return nil
}
