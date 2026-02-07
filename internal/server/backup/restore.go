package backup

import (
	"context"
	"encoding/json"
	"fmt"
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

	if !lo.Contains([]string{BackupVersion, BackupVersionV1}, backupData.Version) {
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

	if opts.IncludeAPIKeys {
		if err := svc.restoreAPIKeys(ctx, db, backupData.APIKeys, opts, channelIDMap); err != nil {
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
