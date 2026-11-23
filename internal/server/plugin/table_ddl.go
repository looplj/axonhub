// 职责: 将插件表声明转换成 Ent 表定义，执行受限的增量迁移和清理。
// 关联文件:
//
//	internal/server/plugin/table_manifest.go
//	internal/server/plugin/table_storage.go
//	internal/server/plugin/hostapi_tables.go
//
// 注意事项:
//
//	恢复迁移或删表前复查物理表归属。升级只添加列和索引，保留旧表的物理类型。
package plugin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	amigrate "ariga.io/atlas/sql/migrate"
	aschema "ariga.io/atlas/sql/schema"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	eschema "entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/schema/field"

	"github.com/looplj/axonhub/internal/objects"
)

var errPluginTableOwnership = errors.New("plugin table has different ownership")

func physicalPluginTable(storageID int, name string) string {
	return fmt.Sprintf("axp_%d_%s", storageID, name)
}

func pluginTableOwnerColumn(storageID int) string {
	return fmt.Sprintf("axp_owner_%d", storageID)
}

func quoteTableIdentifier(driver, name string) string {
	if driver == dialect.MySQL {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func reservePluginTableNames(ctx context.Context, driver dialect.Driver, storageID int, tables map[string]objects.PluginTableSpec, names map[string]string) error {
	for _, name := range slices.Sorted(maps.Keys(tables)) {
		if names[name] != "" {
			continue
		}
		for attempt := range 10 {
			candidate := physicalPluginTable(storageID, name)
			if attempt > 0 {
				// A double underscore after the storage ID separates fallback
				// names from all logical names, which start with a letter.
				candidate = fmt.Sprintf("axp_%d__%d_%s", storageID, attempt, name)
			}
			columns, err := ownedPluginTableColumns(ctx, driver, candidate, pluginTableOwnerColumn(storageID))
			if errors.Is(err, errPluginTableOwnership) || len(columns) > 0 {
				continue
			}
			if err != nil {
				return err
			}
			names[name] = candidate
			break
		}
		if names[name] == "" {
			return fmt.Errorf("no available physical name for plugin table %q", name)
		}
	}
	return nil
}

func provisionPluginTables(ctx context.Context, driver dialect.Driver, storageID int, tables map[string]objects.PluginTableSpec, physicalNames map[string]string) error {
	for _, name := range slices.Sorted(maps.Keys(tables)) {
		if err := provisionPluginTable(ctx, driver, storageID, physicalNames[name], tables[name]); err != nil {
			return fmt.Errorf("provision plugin table %s: %w", name, err)
		}
	}
	return nil
}

func provisionPluginTable(ctx context.Context, driver dialect.Driver, storageID int, table string, spec objects.PluginTableSpec) error {
	if table == "" {
		return fmt.Errorf("plugin table has no reserved physical name")
	}
	owner := pluginTableOwnerColumn(storageID)
	columns, err := ownedPluginTableColumns(ctx, driver, table, owner)
	if err != nil {
		return err
	}
	migrator, err := eschema.NewMigrate(driver,
		eschema.WithForeignKeys(false),
		eschema.WithDiffHook(pluginTableAdditiveDiff(driver.Dialect(), table, owner)),
		eschema.WithApplyHook(guardPluginTableDDL),
	)
	if err != nil {
		return err
	}
	if driver.Dialect() == dialect.SQLite && len(columns) > 0 && slices.ContainsFunc(spec.Indexes, func(index objects.PluginTableIndex) bool {
		return slices.ContainsFunc(index.Columns, func(column string) bool {
			return !slices.Contains(columns, column)
		})
	}) {
		// SQLite plans a table rebuild if a newly added column already has
		// an index reference. Apply the columns before planning its indexes.
		columnsOnly := spec
		columnsOnly.Indexes = nil
		if err := migrator.Create(ctx, pluginEntTable(table, owner, columnsOnly)); err != nil {
			return err
		}
	}
	return migrator.Create(ctx, pluginEntTable(table, owner, spec))
}

func pluginEntTable(name, owner string, spec objects.PluginTableSpec) *eschema.Table {
	table := eschema.NewTable(name).
		AddPrimary(&eschema.Column{
			Name: "id", Type: field.TypeInt64, Increment: true,
			SchemaType: map[string]string{dialect.Postgres: "bigserial"},
		}).
		AddColumn(&eschema.Column{Name: owner, Type: field.TypeInt32, Nullable: true})
	for _, column := range slices.Sorted(maps.Keys(spec.Columns)) {
		table.AddColumn(pluginEntColumn(column, spec.Columns[column]))
	}
	for _, index := range spec.Indexes {
		indexName := "axp_i_" + sha256Hex([]byte(name + "|" + tableIndexKey(index)))[:32]
		table.AddIndex(indexName, index.Unique, index.Columns)
	}
	return table
}

func pluginEntColumn(name string, spec objects.PluginTableColumn) *eschema.Column {
	column := &eschema.Column{Name: name, Nullable: spec.Nullable}
	switch spec.Type {
	case "string":
		column.Type = field.TypeString
		column.Size = int64(spec.MaxLength)
		column.SchemaType = map[string]string{dialect.Postgres: fmt.Sprintf("varchar(%d)", spec.MaxLength)}
	case "integer":
		column.Type = field.TypeInt64
	case "number":
		column.Type = field.TypeFloat64
	case "boolean":
		column.Type = field.TypeBool
	case "text":
		column.Type = field.TypeString
		column.SchemaType = map[string]string{dialect.Postgres: "text", dialect.MySQL: "mediumtext"}
	}
	return column
}

func pluginTableAdditiveDiff(driver, table, owner string) eschema.DiffHook {
	return func(next eschema.Differ) eschema.Differ {
		return eschema.DiffFunc(func(current, desired *aschema.Schema) ([]aschema.Change, error) {
			if existing, ok := current.Table(table); ok {
				_, hasOwner := existing.Column(owner)
				_, hasID := existing.Column("id")
				if !hasOwner || !hasID {
					return nil, fmt.Errorf("%w: %s", errPluginTableOwnership, table)
				}
			}
			changes, err := next.Diff(current, desired)
			if err != nil {
				return nil, err
			}
			var additive []aschema.Change
			for _, change := range changes {
				switch change := change.(type) {
				case *aschema.AddTable:
					if driver == dialect.MySQL {
						// Managed tables inherit the primary database collation.
						change.T.Attrs = nil
					}
					additive = append(additive, change)
				case *aschema.ModifyTable:
					modify := &aschema.ModifyTable{T: change.T}
					for _, fieldChange := range change.Changes {
						switch fieldChange := fieldChange.(type) {
						case *aschema.AddColumn:
							if !fieldChange.C.Type.Null {
								return nil, fmt.Errorf("cannot add required column %s to existing table %s", fieldChange.C.Name, table)
							}
							modify.Changes = append(modify.Changes, fieldChange)
						case *aschema.AddIndex:
							modify.Changes = append(modify.Changes, fieldChange)
						}
					}
					if len(modify.Changes) > 0 {
						additive = append(additive, modify)
					}
				}
			}
			return additive, nil
		})
	}
}

func guardPluginTableDDL(next eschema.Applier) eschema.Applier {
	return eschema.ApplyFunc(func(ctx context.Context, conn dialect.ExecQuerier, plan *amigrate.Plan) error {
		for _, change := range plan.Changes {
			statement := strings.ToUpper(strings.TrimSpace(change.Cmd))
			switch {
			case strings.HasPrefix(statement, "CREATE TABLE "),
				strings.HasPrefix(statement, "CREATE INDEX "),
				strings.HasPrefix(statement, "CREATE UNIQUE INDEX "):
			case strings.HasPrefix(statement, "ALTER TABLE ") && strings.Contains(statement, " ADD ") &&
				!strings.Contains(statement, " DROP ") && !strings.Contains(statement, " RENAME "):
			default:
				return fmt.Errorf("unsupported managed plugin table migration step: %s", change.Comment)
			}
		}
		return next.Apply(ctx, conn, plan)
	})
}

func ownedPluginTableColumns(ctx context.Context, driver dialect.Driver, table, owner string) ([]string, error) {
	var query string
	switch driver.Dialect() {
	case dialect.SQLite:
		query = "SELECT name FROM pragma_table_info(?)"
	case dialect.Postgres:
		query = "SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1"
	case dialect.MySQL:
		query = "SELECT COLUMN_NAME FROM information_schema.columns WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?"
	default:
		return nil, fmt.Errorf("unsupported database dialect %q", driver.Dialect())
	}
	columns, err := tableQueryStrings(ctx, driver, query, table)
	if err != nil {
		return nil, err
	}
	if len(columns) > 0 && (!slices.Contains(columns, owner) || !slices.Contains(columns, "id")) {
		return nil, fmt.Errorf("%w: %s", errPluginTableOwnership, table)
	}
	return columns, nil
}

func tableQueryStrings(ctx context.Context, driver dialect.Driver, query string, arg any) ([]string, error) {
	var rows entsql.Rows
	if err := driver.Query(ctx, query, []any{arg}, &rows); err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func dropPluginTables(ctx context.Context, driver dialect.Driver, id int, names map[string]string) error {
	for _, name := range slices.Sorted(maps.Keys(names)) {
		table := names[name]
		columns, err := ownedPluginTableColumns(ctx, driver, table, pluginTableOwnerColumn(id))
		if err != nil {
			return err
		}
		if len(columns) > 0 {
			if err := driver.Exec(ctx, "DROP TABLE "+quoteTableIdentifier(driver.Dialect(), table), []any{}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
