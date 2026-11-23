// 职责: 为托管插件表分配存储、记录迁移意图并执行可重试的清理。
// 关联文件:
//
//	internal/ent/schema/plugin_table_storage.go
//	internal/server/plugin/manager.go
//	internal/server/plugin/table_ddl.go
//
// 注意事项:
//
//	MySQL 和 TiDB 的 DDL 需要独立连接。迁移意图先提交，随后才允许执行 DDL。
package plugin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/ent/plugintableoperation"
	"github.com/looplj/axonhub/internal/ent/plugintablestorage"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

const (
	tableProvision     = "provision"
	tablePurge         = "purge"
	tableSchedulerName = "plugin-tables"
)

var errTableIntentRequired = errors.New("plugin table migration must be recorded before execution")

type pluginTablePlan struct {
	installationID int
	pluginID       string
	fromSHA        string
	toSHA          string
	tables         map[string]objects.PluginTableSpec
}

type tableProvisionFn func(context.Context, pluginRecord, string, map[string]objects.PluginTableSpec) error

// writeWithPluginTables runs the lifecycle mutation again after persisting its
// migration intent. The first attempt rolls back before the intent is written.
func (m *Manager) writeWithPluginTables(ctx context.Context, apply func(context.Context, tableProvisionFn) error) error {
	release, err := m.acquirePluginTableDDLSlot(ctx)
	if err != nil {
		return err
	}
	defer release()
	var pending *pluginTablePlan
	run := func(txCtx context.Context) error {
		return apply(txCtx, func(txCtx context.Context, row pluginRecord, sha string, tables map[string]objects.PluginTableSpec) error {
			plan := pluginTablePlan{row.ID, row.PluginID, row.PackageSHA256, sha, tables}
			pending = &plan
			return m.applyPluginTablePlan(txCtx, plan)
		})
	}
	err = m.runInTx(ctx, run)
	if !errors.Is(err, errTableIntentRequired) && !errors.Is(err, errPluginTableOwnership) {
		if err != nil && pending != nil && ent.TxFromContext(ctx) == nil {
			m.recordTablePlanError(ctx, *pending, err)
		}
		return err
	}
	if ent.TxFromContext(ctx) != nil {
		return fmt.Errorf("plugin table lifecycle requires an independent transaction")
	}
	if err := m.reservePluginTablePlan(ctx, *pending); err != nil {
		return err
	}
	plan := *pending
	err = m.runInTx(ctx, run)
	if errors.Is(err, errTableIntentRequired) {
		err = fmt.Errorf("plugin table plan changed concurrently, retry the operation")
	}
	if err != nil {
		m.recordTablePlanError(ctx, plan, err)
	}
	return err
}

