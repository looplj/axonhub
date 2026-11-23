// 职责: 为插件脚本和页面执行按安装授权的托管表操作。
// 关联文件:
//
//	internal/server/plugin/hostapi.go
//	internal/server/plugin/table_rows.go
//	internal/server/plugin/table_storage.go
//	internal/server/plugin/hostapi_sql.go
//	internal/server/plugin/table_ui.go
//
// 注意事项:
//
//	旧运行时的写入和卸载、升级使用同一安装记录串行。
package plugin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"

	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/ent/plugintablestorage"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

type pluginTableCaller struct {
	installationID int
	pluginID       string
	fromUI         bool
}

func (h *hostCall) attachTablesAPI(ctx *moejs.Object) error {
	api := h.rt.Realm().NewObject()
	if err := defineHostFn(h.rt, api, "open", 1, h.openPluginTable); err != nil {
		return err
	}
	return defineValue(h.rt, ctx, "tables", engine.ObjectValue(api))
}

func (h *hostCall) openPluginTable(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.checkTableCall(); err != nil {
		return moejs.Undefined(), err
	}
	name, err := h.requireString(moejs.Arg(args, 0), "table")
	if err != nil {
		return moejs.Undefined(), err
	}
	if !tableIdentifier.MatchString(name) {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "invalid table name")
	}
	handle := h.rt.Realm().NewObject()
	for _, method := range []struct {
		name  string
		args  int
		write bool
	}{
		{"get", 1, false},
		{"list", 1, false},
		{"insert", 1, true},
		{"update", 2, true},
		{"delete", 1, true},
		{"upsert", 1, true},
	} {
		if err := defineHostFn(h.rt, handle, method.name, method.args, h.pluginTableMethod(name, method.name, method.write)); err != nil {
			return moejs.Undefined(), err
		}
	}
	return engine.ObjectValue(handle), nil
}

func (h *hostCall) checkTableCall() error {
	if err := h.enter(false); err != nil {
		return err
	}
	if _, granted := h.granted[permTables]; !granted {
		return h.fail(hostPermissionDenied, "tables permission is required")
	}
	return nil
}

func (h *hostCall) pluginTableMethod(name, method string, write bool) moejs.NativeFunc {
	return func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if err := h.checkTableCall(); err != nil {
			return moejs.Undefined(), err
		}
		input := make([][]byte, min(len(args), 2))
		for i := range input {
			if args[i].IsUndefined() {
				continue
			}
			raw, err := h.rt.AppendJSON(nil, args[i])
			if err != nil {
				return moejs.Undefined(), err
			}
			if len(raw) > maxPluginTableRowBytes {
				return moejs.Undefined(), h.fail(hostLimitExceeded, "table arguments exceed 64 KiB")
			}
			input[i] = raw
		}
		if write {
			if err := h.beginWrite(); err != nil {
				return moejs.Undefined(), err
			}
		}
		caller := pluginTableCaller{installationID: h.installationID, pluginID: h.pluginID}
		result, err := h.manager.runPluginTableMethod(h.ctx, caller, name, method, write, input)
		if err != nil {
			if detail, ok := errors.AsType[*pluginTableError](err); ok {
				return moejs.Undefined(), h.fail(detail.code, detail.message)
			}
			if isPluginTableConflict(err) {
				return moejs.Undefined(), h.fail(hostConflict, "plugin table unique constraint failed")
			}
			return moejs.Undefined(), h.sqlFailure(h.ctx, err)
		}
		return h.rt.ParseJSON(result)
	}
}

func (m *Manager) runPluginTableMethod(ctx context.Context, caller pluginTableCaller, name, method string, write bool, input [][]byte) ([]byte, error) {
	driver := m.db.Driver()
	pool, ok := driver.(interface{ DB() *sql.DB })
	if !ok {
		return nil, &pluginTableError{hostUnavailable, "plugin tables are unavailable for this connection"}
	}
	ctx, cancel := context.WithTimeout(ctx, pluginSQLTimeout)
	defer cancel()
	select {
	case m.sqlSlots <- struct{}{}:
		defer func() { <-m.sqlSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	tx, err := pool.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Warn(ctx, "plugin table transaction rollback failed",
				log.Int("installation_id", caller.installationID), log.Cause(err))
		}
	}()
	d := driver.Dialect()
	if write {
		query, args := pluginTableInstallationLock(d, caller.installationID)
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return nil, err
		}
	}
	table, err := caller.resolvePluginTable(ctx, tx, d, name)
	if err != nil {
		return nil, err
	}
	result, err := table.run(ctx, method, input)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if len(raw) > pluginSQLBytes {
		return nil, &pluginTableError{hostLimitExceeded, "plugin table result exceeds 1 MiB"}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return raw, nil
}

func (caller pluginTableCaller) resolvePluginTable(ctx context.Context, tx *sql.Tx, driver, name string) (*pluginTableRows, error) {
	var grants, manifestJSON []byte
	builder := entsql.Dialect(driver)
	query, args := builder.Select(entplugin.FieldGrantedPermissions, entplugin.FieldManifest).
		From(builder.Table(entplugin.Table)).
		Where(entsql.And(
			entsql.EQ(entplugin.FieldID, caller.installationID),
			entsql.EQ(entplugin.FieldPluginID, caller.pluginID),
			entsql.EQ(entplugin.FieldEnabled, true),
			entsql.EQ(entplugin.FieldDeletedAt, 0),
		)).Query()
	err := tx.QueryRowContext(ctx, query, args...).Scan(&grants, &manifestJSON)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var current []string
	if err != nil || json.Unmarshal(grants, &current) != nil || !slices.Contains(current, permTables) {
		return nil, &pluginTableError{hostPermissionDenied, "tables permission is no longer granted"}
	}
	var manifest struct {
		Tables map[string]objects.PluginTableSpec `json:"tables"`
		UI     *objects.PluginUI                  `json:"ui"`
	}
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return nil, err
	}
	if caller.fromUI && !manifest.UI.HasEntries() {
		return nil, &pluginTableError{hostPermissionDenied, "plugin UI is no longer enabled"}
	}
	spec, ok := manifest.Tables[name]
	if !ok {
		return nil, &pluginTableError{hostNotFound, "plugin table is not declared by this package"}
	}
	var rawNames []byte
	query, args = builder.Select(plugintablestorage.FieldNames).
		From(builder.Table(plugintablestorage.Table)).
		Where(entsql.EQ(plugintablestorage.FieldActivePluginID, caller.pluginID)).
		Query()
	err = tx.QueryRowContext(ctx, query, args...).Scan(&rawNames)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &pluginTableError{hostNotFound, "plugin table does not exist"}
	}
	if err != nil {
		return nil, err
	}
	var names map[string]string
	if err := json.Unmarshal(rawNames, &names); err != nil {
		return nil, err
	}
	if names[name] == "" {
		return nil, &pluginTableError{hostNotFound, "plugin table does not exist"}
	}
	return &pluginTableRows{tx: tx, dialect: driver, name: names[name], spec: spec}, nil
}

func decodeTableArgument(raw []byte, target any) error {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return tableArgumentError("missing table argument")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return tableArgumentError(err.Error())
	}
	return nil
}

type pluginTableError struct {
	code    string
	message string
}

func (e *pluginTableError) Error() string { return e.message }

func tableArgumentError(message string) error {
	return &pluginTableError{hostInvalidArgument, message}
}
