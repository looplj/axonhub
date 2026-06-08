package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

func (svc *BackupService) Restore(ctx context.Context, data []byte, opts RestoreOptions) error {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return fmt.Errorf("user not found in context")
	}

	if !user.IsOwner {
		return fmt.Errorf("only owners can perform restore operations")
	}

	var backupData BackupData
	if err := json.Unmarshal(data, &backupData); err != nil {
		return err
	}

	if !lo.Contains([]string{BackupVersion, BackupVersionV3, BackupVersionV2, BackupVersionV1}, backupData.Version) {
		log.Warn(ctx, "backup version mismatch",
			log.String("expected", BackupVersion),
			log.String("got", backupData.Version))

		return fmt.Errorf("backup version mismatch: expected %s, got %s", BackupVersion, backupData.Version)
	}

	tx, err := svc.db.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	committed := false

	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	txClient := tx.Client()

	if err := svc.restore(ctx, txClient, backupData, opts); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	committed = true

	return nil
}

func (svc *BackupService) restore(ctx context.Context, db *ent.Client, backupData BackupData, opts RestoreOptions) error {
	if opts.IncludeChannels {
		if err := svc.restoreChannels(ctx, db, backupData.Channels, opts); err != nil {
			return err
		}
	}

	channelIDMap, err := svc.buildChannelIDMap(ctx, db, backupData.Channels)
	if err != nil {
		return err
	}

	if opts.IncludeModelPrices {
		if err := svc.restoreChannelModelPrices(ctx, db, backupData.ChannelModelPrices, opts); err != nil {
			return err
		}
	}

	if opts.IncludeModels {
		if err := svc.restoreModels(ctx, db, backupData.Models, opts, channelIDMap); err != nil {
			return err
		}
	}

	if opts.IncludeProjects {
		for _, projData := range backupData.Projects {
			if projData == nil {
				continue
			}

			remapProjectProfilesChannelIDs(projData.Profiles, channelIDMap)
		}

		if err := svc.restoreProjects(ctx, db, backupData.Projects, opts); err != nil {
			return err
		}
	}

	if opts.IncludeAPIKeys {
		if err := svc.restoreAPIKeys(ctx, db, backupData.APIKeys, opts, channelIDMap); err != nil {
			return err
		}
	}

	if opts.IncludeUsageStats || opts.IncludeRequestLogs {
		if err := svc.restoreUsageData(ctx, db, backupData.UsageRequests, backupData.UsageLogs, opts); err != nil {
			return err
		}
	}

	return nil
}

func (svc *BackupService) buildChannelIDMap(ctx context.Context, db *ent.Client, channels []*BackupChannel) (map[int]int, error) {
	idMap := map[int]int{}
	if len(channels) == 0 {
		return idMap, nil
	}

	// Collect all channel names and create a map from name to old ID
	nameToOldID := make(map[string]int)
	names := make([]string, 0, len(channels))

	for _, chData := range channels {
		if chData == nil {
			continue
		}

		oldID := chData.ID
		if oldID == 0 || chData.Name == "" {
			continue
		}

		nameToOldID[chData.Name] = oldID
		names = append(names, chData.Name)
	}

	if len(names) == 0 {
		return idMap, nil
	}

	// Batch query all channels by names, only select needed fields (id, name)
	existingChannels, err := db.Channel.Query().
		Where(channel.NameIn(names...)).
		Select(channel.FieldID, channel.FieldName).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// Build the ID mapping
	for _, ch := range existingChannels {
		if oldID, ok := nameToOldID[ch.Name]; ok {
			idMap[oldID] = ch.ID
		}
	}

	return idMap, nil
}

type usageRestoreResolver struct {
	projectNames map[string]int
	projectIDs   map[int]struct{}
	channelNames map[string]int
	channelIDs   map[int]struct{}
	apiKeyKeys   map[string]int
}

func newUsageRestoreResolver(ctx context.Context, db *ent.Client) (*usageRestoreResolver, error) {
	projects, err := db.Project.Query().
		Select(project.FieldID, project.FieldName).
		All(ctx)
	if err != nil {
		return nil, err
	}

	channels, err := db.Channel.Query().
		Select(channel.FieldID, channel.FieldName).
		All(ctx)
	if err != nil {
		return nil, err
	}

	apiKeys, err := db.APIKey.Query().
		Select(apikey.FieldID, apikey.FieldKey).
		All(ctx)
	if err != nil {
		return nil, err
	}

	resolver := &usageRestoreResolver{
		projectNames: make(map[string]int, len(projects)),
		projectIDs:   make(map[int]struct{}, len(projects)),
		channelNames: make(map[string]int, len(channels)),
		channelIDs:   make(map[int]struct{}, len(channels)),
		apiKeyKeys:   make(map[string]int, len(apiKeys)),
	}

	for _, proj := range projects {
		resolver.projectIDs[proj.ID] = struct{}{}
		resolver.projectNames[proj.Name] = proj.ID
	}

	for _, ch := range channels {
		resolver.channelIDs[ch.ID] = struct{}{}
		resolver.channelNames[ch.Name] = ch.ID
	}

	for _, ak := range apiKeys {
		resolver.apiKeyKeys[ak.Key] = ak.ID
	}

	return resolver, nil
}

