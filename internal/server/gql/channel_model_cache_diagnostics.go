package gql

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	entmodel "github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
)

type channelModelCacheDiagnosticsExport struct {
	ExportedAt string                               `json:"exportedAt"`
	Targets    []string                             `json:"targets"`
	Database   channelModelCacheDiagnosticsDatabase `json:"database"`
	Cache      channelModelCacheDiagnosticsCache    `json:"cache"`
}

type channelModelCacheDiagnosticsDatabase struct {
	Channels []channelModelCacheDiagnosticsChannelDB `json:"channels"`
	Models   []channelModelCacheDiagnosticsModelView `json:"models"`
}

type channelModelCacheDiagnosticsCache struct {
	EnabledChannelCache   channelModelEnabledCacheSnapshot            `json:"enabledChannelCache"`
	ModelAssociationCache []channelModelAssociationCacheEntrySnapshot `json:"modelAssociationCache"`
}

type channelModelAssociationCacheEntrySnapshot struct {
	ModelID                 string                      `json:"modelId"`
	Associations            []*objects.ModelAssociation `json:"associations"`
	CandidateCount          int                         `json:"candidateCount"`
	ChannelCount            int                         `json:"channelCount"`
	LatestChannelUpdateTime string                      `json:"latestChannelUpdateTime"`
	LatestModelUpdateTime   string                      `json:"latestModelUpdateTime"`
	ChannelCacheVersion     int64                       `json:"channelCacheVersion"`
	CachedAt                string                      `json:"cachedAt"`
}

type channelModelEnabledCacheSnapshot struct {
	ChannelCount  int                                         `json:"channelCount"`
	CacheVersion  int64                                       `json:"cacheVersion"`
	ChannelIDs    []int                                       `json:"channelIDs"`
	ChannelNames  []string                                    `json:"channelNames"`
	Channels      []channelModelCacheDiagnosticsChannelCached `json:"channels"`
	CapturedAtUTC string                                      `json:"capturedAtUtc"`
}

type channelModelCacheDiagnosticsChannelDB struct {
	ID                      int                                  `json:"id"`
	Name                    string                               `json:"name"`
	Type                    string                               `json:"type"`
	Status                  string                               `json:"status"`
	UpdatedAt               string                               `json:"updatedAt"`
	BaseURL                 string                               `json:"baseUrl"`
	Tags                    []string                             `json:"tags"`
	SupportedModels         []string                             `json:"supportedModels"`
	DefaultTestModel        string                               `json:"defaultTestModel"`
	AutoSyncSupportedModels bool                                 `json:"autoSyncSupportedModels"`
	Settings                channelModelCacheDiagnosticsSettings `json:"settings"`
}

type channelModelCacheDiagnosticsChannelCached struct {
	ID                  int                                  `json:"id"`
	Name                string                               `json:"name"`
	Type                string                               `json:"type"`
	Status              string                               `json:"status"`
	UpdatedAt           string                               `json:"updatedAt"`
	Tags                []string                             `json:"tags"`
	SupportedModels     []string                             `json:"supportedModels"`
	DefaultTestModel    string                               `json:"defaultTestModel"`
	EnabledAPIKeyCount  int                                  `json:"enabledApiKeyCount"`
	DisabledAPIKeyCount int                                  `json:"disabledApiKeyCount"`
	Settings            channelModelCacheDiagnosticsSettings `json:"settings"`
	ModelEntries        []channelModelCacheDiagnosticsEntry  `json:"modelEntries"`
}

type channelModelCacheDiagnosticsSettings struct {
	ExtraModelPrefix        string                                     `json:"extraModelPrefix,omitempty"`
	AutoTrimedModelPrefixes []string                                   `json:"autoTrimedModelPrefixes,omitempty"`
	HideMappedModels        bool                                       `json:"hideMappedModels"`
	HideOriginalModels      bool                                       `json:"hideOriginalModels"`
	ModelMappings           []channelModelCacheDiagnosticsModelMapping `json:"modelMappings,omitempty"`
}

type channelModelCacheDiagnosticsModelMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type channelModelCacheDiagnosticsEntry struct {
	RequestModel string `json:"requestModel"`
	ActualModel  string `json:"actualModel"`
	Source       string `json:"source"`
}

type channelModelCacheDiagnosticsModelView struct {
	ID                        int                                            `json:"id"`
	ModelID                   string                                         `json:"modelId"`
	Status                    string                                         `json:"status"`
	UpdatedAt                 string                                         `json:"updatedAt"`
	AssociationCount          int                                            `json:"associationCount"`
	Associations              any                                            `json:"associations"`
	CandidateSelectionPreview []channelModelCacheDiagnosticsCandidatePreview `json:"candidateSelectionPreview"`
}

type channelModelCacheDiagnosticsCandidatePreview struct {
	ChannelID   int                                 `json:"channelId"`
	ChannelName string                              `json:"channelName"`
	Priority    int                                 `json:"priority"`
	Models      []channelModelCacheDiagnosticsEntry `json:"models"`
}

