// 职责: 绘制 Channels 页：状态、启停和手动测试。
// 关联文件:
//   cmd/axonhub-desktop/panel_channels.go
//   cmd/axonhub-desktop/panel_keys_view.go
//   cmd/axonhub-desktop/panel_view.go
//   cmd/axonhub-desktop/locales/en.json
//   cmd/axonhub-desktop/locales/zh-Hans.json
// 遵循的 Rules:
//   .agents/rules/go-general.md

package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

func channelStatusText(kind string) string {
	switch kind {
	case "auto_disabled":
		return locales.AutoDisabled()
	case "disabled":
		return locales.Disabled()
	case "quota_exhausted":
		return locales.QuotaExhausted()
	case "quota_warning":
		return locales.QuotaWarning()
	default:
		return ""
	}
}

func channelsErrText(err string) string {
	if err == panelChannelUpdateError {
		return locales.CouldntUpdateChannel()
	}
	return loadErrText(err)
}

func (s *shell) channelsPage(c *ui.Context, p panelPalette) {
	ui.SearchField(c, &s.channels.query).Placeholder(locales.SearchChannels()).Label(locales.SearchChannels()).Shrink(0)
	if s.channels.err != "" {
		s.notice(c, p, channelsErrText(s.channels.err))
	}
	ui.Scroll(c).Grow(1).MinHeight(0).Gap(8).Children(func() {
		shown := filterChannels(s.channels.items, s.channels.query)
		switch {
		case len(shown) > 0:
			card(c, p).Children(func() {
				for i, channel := range shown {
					s.channelRow(c.Key(channel.ID), p, channel, i > 0)
				}
			})
			ui.Row(c).PaddingX(6).Children(func() {
				ui.Text(c, locales.TestNote()).FontSize(11).TextColor(p.muted)
			})
		case !s.channels.loaded:
			if s.channels.err == "" {
				skeletonRows(c, p)
			}
		case len(s.channels.items) == 0:
			emptyNote(c, p, locales.NoChannels())
		default:
			emptyNote(c, p, locales.NoMatch())
		}
	})
}

// channelRow is the name and status over the type and model count, with
// the last test's result under them, then Test and the enabling switch.
func (s *shell) channelRow(c *ui.Context, p panelPalette, channel panelChannel, divided bool) {
	test, tested := s.channels.tests[channel.ID]
	row := ui.Row(c).MinHeight(52).Padding(8, 10, 8, 12).Gap(8).AlignItems(ui.Center)
	if divided {
		row.BorderWidth(1, 0, 0, 0).BorderColor(p.hairline)
	}
	row.Children(func() {
		ink := p.text
		if !channel.Enabled {
			ink = p.muted
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				dotColor := p.success
				if !channel.Enabled {
					dotColor = p.muted.Alpha(0.5)
				}
				dot(c, dotColor, 7)
				ui.Text(c, panelLabel(channel.Name)).FontSize(12.5).FontWeight(500).TextColor(ink).SingleLine().Shrink(1)
				if kind, tone := channelStatus(channel); kind != "" {
					chip(c, p, channelStatusText(kind), tone)
				}
			})
			ui.Text(c, channel.Type+" · "+locales.ModelCount(formatCompact(int64(len(channel.Models))))).
				FontSize(11).TextColor(p.muted).SingleLine()
			switch {
			case tested && test.Running:
				ui.Text(c, locales.Testing()).FontSize(11).TextColor(p.warning).SingleLine()
			case tested && test.OK:
				ui.Text(c, "✓ "+formatLatency(test.LatencyMs)).FontSize(11).FontFeatures("tnum").TextColor(p.success).SingleLine()
			case tested:
				detail := locales.TestFailed()
				if test.Error != "" {
					detail += " · " + test.Error
				}
				ui.Text(c, detail).FontSize(11).TextColor(p.danger).MaxLines(2)
			case !channel.Enabled && channel.Error != "":
				ui.Text(c, channel.Error).FontSize(11).TextColor(p.danger).MaxLines(2)
			}
		})
		if textButton(c.Key("test"), p, locales.Test(), p.raised, p.text).Height(26).PaddingX(10).Radius(7).
			Disabled(test.Running).Clicked() {
			s.testChannel(channel.ID)
		}
		enabled := channel.Enabled
		if ui.Switch(c.Key("enabled"), &enabled).Label(locales.ChannelEnabled()).Tooltip(locales.ChannelEnabled()).Changed() {
			s.setChannelEnabled(channel.ID, enabled)
		}
	})
}