func (r *usageRestoreResolver) resolveProjectID(projectID int, projectName string) (int, bool) {
	if projectName != "" {
		id, ok := r.projectNames[projectName]
		return id, ok
	}

	if projectID == 0 {
		return 0, false
	}

	_, ok := r.projectIDs[projectID]
	return projectID, ok
}

func (r *usageRestoreResolver) resolveChannelID(channelID int, channelName string) (int, bool) {
	if channelName != "" {
		id, ok := r.channelNames[channelName]
		return id, ok
	}

	if channelID == 0 {
		return 0, false
	}

	_, ok := r.channelIDs[channelID]
	return channelID, ok
}

func (r *usageRestoreResolver) resolveAPIKeyID(apiKeyKey string) (int, bool) {
	if apiKeyKey != "" {
		id, ok := r.apiKeyKeys[apiKeyKey]
		return id, ok
	}

	return 0, false
}

func remapModelSettingsChannelIDs(settings *objects.ModelSettings, channelIDMap map[int]int) {
	if settings == nil || len(channelIDMap) == 0 {
		return
	}

	for _, assoc := range settings.Associations {
		if assoc == nil {
			continue
		}

		if assoc.ChannelModel != nil {
			if newID, ok := channelIDMap[assoc.ChannelModel.ChannelID]; ok {
				assoc.ChannelModel.ChannelID = newID
			}
		}

		if assoc.ChannelRegex != nil {
			if newID, ok := channelIDMap[assoc.ChannelRegex.ChannelID]; ok {
				assoc.ChannelRegex.ChannelID = newID
			}
		}

		if assoc.Regex != nil {
			remapExcludeAssociationChannelIDs(assoc.Regex.Exclude, channelIDMap)
		}

		if assoc.ModelID != nil {
			remapExcludeAssociationChannelIDs(assoc.ModelID.Exclude, channelIDMap)
		}
	}
}

func remapExcludeAssociationChannelIDs(exclude []*objects.ExcludeAssociation, channelIDMap map[int]int) {
	for _, ex := range exclude {
		if ex == nil || len(ex.ChannelIds) == 0 {
			continue
		}

		for i, oldID := range ex.ChannelIds {
			if newID, ok := channelIDMap[oldID]; ok {
				ex.ChannelIds[i] = newID
			}
		}
	}
}

func remapAPIKeyProfilesChannelIDs(profiles *objects.APIKeyProfiles, channelIDMap map[int]int) {
	if profiles == nil || len(channelIDMap) == 0 {
		return
	}

	for i := range profiles.Profiles {
		profile := &profiles.Profiles[i]
		if len(profile.ChannelIDs) == 0 {
			continue
		}

		for j, oldID := range profile.ChannelIDs {
			if newID, ok := channelIDMap[oldID]; ok {
				profile.ChannelIDs[j] = newID
			}
		}
	}
}

func remapProjectProfilesChannelIDs(profiles *objects.ProjectProfiles, channelIDMap map[int]int) {
	if profiles == nil || len(channelIDMap) == 0 {
		return
	}

	for i := range profiles.Profiles {
		profile := &profiles.Profiles[i]
		if len(profile.ChannelIDs) == 0 {
			continue
		}

		for j, oldID := range profile.ChannelIDs {
			if newID, ok := channelIDMap[oldID]; ok {
				profile.ChannelIDs[j] = newID
			}
		}
	}
}

func (svc *BackupService) restoreProjects(ctx context.Context, db *ent.Client, projectsData []*BackupProject, opts RestoreOptions) error {
	if len(projectsData) == 0 {
		return nil
	}

	for _, projData := range projectsData {
		if projData == nil {
			continue
		}

		existing, err := db.Project.Query().
			Where(project.Name(projData.Name)).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return err
		}

		if existing != nil {
			switch opts.ProjectConflictStrategy {
			case ConflictStrategySkip:
				log.Info(ctx, "skipping existing project", log.String("name", projData.Name))
				continue
			case ConflictStrategyError:
				log.Error(ctx, "project already exists", log.String("name", projData.Name))
				return fmt.Errorf("project %s already exists", projData.Name)
			case ConflictStrategyOverwrite:
				_, err = db.Project.UpdateOneID(existing.ID).
					SetName(projData.Name).
					SetDescription(projData.Description).
					SetStatus(projData.Status).
					SetProfiles(projData.Profiles).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to restore project %s: %w", projData.Name, err)
				}
			}

			continue
		}

		_, err = db.Project.Create().
			SetName(projData.Name).
			SetDescription(projData.Description).
			SetStatus(projData.Status).
			SetProfiles(projData.Profiles).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create project %s: %w", projData.Name, err)
		}
	}

	return nil
}

