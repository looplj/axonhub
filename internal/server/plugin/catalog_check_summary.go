// 职责: 按插件安装保存和读取最近成功的模型目录检查摘要。
// 关联文件:
//   internal/server/plugin/tasks.go
//   internal/server/plugin/hostapi_catalog.go
//   internal/server/plugin/kv.go
//   internal/server/gql/plugin.resolvers.go
// 注意事项:
//   宿主命名空间在公共 KV 名称语法之外，卸载时与安装数据一起删除。
//   等待行锁期间，其他实例可能停用插件或撤销 read_channels 授权，因此保存摘要前必须在持锁事务内复核授权。
//   摘要与成功状态必须在受任务取消信号控制的同一事务中提交。

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/ent/pluginstorage"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
)

const (
	catalogSummaryNamespace = "/host"
	catalogSummaryKey       = "model-catalog-check"
)

type CatalogCheckSummary struct {
	biz.CatalogCheckCounts

	LastSuccessAt time.Time `json:"lastSuccessAt"`
}

type pendingCatalogCheck struct {
	installationID int
	pluginID       string
	counts         biz.CatalogCheckCounts
}

// finishCatalogCheck runs after the installation and schedule rows are locked.
func (m *Manager) finishCatalogCheck(ctx context.Context, check *pendingCatalogCheck, started, finished time.Time) (string, error) {
	if time.Now().Unix() >= taskLeaseUntil(started) {
		return runStatusUnknown, nil
	}
	if err := m.requireCatalogGrant(ctx, check.installationID, check.pluginID, false); err != nil {
		if coded, ok := xerrors.IsCodedError(err); ok && coded.Code == xerrors.ErrCodeForbidden {
			return "canceled", nil
		}
		return "", err
	}
	return "succeeded", check.save(ctx, m.client(ctx), finished)
}

// save runs inside the successful task completion transaction after the active run has been matched.
func (p *pendingCatalogCheck) save(ctx context.Context, client *ent.Client, finished time.Time) error {
	previous, err := readCatalogCheckSummary(ctx, client, p.installationID)
	if err != nil {
		return err
	}
	if previous != nil && !finished.After(previous.LastSuccessAt) {
		return nil
	}
	body, err := json.Marshal(CatalogCheckSummary{CatalogCheckCounts: p.counts, LastSuccessAt: finished})
	if err != nil {
		return fmt.Errorf("encode model catalog check summary: %w", err)
	}
	err = client.PluginStorage.Create().
		SetInstallationID(p.installationID).SetNamespace(catalogSummaryNamespace).SetKey(catalogSummaryKey).SetValue(string(body)).
		OnConflictColumns(pluginstorage.FieldInstallationID, pluginstorage.FieldNamespace, pluginstorage.FieldKey).
		Update(func(u *ent.PluginStorageUpsert) { u.UpdateValue().UpdateUpdatedAt() }).Exec(ctx)
	if err != nil {
		return fmt.Errorf("save model catalog check summary: %w", err)
	}
	return nil
}

// requireCatalogGrant reads the current installation and grants on the primary.
func (m *Manager) requireCatalogGrant(ctx context.Context, id int, manifestID string, fromUI bool) error {
	row, err := m.client(ctx).Plugin.Query().Where(entplugin.IDEQ(id), entplugin.PluginIDEQ(manifestID), entplugin.EnabledEQ(true), entplugin.DeletedAtEQ(0)).
		Select(entplugin.FieldGrantedPermissions, entplugin.FieldManifest).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("read model catalog grant: %w", err)
	}
	if row == nil || !slices.Contains(row.GrantedPermissions, permReadChannels) || (fromUI && !row.Manifest.UI.HasEntries()) {
		return xerrors.ForbiddenError("model catalog access is not granted")
	}
	return nil
}

func (m *Manager) CatalogCheckSummary(ctx context.Context) (*CatalogCheckSummary, error) {
	call, ok := CallFromContext(ctx)
	if !ok || !call.CanReadCatalogSummary || !call.matchesPrincipal(ctx) {
		return nil, xerrors.ForbiddenError("permission denied")
	}
	client := m.client(ctx)
	if tx := ent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	return authz.RunAsHost(ctx, client, "read-plugin-catalog-check-summary", func(hostCtx context.Context) (*CatalogCheckSummary, error) {
		var summary *CatalogCheckSummary
		err := m.runInTx(hostCtx, func(txCtx context.Context) error {
			if err := m.requireCatalogGrant(txCtx, call.InstallationID, call.ManifestID, true); err != nil {
				return err
			}
			var err error
			summary, err = readCatalogCheckSummary(txCtx, m.client(txCtx), call.InstallationID)
			return err
		})
		if err != nil {
			return nil, err
		}
		return summary, nil
	})
}

func readCatalogCheckSummary(ctx context.Context, client *ent.Client, installationID int) (*CatalogCheckSummary, error) {
	row, err := client.PluginStorage.Query().Where(pluginstorage.InstallationIDEQ(installationID),
		pluginstorage.NamespaceEQ(catalogSummaryNamespace), pluginstorage.KeyEQ(catalogSummaryKey)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read model catalog check summary: %w", err)
	}
	var summary CatalogCheckSummary
	if err := json.Unmarshal([]byte(row.Value), &summary); err != nil {
		return nil, fmt.Errorf("decode model catalog check summary: %w", err)
	}
	return &summary, nil
}
