package biz

import (
	"context"
	"fmt"

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

// deduplicationTracker tracks which (channelID, modelID) combinations have been added.
type deduplicationTracker map[string]bool

// makeKey creates a unique key for (channelID, modelID) combination.
func (d deduplicationTracker) makeKey(channelID int, modelID string) string {
	return fmt.Sprintf("%d:%s", channelID, modelID)
}

// add marks a (channelID, modelID) combination as added.
// Returns true if it was newly added, false if it already existed.
func (d deduplicationTracker) add(channelID int, modelID string) bool {
	key := d.makeKey(channelID, modelID)
	if d[key] {
		return false
	}

	d[key] = true

	return true
}

// MatchAssociations matches associations against channels and their supported models.
// Returns ModelChannelConnection with priority for each match.
// Results are ordered by the matching order of associations.
// Deduplication: Same (channel, model) combination will only appear once.
func MatchAssociations(
	ctx context.Context,
	associations []*objects.ModelAssociation,
	channels []*Channel,
) ([]*ModelChannelConnection, error) {
	result := make([]*ModelChannelConnection, 0)
	tracker := make(deduplicationTracker)

	for _, assoc := range associations {
		connections := matchSingleAssociation(assoc, channels, tracker)
		result = append(result, connections...)
	}

	return result, nil
}

// matchSingleAssociation matches a single association against all channels.
func matchSingleAssociation(
	assoc *objects.ModelAssociation,
	channels []*Channel,
	tracker deduplicationTracker,
) []*ModelChannelConnection {
	connections := make([]*ModelChannelConnection, 0)

	switch assoc.Type {
	case "channel_model":
		connections = matchChannelModel(assoc, channels, tracker)
	case "channel_regex":
		connections = matchChannelRegex(assoc, channels, tracker)
	case "regex":
		connections = matchRegex(assoc, channels, tracker)
	case "model":
		connections = matchModel(assoc, channels, tracker)
	}

	return connections
}

// matchChannelModel handles channel_model type association.
func matchChannelModel(assoc *objects.ModelAssociation, channels []*Channel, tracker deduplicationTracker) []*ModelChannelConnection {
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

	// Check deduplication
	if !tracker.add(ch.ID, assoc.ChannelModel.ModelID) {
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
func matchChannelRegex(assoc *objects.ModelAssociation, channels []*Channel, tracker deduplicationTracker) []*ModelChannelConnection {
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
			// Check deduplication
			if tracker.add(ch.ID, modelID) {
				models = append(models, entry)
			}
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
func matchRegex(assoc *objects.ModelAssociation, channels []*Channel, tracker deduplicationTracker) []*ModelChannelConnection {
	if assoc.Regex == nil {
		return nil
	}

	connections := make([]*ModelChannelConnection, 0)

	for _, ch := range channels {
		// Check if channel should be excluded
		if shouldExcludeChannel(ch, assoc.Regex.Exclude) {
			continue
		}

		entries := ch.GetModelEntries()

		var models []ChannelModelEntry

		for modelID, entry := range entries {
			if xregexp.MatchString(assoc.Regex.Pattern, modelID) {
				// Check deduplication
				if tracker.add(ch.ID, modelID) {
					models = append(models, entry)
				}
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
func matchModel(assoc *objects.ModelAssociation, channels []*Channel, tracker deduplicationTracker) []*ModelChannelConnection {
	if assoc.ModelID == nil {
		return nil
	}

	modelID := assoc.ModelID.ModelID
	connections := make([]*ModelChannelConnection, 0)

	for _, ch := range channels {
		// Check if channel should be excluded
		if shouldExcludeChannel(ch, assoc.ModelID.Exclude) {
			continue
		}

		entries := ch.GetModelEntries()
		entry, contains := entries[modelID]

		if !contains {
			continue
		}

		// Check deduplication
		if !tracker.add(ch.ID, modelID) {
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

// shouldExcludeChannel checks if a channel should be excluded based on exclude rules.
func shouldExcludeChannel(ch *Channel, excludes []*objects.ExcludeAssociation) bool {
	if len(excludes) == 0 {
		return false
	}

	for _, exclude := range excludes {
		// Check channel name pattern
		if exclude.ChannelNamePattern != "" {
			if xregexp.MatchString(exclude.ChannelNamePattern, ch.Name) {
				return true
			}
		}

		// Check channel IDs
		if len(exclude.ChannelIds) > 0 {
			if lo.Contains(exclude.ChannelIds, ch.ID) {
				return true
			}
		}
	}

	return false
}