func (svc *BackupService) restoreChannelModelPrices(
	ctx context.Context,
	db *ent.Client,
	prices []*BackupChannelModelPrice,
	opts RestoreOptions,
) error {
	if len(prices) == 0 {
		return nil
	}

	now := time.Now()
	channelCache := map[string]*ent.Channel{}
	updatedChannels := map[int]struct{}{}

	getChannel := func(name string) (*ent.Channel, error) {
		if ch, ok := channelCache[name]; ok {
			return ch, nil
		}

		ch, err := db.Channel.Query().
			Where(channel.Name(name)).
			First(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				channelCache[name] = nil
				return nil, nil
			}

			return nil, err
		}

		channelCache[name] = ch

		return ch, nil
	}

	for _, pData := range prices {
		if pData == nil {
			continue
		}

		if err := pData.Price.Validate(); err != nil {
			return fmt.Errorf("invalid channel model price: channel=%s model_id=%s: %w", pData.ChannelName, pData.ModelID, err)
		}

		ch, err := getChannel(pData.ChannelName)
		if err != nil {
			return err
		}

		if ch == nil {
			log.Warn(ctx, "channel not found for restoring channel model price, skipping",
				log.String("channel", pData.ChannelName),
				log.String("model_id", pData.ModelID),
			)

			continue
		}

		existing, err := db.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID(pData.ModelID),
			).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return err
		}

		refID := pData.ReferenceID
		if refID == "" {
			return fmt.Errorf("channel model price reference ID is empty: channel=%s model_id=%s", pData.ChannelName, pData.ModelID)
		}

		if existing != nil {
			if existing.ReferenceID == refID && existing.Price.Equals(pData.Price) {
				continue
			}

			switch opts.ModelPriceConflictStrategy {
			case ConflictStrategySkip:
				continue
			case ConflictStrategyError:
				return fmt.Errorf("channel model price already exists: channel=%s model_id=%s", pData.ChannelName, pData.ModelID)
			case ConflictStrategyOverwrite:
				// Archive old versions
				_, err = db.ChannelModelPriceVersion.Update().
					Where(
						channelmodelpriceversion.ChannelModelPriceIDEQ(existing.ID),
						channelmodelpriceversion.StatusEQ(channelmodelpriceversion.StatusActive),
					).
					SetStatus(channelmodelpriceversion.StatusArchived).
					SetEffectiveEndAt(now).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to archive old channel model price versions: %w", err)
				}

				if _, err := db.ChannelModelPrice.UpdateOneID(existing.ID).
					SetPrice(pData.Price).
					SetReferenceID(refID).
					Save(ctx); err != nil {
					return fmt.Errorf("failed to restore channel model price: channel=%s model_id=%s: %w", pData.ChannelName, pData.ModelID, err)
				}

				// Create new version
				_, err = db.ChannelModelPriceVersion.Create().
					SetChannelID(ch.ID).
					SetModelID(pData.ModelID).
					SetChannelModelPriceID(existing.ID).
					SetPrice(pData.Price).
					SetStatus(channelmodelpriceversion.StatusActive).
					SetEffectiveStartAt(now).
					SetReferenceID(refID).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to create channel model price version: %w", err)
				}

				updatedChannels[ch.ID] = struct{}{}
			}

			continue
		}

		entity, err := db.ChannelModelPrice.Create().
			SetChannelID(ch.ID).
			SetModelID(pData.ModelID).
			SetPrice(pData.Price).
			SetReferenceID(refID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create channel model price: channel=%s model_id=%s: %w", pData.ChannelName, pData.ModelID, err)
		}

		// Create new version
		_, err = db.ChannelModelPriceVersion.Create().
			SetChannelID(ch.ID).
			SetModelID(pData.ModelID).
			SetChannelModelPriceID(entity.ID).
			SetPrice(pData.Price).
			SetStatus(channelmodelpriceversion.StatusActive).
			SetEffectiveStartAt(now).
			SetReferenceID(refID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create channel model price version: %w", err)
		}

		updatedChannels[ch.ID] = struct{}{}
	}

	// Force update channel updated_at to trigger reload cache.
	for chID := range updatedChannels {
		if err := db.Channel.UpdateOneID(chID).
			SetUpdatedAt(now).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to update channel updated_at: %w", err)
		}
	}

	return nil
}

