package plugin

import (
	"encoding/json"

	"github.com/looplj/axonhub/internal/objects"
)

// UIPage is one sidebar page. Title keeps the manifest JSON form:
// a string or a language map.
type UIPage struct {
	PluginID string
	Key      string
	Entry    string
	Title    objects.JSONRawMessage
}

// UIEntries is the page list for any logged-in user.
// It carries no package bytes, grants, or config.
type UIEntries struct {
	Pages []*UIPage
}

// UISession is the iframe credential and the masked config for one entry.
type UISession struct {
	Token      string
	Config     map[string]any
	CanWriteKV bool
}

// UIEntries lists pages declared by enabled plugins in the current generation.
// The generation is loaded under the manager's system bypass. Disabled plugins
// are omitted. Title bytes are the original string or object JSON.
func (m *Manager) UIEntries() *UIEntries {
	pages := make([]*UIPage, 0)
	if m == nil {
		return &UIEntries{Pages: pages}
	}
	for _, row := range m.Current().Plugins() {
		if row == nil || !row.Enabled || row.Manifest.UI == nil {
			continue
		}
		for _, page := range row.Manifest.UI.Pages {
			raw, err := json.Marshal(page.Title)
			if err != nil {
				continue
			}
			pages = append(pages, &UIPage{
				PluginID: row.PluginID,
				Key:      page.Key,
				Entry:    page.Entry,
				Title:    objects.JSONRawMessage(raw),
			})
		}
	}
	return &UIEntries{Pages: pages}
}

// MaskedUIConfig is the config handed to a plugin page.
// Secret values are maskConfig's mask. The stored map is not modified.
func (m *Manager) MaskedUIConfig(pluginID string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	row := m.Current().ByPluginID(pluginID)
	if row == nil {
		return map[string]any{}
	}
	return maskConfig(row.Manifest.ConfigSchema, row.Config)
}
