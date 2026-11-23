// 职责: 验证托管表在升级、重装、隔离和清理中的完整生命周期。
// 关联文件:
//
//	internal/server/plugin/table_storage.go
//	internal/server/plugin/hostapi_tables.go
//	internal/server/plugin/table_ui.go
//	internal/server/plugin/manager_test.go
package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

func TestManagedPluginTablesLifecycle(t *testing.T) {
	testManagedPluginTablesLifecycle(t, newPluginDB(t))
}

func TestManagedPluginTablesExternalDatabases(t *testing.T) {
	for _, db := range []struct{ name, driver, dialect, env string }{
		{"Postgres", "pgx", dialect.Postgres, "AXONHUB_TEST_PLUGIN_TABLE_POSTGRES_DSN"},
		{"MySQL", "mysql", dialect.MySQL, "AXONHUB_TEST_PLUGIN_TABLE_MYSQL_DSN"},
		{"TiDB", "mysql", dialect.MySQL, "AXONHUB_TEST_PLUGIN_TABLE_TIDB_DSN"},
	} {
		t.Run(db.name, func(t *testing.T) {
			dsn := os.Getenv(db.env)
			if dsn == "" {
				t.Skip(db.env + " is not configured")
			}
			conn, err := sql.Open(db.driver, dsn)
			require.NoError(t, err)
			client := ent.NewClient(ent.Driver(entsql.OpenDB(db.dialect, conn)))
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			require.NoError(t, migrate.Create(t.Context(), client.Schema, []*schema.Table{
				migrate.PluginsTable, migrate.PluginStoragesTable, migrate.PluginSchedulesTable,
				migrate.PluginScheduleRunsTable, migrate.PluginTableStoragesTable, migrate.PluginTableOperationsTable,
			}, migrate.WithForeignKeys(false)))
			testManagedPluginTablesLifecycle(t, client)
		})
	}
}