func (m *Manager) acquirePluginTableDDLSlot(ctx context.Context) (func(), error) {
	if m.db.Driver().Dialect() != dialect.MySQL {
		return func() {}, nil
	}
	select {
	case m.tableDDLSlot <- struct{}{}:
		return func() { <-m.tableDDLSlot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *Manager) checkPluginTableDDLPool() error {
	if m.db.Driver().Dialect() == dialect.MySQL {
		if pool, ok := m.db.Driver().(interface{ DB() *sql.DB }); ok && pool.DB().Stats().MaxOpenConnections == 1 {
			return fmt.Errorf("plugin table DDL needs at least two primary database connections on MySQL or TiDB")
		}
	}
	return nil
}

func (m *Manager) activeTableStorage(ctx context.Context, pluginID string) (*ent.PluginTableStorage, error) {
	return m.client(ctx).PluginTableStorage.Query().
		Where(plugintablestorage.ActivePluginIDEQ(pluginID)).Only(ctx)
}

func (m *Manager) tableOperation(ctx context.Context, storageID int, kind string) (*ent.PluginTableOperation, error) {
	return m.client(ctx).PluginTableOperation.Query().
		Where(plugintableoperation.StorageIDEQ(storageID), plugintableoperation.KindEQ(kind)).Only(ctx)
}

func pluginTableInstallationLock(driver string, id int) (string, []any) {
	return entsql.Dialect(driver).Update(entplugin.Table).
		Set(entplugin.FieldEnabled, entsql.ExprFunc(func(b *entsql.Builder) {
			b.Ident(entplugin.FieldEnabled)
		})).
		Where(entsql.And(entsql.EQ(entplugin.FieldID, id), entsql.EQ(entplugin.FieldDeletedAt, 0))).
		Query()
}

func (m *Manager) lockTableInstallation(ctx context.Context, id int) error {
	driver := m.client(ctx).Driver()
	query, args := pluginTableInstallationLock(driver.Dialect(), id)
	if err := driver.Exec(ctx, query, args, nil); err != nil {
		return fmt.Errorf("lock plugin installation: %w", err)
	}
	return nil
}

func (m *Manager) reservePluginTablePlan(ctx context.Context, plan pluginTablePlan) error {
	return m.runInTx(ctx, func(txCtx context.Context) error {
		if err := m.lockTableInstallation(txCtx, plan.installationID); err != nil {
			return err
		}
		row, err := m.oneRecord(txCtx, plan.installationID)
		if err != nil {
			return err
		}
		if row.PackageSHA256 != plan.fromSHA {
			return fmt.Errorf("plugin package changed, retry the operation")
		}
		client := m.client(txCtx)
		storage, err := m.activeTableStorage(txCtx, plan.pluginID)
		if ent.IsNotFound(err) {
			storage, err = client.PluginTableStorage.Create().
				SetPluginID(plan.pluginID).SetActivePluginID(plan.pluginID).Save(txCtx)
		}
		if err != nil {
			return err
		}
		if err := compatiblePluginTables(storage.Tables, plan.tables); err != nil {
			return err
		}
		op, err := m.tableOperation(txCtx, storage.ID, tableProvision)
		names := maps.Clone(storage.Names)
		if names == nil {
			names = make(map[string]string)
		}
		if ent.IsNotFound(err) {
			if err := reservePluginTableNames(txCtx, client.Driver(), storage.ID, plan.tables, names); err != nil {
				return err
			}
			return client.PluginTableOperation.Create().
				SetStorageID(storage.ID).SetKind(tableProvision).
				SetTables(plan.tables).SetNames(names).SetPackageSha(plan.toSHA).Exec(txCtx)
		}
		if err != nil {
			return err
		}
		// A prior attempt may have applied part of its DDL before rolling back.
		if err := compatiblePluginTables(op.Tables, plan.tables); err != nil {
			return fmt.Errorf("resolve the pending table migration before replacing it: %w", err)
		}
		maps.Copy(names, op.Names)
		for name, physical := range op.Names {
			if storage.Names[name] != "" {
				continue
			}
			_, err := ownedPluginTableColumns(txCtx, client.Driver(), physical, pluginTableOwnerColumn(storage.ID))
			if errors.Is(err, errPluginTableOwnership) {
				delete(names, name)
			} else if err != nil {
				return err
			}
		}
		if err := reservePluginTableNames(txCtx, client.Driver(), storage.ID, plan.tables, names); err != nil {
			return err
		}
		return client.PluginTableOperation.UpdateOneID(op.ID).SetTables(plan.tables).
			SetNames(names).SetPackageSha(plan.toSHA).SetLastError("").Exec(txCtx)
	})
}

func (m *Manager) applyPluginTablePlan(ctx context.Context, plan pluginTablePlan) error {
	storage, err := m.activeTableStorage(ctx, plan.pluginID)
	if ent.IsNotFound(err) {
		if len(plan.tables) == 0 {
			return nil
		}
		return errTableIntentRequired
	}
	if err != nil {
		return err
	}
	if err := compatiblePluginTables(storage.Tables, plan.tables); err != nil {
		return err
	}
	op, err := m.tableOperation(ctx, storage.ID, tableProvision)
	if ent.IsNotFound(err) {
		if maps.EqualFunc(storage.Tables, plan.tables, equalPluginTableSpec) {
			return nil
		}
		return errTableIntentRequired
	}
	if err != nil {
		return err
	}
	if op.PackageSha != plan.toSHA || !maps.EqualFunc(op.Tables, plan.tables, equalPluginTableSpec) {
		// The currently published schema stays usable after a failed upgrade.
		if maps.EqualFunc(storage.Tables, plan.tables, equalPluginTableSpec) {
			return nil
		}
		return errTableIntentRequired
	}
	if err := m.checkPluginTableDDLPool(); err != nil {
		return err
	}
	ddl := m.tableDDLDriver(ctx)
	if err := provisionPluginTables(ctx, ddl, storage.ID, plan.tables, op.Names); err != nil {
		return err
	}
	client := m.client(ctx)
	if err := client.PluginTableStorage.UpdateOneID(storage.ID).SetTables(plan.tables).SetNames(op.Names).Exec(ctx); err != nil {
		return err
	}
	return client.PluginTableOperation.DeleteOneID(op.ID).Exec(ctx)
}

// Multiple uninstall/reinstall cycles in one second need distinct tombstones
// because (plugin_id, deleted_at) is unique.
func (m *Manager) softDeletePlugin(ctx context.Context, row pluginRecord) error {
	client := m.client(ctx)
	deletedAt := int(time.Now().Unix())
	prior, err := client.Plugin.Query().
		Where(entplugin.PluginIDEQ(row.PluginID), entplugin.DeletedAtGTE(deletedAt)).
		Order(ent.Desc(entplugin.FieldDeletedAt)).
		Select(entplugin.FieldDeletedAt).
		First(schematype.SkipSoftDelete(ctx))
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if prior != nil {
		deletedAt = prior.DeletedAt + 1
	}
	return client.Plugin.UpdateOneID(row.ID).SetDeletedAt(deletedAt).Exec(ctx)
}

func (m *Manager) tableDDLDriver(ctx context.Context) dialect.Driver {
	if m.db.Driver().Dialect() == dialect.MySQL {
		return m.db.Driver()
	}
	return m.client(ctx).Driver()
}

func (m *Manager) recordTablePlanError(ctx context.Context, plan pluginTablePlan, failure error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if err := m.runInTx(ctx, func(txCtx context.Context) error {
		storage, err := m.activeTableStorage(txCtx, plan.pluginID)
		if ent.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = m.client(txCtx).PluginTableOperation.Update().
			Where(plugintableoperation.StorageIDEQ(storage.ID), plugintableoperation.KindEQ(tableProvision),
				plugintableoperation.PackageShaEQ(plan.toSHA)).
			SetLastError(tableOperationError(failure)).Save(txCtx)
		return err
	}); err != nil {
		log.Warn(ctx, "recording plugin table migration failure", log.Cause(err))
	}
}

func tableOperationError(err error) string {
	runes := []rune(err.Error())
	return string(runes[:min(len(runes), 512)])
}

func (m *Manager) queueTablePurge(ctx context.Context, pluginID string) error {
	storage, err := m.activeTableStorage(ctx, pluginID)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	client := m.client(ctx)
	names := storage.Names
	pending, err := m.tableOperation(ctx, storage.ID, tableProvision)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if pending != nil {
		names = pending.Names
		if err := client.PluginTableOperation.DeleteOneID(pending.ID).Exec(ctx); err != nil {
			return err
		}
	}
	if err := client.PluginTableStorage.UpdateOneID(storage.ID).ClearActivePluginID().Exec(ctx); err != nil {
		return err
	}
	return client.PluginTableOperation.Create().SetStorageID(storage.ID).
		SetKind(tablePurge).SetNames(names).Exec(ctx)
}

func (m *Manager) scanTablePurges(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	ops, err := m.db.PluginTableOperation.Query().Where(plugintableoperation.KindEQ(tablePurge), plugintableoperation.RetryAtLTE(time.Now().Unix())).
		Order(plugintableoperation.ByRetryAt(), plugintableoperation.ByID()).Limit(8).All(ctx)
	if err != nil {
		log.Warn(ctx, "listing plugin table purges", log.Cause(err))
		return
	}
	for _, op := range ops {
		if err := m.purgePluginTables(ctx, op); err != nil {
			log.Warn(ctx, "purging plugin tables", log.Int("storage_id", op.StorageID), log.Cause(err))
			m.recordTablePurgeError(ctx, op.ID, err)
		}
	}
}

func (m *Manager) recordTablePurgeError(ctx context.Context, id int, failure error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if _, err := m.db.PluginTableOperation.Update().Where(plugintableoperation.IDEQ(id)).
		SetLastError(tableOperationError(failure)).SetRetryAt(time.Now().Add(time.Minute).Unix()).Save(ctx); err != nil {
		log.Warn(ctx, "recording plugin table purge failure", log.Cause(err))
	}
}

func (m *Manager) purgePluginTables(ctx context.Context, op *ent.PluginTableOperation) error {
	release, err := m.acquirePluginTableDDLSlot(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := m.checkPluginTableDDLPool(); err != nil {
		return err
	}
	return m.runInTx(ctx, func(txCtx context.Context) error {
		client := m.client(txCtx)
		count, err := client.PluginTableStorage.Update().Where(
			plugintablestorage.IDEQ(op.StorageID), plugintablestorage.ActivePluginIDIsNil()).
			AddRevision(1).Save(txCtx)
		if err != nil || count == 0 {
			return err
		}
		exists, err := client.PluginTableOperation.Query().Where(plugintableoperation.IDEQ(op.ID)).Exist(txCtx)
		if err != nil || !exists {
			return err
		}
		if err := dropPluginTables(txCtx, m.tableDDLDriver(txCtx), op.StorageID, op.Names); err != nil {
			return err
		}
		// Keep the storage ID reserved so physical names are never reused.
		if err := client.PluginTableStorage.UpdateOneID(op.StorageID).
			SetTables(map[string]objects.PluginTableSpec{}).SetNames(map[string]string{}).Exec(txCtx); err != nil {
			return err
		}
		return client.PluginTableOperation.DeleteOneID(op.ID).Exec(txCtx)
	})
}
