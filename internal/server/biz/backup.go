package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

type BackupServiceParams struct {
	fx.In

	ChannelService *ChannelService
	ModelService   *ModelService
	Ent            *ent.Client
}

func NewBackupService(params BackupServiceParams) *BackupService {
	return &BackupService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		channelService: params.ChannelService,
		modelService:   params.ModelService,
	}
}

type BackupService struct {
	*AbstractService

	channelService *ChannelService
	modelService   *ModelService
}

type BackupData struct {
	Version            string                     `json:"version"`
	Timestamp          time.Time                  `json:"timestamp"`
	Channels           []*BackupChannel           `json:"channels"`
	Models             []*BackupModel             `json:"models"`
	ChannelModelPrices []*BackupChannelModelPrice `json:"channel_model_prices,omitempty"`
	APIKeys            []*BackupAPIKey            `json:"api_keys,omitempty"`
}

type BackupChannel struct {
	ent.Channel

	Credentials *objects.ChannelCredentials `json:"credentials,omitempty"`
}

type BackupModel struct {
	ent.Model
}

type BackupAPIKey struct {
	ent.APIKey

	ProjectName string `json:"project_name"`
}

type BackupChannelModelPrice struct {
	ChannelName string             `json:"channel_name"`
	ModelID     string             `json:"model_id"`
	Price       objects.ModelPrice `json:"price"`
	ReferenceID string             `json:"reference_id"`
}

const (
	BackupVersion   = "1.1"
	BackupVersionV1 = "1.0"
)

type BackupOptions struct {
	IncludeChannels    bool
	IncludeModels      bool
	IncludeAPIKeys     bool
	IncludeModelPrices bool
}

type ConflictStrategy string

const (
	ConflictStrategySkip      ConflictStrategy = "skip"
	ConflictStrategyOverwrite ConflictStrategy = "overwrite"
	ConflictStrategyError     ConflictStrategy = "error"
)

type RestoreOptions struct {
	IncludeChannels         bool
	IncludeModels           bool
	IncludeAPIKeys          bool
	IncludeModelPrices      bool
	ChannelConflictStrategy ConflictStrategy
	ModelConflictStrategy   ConflictStrategy
	APIKeyConflictStrategy  ConflictStrategy
}

func (svc *BackupService) Backup(ctx context.Context, opts BackupOptions) ([]byte, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	if !user.IsOwner {
		return nil, fmt.Errorf("only owners can perform backup operations")
	}

	var (
		channelDataList           []*BackupChannel
		channelModelPriceDataList []*BackupChannelModelPrice
	)

	if opts.IncludeChannels {
		channels, err := svc.entFromContext(ctx).Channel.Query().All(ctx)
		if err != nil {
			return nil, err
		}

		channelDataList = lo.Map(channels, func(ch *ent.Channel, _ int) *BackupChannel {
			return &BackupChannel{
				Channel:     *ch,
				Credentials: ch.Credentials,
			}
		})
	}

	if opts.IncludeModelPrices {
		prices, err := svc.entFromContext(ctx).ChannelModelPrice.Query().
			WithChannel().
			All(ctx)
		if err != nil {
			return nil, err
		}

		channelModelPriceDataList = lo.FilterMap(prices, func(p *ent.ChannelModelPrice, _ int) (*BackupChannelModelPrice, bool) {
			if p.Edges.Channel == nil {
				return nil, false
			}

			return &BackupChannelModelPrice{
				ChannelName: p.Edges.Channel.Name,
				ModelID:     p.ModelID,
				Price:       p.Price,
				ReferenceID: p.ReferenceID,
			}, true
		})
	}

	var modelDataList []*BackupModel

	if opts.IncludeModels {
		models, err := svc.entFromContext(ctx).Model.Query().All(ctx)
		if err != nil {
			return nil, err
		}

		modelDataList = lo.Map(models, func(m *ent.Model, _ int) *BackupModel {
			return &BackupModel{
				Model: *m,
			}
		})
	}

	var apiKeyDataList []*BackupAPIKey

	if opts.IncludeAPIKeys {
		apiKeys, err := svc.entFromContext(ctx).APIKey.Query().WithProject().All(ctx)
		if err != nil {
			return nil, err
		}

		apiKeyDataList = lo.Map(apiKeys, func(ak *ent.APIKey, _ int) *BackupAPIKey {
			projectName := ""
			if ak.Edges.Project != nil {
				projectName = ak.Edges.Project.Name
			}

			return &BackupAPIKey{
				APIKey:      *ak,
				ProjectName: projectName,
			}
		})
	}

	backupData := &BackupData{
		Version:            BackupVersion,
		Timestamp:          time.Now(),
		Channels:           channelDataList,
		Models:             modelDataList,
		ChannelModelPrices: channelModelPriceDataList,
		APIKeys:            apiKeyDataList,
	}

	return json.MarshalIndent(backupData, "", "  ")
}

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

	return svc.RunInTransaction(ctx, func(ctx context.Context) error {
		return svc.restore(ctx, backupData, opts)
	})
}