func testManagedPluginTablesLifecycle(t *testing.T, client *ent.Client) {
	t.Helper()
	manager := newTestManager(t, client)
	ctx := pluginCtx()
	source := `
export function run(ctx, args) {
  if (args?.mode === "probe") {
    try { return ctx.tables.open("daily_usage").list(); }
    catch (error) { return error.code; }
  }
  const usage = ctx.tables.open(args?.table ?? "daily_usage");
  switch (args?.mode) {
  case "insert": return usage.insert({ day: args.day, requests: 1, active: true });
  case "insert-empty": return usage.insert({});
  case "upsert": return usage.upsert({ conflict: ["day"], row: { day: args.day, requests: args.count, active: true } });
  case "update": return usage.update(args.id, { requests: args.count });
  case "get": return usage.get(args.id);
  case "delete": return usage.delete(args.id);
  default: return usage.list(args?.options);
  }
}
`
	makePackage := func(id, version string, extra bool) []byte {
		return uiZip(t, func(_ *testing.T, manifest map[string]any) {
			manifest["id"] = id
			manifest["version"] = version
			manifest["permissions"] = []string{permTables}
			spec := objects.PluginTableSpec{
				Columns: map[string]objects.PluginTableColumn{
					"day":      {Type: "string", MaxLength: 10},
					"requests": {Type: "integer"},
					"active":   {Type: "boolean"},
				},
				Indexes: []objects.PluginTableIndex{{Columns: []string{"day"}, Unique: true}},
			}
			if extra {
				spec.Columns["note"] = objects.PluginTableColumn{Type: "text", Nullable: true}
				spec.Indexes = append(spec.Indexes, objects.PluginTableIndex{Columns: []string{"requests"}})
			}
			manifest["tables"] = map[string]objects.PluginTableSpec{
				"daily_usage": spec,
				"empty_rows":  {Columns: map[string]objects.PluginTableColumn{"note": {Type: "text", Nullable: true}}},
			}
		}, map[string]string{"plugin.js": source, "ui/page.html": "<html></html>"})
	}
	install := func(id, version string, extra bool) int {
		result, err := manager.Install(ctx, makePackage(id, version, extra), nil, nil)
		require.NoError(t, err)
		require.NoError(t, manager.Enable(ctx, result.Plugin.ID, []string{permTables}, nil))
		return result.Plugin.ID
	}
	invoke := func(id int, args string) string {
		result, err := manager.Invoke(context.Background(), InvokeRequest{InstallationID: id, Export: "run", Args: json.RawMessage("[" + args + "]")})
		require.NoError(t, err)
		return string(result.Value)
	}
	id := install("acme.tables", "1.0.0", false)
	other := install("acme.other", "1.0.0", false)
	require.JSONEq(t, `{"id":1,"note":null}`, invoke(id, `{"table":"empty_rows","mode":"insert-empty"}`))
	require.JSONEq(t, `{"rows":[{"id":1,"note":null}],"nextCursor":null}`,
		invoke(id, `{"table":"empty_rows","options":{"where":[{"column":"note","op":"eq","value":null},{"column":"id","op":"gte","value":1}]}}`))
	storage, err := manager.activeTableStorage(ctx, "acme.tables")
	require.NoError(t, err)
	require.JSONEq(t, `{"id":1,"day":"2026-11-20","requests":1,"active":true}`,
		invoke(id, `{"mode":"insert","day":"2026-11-20"}`))
	require.JSONEq(t, `{"id":1,"day":"2026-11-20","requests":9,"active":true}`,
		invoke(id, `{"mode":"upsert","day":"2026-11-20","count":9}`))
	require.JSONEq(t, `{"id":1,"day":"2026-11-20","requests":8,"active":true}`,
		invoke(id, `{"mode":"update","id":1,"count":8}`))
	require.JSONEq(t, `{"id":1,"day":"2026-11-20","requests":8,"active":true}`,
		invoke(id, `{"mode":"get","id":1}`))
	require.JSONEq(t, `{"rows":[],"nextCursor":null}`, invoke(other, `{}`))
	invoke(id, `{"mode":"insert","day":"2026-11-21"}`)
	var page struct {
		NextCursor string `json:"nextCursor"`
	}
	require.NoError(t, json.Unmarshal([]byte(invoke(id,
		`{"options":{"first":1,"orderBy":[{"column":"day","direction":"desc"}]}}`)), &page))
	require.NotEmpty(t, page.NextCursor)
	args, err := json.Marshal(map[string]any{"options": map[string]any{
		"first": 1, "orderBy": []map[string]string{{"column": "day", "direction": "desc"}}, "after": page.NextCursor,
	}})
	require.NoError(t, err)
	require.JSONEq(t, `{"rows":[{"id":1,"day":"2026-11-20","requests":8,"active":true}],"nextCursor":null}`, invoke(id, string(args)))
	require.JSONEq(t, `true`, invoke(id, `{"mode":"delete","id":2}`))

	upgraded, err := manager.Install(ctx, makePackage("acme.tables", "1.1.0", true), nil, nil)
	require.NoError(t, err)
	require.NotNil(t, upgraded.Preview)
	grants := []string{permTables}
	upgraded, err = manager.Install(ctx, makePackage("acme.tables", "1.1.0", true), &grants, nil)
	require.NoError(t, err)
	require.Equal(t, id, upgraded.Plugin.ID)
	require.JSONEq(t, `{"id":1,"day":"2026-11-20","requests":8,"active":true,"note":null}`,
		invoke(id, `{"mode":"get","id":1}`))

	require.NoError(t, manager.Delete(ctx, id))
	reinstalled := install("acme.tables", "1.1.0", true)
	require.NotEqual(t, id, reinstalled)
	restored, err := manager.activeTableStorage(ctx, "acme.tables")
	require.NoError(t, err)
	require.Equal(t, storage.ID, restored.ID)
	require.Equal(t, storage.Names, restored.Names)
	require.JSONEq(t, `{"id":1,"day":"2026-11-20","requests":8,"active":true,"note":null}`,
		invoke(reinstalled, `{"mode":"get","id":1}`))

	require.NoError(t, manager.DeleteWithTables(ctx, reinstalled, true))
	empty := install("acme.tables", "1.1.0", true)
	fresh, err := manager.activeTableStorage(ctx, "acme.tables")
	require.NoError(t, err)
	require.NotEqual(t, storage.ID, fresh.ID)
	invoke(empty, `{"mode":"insert","day":"2026-11-22"}`)
	manager.scanTablePurges(ctx)
	columns, err := ownedPluginTableColumns(ctx, client.Driver(), storage.Names["daily_usage"], pluginTableOwnerColumn(storage.ID))
	require.NoError(t, err)
	require.Empty(t, columns)
	require.JSONEq(t, `{"rows":[{"id":1,"day":"2026-11-22","requests":1,"active":true,"note":null}],"nextCursor":null}`,
		invoke(empty, `{}`))
	cachedSHA := manager.Current().ByID(empty).PackageSHA256
	nextSHA := sha256Hex([]byte("another plugin package"))
	require.NotEqual(t, cachedSHA, nextSHA)
	require.NoError(t, client.Plugin.UpdateOneID(empty).SetPackageSha256(nextSHA).Exec(ctx))
	require.Equal(t, cachedSHA, manager.Current().ByID(empty).PackageSHA256)
	require.JSONEq(t, `{"id":1,"day":"2026-11-22","requests":1,"active":true,"note":null}`,
		invoke(empty, `{"mode":"get","id":1}`))

	require.NoError(t, client.Plugin.UpdateOneID(empty).SetGrantedPermissions([]string{}).Exec(ctx))
	require.Contains(t, manager.Current().ByID(empty).GrantedPermissions, permTables)
	require.JSONEq(t, `"PERMISSION_DENIED"`, invoke(empty, `{"mode":"probe"}`))
}

