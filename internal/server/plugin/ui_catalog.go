package plugin

import (
	"encoding/json"
	"sort"

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

type UIAction struct {
	PluginID string
	Key      string
	Slot     string
	Title    objects.JSONRawMessage
	Icon     string
	Effect   objects.PluginUIActionEffect
	Revision string
}

// UIEntries lists UI contributions for any logged-in user.
// It carries no package bytes, grants, or config.
type UIEntries struct {
	Pages   []*UIPage
	Actions []*UIAction
}

// UISession is the iframe credential and the masked config for one entry.
type UISession struct {
	Token      string
	Config     map[string]any
	CanWriteKV bool
}

// UIEntries lists contributions declared by active plugins in the current generation.
// The generation is loaded under the manager's system bypass. Disabled plugins
// are omitted. Title bytes are the original string or object JSON.
func (m *Manager) UIEntries() *UIEntries {
	pages := make([]*UIPage, 0)
	actions := make([]*UIAction, 0)
	if m == nil {
		return &UIEntries{Pages: pages, Actions: actions}
	}
	for _, row := range m.Current().Plugins() {
		if row == nil || !row.Enabled || row.Status != RuntimeStatusActive || row.Manifest.UI == nil {
			continue
		}
		for _, action := range row.Manifest.UI.Actions {
			raw, err := json.Marshal(action.Title)
			if err != nil {
				continue
			}
			icon := action.Icon
			if icon == "" {
				icon = "puzzle"
			}
			actions = append(actions, &UIAction{
				PluginID: row.PluginID,
				Key:      action.Key,
				Slot:     action.Slot,
				Title:    objects.JSONRawMessage(raw),
				Icon:     icon,
				Effect:   action.Effect,
				Revision: row.PackageSHA256,
			})
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
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].PluginID < actions[j].PluginID })
	return &UIEntries{Pages: pages, Actions: actions}
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
