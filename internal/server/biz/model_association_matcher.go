package biz

import (
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
	KeyGroup *KeyGroupRef        `json:"keyGroup,omitempty"`
	Models   []ChannelModelEntry `json:"models"`
	Priority int                 `json:"priority"`
}

// KeyGroupRef identifies the API-key group that produced a model match. It is
// nil for legacy channel-level connections without keyGroups.
type KeyGroupRef struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// keyGroupsForModel returns the key-group refs whose model allowlist contains
// the requested model. When a channel has no key groups, a single nil ref is
// returned so the legacy channel-level behavior remains unchanged.
func keyGroupsForModel(ch *Channel, model string) []*KeyGroupRef {
	if ch == nil || !ch.Credentials.HasKeyGroups() {
		return []*KeyGroupRef{nil}
	}

	refs := make([]*KeyGroupRef, 0, len(ch.Credentials.KeyGroups))
	for index, group := range ch.Credentials.KeyGroups {
		if !group.Enabled {
			continue
		}
		if len(group.Models) > 0 && !lo.Contains(group.Models, model) {
			continue
		}
		name := group.Name
		if name == "" {
			name = fmt.Sprintf("分组%d", index+1)
		}
		refs = append(refs, &KeyGroupRef{Index: index, Name: name})
	}

	return refs
}

// modelEntriesForGroup returns the channel model entries visible to one key
// group. Group-scoped models restrict the global channel model entries to the
// group allowlist; an unscoped group keeps the full channel catalog.
func modelEntriesForGroup(ch *Channel, group *KeyGroupRef) map[string]ChannelModelEntry {
	entries := ch.GetModelEntries()
	if group == nil {
		return entries
	}

	groupModels := ch.Credentials.KeyGroups[group.Index].Models
	if len(groupModels) == 0 {
		return entries
	}

	allowed := make(map[string]ChannelModelEntry, len(groupModels))
	for _, model := range groupModels {
		if entry, ok := entries[model]; ok {
			allowed[model] = entry
		}
	}
	return allowed
}

func appendGroupedConnection(
	connections []*ModelChannelConnection,
	ch *Channel,
	modelID string,
	entry ChannelModelEntry,
	priority int,
	tracker *DuplicateKeyTracker,
) []*ModelChannelConnection {
	for _, group := range keyGroupsForModel(ch, modelID) {
		groupIndex := -1
		if group != nil {
			groupIndex = group.Index
		}
		if !tracker.AddGroup(ch.ID, groupIndex, modelID) {
			continue
		}
		connections = append(connections, &ModelChannelConnection{
			Channel:  ch.Channel,
			KeyGroup: group,
			Models:   []ChannelModelEntry{entry},
			Priority: priority,
		})
	}
	return connections
}

// AssociationMatch groups the connections produced by a single association.
// Keeping this boundary lets higher layers apply request-specific filtering
// after structural association resolution has completed.
type AssociationMatch struct {
	Association *objects.ModelAssociation `json:"association"`
	Connections []*ModelChannelConnection `json:"connections"`
}

// ChannelModelKey represents a unique combination of channel and model.
type ChannelModelKey struct {
	ChannelID  int
	GroupIndex int
	ModelID    string
}

// DuplicateKeyTracker tracks duplicate combinations using a struct key.
type DuplicateKeyTracker struct {
	seen map[ChannelModelKey]bool
}

// NewDuplicateKeyTracker creates a new duplicate key tracker.
func NewDuplicateKeyTracker() *DuplicateKeyTracker {
	return &DuplicateKeyTracker{
		seen: make(map[ChannelModelKey]bool),
	}
}

// Add checks if a channel-model combination exists and adds it if it doesn't.
// Returns true if the combination was newly added, false if it already existed.
func (d *DuplicateKeyTracker) Add(channelID int, modelID string) bool {
	return d.AddGroup(channelID, -1, modelID)
}

// AddGroup checks the channel/key-group/model combination. GroupIndex -1 is the
// legacy no-group identity.
func (d *DuplicateKeyTracker) AddGroup(channelID, groupIndex int, modelID string) bool {
	key := ChannelModelKey{
		ChannelID:  channelID,
		GroupIndex: groupIndex,
		ModelID:    modelID,
	}
	if d.seen[key] {
		return false
	}

	d.seen[key] = true

	return true
}

// String returns the string representation of the key for compatibility.
func (k ChannelModelKey) String() string {
	return fmt.Sprintf("%d:%d:%s", k.ChannelID, k.GroupIndex, k.ModelID)
}

// MatchConnections matches associations against channels and their supported models.
// Returns ModelChannelConnection with priority for each match.
// Results are ordered by the matching order of associations.
// Deduplication: Same (channel, model) combination will only appear once.
func MatchConnections(
	associations []*objects.ModelAssociation,
	channels []*Channel,
) []*ModelChannelConnection {
	result := make([]*ModelChannelConnection, 0)
	tracker := NewDuplicateKeyTracker()

	for _, assoc := range associations {
		if assoc.Disabled {
			continue
		}
		connections := matchSingleAssociation(assoc, channels, tracker)
		result = append(result, connections...)
	}

	return result
}