func (svc *BackupService) restoreChannels(ctx context.Context, db *ent.Client, channels []*BackupChannel, opts RestoreOptions) error {
	for _, chData := range channels {
		existing, err := db.Channel.Query().
			Where(channel.Name(chData.Name)).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return err
		}

		credentials := chData.Credentials
		// Check if credentials are empty (no API key and no OAuth)
		if credentials.APIKey == "" && len(credentials.APIKeys) == 0 && credentials.OAuth == nil {
			continue
		}

		var baseURL *string
		if chData.BaseURL != "" {
			baseURL = &chData.BaseURL
		}

		if existing != nil {
			switch opts.ChannelConflictStrategy {
			case ConflictStrategySkip:
				log.Info(ctx, "skipping existing channel", log.String("channel", chData.Name))
				continue
			case ConflictStrategyError:
				log.Error(ctx, "channel already exists",
					log.String("channel", chData.Name))

				return fmt.Errorf("channel %s already exists", chData.Name)
			case ConflictStrategyOverwrite:
				update := db.Channel.UpdateOneID(existing.ID).
					SetNillableBaseURL(baseURL).
					SetStatus(chData.Status).
					SetCredentials(credentials).
					SetSupportedModels(chData.SupportedModels).
					SetNillableAutoSyncSupportedModels(lo.ToPtr(chData.AutoSyncSupportedModels)).
					SetAutoSyncModelPattern(chData.AutoSyncModelPattern).
					SetManualModels(chData.ManualModels).
					SetTags(chData.Tags).
					SetDefaultTestModel(chData.DefaultTestModel).
					SetSettings(chData.Settings).
					SetOrderingWeight(chData.OrderingWeight)

				if chData.Remark != nil {
					update.SetRemark(*chData.Remark)
				} else {
					update.ClearRemark()
				}

				if _, err := update.Save(ctx); err != nil {
					log.Error(ctx, "failed to restore channel",
						log.String("channel", chData.Name),
						log.Cause(err))

					return fmt.Errorf("failed to restore channel %s: %w", chData.Name, err)
				}
			}
		} else {
			create := db.Channel.Create().
				SetName(chData.Name).
				SetType(chData.Type).
				SetNillableBaseURL(baseURL).
				SetStatus(chData.Status).
				SetCredentials(credentials).
				SetSupportedModels(chData.SupportedModels).
				SetNillableAutoSyncSupportedModels(lo.ToPtr(chData.AutoSyncSupportedModels)).
				SetAutoSyncModelPattern(chData.AutoSyncModelPattern).
				SetManualModels(chData.ManualModels).
				SetTags(chData.Tags).
				SetDefaultTestModel(chData.DefaultTestModel).
				SetSettings(chData.Settings).
				SetOrderingWeight(chData.OrderingWeight)

			if chData.Remark != nil {
				create.SetRemark(*chData.Remark)
			}

			if _, err := create.Save(ctx); err != nil {
				log.Error(ctx, "failed to create channel",
					log.String("channel", chData.Name),
					log.Cause(err))

				return fmt.Errorf("failed to create channel %s: %w", chData.Name, err)
			}
		}
	}

	return nil
}

func (svc *BackupService) restoreModels(ctx context.Context, db *ent.Client, models []*BackupModel, opts RestoreOptions, channelIDMap map[int]int) error {
	for _, modelData := range models {
		if modelData == nil {
			continue
		}

		remapModelSettingsChannelIDs(modelData.Settings, channelIDMap)

		existing, err := db.Model.Query().
			Where(
				model.Developer(modelData.Developer),
				model.ModelID(modelData.ModelID),
			).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return err
		}

		if existing != nil {
			switch opts.ModelConflictStrategy {
			case ConflictStrategySkip:
				log.Info(ctx, "skipping existing model", log.String("model", modelData.ModelID))
				continue
			case ConflictStrategyError:
				log.Error(ctx, "model already exists",
					log.String("model", modelData.ModelID))

				return fmt.Errorf("model %s already exists", modelData.ModelID)
			case ConflictStrategyOverwrite:
				update := db.Model.UpdateOneID(existing.ID).
					SetName(modelData.Name).
					SetIcon(modelData.Icon).
					SetGroup(modelData.Group).
					SetModelCard(modelData.ModelCard).
					SetSettings(modelData.Settings).
					SetStatus(modelData.Status)

				if modelData.Remark != nil {
					update.SetRemark(*modelData.Remark)
				} else {
					update.ClearRemark()
				}

				if _, err := update.Save(ctx); err != nil {
					log.Error(ctx, "failed to restore model",
						log.String("model", modelData.ModelID),
						log.Cause(err))

					return fmt.Errorf("failed to restore model %s: %w", modelData.ModelID, err)
				}
			}
		} else {
			create := db.Model.Create().
				SetDeveloper(modelData.Developer).
				SetModelID(modelData.ModelID).
				SetType(modelData.Type).
				SetName(modelData.Name).
				SetIcon(modelData.Icon).
				SetGroup(modelData.Group).
				SetModelCard(modelData.ModelCard).
				SetSettings(modelData.Settings).
				SetStatus(modelData.Status)

			if modelData.Remark != nil {
				create.SetRemark(*modelData.Remark)
			}

			if _, err := create.Save(ctx); err != nil {
				log.Error(ctx, "failed to create model",
					log.String("model", modelData.ModelID),
					log.Cause(err))

				return fmt.Errorf("failed to create model %s: %w", modelData.ModelID, err)
			}
		}
	}

	return nil
}