func TestPluginUITableDeadlineCode(t *testing.T) {
	err := pluginUITableError(context.DeadlineExceeded)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var coded *xerrors.CodedError
	require.ErrorAs(t, err, &coded)
	require.Equal(t, xerrors.ErrorCode("TIMEOUT"), coded.Code)
}

func TestManagedPluginTablesResumeAfterDDL(t *testing.T) {
	manager := newTestManager(t, newPluginDB(t))
	ctx := pluginCtx()
	tables := map[string]objects.PluginTableSpec{
		"usage": {Columns: map[string]objects.PluginTableColumn{"amount": {Type: "integer"}}},
	}
	archive := uiZip(t, func(_ *testing.T, manifest map[string]any) {
		manifest["id"] = "acme.resume"
		manifest["permissions"] = []string{permTables}
		manifest["tables"] = tables
	}, map[string]string{"ui/page.html": "<html></html>"})
	result, err := manager.Install(ctx, archive, nil, nil)
	require.NoError(t, err)
	row := result.Plugin
	require.NoError(t, manager.reservePluginTablePlan(ctx, pluginTablePlan{
		installationID: row.ID, pluginID: row.PluginID,
		fromSHA: row.PackageSha256, toSHA: row.PackageSha256, tables: tables,
	}))
	storage, err := manager.activeTableStorage(ctx, row.PluginID)
	require.NoError(t, err)
	op, err := manager.tableOperation(ctx, storage.ID, tableProvision)
	require.NoError(t, err)
	// Model a crash after physical DDL completed but before the catalog commit.
	require.NoError(t, provisionPluginTables(ctx, manager.db.Driver(), storage.ID, tables, op.Names))
	require.NoError(t, manager.Enable(ctx, row.ID, []string{permTables}, nil))
	storage, err = manager.activeTableStorage(ctx, row.PluginID)
	require.NoError(t, err)
	require.Equal(t, op.Names, storage.Names)
	_, err = manager.tableOperation(ctx, storage.ID, tableProvision)
	require.True(t, ent.IsNotFound(err), "expected the completed operation to be removed: %v", err)
}

