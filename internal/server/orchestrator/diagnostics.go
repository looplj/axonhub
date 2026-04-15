package orchestrator

import (
	"sort"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/objects"
)

type AssociationCacheEntrySnapshot struct {
	ModelID                 string                      `json:"modelId"`
	Associations            []*objects.ModelAssociation `json:"associations"`
	CandidateCount          int                         `json:"candidateCount"`
	ChannelCount            int                         `json:"channelCount"`
	LatestChannelUpdateTime time.Time                   `json:"latestChannelUpdateTime"`
	LatestModelUpdateTime   time.Time                   `json:"latestModelUpdateTime"`
	ChannelCacheVersion     int64                       `json:"channelCacheVersion"`
	CachedAt                time.Time                   `json:"cachedAt"`
}

type CandidateSelectorDiagnostics struct {
	defaultSelector *DefaultSelector
}

func NewCandidateSelectorDiagnostics(defaultSelector *DefaultSelector) *CandidateSelectorDiagnostics {
	return &CandidateSelectorDiagnostics{
		defaultSelector: defaultSelector,
	}
}

func (d *CandidateSelectorDiagnostics) ReadAssociationCache() []AssociationCacheEntrySnapshot {
	d.defaultSelector.cacheMu.RLock()
	defer d.defaultSelector.cacheMu.RUnlock()

	modelIDs := lo.Keys(d.defaultSelector.associationCache)
	sort.Strings(modelIDs)

	return lo.Map(modelIDs, func(modelID string, _ int) AssociationCacheEntrySnapshot {
		entry := d.defaultSelector.associationCache[modelID]
		if entry == nil {
			return AssociationCacheEntrySnapshot{ModelID: modelID}
		}

		return AssociationCacheEntrySnapshot{
			ModelID:                 modelID,
			Associations:            append([]*objects.ModelAssociation(nil), entry.associations...),
			CandidateCount:          len(entry.candidates),
			ChannelCount:            entry.channelCount,
			LatestChannelUpdateTime: entry.latestChannelUpdateTime,
			LatestModelUpdateTime:   entry.latestModelUpdateTime,
			ChannelCacheVersion:     entry.channelCacheVersion,
			CachedAt:                entry.cachedAt,
		}
	})
}
