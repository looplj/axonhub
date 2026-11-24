// 职责: 按插件安装保存并读取页面和顶部动作的导航角标。
// 关联文件:
//   internal/server/plugin/ui_catalog.go
//   internal/server/plugin/hostapi.go
//   internal/server/plugin/kv.go
//   internal/ent/schema/plugin_storage.go
// 注意事项:
//   保留命名空间不属于公开 KV，卸载会统一清除角标。

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/MoeForge/moejs"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/pluginstorage"
	"github.com/looplj/axonhub/internal/objects"
)

const uiBadgeNamespace = "/host/ui-badge"

type UIBadgeTone string

const (
	UIBadgeAccent  UIBadgeTone = "accent"
	UIBadgeWarning UIBadgeTone = "warning"
	UIBadgeDanger  UIBadgeTone = "danger"
)

type UIBadge struct {
	Count *int        `json:"count,omitempty"`
	Dot   bool        `json:"dot,omitempty"`
	Tone  UIBadgeTone `json:"tone"`
}

func (b UIBadge) visible() bool {
	return (b.Count != nil && *b.Count > 0) || b.Dot
}

func (t UIBadgeTone) valid() bool {
	return t == UIBadgeAccent || t == UIBadgeWarning || t == UIBadgeDanger
}

type uiBadgeTarget struct {
	Kind string `json:"type"`
	Key  string `json:"key"`
}

func (t uiBadgeTarget) storageKey() string { return t.Kind + ":" + t.Key }

func (t uiBadgeTarget) declared(ui *objects.PluginUI) bool {
	if ui == nil {
		return false
	}
	switch t.Kind {
	case "page":
		return slices.ContainsFunc(ui.Pages, func(page objects.PluginUIPage) bool { return page.Key == t.Key })
	case "action":
		return slices.ContainsFunc(ui.Actions, func(action objects.PluginUIAction) bool { return action.Key == t.Key })
	default:
		return false
	}
}

var errUIBadgeTarget = errors.New("badge target is not declared by the active plugin")

func (m *Manager) setUIBadge(ctx context.Context, id int, pluginID string, target uiBadgeTarget, badge UIBadge) error {
	_, err := authz.RunAsHost(ctx, m.db, "set-plugin-ui-badge", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, m.runInTx(ctx, func(ctx context.Context) error {
			// Uninstall and upgrade use the same installation lock.
			if err := m.lockTableInstallation(ctx, id); err != nil {
				return err
			}
			row, err := m.oneRecord(ctx, id)
			if err != nil {
				return err
			}
			if !row.Enabled || row.DeletedAt != 0 || row.PluginID != pluginID || !target.declared(row.Manifest.UI) {
				return errUIBadgeTarget
			}
			client := m.client(ctx)
			key := target.storageKey()
			if !badge.visible() {
				_, err := client.PluginStorage.Delete().Where(pluginstorage.InstallationIDEQ(id),
					pluginstorage.NamespaceEQ(uiBadgeNamespace), pluginstorage.KeyEQ(key)).Exec(ctx)
				return err
			}
			raw, err := json.Marshal(badge)
			if err != nil {
				return err
			}
			return client.PluginStorage.Create().SetInstallationID(id).SetNamespace(uiBadgeNamespace).SetKey(key).SetValue(string(raw)).
				OnConflictColumns(pluginstorage.FieldInstallationID, pluginstorage.FieldNamespace, pluginstorage.FieldKey).
				Update(func(u *ent.PluginStorageUpsert) { u.UpdateValue().UpdateUpdatedAt() }).Exec(ctx)
		})
	})
	return err
}

func (m *Manager) UIEntriesWithBadges(ctx context.Context) (*UIEntries, error) {
	entries := m.UIEntries()
	type badgeKey struct {
		id  int
		key string
	}
	badges := make(map[badgeKey]**UIBadge, len(entries.Pages)+len(entries.Actions))
	ids := make(map[int]bool)
	for _, page := range entries.Pages {
		ids[page.installationID] = true
		badges[badgeKey{page.installationID, (uiBadgeTarget{Kind: "page", Key: page.Key}).storageKey()}] = &page.Badge
	}
	for _, action := range entries.Actions {
		ids[action.installationID] = true
		badges[badgeKey{action.installationID, (uiBadgeTarget{Kind: "action", Key: action.Key}).storageKey()}] = &action.Badge
	}
	if len(ids) == 0 {
		return entries, nil
	}
	installationIDs := make([]int, 0, len(ids))
	for id := range ids {
		installationIDs = append(installationIDs, id)
	}
	rows, err := authz.RunAsHost(ctx, m.client(ctx), "read-plugin-ui-badges", func(ctx context.Context) ([]*ent.PluginStorage, error) {
		return m.client(ctx).PluginStorage.Query().Where(pluginstorage.InstallationIDIn(installationIDs...), pluginstorage.NamespaceEQ(uiBadgeNamespace)).All(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("read plugin UI badges: %w", err)
	}
	for _, row := range rows {
		destination := badges[badgeKey{row.InstallationID, row.Key}]
		if destination == nil {
			continue
		}
		var badge UIBadge
		if json.Unmarshal([]byte(row.Value), &badge) == nil && badge.visible() && badge.Tone.valid() {
			*destination = &badge
		}
	}
	return entries, nil
}

func (h *hostCall) setUIBadge(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	targetValue, err := h.optionsAt(args, 0)
	if err != nil {
		return moejs.Undefined(), err
	}
	kind, err := h.optionString(targetValue, "type")
	if err != nil {
		return moejs.Undefined(), err
	}
	key, err := h.optionString(targetValue, "key")
	if err != nil {
		return moejs.Undefined(), err
	}
	if (kind != "page" && kind != "action") || key == "" {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "badge target requires a page or action key")
	}
	badge, err := h.readUIBadge(moejs.Arg(args, 1))
	if err != nil {
		return moejs.Undefined(), err
	}
	if err := h.beginWrite(); err != nil {
		return moejs.Undefined(), err
	}
	err = h.manager.setUIBadge(h.ctx, h.installationID, h.pluginID, uiBadgeTarget{Kind: kind, Key: key}, badge)
	if errors.Is(err, errUIBadgeTarget) {
		return moejs.Undefined(), h.fail(hostNotFound, err.Error())
	}
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	return moejs.Undefined(), nil
}

func (h *hostCall) readUIBadge(value moejs.Value) (UIBadge, error) {
	badge := UIBadge{Tone: UIBadgeAccent}
	if value.IsNull() {
		return badge, nil
	}
	fields, err := h.jsonObject(value, "badge")
	if err != nil {
		return badge, err
	}
	for key, field := range fields {
		switch key {
		case "count":
			n, ok := field.(float64)
			if !ok || n < 0 || n > math.MaxInt32 || math.Trunc(n) != n {
				return badge, h.fail(hostInvalidArgument, "badge count must be a non-negative integer")
			}
			count := int(n)
			badge.Count = &count
		case "dot":
			dot, ok := field.(bool)
			if !ok {
				return badge, h.fail(hostInvalidArgument, "badge dot must be boolean")
			}
			badge.Dot = dot
		case "tone":
			tone, ok := field.(string)
			if !ok || !UIBadgeTone(tone).valid() {
				return badge, h.fail(hostInvalidArgument, "invalid badge tone")
			}
			badge.Tone = UIBadgeTone(tone)
		default:
			return badge, h.fail(hostInvalidArgument, "unknown badge field")
		}
	}
	return badge, nil
}
