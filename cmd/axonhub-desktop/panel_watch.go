// 职责: 面板隐藏时在后台低频检查自动停用的渠道和耗尽的配额，并发系统通知。
// 关联文件:
//   cmd/axonhub-desktop/panel_channels.go
//   cmd/axonhub-desktop/panel_prefs.go
//   cmd/axonhub-desktop/panel_settings.go
//   cmd/axonhub-desktop/locales/en.json
//   cmd/axonhub-desktop/locales/zh-Hans.json
// 遵循的 Rules:
//   .agents/rules/go-general.md
// 注意事项:
//   默认关闭，只有设置里打开后才启动定时器。
//   检查只读本地数据库里已有的记录，不测试渠道、不触发配额采集，不产生上游请求。
//   首次检查只记基线，不为启动前就存在的停用渠道或耗尽配额发通知。
//   一次最多发 3 条，其余留到下一次检查；没发出去的不记成已报过。

package main

import (
	"context"
	"time"

	"github.com/egoist/mygo"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
	"github.com/looplj/axonhub/internal/log"
)

const (
	watchEvery     = time.Minute
	alertChannel   = "channel"
	alertQuota     = "quota"
	watchAlertsMax = 3
)

// watchAlert is one notification to show. Name is the channel name.
type watchAlert struct {
	Kind string
	ID   string
	Name string
}

// watchState remembers which channels were already reported, so a
// condition that lasts is reported once. Pending holds alerts that did
// not fit in the last batch. It is touched on the main thread only.
type watchState struct {
	primed    bool
	down      map[string]bool
	exhausted map[string]bool
	pending   []watchAlert
	cancel    context.CancelFunc
}

// alerts turns one check into the notifications it calls for. A channel
// is reported when it becomes auto-disabled or its quota becomes
// exhausted, and again only after it has recovered in between. Alerts
// past the batch size stay pending, and a pending alert is dropped when
// the condition clears before it is shown.
func (w *watchState) alerts(channels []panelChannel) []watchAlert {
	down, exhausted := map[string]bool{}, map[string]bool{}
	var fresh []watchAlert
	for _, channel := range channels {
		if !channel.Enabled && channel.AutoDisabled {
			down[channel.ID] = true
			if w.primed && !w.down[channel.ID] && !alertPending(w.pending, alertChannel, channel.ID) {
				fresh = append(fresh, watchAlert{Kind: alertChannel, ID: channel.ID, Name: channel.Name})
			}
		}
		if channel.Enabled && channel.Quota == quotaExhausted {
			exhausted[channel.ID] = true
			if w.primed && !w.exhausted[channel.ID] && !alertPending(w.pending, alertQuota, channel.ID) {
				fresh = append(fresh, watchAlert{Kind: alertQuota, ID: channel.ID, Name: channel.Name})
			}
		}
	}
	pending := lo.Filter(append(append([]watchAlert{}, w.pending...), fresh...), func(alert watchAlert, _ int) bool {
		switch alert.Kind {
		case alertChannel:
			return down[alert.ID]
		case alertQuota:
			return exhausted[alert.ID]
		default:
			return false
		}
	})
	shown := lo.Slice(pending, 0, watchAlertsMax)
	w.pending = lo.Slice(pending, watchAlertsMax, len(pending))
	w.primed = true
	w.down = reportedIDs(down, w.pending, alertChannel)
	w.exhausted = reportedIDs(exhausted, w.pending, alertQuota)
	return shown
}

func alertPending(pending []watchAlert, kind, id string) bool {
	return lo.ContainsBy(pending, func(alert watchAlert) bool {
		return alert.Kind == kind && alert.ID == id
	})
}

// reportedIDs marks every current condition except those still waiting
// to be shown, so a later check does not treat them as already sent.
func reportedIDs(current map[string]bool, pending []watchAlert, kind string) map[string]bool {
	reported := map[string]bool{}
	for id := range current {
		if !alertPending(pending, kind, id) {
			reported[id] = true
		}
	}
	return reported
}

func alertText(alert watchAlert) (title, body string) {
	if alert.Kind == alertQuota {
		return locales.AlertQuota(), alert.Name
	}
	return locales.AlertChannelDown(), alert.Name
}

// setNotify turns channel and quota alerts on or off and saves the choice.
func (s *shell) setNotify(on bool) {
	s.prefs.Notify = on
	s.savePrefs()
	if on {
		s.startWatch()
	} else {
		s.stopWatch()
	}
}

// startWatch checks once a minute until stopWatch. It runs whether or
// not the panel is visible, and only while alerts are on.
func (s *shell) startWatch() {
	if s.watch.cancel != nil || s.offline || s.win == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.watch = watchState{cancel: cancel}
	origin := s.origin
	go func() {
		defer recoverLog("panel watch")
		ticker := time.NewTicker(watchEvery)
		defer ticker.Stop()
		s.checkWatch(ctx, origin)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkWatch(ctx, origin)
			}
		}
	}()
}

func (s *shell) stopWatch() {
	if s.watch.cancel == nil {
		return
	}
	s.watch.cancel()
	s.watch = watchState{}
}

// checkWatch runs one check off the main thread and shows its alerts
// there. A failed check leaves the remembered channels unchanged.
func (s *shell) checkWatch(ctx context.Context, origin string) {
	loadCtx, cancel := context.WithTimeout(ctx, panelLoadTimeout)
	defer cancel()
	session, err := s.currentSession(loadCtx)
	var channels []panelChannel
	if err == nil {
		channels, err = loadPanelChannels(loadCtx, nil, origin, session.Token)
	}
	if err != nil {
		if ctx.Err() == nil {
			log.Warn(context.Background(), "desktop channel check failed", log.Cause(err))
		}
		return
	}
	s.win.Update(func() {
		// stopWatch cancels ctx and clears the state; a check that was
		// already in flight must not write into the next watch.
		if ctx.Err() != nil {
			return
		}
		for _, alert := range s.watch.alerts(channels) {
			s.notify(alert)
		}
	})
}

func (s *shell) notify(alert watchAlert) {
	title, body := alertText(alert)
	n := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body})
	n.OnClick(s.show)
	go func() {
		defer recoverLog("panel notification")
		if err := n.Show(); err != nil {
			log.Warn(context.Background(), "desktop notification not shown", log.Cause(err))
		}
	}()
}
