// 职责: 定义插件 manifest 的托管表、字段和索引声明。
// 关联文件:
//
//	internal/objects/plugin.go
//	internal/server/plugin/table_manifest.go
//	internal/ent/schema/plugin_table_storage.go
package objects

import (
	"bytes"
	"encoding/json"
)

type PluginTableSpec struct {
	Columns map[string]PluginTableColumn `json:"columns"`
	Indexes []PluginTableIndex           `json:"indexes,omitempty"`
}

type PluginTableColumn struct {
	Type      string `json:"type"`
	MaxLength int    `json:"maxLength,omitempty"`
	Nullable  bool   `json:"nullable,omitempty"`
}

type PluginTableIndex struct {
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
}

func decodePluginTables(raw []byte) (map[string]PluginTableSpec, error) {
	var tables map[string]PluginTableSpec
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tables); err != nil {
		return nil, err
	}
	return tables, nil
}
