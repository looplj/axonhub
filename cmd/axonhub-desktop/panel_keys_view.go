// 职责: 绘制面板的页签栏和 Keys 页：Base URL、搜索、新建，以及按项目分组的密钥行。
// 关联文件:
//   cmd/axonhub-desktop/panel_keys.go
//   cmd/axonhub-desktop/panel_view.go
//   cmd/axonhub-desktop/panel.go
//   cmd/axonhub-desktop/locales/en.json
//   cmd/axonhub-desktop/locales/zh-Hans.json
// 遵循的 Rules:
//   .agents/rules/go-general.md
// 注意事项:
//   密钥默认打码，只有点了眼睛的那一行画明文；复制不需要先显示。

package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

// panelTab is the page the panel shows under its header.
type panelTab int

const (
	tabTraffic panelTab = iota
	tabKeys
	tabChannels
	tabModels
)

// tabBar switches pages. A page other than Traffic loads when it is
// opened and on Refresh, and is not reloaded by the timer.
func (s *shell) tabBar(c *ui.Context, p panelPalette) {
	tab := int(s.tab)
	ui.Row(c).Shrink(0).Children(func() {
		if segmented(c, p, &tab, locales.TabTraffic(), locales.TabKeys(), locales.TabChannels(), locales.TabModels()) {
			s.openTab(panelTab(tab))
		}
	})
}

func (s *shell) keysPage(c *ui.Context, p panelPalette) {
	card(c, p).Children(func() {
		connectRow(c, p, locales.BaseUrl(), false, func() {
			ui.Text(c, openAIBaseURL(s.origin)).Font("monospace").FontSize(11.5).SingleLine().Grow(1).MinWidth(0)
			if s.copyButton(c, p, "url", false).Clicked() {
				s.copyBaseURL()
			}
		})
		if lan := s.lanBaseURL(); lan != "" {
			connectRow(c, p, locales.LanUrl(), true, func() {
				ui.Text(c, lan).Font("monospace").FontSize(11.5).SingleLine().Grow(1).MinWidth(0)
				if s.copyButton(c, p, "lan", false).Clicked() {
					s.copyText(lan, "lan")
				}
			})
		}
	})
	ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.SearchField(c, &s.keys.query).Placeholder(locales.SearchKeys()).Label(locales.SearchKeys()).Grow(1).MinWidth(0)
		if textButton(c, p, locales.NewKey(), p.raised, p.text).Clicked() {
			s.keys.creating = !s.keys.creating
			s.keys.newName = ""
		}
	})
	if s.keys.creating {
		s.newKeyCard(c, p)
	}
	if s.keys.err != "" {
		s.notice(c, p, keysErrText(s.keys.err))
	}
	ui.Scroll(c).Grow(1).MinHeight(0).Gap(8).Children(func() {
		shown := filterKeys(s.keys.items, s.keys.query)
		switch {
		case len(shown) > 0:
			for _, group := range groupKeys(shown, s.keys.desktopProject) {
				s.keyGroup(c.Key(group.ProjectID), p, group)
			}
		case !s.keys.loaded:
			if s.keys.err == "" {
				skeletonRows(c, p)
			}
		case len(s.keys.items) == 0:
			emptyNote(c, p, locales.NoKeys())
		default:
			emptyNote(c, p, locales.NoKeysMatch())
		}
	})
}

func keysErrText(err string) string {
	switch err {
	case panelKeyUpdateError:
		return locales.CouldntUpdateKey()
	case panelKeyCreateError:
		return locales.CouldntCreateKey()
	default:
		return loadErrText(err)
	}
}

func (s *shell) newKeyCard(c *ui.Context, p panelPalette) {
	card(c.Key("new-key"), p).Padding(12).Gap(8).Transition(rowEnter).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			input := ui.TextInput(c, &s.keys.newName).Grow(1).MinWidth(0).FontSize(12.5).
				Placeholder(locales.KeyName()).Label(locales.KeyName())
			create := textButton(c, p, locales.Create(), p.primary, p.onPrimary).FontWeight(600).Disabled(s.keys.saving)
			if create.Clicked() || input.Submitted() {
				s.createKey()
			}
		})
		ui.Text(c, locales.NewKeyNote(desktopProjectName)).FontSize(11).TextColor(p.muted)
	})
}

func (s *shell) keyGroup(c *ui.Context, p panelPalette, group panelKeyGroup) {
	ui.Column(c).Shrink(0).Gap(6).Children(func() {
		ui.Row(c).PaddingX(6).Children(func() {
			sectionLabel(c, p, panelLabel(group.ProjectName))
		})
		card(c, p).Children(func() {
			for i, key := range group.Keys {
				s.keyRow(c.Key(key.ID), p, key, i > 0)
			}
		})
	})
}

// keyRow is the name over the masked secret, then Show, Copy and the
// switch that enables the key.
func (s *shell) keyRow(c *ui.Context, p panelPalette, key panelKey, divided bool) {
	revealed := s.keys.revealed[key.ID]
	row := ui.Row(c).MinHeight(52).Padding(8, 10, 8, 12).Gap(8).AlignItems(ui.Center)
	if divided {
		row.BorderWidth(1, 0, 0, 0).BorderColor(p.hairline)
	}
	row.Children(func() {
		ink := p.text
		if !key.Enabled {
			ink = p.muted
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconKey).Size(13, 13).TextColor(p.soft).Shrink(0)
				ui.Text(c, panelLabel(key.Name)).FontSize(12.5).FontWeight(500).TextColor(ink).SingleLine().Shrink(1)
				if key.Type != "" && key.Type != "user" {
					chip(c, p, key.Type, toneMuted)
				}
			})
			secret := maskKey(key.Secret)
			if revealed {
				secret = key.Secret
			}
			ui.Text(c, secret).Font("monospace").FontSize(11).TextColor(p.muted).SingleLine()
		})
		icon, label := iconEye, locales.ShowKey()
		if revealed {
			icon, label = iconEyeOff, locales.HideKey()
		}
		if iconButton(c.Key("eye"), p, icon, label).Clicked() {
			if s.keys.revealed == nil {
				s.keys.revealed = map[string]bool{}
			}
			s.keys.revealed[key.ID] = !revealed
		}
		if s.copyButton(c, p, "key:"+key.ID, false).Clicked() {
			s.copyKey(key)
		}
		enabled := key.Enabled
		if ui.Switch(c.Key("enabled"), &enabled).Label(locales.KeyEnabled()).Tooltip(locales.KeyEnabled()).Changed() {
			s.setKeyEnabled(key.ID, enabled)
		}
	})
}

func emptyNote(c *ui.Context, p panelPalette, message string) {
	ui.Column(c).Grow(1).Center().Padding(24).Children(func() {
		ui.Text(c, message).FontSize(12.5).TextColor(p.muted)
	})
}