func (svc *BackupService) restore(ctx context.Context, backupData BackupData, opts RestoreOptions) error {
	if opts.IncludeChannels {
		if err := svc.restoreChannels(ctx, backupData.Channels, opts); err != nil {
			return err
		}
	}

	if opts.IncludeModelPrices {
		if err := svc.restoreChannelModelPrices(ctx, backupData.ChannelModelPrices, opts); err != nil {
			return err
		}
	}

	if opts.IncludeModels {
		if err := svc.restoreModels(ctx, backupData.Models, opts); err != nil {
			return err
		}
	}

	if opts.IncludeAPIKeys {
		if err := svc.restoreAPIKeys(ctx, backupData.APIKeys, opts); err != nil {
			return err
		}
	}

	svc.channelService.asyncReloadChannels()

	return nil
}

func (svc *BackupService) restoreChannelModelPrices(
	ctx context.Context,
	prices []*BackupChannelModelPrice,
	opts RestoreOptions,
) error {
	if len(prices) == 0 {
		return nil
	}

	db := svc.entFromContext(ctx)
	channelCache := map[string]*ent.Channel{}

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
			refID = generateReferenceID()
		}

		if existing != nil {
			switch opts.ChannelConflictStrategy {
			case ConflictStrategySkip:
				continue
			case ConflictStrategyError:
				return fmt.Errorf("channel model price already exists: channel=%s model_id=%s", pData.ChannelName, pData.ModelID)
			case ConflictStrategyOverwrite:
				if _, err := db.ChannelModelPrice.UpdateOneID(existing.ID).
					SetPrice(pData.Price).
					SetReferenceID(refID).
					Save(ctx); err != nil {
					return fmt.Errorf("failed to restore channel model price: channel=%s model_id=%s: %w", pData.ChannelName, pData.ModelID, err)
				}
			}

			continue
		}

		if _, err := db.ChannelModelPrice.Create().
			SetChannelID(ch.ID).
			SetModelID(pData.ModelID).
			SetPrice(pData.Price).
			SetReferenceID(refID).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to create channel model price: channel=%s model_id=%s: %w", pData.ChannelName, pData.ModelID, err)
		}
	}

	return nil
}

func (svc *BackupService) restoreChannels(ctx context.Context, channels []*BackupChannel, opts RestoreOptions) error {
	for _, chData := range channels {
		existing, err := svc.entFromContext(ctx).Channel.Query().
			Where(channel.Name(chData.Name)).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return err
		}

		credentials := chData.Credentials
		if credentials == nil {
			continue
		}

		var baseURL *string
		if chData.BaseURL != "" {
			baseURL = &chData.BaseURL
		}

		autoSync := &chData.AutoSyncSupportedModels

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
				update := svc.entFromContext(ctx).Channel.UpdateOneID(existing.ID).
					SetNillableBaseURL(baseURL).
					SetStatus(chData.Status).
					SetCredentials(credentials).
					SetSupportedModels(chData.SupportedModels).
					SetNillableAutoSyncSupportedModels(autoSync).
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
			create := svc.entFromContext(ctx).Channel.Create().
				SetName(chData.Name).
				SetType(chData.Type).
				SetNillableBaseURL(baseURL).
				SetStatus(chData.Status).
				SetCredentials(credentials).
				SetSupportedModels(chData.SupportedModels).
				SetNillableAutoSyncSupportedModels(autoSync).
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

func (svc *BackupService) restoreModels(ctx context.Context, models []*BackupModel, opts RestoreOptions) error {
	for _, modelData := range models {
		existing, err := svc.entFromContext(ctx).Model.Query().
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
				update := svc.entFromContext(ctx).Model.UpdateOneID(existing.ID).
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
			create := svc.entFromContext(ctx).Model.Create().
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

func (svc *BackupService) restoreAPIKeys(ctx context.Context, apiKeys []*BackupAPIKey, opts RestoreOptions) error {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return fmt.Errorf("user not found in context for restoring API keys")
	}

	for _, akData := range apiKeys {
		existing, err := svc.entFromContext(ctx).APIKey.Query().
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
				update := svc.entFromContext(ctx).APIKey.UpdateOneID(existing.ID).
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

			proj, err := svc.entFromContext(ctx).Project.Query().
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

			create := svc.entFromContext(ctx).APIKey.Create().
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
