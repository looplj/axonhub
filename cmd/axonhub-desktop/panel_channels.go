// 职责: 读取 Channels 页的渠道列表，并在面板内启停、手动测试渠道。
// 关联文件:
//   cmd/axonhub-desktop/panel_channels_view.go
//   cmd/axonhub-desktop/panel_load.go
//   internal/server/gql/axonhub.graphql
//   internal/server/orchestrator/tester.go
// 遵循的 Rules:
//   .agents/rules/go-general.md
// 注意事项:
//   面板只读已入库的 providerQuotaStatus，从不触发配额检查。
//   testChannel 会向上游发一次真实请求，只在用户点击 Test 时调用，不放进任何定时器。
//   列表按游标翻完，不在第一页截断。后台提醒走同一份列表。

package main

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
)

const (
	panelChannelLimit       = 200
	panelChannelTestTimeout = 60 * time.Second
	panelChannelUpdateError = "Couldn't update the channel"
	quotaWarning            = "warning"
	quotaExhausted          = "exhausted"
)

const panelChannelsQuery = `query DesktopPanelChannels($first: Int!, $after: Cursor, $where: ChannelWhereInput, $orderBy: ChannelOrder) {
  channels(first: $first, after: $after, where: $where, orderBy: $orderBy) {
    edges {
      node {
        id
        name
        type
        status
        errorMessage
        autoDisabledAt
        supportedModels
        providerQuotaStatus {
          status
        }
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}`

const updateChannelStatusMutation = `mutation UpdateChannelStatus($id: ID!, $status: ChannelStatus!) {
  updateChannelStatus(id: $id, status: $status) {
    id
    status
  }
}`

const testChannelMutation = `mutation TestChannel($input: TestChannelInput!) {
  testChannel(input: $input) {
    latency
    success
    error
  }
}`

// panelChannel is one row of the Channels page. Quota is the stored
// provider quota status, empty when the server collects none for it.
type panelChannel struct {
	ID           string
	Name         string
	Type         string
	Enabled      bool
	AutoDisabled bool
	Error        string
	Models       []string
	Quota        string
}

// channelTest is the last manual test of one channel.
type channelTest struct {
	Running   bool
	OK        bool
	LatencyMs int
	Error     string
}

type panelChannelNode struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	ErrorMessage    *string    `json:"errorMessage"`
	AutoDisabledAt  *time.Time `json:"autoDisabledAt"`
	SupportedModels []string   `json:"supportedModels"`
	Quota           *struct {
		Status string `json:"status"`
	} `json:"providerQuotaStatus"`
}

type panelChannelsData struct {
	Channels panelConnection[panelChannelNode] `json:"channels"`
}

type testChannelData struct {
	TestChannel struct {
		Latency float64 `json:"latency"`
		Success bool    `json:"success"`
		Error   *string `json:"error"`
	} `json:"testChannel"`
}

// channelsState is the Channels page. It is touched on the main thread only.
type channelsState struct {
	items  []panelChannel
	loaded bool
	err    string
	query  string
	busy   map[string]bool
	tests  map[string]channelTest
	seq    uint64
}

func panelChannelsVariables(after string) map[string]any {
	return panelPageVariables(panelChannelLimit, after, map[string]any{
		"statusIn": []string{statusKeyEnabled, statusKeyDisabled},
	}, map[string]string{
		"field":     "ORDERING_WEIGHT",
		"direction": "DESC",
	})
}

func loadPanelChannels(ctx context.Context, client *http.Client, origin, token string) ([]panelChannel, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	global := desktopSession{Token: token}
	origin = strings.TrimRight(origin, "/")
	var channels []panelChannel
	after := ""
	for {
		data, err := postGraphQL[panelChannelsData](ctx, client, origin, global, "DesktopPanelChannels", panelChannelsQuery, panelChannelsVariables(after))
		if err != nil {
			return nil, err
		}
		channels = append(channels, lo.Map(connectionNodes(data.Channels), func(node panelChannelNode, _ int) panelChannel {
			return mapPanelChannel(node)
		})...)
		after = nextPageCursor(data.Channels, after)
		if after == "" {
			return channels, nil
		}
	}
}

func mapPanelChannel(node panelChannelNode) panelChannel {
	channel := panelChannel{
		ID:           node.ID,
		Name:         node.Name,
		Type:         node.Type,
		Enabled:      node.Status == statusKeyEnabled,
		AutoDisabled: node.AutoDisabledAt != nil,
		Error:        strings.TrimSpace(lo.FromPtr(node.ErrorMessage)),
		Models:       node.SupportedModels,
	}
	if node.Quota != nil {
		channel.Quota = node.Quota.Status
	}
	return channel
}

// channelStatus is the chip beside a channel's name. An enabled channel
// with nothing to report has none.
func channelStatus(channel panelChannel) (kind string, tone statusTone) {
	switch {
	case !channel.Enabled && channel.AutoDisabled:
		return "auto_disabled", toneDanger
	case !channel.Enabled:
		return "disabled", toneMuted
	case channel.Quota == quotaExhausted:
		return "quota_exhausted", toneDanger
	case channel.Quota == quotaWarning:
		return "quota_warning", toneActive
	default:
		return "", toneMuted
	}
}

