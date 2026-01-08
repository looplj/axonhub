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
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
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
	Version   string           `json:"version"`
	Timestamp time.Time        `json:"timestamp"`
	Channels  []*BackupChannel `json:"channels"`
	Models    []*BackupModel   `json:"models"`
}

type BackupChannel struct {
	ent.Channel

	Credentials objects.ChannelCredentials `json:"credentials"`
}

type BackupModel struct {
	ent.Model
}

const BackupVersion = "1.0"

type BackupOptions struct {
	IncludeChannels bool
	IncludeModels   bool
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
	ChannelConflictStrategy ConflictStrategy
	ModelConflictStrategy   ConflictStrategy
}

func (svc *BackupService) Backup(ctx context.Context, opts BackupOptions) ([]byte, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	if !user.IsOwner {
		return nil, fmt.Errorf("only owners can perform backup operations")
	}

	var channelDataList []*BackupChannel

	if opts.IncludeChannels {
		channels, err := svc.entFromContext(ctx).Channel.Query().All(ctx)
		if err != nil {
			return nil, err
		}

		channelDataList = lo.Map(channels, func(ch *ent.Channel, _ int) *BackupChannel {
			var credentials objects.ChannelCredentials
			if ch.Credentials != nil {
				credentials = *ch.Credentials
			}

			return &BackupChannel{
				Channel:     *ch,
				Credentials: credentials,
			}
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

	backupData := &BackupData{
		Version:   BackupVersion,
		Timestamp: time.Now(),
		Channels:  channelDataList,
		Models:    modelDataList,
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

	if backupData.Version != BackupVersion {
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
		for _, chData := range backupData.Channels {
			existing, err := svc.entFromContext(ctx).Channel.Query().
				Where(channel.Name(chData.Name)).
				First(ctx)
			if err != nil && !ent.IsNotFound(err) {
				return err
			}

			credentials := chData.Credentials

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
						SetCredentials(&credentials).
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

						continue
					}
				}
			} else {
				create := svc.entFromContext(ctx).Channel.Create().
					SetName(chData.Name).
					SetType(chData.Type).
					SetNillableBaseURL(baseURL).
					SetStatus(chData.Status).
					SetCredentials(&credentials).
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

					continue
				}
			}
		}
	}

	if opts.IncludeModels {
		for _, modelData := range backupData.Models {
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

						continue
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

					continue
				}
			}
		}
	}

	svc.channelService.asyncReloadChannels()

	return nil
}
