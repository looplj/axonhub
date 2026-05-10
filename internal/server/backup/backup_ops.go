package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
)

func (svc *BackupService) Backup(ctx context.Context, opts BackupOptions) ([]byte, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	if !user.IsOwner {
		return nil, fmt.Errorf("only owners can perform backup operations")
	}

	return svc.doBackup(ctx, opts)
}

// BackupWithoutAuth performs backup without user authentication check.
// This is used by the auto-backup scheduler which runs in a privileged context.
func (svc *BackupService) BackupWithoutAuth(ctx context.Context, opts BackupOptions) ([]byte, error) {
	return svc.doBackup(ctx, opts)
}

func (svc *BackupService) doBackup(ctx context.Context, opts BackupOptions) ([]byte, error) {
	var (
		projectDataList           []*BackupProject
		channelDataList           []*BackupChannel
		channelModelPriceDataList []*BackupChannelModelPrice
	)

	if opts.IncludeProjects {
		projects, err := svc.db.Project.Query().All(ctx)
		if err != nil {
			return nil, err
		}

		projectDataList = lo.Map(projects, func(proj *ent.Project, _ int) *BackupProject {
			return &BackupProject{Project: *proj}
		})
	}

	if opts.IncludeChannels {
		channels, err := svc.db.Channel.Query().All(ctx)
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
		prices, err := svc.db.ChannelModelPrice.Query().
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
		models, err := svc.db.Model.Query().All(ctx)
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
		apiKeys, err := svc.db.APIKey.Query().WithProject().All(ctx)
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

	var (
		usageRequestDataList []*BackupUsageRequest
		usageLogDataList     []*BackupUsageLog
	)

	if opts.IncludeUsageStats {
		usageRequests, err := svc.db.Request.Query().
			Order(ent.Asc(request.FieldID)).
			WithProject().
			WithChannel().
			WithAPIKey().
			All(ctx)
		if err != nil {
			return nil, err
		}

		usageRequestDataList = lo.Map(usageRequests, func(req *ent.Request, _ int) *BackupUsageRequest {
			data := &BackupUsageRequest{Request: *req}
			if req.Edges.Project != nil {
				data.ProjectName = req.Edges.Project.Name
			}
			if req.Edges.Channel != nil {
				data.ChannelName = req.Edges.Channel.Name
			}
			if req.Edges.APIKey != nil {
				data.APIKeyKey = req.Edges.APIKey.Key
			}

			return data
		})

		usageLogs, err := svc.db.UsageLog.Query().
			WithProject().
			WithChannel().
			WithRequest(func(q *ent.RequestQuery) {
				q.WithAPIKey()
			}).
			All(ctx)
		if err != nil {
			return nil, err
		}

		usageLogDataList = lo.Map(usageLogs, func(ul *ent.UsageLog, _ int) *BackupUsageLog {
			data := &BackupUsageLog{UsageLog: *ul}
			if ul.Edges.Project != nil {
				data.ProjectName = ul.Edges.Project.Name
			}
			if ul.Edges.Channel != nil {
				data.ChannelName = ul.Edges.Channel.Name
			}
			if ul.Edges.Request != nil && ul.Edges.Request.Edges.APIKey != nil {
				data.APIKeyKey = ul.Edges.Request.Edges.APIKey.Key
			}

			return data
		})
	}

	backupData := &BackupData{
		Version:            BackupVersion,
		Timestamp:          time.Now(),
		Projects:           projectDataList,
		Channels:           channelDataList,
		Models:             modelDataList,
		ChannelModelPrices: channelModelPriceDataList,
		APIKeys:            apiKeyDataList,
		UsageRequests:      usageRequestDataList,
		UsageLogs:          usageLogDataList,
	}

	return json.MarshalIndent(backupData, "", "  ")
}