// MatchAssociations matches associations against channels while
// preserving the originating association for each batch of resolved connections.
// Deduplication applies within each association. Cross-association deduplication
// must happen after request-specific conditions have been evaluated.
func MatchAssociations(
	associations []*objects.ModelAssociation,
	channels []*Channel,
) []*AssociationMatch {
	result := make([]*AssociationMatch, 0, len(associations))

	for _, assoc := range associations {
		if assoc == nil || assoc.Disabled {
			continue
		}

		tracker := NewDuplicateKeyTracker()
		connections := matchSingleAssociation(assoc, channels, tracker)
		if len(connections) == 0 {
			continue
		}

		result = append(result, &AssociationMatch{
			Association: assoc,
			Connections: connections,
		})
	}

	return result
}

// matchSingleAssociation matches a single association against all channels.
func matchSingleAssociation(
	assoc *objects.ModelAssociation,
	channels []*Channel,
	tracker *DuplicateKeyTracker,
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
	case "channel_tags_model":
		connections = matchChannelTagsModel(assoc, channels, tracker)
	case "channel_tags_regex":
		connections = matchChannelTagsRegex(assoc, channels, tracker)
	}

	return connections
}

// matchChannelModel handles channel_model type association.
func matchChannelModel(assoc *objects.ModelAssociation, channels []*Channel, tracker *DuplicateKeyTracker) []*ModelChannelConnection {
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

	return appendGroupedConnection(nil, ch, assoc.ChannelModel.ModelID, entry, assoc.Priority, tracker)
}

// matchChannelRegex handles channel_regex type association.
func matchChannelRegex(assoc *objects.ModelAssociation, channels []*Channel, tracker *DuplicateKeyTracker) []*ModelChannelConnection {
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

	var matched []string
	for modelID := range entries {
		if xregexp.MatchString(assoc.ChannelRegex.Pattern, modelID) {
			matched = append(matched, modelID)
		}
	}

	var models []ChannelModelEntry
	for _, modelID := range matched {
		if tracker.Add(ch.ID, modelID) {
			models = append(models, entries[modelID])
		}
	}
	if len(models) == 0 {
		return nil
	}
	return []*ModelChannelConnection{{
		Channel:  ch.Channel,
		Models:   models,
		Priority: assoc.Priority,
	}}
}

// matchRegex handles regex type association.
func matchRegex(assoc *objects.ModelAssociation, channels []*Channel, tracker *DuplicateKeyTracker) []*ModelChannelConnection {
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
				if tracker.Add(ch.ID, modelID) {
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
func matchModel(assoc *objects.ModelAssociation, channels []*Channel, tracker *DuplicateKeyTracker) []*ModelChannelConnection {
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

		connections = appendGroupedConnection(connections, ch, modelID, entry, assoc.Priority, tracker)
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

		// Check channel tags
		if len(exclude.ChannelTags) > 0 {
			for _, excludedTag := range exclude.ChannelTags {
				if lo.Contains(ch.Tags, excludedTag) {
					return true
				}
			}
		}
	}

	return false
}

// matchChannelTagsModel handles channel_tags_model type association.
// Matches channels that have any of the specified tags (OR logic) and returns the specified model.
func matchChannelTagsModel(assoc *objects.ModelAssociation, channels []*Channel, tracker *DuplicateKeyTracker) []*ModelChannelConnection {
	if assoc.ChannelTagsModel == nil {
		return nil
	}

	if len(assoc.ChannelTagsModel.ChannelTags) == 0 {
		return nil
	}

	connections := make([]*ModelChannelConnection, 0)
	modelID := assoc.ChannelTagsModel.ModelID

	for _, ch := range channels {
		// Check if channel has any of the specified tags (OR logic)
		hasTag := false

		for _, tag := range assoc.ChannelTagsModel.ChannelTags {
			if lo.Contains(ch.Tags, tag) {
				hasTag = true
				break
			}
		}

		if !hasTag {
			continue
		}

		// Check if channel has the specified model
		entries := ch.GetModelEntries()
		entry, contains := entries[modelID]

		if !contains {
			continue
		}

		connections = appendGroupedConnection(connections, ch, modelID, entry, assoc.Priority, tracker)
	}

	return connections
}

// matchChannelTagsRegex handles channel_tags_regex type association.
// Matches channels that have any of the specified tags (OR logic) and returns models matching the pattern.
func matchChannelTagsRegex(assoc *objects.ModelAssociation, channels []*Channel, tracker *DuplicateKeyTracker) []*ModelChannelConnection {
	if assoc.ChannelTagsRegex == nil {
		return nil
	}

	if len(assoc.ChannelTagsRegex.ChannelTags) == 0 {
		return nil
	}

	connections := make([]*ModelChannelConnection, 0)

	for _, ch := range channels {
		// Check if channel has any of the specified tags (OR logic)
		hasTag := false

		for _, tag := range assoc.ChannelTagsRegex.ChannelTags {
			if lo.Contains(ch.Tags, tag) {
				hasTag = true
				break
			}
		}

		if !hasTag {
			continue
		}

		entries := ch.GetModelEntries()

		var models []ChannelModelEntry
		for modelID, entry := range entries {
			if xregexp.MatchString(assoc.ChannelTagsRegex.Pattern, modelID) {
				if tracker.Add(ch.ID, modelID) {
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
