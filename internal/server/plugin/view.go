package plugin

import (
	"context"
	"encoding/json"
	"maps"
	"math"
	"slices"
	"strconv"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

// InstalledPlugin is the admin GraphQL view of one plugin row.
// It never carries package bytes.
type InstalledPlugin struct {
	ID                  objects.GUID
	PluginID            string
	Name                string
	Version             string
	Description         string
	Enabled             bool
	Priority            int
	Builtin             bool
	GrantedPermissions  []string
	GrantedNetwork      []string
	ManifestPermissions []string
	ManifestNetwork     []string
	ConfigFields        []ConfigField
	Config              map[string]any
	RuntimeStatus       string
	RuntimeError        string
}

// ConfigField is one configSchema property for the admin form.
// DefaultJSON is a JSON literal, or empty when the property has no default.
type ConfigField struct {
	Name        string
	Type        string
	Title       string
	Description string
	Format      string
	EnumValues  []string
	Minimum     *float64
	Maximum     *float64
	DefaultJSON string
}

// RuntimeStatusView is one process-local runtime status row.
type RuntimeStatusView struct {
	ID       objects.GUID
	PluginID string
	Status   string
	Reason   string
}

// InstallView is the GraphQL install payload. UpgradePreview is set when
// nothing was written.
type InstallView struct {
	Plugin         *InstalledPlugin
	UpgradePreview *UpgradePreview
}

// List returns non-deleted plugins without reading the package column.
func (m *Manager) List(ctx context.Context) ([]*InstalledPlugin, error) {
	rows, err := m.queryRecords(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(rows, func(a, b pluginRecord) int {
		if a.Priority != b.Priority {
			if a.Priority < b.Priority {
				return -1
			}
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	statuses := m.statusByID()
	out := make([]*InstalledPlugin, 0, len(rows))
	for _, row := range rows {
		item := m.presentRecord(row, statuses)
		out = append(out, &item)
	}
	return out, nil
}

// Get returns one plugin view, or an error when the row does not exist.
func (m *Manager) Get(ctx context.Context, id int) (*InstalledPlugin, error) {
	row, err := m.oneRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	item := m.presentRecord(row, m.statusByID())
	return &item, nil
}

// RuntimeStatusViews returns this process's runtime status rows.
func (m *Manager) RuntimeStatusViews() []*RuntimeStatusView {
	entries := m.RuntimeStatuses()
	out := make([]*RuntimeStatusView, 0, len(entries))
	for _, entry := range entries {
		out = append(out, &RuntimeStatusView{
			ID:       pluginGUID(entry.ID),
			PluginID: entry.PluginID,
			Status:   string(entry.Status),
			Reason:   entry.Reason,
		})
	}
	return out
}

// PresentInstall builds the GraphQL payload from an install result.
// Package bytes on the ent row are ignored. During an upgrade preview, manifest
// permissions come from the uploaded package so the grant dialog can list the
// new declaration. The stored row is unchanged.
func (m *Manager) PresentInstall(result *InstallResult, zip []byte) (*InstallView, error) {
	if result == nil {
		return &InstallView{}, nil
	}
	view := &InstallView{UpgradePreview: normalizePreview(result.Preview)}
	if result.Plugin == nil {
		return view, nil
	}
	installed := m.presentRecord(recordFromEntity(result.Plugin), m.statusByID())
	if result.Preview != nil {
		pkg, err := ParsePackage(zip)
		if err != nil {
			return nil, err
		}
		installed.ManifestPermissions = cloneStrings(pkg.Manifest.Permissions)
		installed.ManifestNetwork = cloneStrings(pkg.Manifest.NetworkGrants())
	}
	view.Plugin = &installed
	return view, nil
}

func (m *Manager) presentRecord(row pluginRecord, statuses map[int]RuntimeStatusEntry) InstalledPlugin {
	status := string(RuntimeStatusDisabled)
	reason := ""
	if entry, ok := statuses[row.ID]; ok {
		status = string(entry.Status)
		reason = entry.Reason
	}
	if status == "" {
		status = string(RuntimeStatusDisabled)
	}
	return InstalledPlugin{
		ID:                  pluginGUID(row.ID),
		PluginID:            row.PluginID,
		Name:                row.Name,
		Version:             row.Version,
		Description:         row.Description,
		Enabled:             row.Enabled,
		Priority:            row.Priority,
		Builtin:             m.IsBuiltin(row.PluginID),
		GrantedPermissions:  cloneStrings(row.Granted),
		GrantedNetwork:      cloneStrings(row.GrantedNetwork),
		ManifestPermissions: cloneStrings(row.Manifest.Permissions),
		ManifestNetwork:     cloneStrings(row.Manifest.NetworkGrants()),
		ConfigFields:        configFields(row.Manifest.ConfigSchema),
		Config:              maskConfig(row.Manifest.ConfigSchema, row.Config),
		RuntimeStatus:       status,
		RuntimeError:        reason,
	}
}

func (m *Manager) statusByID() map[int]RuntimeStatusEntry {
	entries := m.RuntimeStatuses()
	out := make(map[int]RuntimeStatusEntry, len(entries))
	for _, entry := range entries {
		out[entry.ID] = entry
	}
	return out
}

func recordFromEntity(row *ent.Plugin) pluginRecord {
	if row == nil {
		return pluginRecord{}
	}
	return pluginRecord{
		ID:             row.ID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		DeletedAt:      row.DeletedAt,
		PackageSHA256:  row.PackageSha256,
		PluginID:       row.PluginID,
		Name:           row.Name,
		Version:        row.Version,
		Description:    row.Description,
		Manifest:       row.Manifest,
		Enabled:        row.Enabled,
		Priority:       row.Priority,
		Granted:        cloneStrings(row.GrantedPermissions),
		GrantedNetwork: cloneStrings(row.GrantedNetwork),
		Config:         cloneConfig(row.Config),
	}
}

func normalizePreview(preview *UpgradePreview) *UpgradePreview {
	if preview == nil {
		return nil
	}
	return &UpgradePreview{
		CurrentVersion: preview.CurrentVersion,
		NewVersion:     preview.NewVersion,
		Permissions:    cloneStrings(preview.Permissions),
		Network:        cloneStrings(preview.Network),
		Events:         cloneStrings(preview.Events),
		Hooks:          cloneStrings(preview.Hooks),
		Pages:          cloneStrings(preview.Pages),
	}
}

func configFields(schema *objects.PluginConfigSchema) []ConfigField {
	if schema == nil || len(schema.Properties) == 0 {
		return []ConfigField{}
	}
	names := slices.Sorted(maps.Keys(schema.Properties))
	fields := make([]ConfigField, 0, len(names))
	for _, name := range names {
		prop := schema.Properties[name]
		fields = append(fields, ConfigField{
			Name:        name,
			Type:        prop.Type,
			Title:       prop.Title,
			Description: prop.Description,
			Format:      prop.Format,
			EnumValues:  enumStrings(prop.Enum),
			Minimum:     prop.Minimum,
			Maximum:     prop.Maximum,
			DefaultJSON: defaultJSON(prop.Default),
		})
	}
	return fields
}

func enumStrings(values []any) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		switch item := value.(type) {
		case string:
			out = append(out, item)
		case bool:
			out = append(out, strconv.FormatBool(item))
		case float64:
			out = append(out, formatNumber(item))
		case json.Number:
			out = append(out, item.String())
		default:
			encoded, err := json.Marshal(item)
			if err != nil {
				continue
			}
			out = append(out, string(encoded))
		}
	}
	return out
}

func formatNumber(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ""
	}
	if value == math.Trunc(value) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func defaultJSON(value any) string {
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func pluginGUID(id int) objects.GUID {
	return objects.GUID{Type: "Plugin", ID: id}
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return slices.Clone(items)
}

func cloneConfig(config map[string]any) map[string]any {
	if len(config) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(config))
	maps.Copy(out, config)
	return out
}
