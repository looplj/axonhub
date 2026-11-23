// 职责: 校验插件声明的表结构并检查迁移是否为兼容的增量变更。
// 关联文件:
//
//	internal/objects/plugin_tables.go
//	internal/server/plugin/manifest.go
//	internal/server/plugin/table_storage.go
package plugin

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/objects"
)

const permTables = "tables"

var tableIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func validatePluginTables(tables map[string]objects.PluginTableSpec, permissions []string) error {
	if len(tables) > 16 {
		return fmt.Errorf("tables may contain at most 16 entries")
	}
	if len(tables) > 0 && !slices.Contains(permissions, permTables) {
		return fmt.Errorf("declaring tables requires the tables permission")
	}
	for _, name := range slices.Sorted(maps.Keys(tables)) {
		spec := tables[name]
		if !tableIdentifier.MatchString(name) || len(spec.Columns) == 0 || len(spec.Columns) > 32 || len(spec.Indexes) > 8 {
			return fmt.Errorf("tables.%s: expected a valid name, 1–32 columns and at most 8 indexes", name)
		}
		stringBytes := 0
		for _, column := range slices.Sorted(maps.Keys(spec.Columns)) {
			def := spec.Columns[column]
			if !tableIdentifier.MatchString(column) || column == "id" || strings.HasPrefix(column, "axp_") {
				return fmt.Errorf("tables.%s: invalid or reserved column %q", name, column)
			}
			switch def.Type {
			case "string":
				if def.MaxLength < 1 || def.MaxLength > 1024 {
					return fmt.Errorf("tables.%s.%s: string maxLength must be 1–1024", name, column)
				}
				stringBytes += def.MaxLength * 4
			case "integer", "number", "boolean", "text":
				if def.MaxLength != 0 {
					return fmt.Errorf("tables.%s.%s: maxLength is only supported on string columns", name, column)
				}
			default:
				return fmt.Errorf("tables.%s.%s: unknown column type %q", name, column, def.Type)
			}
		}
		if stringBytes > 48<<10 {
			return fmt.Errorf("tables.%s: combined string columns exceed the portable row size limit", name)
		}
		seen := make(map[string]bool, len(spec.Indexes))
		for _, index := range spec.Indexes {
			if err := validatePluginTableIndex(spec, index); err != nil {
				return fmt.Errorf("tables.%s: %w", name, err)
			}
			key := tableIndexKey(index)
			if seen[key] {
				return fmt.Errorf("tables.%s: duplicate index %q", name, key)
			}
			seen[key] = true
		}
	}
	return nil
}

func validatePluginTableIndex(spec objects.PluginTableSpec, index objects.PluginTableIndex) error {
	if len(index.Columns) == 0 || len(index.Columns) > 4 {
		return fmt.Errorf("an index must reference 1–4 columns")
	}
	size := 0
	seen := make(map[string]bool, len(index.Columns))
	for _, name := range index.Columns {
		column, ok := spec.Columns[name]
		if !ok || seen[name] || column.Type == "text" || (index.Unique && column.Nullable) {
			return fmt.Errorf("invalid index column %q", name)
		}
		seen[name] = true
		if column.Type == "string" {
			size += 4 * column.MaxLength
		} else {
			size += 8
		}
	}
	if size > 2048 {
		return fmt.Errorf("index may exceed the portable key size limit")
	}
	return nil
}

func tableIndexKey(index objects.PluginTableIndex) string {
	return fmt.Sprintf("%t:%s", index.Unique, strings.Join(index.Columns, ","))
}

func equalPluginTableSpec(a, b objects.PluginTableSpec) bool {
	if !maps.Equal(a.Columns, b.Columns) || len(a.Indexes) != len(b.Indexes) {
		return false
	}
	for _, index := range a.Indexes {
		if !slices.ContainsFunc(b.Indexes, func(other objects.PluginTableIndex) bool {
			return tableIndexKey(index) == tableIndexKey(other)
		}) {
			return false
		}
	}
	return true
}

func compatiblePluginTables(old, next map[string]objects.PluginTableSpec) error {
	for _, name := range slices.Sorted(maps.Keys(old)) {
		prior := old[name]
		table, ok := next[name]
		if !ok {
			return fmt.Errorf("cannot remove plugin table %q", name)
		}
		for column, def := range prior.Columns {
			if table.Columns[column] != def {
				return fmt.Errorf("cannot remove or change plugin table column %s.%s", name, column)
			}
		}
		for column, def := range table.Columns {
			if _, exists := prior.Columns[column]; !exists && !def.Nullable {
				return fmt.Errorf("new plugin table column %s.%s must be nullable", name, column)
			}
		}
		if err := compatibleTableIndexes(name, prior.Indexes, table.Indexes); err != nil {
			return err
		}
	}
	return nil
}

func compatibleTableIndexes(name string, old, next []objects.PluginTableIndex) error {
	contains := func(indexes []objects.PluginTableIndex, index objects.PluginTableIndex) bool {
		return slices.ContainsFunc(indexes, func(candidate objects.PluginTableIndex) bool {
			return tableIndexKey(candidate) == tableIndexKey(index)
		})
	}
	for _, index := range old {
		if !contains(next, index) {
			return fmt.Errorf("cannot remove an index from plugin table %q", name)
		}
	}
	for _, index := range next {
		if index.Unique && !contains(old, index) {
			return fmt.Errorf("cannot add a unique index to existing plugin table %q", name)
		}
	}
	return nil
}
