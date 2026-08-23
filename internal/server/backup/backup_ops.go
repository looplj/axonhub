package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/server/biz"
)

const backupBatchSize = 500

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
	var buf bytes.Buffer
	if err := svc.doBackupToWriter(ctx, opts, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// doBackupToWriter streams a compact JSON backup to w without accumulating
// the full dataset in memory. Each entity type is processed sequentially in
// batch-sized pages using an ID-cursor.
func (svc *BackupService) doBackupToWriter(ctx context.Context, opts BackupOptions, w io.Writer) error {
	o := &objWriter{w: w}

	if _, err := w.Write([]byte("{")); err != nil {
		return err
	}

	if b, err := json.Marshal(BackupVersion); err != nil {
		return err
	} else if err := o.rawField("version", b); err != nil {
		return err
	}

	if b, err := json.Marshal(time.Now()); err != nil {
		return err
	} else if err := o.rawField("timestamp", b); err != nil {
		return err
	}

	if err := svc.streamSystemConfigs(ctx, o, opts); err != nil {
		return err
	}
	if err := svc.streamProjects(ctx, o, opts); err != nil {
		return err
	}
	if err := svc.streamChannels(ctx, o, opts); err != nil {
		return err
	}
	if err := svc.streamModels(ctx, o, opts); err != nil {
		return err
	}
	if err := svc.streamChannelModelPrices(ctx, o, opts); err != nil {
		return err
	}
	if err := svc.streamAPIKeys(ctx, o, opts); err != nil {
		return err
	}
	if err := svc.streamUsageRequests(ctx, o, opts); err != nil {
		return err
	}
	if err := svc.streamUsageLogs(ctx, o, opts); err != nil {
		return err
	}

	_, err := w.Write([]byte("}"))
	return err
}

// systemConfigBackupKeys contains settings that are portable across deployments.
// Deployment-specific values are excluded because their references or secrets are
// valid only for the source instance.
var systemConfigBackupKeys = []string{
	biz.SystemKeyBrandName,
	biz.SystemKeyBrandLogo,
	biz.SystemKeyTitle,
	biz.SystemKeyStoreChunks,
	biz.SystemKeyStoragePolicy,
	biz.SystemKeyRetryPolicy,
	biz.SystemKeyWebhookNotifierConfig,
	biz.SystemKeyModelSettings,
	biz.SystemKeyChannelSettings,
	biz.SystemKeyGeneralSettings,
	biz.SystemKeyUserAgentPassThrough,
	biz.SystemKeyPassThrough,
	biz.SystemKeyInjectUsageCost,
	biz.SystemKeyQuotaEnforcementSettings,
	biz.SystemKeySecuritySettings,
	biz.SystemKeyProxyPresets,
}

func (svc *BackupService) streamSystemConfigs(ctx context.Context, o *objWriter, opts BackupOptions) error {
	return streamArrayField(o, "system_configs", opts.IncludeSystemConfigs, true,
		func(lastID int) ([]*ent.System, int, error) {
			rows, err := svc.db.System.Query().
				Where(system.IDGT(lastID), system.KeyIn(systemConfigBackupKeys...)).
				Order(ent.Asc(system.FieldID)).
				Limit(backupBatchSize).
				All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(config *ent.System) ([]byte, bool, error) {
			data, err := json.Marshal(&BackupSystemConfig{Key: config.Key, Value: config.Value})
			return data, true, err
		},
	)
}

func (svc *BackupService) streamProjects(ctx context.Context, o *objWriter, opts BackupOptions) error {
	return streamArrayField(o, "projects", opts.IncludeProjects, true,
		func(lastID int) ([]*ent.Project, int, error) {
			rows, err := svc.db.Project.Query().
				Where(project.IDGT(lastID)).
				Order(ent.Asc(project.FieldID)).
				Limit(backupBatchSize).
				All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(p *ent.Project) ([]byte, bool, error) {
			b, err := json.Marshal(&BackupProject{Project: *p})
			return b, true, err
		},
	)
}

func (svc *BackupService) streamChannels(ctx context.Context, o *objWriter, opts BackupOptions) error {
	return streamArrayField(o, "channels", opts.IncludeChannels, false,
		func(lastID int) ([]*ent.Channel, int, error) {
			rows, err := svc.db.Channel.Query().
				Where(channel.IDGT(lastID)).
				Order(ent.Asc(channel.FieldID)).
				Limit(backupBatchSize).
				All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(ch *ent.Channel) ([]byte, bool, error) {
			b, err := json.Marshal(&BackupChannel{
				Channel:     *ch,
				Credentials: ch.Credentials,
			})
			return b, true, err
		},
	)
}

func (svc *BackupService) streamModels(ctx context.Context, o *objWriter, opts BackupOptions) error {
	return streamArrayField(o, "models", opts.IncludeModels, false,
		func(lastID int) ([]*ent.Model, int, error) {
			rows, err := svc.db.Model.Query().
				Where(model.IDGT(lastID)).
				Order(ent.Asc(model.FieldID)).
				Limit(backupBatchSize).
				All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(m *ent.Model) ([]byte, bool, error) {
			b, err := json.Marshal(&BackupModel{Model: *m})
			return b, true, err
		},
	)
}

func (svc *BackupService) streamChannelModelPrices(ctx context.Context, o *objWriter, opts BackupOptions) error {
	return streamArrayField(o, "channel_model_prices", opts.IncludeModelPrices, true,
		func(lastID int) ([]*ent.ChannelModelPrice, int, error) {
			rows, err := svc.db.ChannelModelPrice.Query().
				WithChannel().
				Where(channelmodelprice.IDGT(lastID)).
				Order(ent.Asc(channelmodelprice.FieldID)).
				Limit(backupBatchSize).
				All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(p *ent.ChannelModelPrice) ([]byte, bool, error) {
			if p.Edges.Channel == nil {
				return nil, false, nil
			}
			b, err := json.Marshal(&BackupChannelModelPrice{
				ChannelName: p.Edges.Channel.Name,
				ModelID:     p.ModelID,
				Price:       p.Price,
				ReferenceID: p.ReferenceID,
			})
			return b, true, err
		},
	)
}

func (svc *BackupService) streamAPIKeys(ctx context.Context, o *objWriter, opts BackupOptions) error {
	return streamArrayField(o, "api_keys", opts.IncludeAPIKeys, true,
		func(lastID int) ([]*ent.APIKey, int, error) {
			rows, err := svc.db.APIKey.Query().
				WithProject().
				Where(apikey.IDGT(lastID)).
				Order(ent.Asc(apikey.FieldID)).
				Limit(backupBatchSize).
				All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(ak *ent.APIKey) ([]byte, bool, error) {
			projectName := ""
			if ak.Edges.Project != nil {
				projectName = ak.Edges.Project.Name
			}
			b, err := json.Marshal(&BackupAPIKey{
				APIKey:      *ak,
				ProjectName: projectName,
			})
			return b, true, err
		},
	)
}

func (svc *BackupService) streamUsageRequests(ctx context.Context, o *objWriter, opts BackupOptions) error {
	return streamArrayField(o, "usage_requests", opts.IncludeRequestLogs, true,
		func(lastID int) ([]*ent.Request, int, error) {
			query := svc.db.Request.Query().
				Where(request.IDGT(lastID)).
				Order(ent.Asc(request.FieldID)).
				Limit(backupBatchSize).
				WithProject().
				WithChannel()
			if opts.IncludeAPIKeys {
				query.WithAPIKey()
			}
			rows, err := query.All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(req *ent.Request) ([]byte, bool, error) {
			b, err := json.Marshal(backupUsageRequest(req, opts.IncludeAPIKeys))
			return b, true, err
		},
	)
}

func (svc *BackupService) streamUsageLogs(ctx context.Context, o *objWriter, opts BackupOptions) error {
	apiKeyKeys := map[int]string{}
	if opts.IncludeUsageStats && opts.IncludeAPIKeys {
		apiKeys, err := svc.db.APIKey.Query().
			Select(apikey.FieldID, apikey.FieldKey).
			All(ctx)
		if err != nil {
			return err
		}
		for _, ak := range apiKeys {
			apiKeyKeys[ak.ID] = ak.Key
		}
	}
	return streamArrayField(o, "usage_logs", opts.IncludeUsageStats, true,
		func(lastID int) ([]*ent.UsageLog, int, error) {
			rows, err := svc.db.UsageLog.Query().
				Where(usagelog.IDGT(lastID)).
				Order(ent.Asc(usagelog.FieldID)).
				Limit(backupBatchSize).
				WithProject().
				WithChannel().
				All(ctx)
			if err != nil {
				return nil, 0, err
			}
			nextID := 0
			if len(rows) > 0 {
				nextID = rows[len(rows)-1].ID
			}
			return rows, nextID, nil
		},
		func(ul *ent.UsageLog) ([]byte, bool, error) {
			b, err := json.Marshal(backupUsageLog(ul, apiKeyKeys))
			return b, true, err
		},
	)
}

// objWriter writes a JSON object incrementally, tracking leading commas.
type objWriter struct {
	w         io.Writer
	needComma bool
}

func (o *objWriter) rawField(name string, raw []byte) error {
	if o.needComma {
		if _, err := o.w.Write([]byte(",")); err != nil {
			return err
		}
	}
	o.needComma = true
	if _, err := fmt.Fprintf(o.w, "%q:", name); err != nil {
		return err
	}
	_, err := o.w.Write(raw)
	return err
}

// streamArrayField streams a JSON array field incrementally, processing rows
// in pages via fetchBatch and transforming each via elem.
func streamArrayField[T any](
	o *objWriter, name string, on bool, omitempty bool,
	fetchBatch func(lastID int) (rows []T, nextID int, err error),
	elem func(T) (jsonBytes []byte, emit bool, err error),
) error {
	if !on {
		if omitempty {
			return nil
		}
		return o.rawField(name, []byte("null"))
	}
	lastID := 0
	opened := false
	for {
		rows, nextID, err := fetchBatch(lastID)
		if err != nil {
			if opened {
				_, _ = o.w.Write([]byte("]"))
			}
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			b, emit, err := elem(r)
			if err != nil {
				if opened {
					_, _ = o.w.Write([]byte("]"))
				}
				return err
			}
			if !emit {
				continue
			}
			if !opened {
				if err := o.rawField(name, []byte("[")); err != nil {
					return err
				}
				opened = true
				if _, err := o.w.Write(b); err != nil {
					return err
				}
			} else {
				if _, err := o.w.Write([]byte(",")); err != nil {
					return err
				}
				if _, err := o.w.Write(b); err != nil {
					return err
				}
			}
		}
		lastID = nextID
		if len(rows) < backupBatchSize {
			break
		}
	}
	if opened {
		_, err := o.w.Write([]byte("]"))
		return err
	}
	if omitempty {
		return nil
	}
	return o.rawField(name, []byte("[]"))
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