func (svc *BackupService) restoreAPIKeys(ctx context.Context, db *ent.Client, apiKeys []*BackupAPIKey, opts RestoreOptions, channelIDMap map[int]int) error {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return fmt.Errorf("user not found in context for restoring API keys")
	}

	for _, akData := range apiKeys {
		if akData == nil {
			continue
		}

		remapAPIKeyProfilesChannelIDs(akData.Profiles, channelIDMap)

		existing, err := db.APIKey.Query().
			Where(apikey.Key(akData.Key)).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return err
		}

		if existing != nil {
			switch opts.APIKeyConflictStrategy {
			case ConflictStrategySkip:
				log.Info(ctx, "skipping existing API key", log.String("name", akData.Name))
				continue
			case ConflictStrategyError:
				log.Error(ctx, "API key already exists",
					log.String("name", akData.Name))

				return fmt.Errorf("API key %s already exists", akData.Name)
			case ConflictStrategyOverwrite:
				update := db.APIKey.UpdateOneID(existing.ID).
					SetName(akData.Name).
					SetType(akData.Type).
					SetStatus(akData.Status).
					SetScopes(akData.Scopes).
					SetProfiles(akData.Profiles)

				if _, err := update.Save(ctx); err != nil {
					log.Error(ctx, "failed to restore API key",
						log.String("name", akData.Name),
						log.Cause(err))

					return fmt.Errorf("failed to restore API key %s: %w", akData.Name, err)
				}
			}
		} else {
			projectName := akData.ProjectName
			if projectName == "" {
				projectName = "Default"
			}

			proj, err := db.Project.Query().
				Where(project.Name(projectName)).
				First(ctx)
			if err != nil {
				if ent.IsNotFound(err) {
					log.Warn(ctx, "project not found, skipping API key",
						log.String("project", projectName),
						log.String("api_key", akData.Name))

					continue
				}

				return err
			}

			create := db.APIKey.Create().
				SetKey(akData.Key).
				SetName(akData.Name).
				SetType(akData.Type).
				SetStatus(akData.Status).
				SetScopes(akData.Scopes).
				SetProfiles(akData.Profiles).
				SetUserID(user.ID).
				SetProjectID(proj.ID)

			if _, err := create.Save(ctx); err != nil {
				log.Error(ctx, "failed to create API key",
					log.String("name", akData.Name),
					log.Cause(err))

				return fmt.Errorf("failed to create API key %s: %w", akData.Name, err)
			}
		}
	}

	return nil
}

func (svc *BackupService) restoreUsageData(
	ctx context.Context,
	db *ent.Client,
	requestsData []*BackupUsageRequest,
	usageLogs []*BackupUsageLog,
	opts RestoreOptions,
) error {
	resolver, err := newUsageRestoreResolver(ctx, db)
	if err != nil {
		return err
	}

	requestIDMap := map[int]int{}
	if opts.IncludeRequestLogs {
		requestIDMap, err = svc.restoreUsageRequests(ctx, db, requestsData, resolver)
		if err != nil {
			return err
		}
	}

	if opts.IncludeUsageStats {
		return svc.restoreUsageLogs(ctx, db, usageLogs, requestIDMap, resolver)
	}

	return nil
}

func (svc *BackupService) restoreUsageRequests(
	ctx context.Context,
	db *ent.Client,
	requestsData []*BackupUsageRequest,
	resolver *usageRestoreResolver,
) (map[int]int, error) {
	idMap := map[int]int{}
	if len(requestsData) == 0 {
		return idMap, nil
	}

	existingRequests, err := existingUsageRequests(ctx, db, requestsData)
	if err != nil {
		return nil, err
	}

	for _, reqData := range requestsData {
		if reqData == nil {
			continue
		}

		oldID := reqData.ID
		if oldID == 0 {
			continue
		}

		projectID, ok := resolver.resolveProjectID(reqData.ProjectID, reqData.ProjectName)
		if !ok {
			log.Warn(ctx, "project not found for restoring usage request, skipping",
				log.Int("request_id", oldID),
				log.String("project", reqData.ProjectName),
			)
			continue
		}

		channelID, ok := resolver.resolveChannelID(reqData.ChannelID, reqData.ChannelName)
		if !ok && hasBackupChannelRef(reqData.ChannelID, reqData.ChannelName) {
			log.Warn(ctx, "channel not found for restoring usage request, restoring with null channel",
				log.Int("request_id", oldID),
				log.Int("channel_id", reqData.ChannelID),
				log.String("channel", reqData.ChannelName),
			)
		}

		apiKeyID, ok := resolver.resolveAPIKeyID(reqData.APIKeyKey)
		if !ok && reqData.APIKeyKey != "" {
			log.Warn(ctx, "API key not found for restoring usage request, restoring with null API key",
				log.Int("request_id", oldID),
			)
		}

		if existing, ok := existingRequests.byID[oldID]; ok {
			if sameUsageRequest(existing, reqData, projectID, channelID, apiKeyID) {
				idMap[oldID] = existing.ID
				continue
			}
		}
		if existing, ok := existingRequests.byFingerprint[usageRequestBackupFingerprint(reqData)]; ok {
			idMap[oldID] = existing.ID
			continue
		}

		created, err := db.Request.Create().
			SetCreatedAt(reqData.CreatedAt).
			SetUpdatedAt(reqData.UpdatedAt).
			SetProjectID(projectID).
			SetSource(reqData.Source).
			SetModelID(reqData.ModelID).
			SetFormat(reqData.Format).
			SetRequestBody(reqData.RequestBody).
			SetStatus(reqData.Status).
			SetStream(reqData.Stream).
			SetClientIP(reqData.ClientIP).
			SetContentSaved(reqData.ContentSaved).
			SetNillableAPIKeyID(nilIfZero(apiKeyID)).
			SetNillableChannelID(nilIfZero(channelID)).
			SetNillableReasoningEffort(nilIfEmpty(reqData.ReasoningEffort)).
			SetRequestHeaders(reqData.RequestHeaders).
			SetResponseBody(reqData.ResponseBody).
			SetResponseChunks(reqData.ResponseChunks).
			SetNillableExternalID(nilIfEmpty(reqData.ExternalID)).
			SetNillableMetricsLatencyMs(reqData.MetricsLatencyMs).
			SetNillableMetricsFirstTokenLatencyMs(reqData.MetricsFirstTokenLatencyMs).
			SetNillableMetricsReasoningDurationMs(reqData.MetricsReasoningDurationMs).
			SetNillableContentStorageID(reqData.ContentStorageID).
			SetNillableContentStorageKey(reqData.ContentStorageKey).
			SetNillableContentSavedAt(reqData.ContentSavedAt).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to restore usage request %d: %w", oldID, err)
		}

		idMap[oldID] = created.ID
	}

	return idMap, nil
}

