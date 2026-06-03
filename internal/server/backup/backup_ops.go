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
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
)

const usageBackupBatchSize = 500

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

	if opts.IncludeRequestLogs {
		var err error
		usageRequestDataList, err = svc.backupUsageRequests(ctx, opts.IncludeAPIKeys)
		if err != nil {
			return nil, err
		}
	}

	if opts.IncludeUsageStats {
		var err error
		usageLogDataList, err = svc.backupUsageLogs(ctx, opts.IncludeAPIKeys)
		if err != nil {
			return nil, err
		}
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

	if opts.IncludeUsageStats || opts.IncludeRequestLogs {
		return json.Marshal(backupData)
	}

	return json.MarshalIndent(backupData, "", "  ")
}

func (svc *BackupService) backupUsageRequests(ctx context.Context, includeAPIKeyValues bool) ([]*BackupUsageRequest, error) {
	var usageRequestDataList []*BackupUsageRequest
	lastID := 0

	for {
		query := svc.db.Request.Query().
			Where(request.IDGT(lastID)).
			Order(ent.Asc(request.FieldID)).
			Limit(usageBackupBatchSize).
			WithProject().
			WithChannel()
		if includeAPIKeyValues {
			query.WithAPIKey()
		}

		usageRequests, err := query.All(ctx)
		if err != nil {
			return nil, err
		}

		if len(usageRequests) == 0 {
			break
		}

		for _, req := range usageRequests {
			usageRequestDataList = append(usageRequestDataList, backupUsageRequest(req, includeAPIKeyValues))
			lastID = req.ID
		}

		if len(usageRequests) < usageBackupBatchSize {
			break
		}
	}

	return usageRequestDataList, nil
}

func backupUsageRequest(req *ent.Request, includeAPIKeyValues bool) *BackupUsageRequest {
	data := &BackupUsageRequest{Request: *req}
	if req.Edges.Project != nil {
		data.ProjectName = req.Edges.Project.Name
	}
	if req.Edges.Channel != nil {
		data.ChannelName = req.Edges.Channel.Name
	}
	if includeAPIKeyValues && req.Edges.APIKey != nil {
		data.APIKeyKey = req.Edges.APIKey.Key
	}
	data.Request.Edges = ent.RequestEdges{}

	return data
}

func (svc *BackupService) backupUsageLogs(ctx context.Context, includeAPIKeyValues bool) ([]*BackupUsageLog, error) {
	var usageLogDataList []*BackupUsageLog
	apiKeyKeys := map[int]string{}
	lastID := 0

	if includeAPIKeyValues {
		apiKeys, err := svc.db.APIKey.Query().
			Select(apikey.FieldID, apikey.FieldKey).
			All(ctx)
		if err != nil {
			return nil, err
		}

		for _, ak := range apiKeys {
			apiKeyKeys[ak.ID] = ak.Key
		}
	}

	for {
		query := svc.db.UsageLog.Query().
			Where(usagelog.IDGT(lastID)).
			Order(ent.Asc(usagelog.FieldID)).
			Limit(usageBackupBatchSize).
			WithProject().
			WithChannel()

		usageLogs, err := query.All(ctx)
		if err != nil {
			return nil, err
		}

		if len(usageLogs) == 0 {
			break
		}

		for _, ul := range usageLogs {
			usageLogDataList = append(usageLogDataList, backupUsageLog(ul, apiKeyKeys))
			lastID = ul.ID
		}

		if len(usageLogs) < usageBackupBatchSize {
			break
		}
	}

	return usageLogDataList, nil
}

func backupUsageLog(ul *ent.UsageLog, apiKeyKeys map[int]string) *BackupUsageLog {
	data := &BackupUsageLog{UsageLog: *ul}
	if ul.Edges.Project != nil {
		data.ProjectName = ul.Edges.Project.Name
	}
	if ul.Edges.Channel != nil {
		data.ChannelName = ul.Edges.Channel.Name
	}
	if ul.APIKeyID != 0 {
		data.APIKeyKey = apiKeyKeys[ul.APIKeyID]
	}
	data.UsageLog.Edges = ent.UsageLogEdges{}

	return data
}
