// 职责: 按插件安装保存和读取最近成功的模型目录检查摘要。
// 关联文件:
//   internal/server/plugin/tasks.go
//   internal/server/plugin/hostapi_catalog.go
//   internal/server/plugin/kv.go
//   internal/server/gql/plugin.resolvers.go
// 注意事项:
//   宿主命名空间在公共 KV 名称语法之外，卸载时与安装数据一起删除。

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

const catalogSummaryNamespace = "/host"
const catalogSummaryKey = "model-catalog-check"

type CatalogCheckSummary struct {
	biz.CatalogCheckCounts
	LastSuccessAt time.Time `json:"lastSuccessAt"`
}

type pendingCatalogCheck struct {
	installationID int
	counts         biz.CatalogCheckCounts
}

// save runs inside the successful task completion transaction after the active run has been matched.
func (p *pendingCatalogCheck) save(ctx context.Context, client *ent.Client, finished time.Time) error {
	body, err := json.Marshal(CatalogCheckSummary{CatalogCheckCounts: p.counts, LastSuccessAt: finished})
	if err != nil {
		return err
	}
	return client.PluginStorage.Create().
		SetInstallationID(p.installationID).SetNamespace(catalogSummaryNamespace).SetKey(catalogSummaryKey).SetValue(string(body)).
		OnConflictColumns(pluginstorage.FieldInstallationID, pluginstorage.FieldNamespace, pluginstorage.FieldKey).
		Update(func(u *ent.PluginStorageUpsert) { u.UpdateValue().UpdateUpdatedAt() }).Exec(ctx)
}

// requireCatalogGrant uses the primary transaction so a peer's revoked grant takes effect before the next check or read.
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
	if !ok || !call.CanReadCatalogSummary || authorizePluginKV(ctx, call.ManifestID, call.InstallationID, false) != nil {
		return nil, xerrors.ForbiddenError("permission denied")
	}
	return authz.RunAsHost(ctx, m.client(ctx), "read-plugin-catalog-check-summary", func(hostCtx context.Context) (*CatalogCheckSummary, error) {
		var summary *CatalogCheckSummary
		err := m.runInTx(hostCtx, func(txCtx context.Context) error {
			if err := m.requireCatalogGrant(txCtx, call.InstallationID, call.ManifestID, true); err != nil {
				return err
			}
			row, err := m.client(txCtx).PluginStorage.Query().Where(pluginstorage.InstallationIDEQ(call.InstallationID),
				pluginstorage.NamespaceEQ(catalogSummaryNamespace), pluginstorage.KeyEQ(catalogSummaryKey)).Only(txCtx)
			if ent.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read model catalog check summary: %w", err)
			}
			summary = new(CatalogCheckSummary)
			return json.Unmarshal([]byte(row.Value), summary)
		})
		if err != nil {
			return nil, err
		}
		return summary, nil
	})
}