func TestManagedPluginTableLegacySQLiteUpgradePreservesTable(t *testing.T) {
	ctx := t.Context()
	driver := newPluginDB(t).Driver()
	const table = "axp_42_daily_usage"
	require.NoError(t, driver.Exec(ctx, "CREATE TABLE "+table+
		" (id INTEGER PRIMARY KEY AUTOINCREMENT, axp_owner_42 INTEGER, day VARCHAR(10) NOT NULL)", []any{}, nil))
	require.NoError(t, driver.Exec(ctx, "INSERT INTO "+table+" (id, day) VALUES (1, '2026-11-23'), (50, '2026-11-24')", []any{}, nil))
	require.NoError(t, driver.Exec(ctx, "DELETE FROM "+table+" WHERE id = 50", []any{}, nil))

	spec := objects.PluginTableSpec{
		Columns: map[string]objects.PluginTableColumn{
			"day":      {Type: "string", MaxLength: 10},
			"requests": {Type: "integer", Nullable: true},
		},
		Indexes: []objects.PluginTableIndex{{Columns: []string{"requests"}}},
	}
	require.NoError(t, provisionPluginTable(ctx, driver, 42, table, spec))
	require.NoError(t, provisionPluginTable(ctx, driver, 42, table, spec))

	types, err := tableQueryStrings(ctx, driver, "SELECT type FROM pragma_table_info(?) WHERE name = 'day'", table)
	require.NoError(t, err)
	require.Equal(t, []string{"VARCHAR(10)"}, types)
	indexes, err := tableQueryStrings(ctx, driver, "SELECT name FROM pragma_index_list(?)", table)
	require.NoError(t, err)
	require.Len(t, indexes, 1)
	columns, err := tableQueryStrings(ctx, driver, "SELECT name FROM pragma_index_info(?)", indexes[0])
	require.NoError(t, err)
	require.Equal(t, []string{"requests"}, columns)
	id, err := tableQueryStrings(ctx, driver, "SELECT CAST(id AS TEXT) FROM "+table+" WHERE day = ? AND requests IS NULL", "2026-11-23")
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, id)
	require.NoError(t, driver.Exec(ctx, "INSERT INTO "+table+" (day, requests) VALUES ('2026-11-25', 3)", []any{}, nil))
	id, err = tableQueryStrings(ctx, driver, "SELECT CAST(id AS TEXT) FROM "+table+" WHERE day = ?", "2026-11-25")
	require.NoError(t, err)
	require.Equal(t, []string{"51"}, id)
}

func TestManagedPluginTablePhysicalNameCollision(t *testing.T) {
	ctx := context.Background()
	driver := newPluginDB(t).Driver()
	existing := physicalPluginTable(1, "daily_usage")
	require.NoError(t, driver.Exec(ctx, "CREATE TABLE "+existing+" (unrelated TEXT)", []any{}, nil))
	names := make(map[string]string)
	tables := map[string]objects.PluginTableSpec{
		"daily_usage": {Columns: map[string]objects.PluginTableColumn{"amount": {Type: "integer"}}},
	}
	require.NoError(t, reservePluginTableNames(ctx, driver, 1, tables, names))
	require.NotEqual(t, existing, names["daily_usage"])
	require.NoError(t, provisionPluginTables(ctx, driver, 1, tables, names))
	require.NoError(t, dropPluginTables(ctx, driver, 1, names))
	columns, err := tableQueryStrings(ctx, driver, "SELECT name FROM pragma_table_info(?)", existing)
	require.NoError(t, err)
	require.Equal(t, []string{"unrelated"}, columns)
}

func TestManagedPluginTableManifestRejectsInvalidDefinitions(t *testing.T) {
	spec := objects.PluginTableSpec{
		Columns: map[string]objects.PluginTableColumn{"day": {Type: "string", MaxLength: 10}},
		Indexes: []objects.PluginTableIndex{{Columns: []string{"day"}, Unique: true}},
	}
	tables := map[string]objects.PluginTableSpec{"daily_usage": spec}
	require.ErrorContains(t, validatePluginTables(tables, nil), "permission")
	require.NoError(t, validatePluginTables(tables, []string{permTables}))
	require.ErrorContains(t, compatiblePluginTables(tables, nil), "cannot remove")
	spec.Columns["id"] = objects.PluginTableColumn{Type: "integer"}
	require.ErrorContains(t, validatePluginTables(tables, []string{permTables}), "reserved column")
}