func filterChannels(channels []panelChannel, query string) []panelChannel {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return channels
	}
	return lo.Filter(channels, func(channel panelChannel, _ int) bool {
		return strings.Contains(strings.ToLower(channel.Name), query) ||
			strings.Contains(strings.ToLower(channel.Type), query)
	})
}

func (s *shell) loadChannelsAsync() {
	if s.win == nil || s.offline {
		return
	}
	s.channels.seq++
	seq := s.channels.seq
	origin := s.origin
	go func() {
		defer recoverLog("panel channels")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		session, err := s.currentSession(ctx)
		var channels []panelChannel
		if err == nil {
			channels, err = loadPanelChannels(ctx, nil, origin, session.Token)
		}
		s.win.Update(func() {
			if seq != s.channels.seq {
				return
			}
			if err != nil {
				log.Warn(context.Background(), "desktop panel channels not loaded", log.Cause(err))
				s.channels.err = panelRefreshError
				return
			}
			s.channels.items = mergeBusyChannels(channels, s.channels.items, s.channels.busy)
			s.channels.loaded = true
			s.channels.err = ""
		})
	}()
}

// setChannelEnabled enables or disables one channel. The row shows the
// new state at once and goes back when the server refuses.
func (s *shell) setChannelEnabled(id string, enabled bool) {
	if s.win == nil || s.offline || s.channels.busy[id] {
		return
	}
	if s.channels.busy == nil {
		s.channels.busy = map[string]bool{}
	}
	// Drop a list load already in flight. Its result is older than this
	// switch and would paint the previous state back.
	s.channels.seq++
	s.channels.busy[id] = true
	s.markChannelEnabled(id, enabled)
	origin := s.origin
	go func() {
		defer recoverLog("panel channel status")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		session, err := s.currentSession(ctx)
		if err == nil {
			_, err = postGraphQL[map[string]any](ctx, &http.Client{Timeout: 15 * time.Second}, origin, desktopSession{Token: session.Token},
				"UpdateChannelStatus", updateChannelStatusMutation, keyStatusVariables(id, enabled))
		}
		s.win.Update(func() {
			delete(s.channels.busy, id)
			if err != nil {
				log.Warn(context.Background(), "desktop panel channel status not saved", log.Cause(err))
				s.markChannelEnabled(id, !enabled)
				s.channels.err = panelChannelUpdateError
				return
			}
			s.channels.err = ""
			s.loadChannelsAsync()
		})
	}()
}

func (s *shell) markChannelEnabled(id string, enabled bool) {
	for i := range s.channels.items {
		if s.channels.items[i].ID == id {
			s.channels.items[i].Enabled = enabled
			if enabled {
				s.channels.items[i].AutoDisabled = false
			}
		}
	}
}

// mergeBusyChannels keeps the switch state of a channel whose save is
// still in flight, so a list read before the save lands cannot overwrite it.
func mergeBusyChannels(incoming, current []panelChannel, busy map[string]bool) []panelChannel {
	if len(busy) == 0 {
		return incoming
	}
	local := make(map[string]panelChannel, len(busy))
	for _, item := range current {
		if busy[item.ID] {
			local[item.ID] = item
		}
	}
	return lo.Map(incoming, func(item panelChannel, _ int) panelChannel {
		if kept, ok := local[item.ID]; ok {
			item.Enabled = kept.Enabled
			item.AutoDisabled = kept.AutoDisabled
		}
		return item
	})
}

// testChannel sends the channel's own test request once, because the
// user clicked Test.
func (s *shell) testChannel(id string) {
	if s.win == nil || s.offline || s.channels.tests[id].Running {
		return
	}
	if s.channels.tests == nil {
		s.channels.tests = map[string]channelTest{}
	}
	s.channels.tests[id] = channelTest{Running: true}
	origin := s.origin
	go func() {
		defer recoverLog("panel channel test")
		ctx, cancel := context.WithTimeout(context.Background(), panelChannelTestTimeout)
		defer cancel()
		result := runChannelTest(ctx, func() (desktopSession, error) { return s.currentSession(ctx) }, origin, id)
		s.win.Update(func() { s.channels.tests[id] = result })
	}()
}

func runChannelTest(ctx context.Context, session func() (desktopSession, error), origin, id string) channelTest {
	current, err := session()
	if err != nil {
		return channelTest{Error: err.Error()}
	}
	data, err := postGraphQL[testChannelData](ctx, &http.Client{Timeout: panelChannelTestTimeout}, origin, desktopSession{Token: current.Token},
		"TestChannel", testChannelMutation, map[string]any{"input": map[string]any{"channelID": id}})
	if err != nil {
		return channelTest{Error: err.Error()}
	}
	return mapChannelTest(data)
}

func mapChannelTest(data testChannelData) channelTest {
	result := data.TestChannel
	return channelTest{
		OK:        result.Success,
		LatencyMs: int(math.Round(result.Latency * 1000)),
		Error:     strings.TrimSpace(lo.FromPtr(result.Error)),
	}
}