func normalizeDiagnosticsTargets(targets []DiagnosticsTarget) []DiagnosticsTarget {
	if len(targets) == 0 {
		return []DiagnosticsTarget{DiagnosticsTargetChannelCache}
	}

	uniq := lo.Uniq(targets)
	slices.Sort(uniq)

	return uniq
}

func buildChannelModelCacheDiagnosticsExport(
	ctx context.Context,
	client *ent.Client,
	channelService *biz.ChannelService,
	modelService *biz.ModelService,
	systemService *biz.SystemService,
	defaultSelector *orchestrator.DefaultSelector,
	candidateSelectorDiagnostics *orchestrator.CandidateSelectorDiagnostics,
	targets []DiagnosticsTarget,
) (*channelModelCacheDiagnosticsExport, error) {
	targets = normalizeDiagnosticsTargets(targets)

	channelsFromDB, err := client.Channel.Query().
		Order(ent.Desc(channel.FieldOrderingWeight)).
		Order(ent.Asc(channel.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	modelsFromDB, err := client.Model.Query().
		Order(ent.Asc(entmodel.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}

	exportedAt := time.Now().UTC().Format(time.RFC3339)
	enabledChannels := channelService.GetEnabledChannels()

	return &channelModelCacheDiagnosticsExport{
		ExportedAt: exportedAt,
		Targets: lo.Map(targets, func(target DiagnosticsTarget, _ int) string {
			return string(target)
		}),
		Database: channelModelCacheDiagnosticsDatabase{
			Channels: lo.Map(channelsFromDB, func(ch *ent.Channel, _ int) channelModelCacheDiagnosticsChannelDB {
				return buildChannelDBSnapshot(ch)
			}),
			Models: buildModelDiagnostics(ctx, modelsFromDB, defaultSelector, channelService, modelService, systemService),
		},
		Cache: buildCacheDiagnostics(enabledChannels, channelService, candidateSelectorDiagnostics, exportedAt),
	}, nil
}

func buildCacheDiagnostics(enabledChannels []*biz.Channel, channelService *biz.ChannelService, candidateSelectorDiagnostics *orchestrator.CandidateSelectorDiagnostics, capturedAt string) channelModelCacheDiagnosticsCache {
	diagnostics := candidateSelectorDiagnostics

	return channelModelCacheDiagnosticsCache{
		EnabledChannelCache:   buildEnabledChannelCacheSnapshot(enabledChannels, channelService.GetCacheVersion(), capturedAt),
		ModelAssociationCache: buildAssociationCacheSnapshot(diagnostics.ReadAssociationCache()),
	}
}

func buildAssociationCacheSnapshot(entries []orchestrator.AssociationCacheEntrySnapshot) []channelModelAssociationCacheEntrySnapshot {
	return lo.Map(entries, func(entry orchestrator.AssociationCacheEntrySnapshot, _ int) channelModelAssociationCacheEntrySnapshot {
		return channelModelAssociationCacheEntrySnapshot{
			ModelID:                 entry.ModelID,
			Associations:            append([]*objects.ModelAssociation(nil), entry.Associations...),
			CandidateCount:          entry.CandidateCount,
			ChannelCount:            entry.ChannelCount,
			LatestChannelUpdateTime: entry.LatestChannelUpdateTime.UTC().Format(time.RFC3339),
			LatestModelUpdateTime:   entry.LatestModelUpdateTime.UTC().Format(time.RFC3339),
			ChannelCacheVersion:     entry.ChannelCacheVersion,
			CachedAt:                entry.CachedAt.UTC().Format(time.RFC3339),
		}
	})
}

func buildEnabledChannelCacheSnapshot(channels []*biz.Channel, cacheVersion int64, capturedAt string) channelModelEnabledCacheSnapshot {
	return channelModelEnabledCacheSnapshot{
		ChannelCount: len(channels),
		CacheVersion: cacheVersion,
		ChannelIDs:   lo.Map(channels, func(ch *biz.Channel, _ int) int { return ch.ID }),
		ChannelNames: lo.Map(channels, func(ch *biz.Channel, _ int) string { return ch.Name }),
		Channels: lo.Map(channels, func(ch *biz.Channel, _ int) channelModelCacheDiagnosticsChannelCached {
			return buildChannelCachedSnapshot(ch)
		}),
		CapturedAtUTC: capturedAt,
	}
}

func buildChannelDBSnapshot(ch *ent.Channel) channelModelCacheDiagnosticsChannelDB {
	return channelModelCacheDiagnosticsChannelDB{
		ID:                      ch.ID,
		Name:                    ch.Name,
		Type:                    ch.Type.String(),
		Status:                  ch.Status.String(),
		UpdatedAt:               ch.UpdatedAt.UTC().Format(time.RFC3339),
		BaseURL:                 ch.BaseURL,
		Tags:                    append([]string(nil), ch.Tags...),
		SupportedModels:         append([]string(nil), ch.SupportedModels...),
		DefaultTestModel:        ch.DefaultTestModel,
		AutoSyncSupportedModels: ch.AutoSyncSupportedModels,
		Settings:                buildSettingsSnapshot(ch.Settings),
	}
}

func buildChannelCachedSnapshot(ch *biz.Channel) channelModelCacheDiagnosticsChannelCached {
	return channelModelCacheDiagnosticsChannelCached{
		ID:                  ch.ID,
		Name:                ch.Name,
		Type:                ch.Type.String(),
		Status:              ch.Status.String(),
		UpdatedAt:           ch.UpdatedAt.UTC().Format(time.RFC3339),
		Tags:                append([]string(nil), ch.Tags...),
		SupportedModels:     append([]string(nil), ch.SupportedModels...),
		DefaultTestModel:    ch.DefaultTestModel,
		EnabledAPIKeyCount:  len(ch.GetEnabledAPIKeys()),
		DisabledAPIKeyCount: len(ch.DisabledAPIKeys),
		Settings:            buildSettingsSnapshot(ch.Settings),
		ModelEntries:        buildSortedEntries(ch.GetModelEntries()),
	}
}

func buildSettingsSnapshot(settings *objects.ChannelSettings) channelModelCacheDiagnosticsSettings {
	if settings == nil {
		return channelModelCacheDiagnosticsSettings{}
	}

	return channelModelCacheDiagnosticsSettings{
		ExtraModelPrefix:        settings.ExtraModelPrefix,
		AutoTrimedModelPrefixes: append([]string(nil), settings.AutoTrimedModelPrefixes...),
		HideMappedModels:        settings.HideMappedModels,
		HideOriginalModels:      settings.HideOriginalModels,
		ModelMappings: lo.Map(settings.ModelMappings, func(mapping objects.ModelMapping, _ int) channelModelCacheDiagnosticsModelMapping {
			return channelModelCacheDiagnosticsModelMapping{
				From: mapping.From,
				To:   mapping.To,
			}
		}),
	}
}

func buildSortedEntries(entries map[string]biz.ChannelModelEntry) []channelModelCacheDiagnosticsEntry {
	keys := lo.Keys(entries)
	sort.Strings(keys)

	result := make([]channelModelCacheDiagnosticsEntry, 0, len(keys))
	for _, key := range keys {
		entry := entries[key]
		result = append(result, channelModelCacheDiagnosticsEntry{
			RequestModel: entry.RequestModel,
			ActualModel:  entry.ActualModel,
			Source:       entry.Source,
		})
	}

	return result
}

func buildModelDiagnostics(
	ctx context.Context,
	models []*ent.Model,
	defaultSelector *orchestrator.DefaultSelector,
	channelService *biz.ChannelService,
	modelService *biz.ModelService,
	systemService *biz.SystemService,
) []channelModelCacheDiagnosticsModelView {
	selector := defaultSelector
	if selector == nil {
		selector = orchestrator.NewDefaultSelector(channelService, modelService, systemService)
	}

	result := make([]channelModelCacheDiagnosticsModelView, 0, len(models))
	for _, mdl := range models {
		var candidates []*orchestrator.ChannelModelsCandidate

		if mdl.Status == entmodel.StatusEnabled {
			selected, err := selector.Select(ctx, &llm.Request{Model: mdl.ModelID})
			if err == nil {
				candidates = selected
			}
		}

		associationCount := 0

		var associations any = nil

		if mdl.Settings != nil {
			associationCount = len(mdl.Settings.Associations)
			associations = mdl.Settings.Associations
		}

		result = append(result, channelModelCacheDiagnosticsModelView{
			ID:                        mdl.ID,
			ModelID:                   mdl.ModelID,
			Status:                    mdl.Status.String(),
			UpdatedAt:                 mdl.UpdatedAt.UTC().Format(time.RFC3339),
			AssociationCount:          associationCount,
			Associations:              associations,
			CandidateSelectionPreview: buildCandidatePreview(candidates),
		})
	}

	return result
}

func buildCandidatePreview(candidates []*orchestrator.ChannelModelsCandidate) []channelModelCacheDiagnosticsCandidatePreview {
	return lo.Map(candidates, func(candidate *orchestrator.ChannelModelsCandidate, _ int) channelModelCacheDiagnosticsCandidatePreview {
		return channelModelCacheDiagnosticsCandidatePreview{
			ChannelID:   candidate.Channel.ID,
			ChannelName: candidate.Channel.Name,
			Priority:    candidate.Priority,
			Models: lo.Map(candidate.Models, func(entry biz.ChannelModelEntry, _ int) channelModelCacheDiagnosticsEntry {
				return channelModelCacheDiagnosticsEntry{
					RequestModel: entry.RequestModel,
					ActualModel:  entry.ActualModel,
					Source:       entry.Source,
				}
			}),
		}
	})
}

func marshalChannelModelCacheDiagnosticsExport(data *channelModelCacheDiagnosticsExport) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal diagnostics export: %w", err)
	}

	return string(bytes), nil
}