func hasBackupChannelRef(channelID int, channelName string) bool {
	return channelID != 0 || channelName != ""
}

type existingUsageRequestLookup struct {
	byID          map[int]*ent.Request
	byFingerprint map[string]*ent.Request
}

func existingUsageRequests(
	ctx context.Context,
	db *ent.Client,
	requestsData []*BackupUsageRequest,
) (*existingUsageRequestLookup, error) {
	ids := make([]int, 0, len(requestsData))
	createdAt := make([]time.Time, 0, len(requestsData))
	createdAtSeen := map[time.Time]struct{}{}
	for _, reqData := range requestsData {
		if reqData == nil {
			continue
		}

		if reqData.ID != 0 {
			ids = append(ids, reqData.ID)
		}

		if !reqData.CreatedAt.IsZero() {
			if _, ok := createdAtSeen[reqData.CreatedAt]; ok {
				continue
			}
			createdAtSeen[reqData.CreatedAt] = struct{}{}
			createdAt = append(createdAt, reqData.CreatedAt)
		}
	}

	lookup := &existingUsageRequestLookup{
		byID:          map[int]*ent.Request{},
		byFingerprint: map[string]*ent.Request{},
	}
	for start := 0; start < len(ids); start += usageBackupBatchSize {
		end := min(start+usageBackupBatchSize, len(ids))
		requests, err := db.Request.Query().
			Where(request.IDIn(ids[start:end]...)).
			WithProject().
			WithChannel().
			WithAPIKey().
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, req := range requests {
			addExistingUsageRequest(lookup, req)
		}
	}

	for start := 0; start < len(createdAt); start += usageBackupBatchSize {
		end := min(start+usageBackupBatchSize, len(createdAt))
		requests, err := db.Request.Query().
			Where(request.CreatedAtIn(createdAt[start:end]...)).
			WithProject().
			WithChannel().
			WithAPIKey().
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, req := range requests {
			addExistingUsageRequest(lookup, req)
		}
	}

	return lookup, nil
}

func addExistingUsageRequest(lookup *existingUsageRequestLookup, req *ent.Request) {
	lookup.byID[req.ID] = req
	lookup.byFingerprint[usageRequestExistingFingerprint(req, true)] = req
	lookup.byFingerprint[usageRequestExistingFingerprint(req, false)] = req
}

func usageRequestBackupFingerprint(req *BackupUsageRequest) string {
	return usageRequestFingerprint(
		req.CreatedAt,
		req.ModelID,
		req.Format,
		string(req.Source),
		string(req.Status),
		req.Stream,
		req.ClientIP,
		req.ExternalID,
		req.ReasoningEffort,
		req.ProjectName,
		req.ChannelName,
		req.APIKeyKey,
	)
}

func usageRequestExistingFingerprint(req *ent.Request, includeAPIKey bool) string {
	projectName := ""
	if req.Edges.Project != nil {
		projectName = req.Edges.Project.Name
	}

	channelName := ""
	if req.Edges.Channel != nil {
		channelName = req.Edges.Channel.Name
	}

	apiKeyKey := ""
	if includeAPIKey && req.Edges.APIKey != nil {
		apiKeyKey = req.Edges.APIKey.Key
	}

	return usageRequestFingerprint(
		req.CreatedAt,
		req.ModelID,
		req.Format,
		string(req.Source),
		string(req.Status),
		req.Stream,
		req.ClientIP,
		req.ExternalID,
		req.ReasoningEffort,
		projectName,
		channelName,
		apiKeyKey,
	)
}

func usageRequestFingerprint(
	createdAt time.Time,
	modelID string,
	format string,
	source string,
	status string,
	stream bool,
	clientIP string,
	externalID string,
	reasoningEffort string,
	projectName string,
	channelName string,
	apiKeyKey string,
) string {
	parts := []string{
		createdAt.UTC().Format(time.RFC3339Nano),
		modelID,
		format,
		source,
		status,
		fmt.Sprintf("%t", stream),
		clientIP,
		externalID,
		reasoningEffort,
		projectName,
		channelName,
		apiKeyKey,
	}

	return strings.Join(parts, "\x00")
}

func sameUsageRequest(existing *ent.Request, backup *BackupUsageRequest, projectID, channelID, apiKeyID int) bool {
	apiKeyMatches := backup.APIKeyKey == "" || existing.APIKeyID == apiKeyID

	return existing.ProjectID == projectID &&
		existing.ChannelID == channelID &&
		apiKeyMatches &&
		existing.ModelID == backup.ModelID &&
		existing.Format == backup.Format &&
		existing.Source == backup.Source &&
		existing.Status == backup.Status &&
		existing.Stream == backup.Stream &&
		existing.ClientIP == backup.ClientIP &&
		existing.ExternalID == backup.ExternalID &&
		existing.ReasoningEffort == backup.ReasoningEffort &&
		existing.CreatedAt.Equal(backup.CreatedAt)
}

