package biz

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
)

// ModelChannelConnection represents a channel and its matched model entries.
// This is used to return association match results.
type ModelChannelConnection struct {
	Channel  *ent.Channel        `json:"channel"`
	Models   []ChannelModelEntry `json:"models"`
	Priority int                 `json:"priority"`
}

// MatchAssociations matches associations against channels and their supported models.
// Returns ModelChannelConnection with priority for each match.
// Results are ordered by the matching order of associations.
func MatchAssociations(
	ctx context.Context,
	associations []*objects.ModelAssociation,
	channels []*Channel,
) ([]*ModelChannelConnection, error) {
	result := make([]*ModelChannelConnection, 0)

	for _, assoc := range associations {
		connections := matchSingleAssociation(assoc, channels)
		result = append(result, connections...)
	}

	return result, nil
}

// matchSingleAssociation matches a single association against all channels.
func matchSingleAssociation(
	assoc *objects.ModelAssociation,
	channels []*Channel,
) []*ModelChannelConnection {
	connections := make([]*ModelChannelConnection, 0)

	switch assoc.Type {
	case "channel_model":
		connections = matchChannelModel(assoc, channels)
	case "channel_regex":
		connections = matchChannelRegex(assoc, channels)
	case "regex":
		connections = matchRegex(assoc, channels)
	case "model":
		connections = matchModel(assoc, channels)
	}

	return connections
}

// matchChannelModel handles channel_model type association.
func matchChannelModel(assoc *objects.ModelAssociation, channels []*Channel) []*ModelChannelConnection {
	if assoc.ChannelModel == nil {
		return nil
	}

	ch, found := lo.Find(channels, func(c *Channel) bool {
		return c.ID == assoc.ChannelModel.ChannelID
	})
	if !found {
		return nil
	}

	entries := ch.GetModelEntries()
	entry, contains := entries[assoc.ChannelModel.ModelID]

	if !contains {
		return nil
	}

	return []*ModelChannelConnection{
		{
			Channel:  ch.Channel,
			Models:   []ChannelModelEntry{entry},
			Priority: assoc.Priority,
		},
	}
}

// matchChannelRegex handles channel_regex type association.
func matchChannelRegex(assoc *objects.ModelAssociation, channels []*Channel) []*ModelChannelConnection {
	if assoc.ChannelRegex == nil {
		return nil
	}

	ch, found := lo.Find(channels, func(c *Channel) bool {
		return c.ID == assoc.ChannelRegex.ChannelID
	})
	if !found {
		return nil
	}

	entries := ch.GetModelEntries()

	var models []ChannelModelEntry

	for modelID, entry := range entries {
		if xregexp.MatchString(assoc.ChannelRegex.Pattern, modelID) {
			models = append(models, entry)
		}
	}

	if len(models) == 0 {
		return nil
	}

	return []*ModelChannelConnection{
		{
			Channel:  ch.Channel,
			Models:   models,
			Priority: assoc.Priority,
		},
	}
}

// matchRegex handles regex type association.
func matchRegex(assoc *objects.ModelAssociation, channels []*Channel) []*ModelChannelConnection {
	if assoc.Regex == nil {
		return nil
	}

	connections := make([]*ModelChannelConnection, 0)

	for _, ch := range channels {
		entries := ch.GetModelEntries()

		var models []ChannelModelEntry

		for modelID, entry := range entries {
			if xregexp.MatchString(assoc.Regex.Pattern, modelID) {
				models = append(models, entry)
			}
		}

		if len(models) == 0 {
			continue
		}

		connections = append(connections, &ModelChannelConnection{
			Channel:  ch.Channel,
			Models:   models,
			Priority: assoc.Priority,
		})
	}

	return connections
}

// matchModel handles model type association.
func matchModel(assoc *objects.ModelAssociation, channels []*Channel) []*ModelChannelConnection {
	if assoc.ModelID == nil {
		return nil
	}

	modelID := assoc.ModelID.ModelID
	connections := make([]*ModelChannelConnection, 0)

	for _, ch := range channels {
		entries := ch.GetModelEntries()
		entry, contains := entries[modelID]

		if !contains {
			continue
		}

		connections = append(connections, &ModelChannelConnection{
			Channel:  ch.Channel,
			Models:   []ChannelModelEntry{entry},
			Priority: assoc.Priority,
		})
	}

	return connections
}