func (svc *BackupService) restoreUsageLogs(
	ctx context.Context,
	db *ent.Client,
	usageLogs []*BackupUsageLog,
	requestIDMap map[int]int,
	resolver *usageRestoreResolver,
) error {
	if len(usageLogs) == 0 {
		return nil
	}

	if err := svc.ensureUsageLogRequests(ctx, db, usageLogs, requestIDMap, resolver); err != nil {
		return err
	}

	requestIDs := make([]int, 0, len(requestIDMap))
	for _, requestID := range requestIDMap {
		requestIDs = append(requestIDs, requestID)
	}

	existingLogRequestIDs := map[int]struct{}{}
	for start := 0; start < len(requestIDs); start += usageBackupBatchSize {
		end := min(start+usageBackupBatchSize, len(requestIDs))
		logs, err := db.UsageLog.Query().
			Where(usagelog.RequestIDIn(requestIDs[start:end]...)).
			Select(usagelog.FieldRequestID).
			All(ctx)
		if err != nil {
			return err
		}

		for _, usageLog := range logs {
			existingLogRequestIDs[usageLog.RequestID] = struct{}{}
		}
	}

	restoredLogRequestIDs := map[int]struct{}{}
	builders := make([]*ent.UsageLogCreate, 0, min(len(usageLogs), usageBackupBatchSize))
	flush := func() error {
		if len(builders) == 0 {
			return nil
		}

		if _, err := db.UsageLog.CreateBulk(builders...).Save(ctx); err != nil {
			return fmt.Errorf("failed to restore usage logs: %w", err)
		}

		builders = builders[:0]

		return nil
	}

	for _, usageData := range usageLogs {
		if usageData == nil {
			continue
		}

		requestID, ok := requestIDMap[usageData.RequestID]
		if !ok {
			log.Warn(ctx, "request not found for restoring usage log, skipping",
				log.Int("usage_log_id", usageData.ID),
				log.Int("request_id", usageData.RequestID),
			)
			continue
		}

		if _, existing := existingLogRequestIDs[requestID]; existing {
			log.Warn(ctx, "usage log already exists for request, skipping",
				log.Int("usage_log_id", usageData.ID),
				log.Int("request_id", usageData.RequestID),
			)
			continue
		}

		if _, duplicate := restoredLogRequestIDs[requestID]; duplicate {
			log.Warn(ctx, "duplicate usage log for request in backup, skipping",
				log.Int("usage_log_id", usageData.ID),
				log.Int("request_id", usageData.RequestID),
			)
			continue
		}

		projectID, ok := resolver.resolveProjectID(usageData.ProjectID, usageData.ProjectName)
		if !ok {
			log.Warn(ctx, "project not found for restoring usage log, skipping",
				log.Int("usage_log_id", usageData.ID),
				log.String("project", usageData.ProjectName),
			)
			continue
		}

		channelID, ok := resolver.resolveChannelID(usageData.ChannelID, usageData.ChannelName)
		if !ok && hasBackupChannelRef(usageData.ChannelID, usageData.ChannelName) {
			log.Warn(ctx, "channel not found for restoring usage log, restoring with null channel",
				log.Int("usage_log_id", usageData.ID),
				log.Int("channel_id", usageData.ChannelID),
				log.String("channel", usageData.ChannelName),
			)
		}

		apiKeyID, ok := resolver.resolveAPIKeyID(usageData.APIKeyKey)
		if !ok && usageData.APIKeyKey != "" {
			log.Warn(ctx, "API key not found for restoring usage log, restoring with null API key",
				log.Int("usage_log_id", usageData.ID),
			)
		}

		builders = append(builders, db.UsageLog.Create().
			SetCreatedAt(usageData.CreatedAt).
			SetUpdatedAt(usageData.UpdatedAt).
			SetRequestID(requestID).
			SetNillableAPIKeyID(nilIfZero(apiKeyID)).
			SetProjectID(projectID).
			SetNillableChannelID(nilIfZero(channelID)).
			SetModelID(usageData.ModelID).
			SetPromptTokens(usageData.PromptTokens).
			SetCompletionTokens(usageData.CompletionTokens).
			SetTotalTokens(usageData.TotalTokens).
			SetPromptAudioTokens(usageData.PromptAudioTokens).
			SetPromptCachedTokens(usageData.PromptCachedTokens).
			SetPromptWriteCachedTokens(usageData.PromptWriteCachedTokens).
			SetPromptWriteCachedTokens5m(usageData.PromptWriteCachedTokens5m).
			SetPromptWriteCachedTokens1h(usageData.PromptWriteCachedTokens1h).
			SetCompletionAudioTokens(usageData.CompletionAudioTokens).
			SetCompletionReasoningTokens(usageData.CompletionReasoningTokens).
			SetCompletionAcceptedPredictionTokens(usageData.CompletionAcceptedPredictionTokens).
			SetCompletionRejectedPredictionTokens(usageData.CompletionRejectedPredictionTokens).
			SetSource(usageData.Source).
			SetFormat(usageData.Format).
			SetNillableTotalCost(usageData.TotalCost).
			SetCostItems(usageData.CostItems).
			SetNillableCostPriceReferenceID(nilIfEmpty(usageData.CostPriceReferenceID)))
		restoredLogRequestIDs[requestID] = struct{}{}

		if len(builders) >= usageBackupBatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	return flush()
}

func (svc *BackupService) ensureUsageLogRequests(
	ctx context.Context,
	db *ent.Client,
	usageLogs []*BackupUsageLog,
	requestIDMap map[int]int,
	resolver *usageRestoreResolver,
) error {
	shellRequests := usageLogRequestShells(usageLogs, requestIDMap)
	existingRequests, err := existingUsageRequests(ctx, db, shellRequests)
	if err != nil {
		return err
	}

	for _, usageData := range usageLogs {
		if usageData == nil || usageData.RequestID == 0 {
			continue
		}

		if _, ok := requestIDMap[usageData.RequestID]; ok {
			continue
		}

		projectID, ok := resolver.resolveProjectID(usageData.ProjectID, usageData.ProjectName)
		if !ok {
			continue
		}

		channelID, ok := resolver.resolveChannelID(usageData.ChannelID, usageData.ChannelName)
		if !ok && hasBackupChannelRef(usageData.ChannelID, usageData.ChannelName) {
			log.Warn(ctx, "channel not found for restoring usage log request shell, restoring with null channel",
				log.Int("usage_log_id", usageData.ID),
				log.Int("channel_id", usageData.ChannelID),
				log.String("channel", usageData.ChannelName),
			)
		}

		apiKeyID, ok := resolver.resolveAPIKeyID(usageData.APIKeyKey)
		if !ok && usageData.APIKeyKey != "" {
			log.Warn(ctx, "API key not found for restoring usage log request shell, restoring with null API key",
				log.Int("usage_log_id", usageData.ID),
			)
		}

		shellData := usageLogRequestShell(usageData)
		if existing, ok := existingRequests.byID[usageData.RequestID]; ok {
			if sameUsageRequest(existing, shellData, projectID, channelID, apiKeyID) {
				requestIDMap[usageData.RequestID] = existing.ID
				continue
			}
		}
		if existing, ok := existingRequests.byFingerprint[usageRequestBackupFingerprint(shellData)]; ok {
			requestIDMap[usageData.RequestID] = existing.ID
			continue
		}
		if apiKeyID == 0 {
			shellData.APIKeyKey = ""
			if existing, ok := existingRequests.byFingerprint[usageRequestBackupFingerprint(shellData)]; ok {
				requestIDMap[usageData.RequestID] = existing.ID
				continue
			}
		}

		created, err := db.Request.Create().
			SetCreatedAt(usageData.CreatedAt).
			SetUpdatedAt(usageData.UpdatedAt).
			SetProjectID(projectID).
			SetSource(request.Source(usageData.Source)).
			SetModelID(usageData.ModelID).
			SetFormat(usageData.Format).
			SetRequestBody(objects.JSONRawMessage("{}")).
			SetStatus(request.StatusCompleted).
			SetStream(false).
			SetClientIP("").
			SetNillableAPIKeyID(nilIfZero(apiKeyID)).
			SetNillableChannelID(nilIfZero(channelID)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create usage log request shell %d: %w", usageData.RequestID, err)
		}

		requestIDMap[usageData.RequestID] = created.ID
	}

	return nil
}

func usageLogRequestShells(
	usageLogs []*BackupUsageLog,
	requestIDMap map[int]int,
) []*BackupUsageRequest {
	shells := make([]*BackupUsageRequest, 0, len(usageLogs))
	seen := map[int]struct{}{}
	for _, usageData := range usageLogs {
		if usageData == nil || usageData.RequestID == 0 {
			continue
		}
		if _, ok := requestIDMap[usageData.RequestID]; ok {
			continue
		}
		if _, ok := seen[usageData.RequestID]; ok {
			continue
		}

		seen[usageData.RequestID] = struct{}{}
		shells = append(shells, usageLogRequestShell(usageData))
	}

	return shells
}

func usageLogRequestShell(usageData *BackupUsageLog) *BackupUsageRequest {
	return &BackupUsageRequest{
		Request: ent.Request{
			ID:          usageData.RequestID,
			CreatedAt:   usageData.CreatedAt,
			UpdatedAt:   usageData.UpdatedAt,
			Source:      request.Source(usageData.Source),
			ModelID:     usageData.ModelID,
			Format:      usageData.Format,
			RequestBody: objects.JSONRawMessage("{}"),
			Status:      request.StatusCompleted,
			Stream:      false,
			ClientIP:    "",
		},
		ProjectName: usageData.ProjectName,
		ChannelName: usageData.ChannelName,
		APIKeyKey:   usageData.APIKeyKey,
	}
}

func nilIfZero(v int) *int {
	if v == 0 {
		return nil
	}

	return &v
}

func nilIfEmpty(v string) *string {
	if v == "" {
		return nil
	}

	return &v
}
