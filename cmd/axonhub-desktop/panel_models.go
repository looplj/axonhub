// 职责: Models 页：读取 Model 实体，按名称速查并复制模型 ID。
// 关联文件:
//   cmd/axonhub-desktop/panel_load.go
//   cmd/axonhub-desktop/panel_view.go
//   internal/ent/schema/model.go
//   internal/server/gql/model.graphql
//   cmd/axonhub-desktop/locales/en.json
//   cmd/axonhub-desktop/locales/zh-Hans.json
// 遵循的 Rules:
//   .agents/rules/go-general.md
// 注意事项:
//   列表来自 models 实体，不是渠道的 supportedModels：只有在后台建了 Model 的才会出现。
//   列表按游标翻完，不在第一页截断。

package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
	"github.com/looplj/axonhub/internal/log"
)

const (
	panelModelLimit = 200
	modelTypeChat   = "chat"
)

const panelModelsQuery = `query DesktopPanelModels($first: Int!, $after: Cursor, $where: ModelWhereInput, $orderBy: ModelOrder) {
  models(first: $first, after: $after, where: $where, orderBy: $orderBy) {
    edges {
      node {
        id
        modelID
        name
        developer
        type
        status
        associatedChannelCount
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}`

// panelModel is one row of the Models page. ModelID is what a client
// puts in its request. Channels counts the channels that serve it.
type panelModel struct {
	ID        string
	ModelID   string
	Name      string
	Developer string
	Type      string
	Enabled   bool
	Channels  int
}

type panelModelNode struct {
	ID        string `json:"id"`
	ModelID   string `json:"modelID"`
	Name      string `json:"name"`
	Developer string `json:"developer"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Channels  int    `json:"associatedChannelCount"`
}

type panelModelsData struct {
	Models panelConnection[panelModelNode] `json:"models"`
}

// modelsState is the Models page. It is touched on the main thread only.
type modelsState struct {
	items  []panelModel
	loaded bool
	err    string
	query  string
	seq    uint64
}

func panelModelsVariables(after string) map[string]any {
	return panelPageVariables(panelModelLimit, after, map[string]any{
		"statusIn": []string{statusKeyEnabled, statusKeyDisabled},
	}, map[string]string{
		"field":     "NAME",
		"direction": "ASC",
	})
}

func loadPanelModels(ctx context.Context, client *http.Client, origin, token string) ([]panelModel, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	origin = strings.TrimRight(origin, "/")
	var models []panelModel
	after := ""
	for {
		data, err := postGraphQL[panelModelsData](ctx, client, origin, desktopSession{Token: token}, "DesktopPanelModels", panelModelsQuery, panelModelsVariables(after))
		if err != nil {
			return nil, err
		}
		models = append(models, lo.Map(connectionNodes(data.Models), func(node panelModelNode, _ int) panelModel {
			return panelModel{
				ID:        node.ID,
				ModelID:   node.ModelID,
				Name:      node.Name,
				Developer: node.Developer,
				Type:      node.Type,
				Enabled:   node.Status == statusKeyEnabled,
				Channels:  node.Channels,
			}
		})...)
		after = nextPageCursor(data.Models, after)
		if after == "" {
			return models, nil
		}
	}
}

// filterModels keeps the models whose id, name or developer contains the
// query, ignoring case. An empty query keeps all.
func filterModels(models []panelModel, query string) []panelModel {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return models
	}
	return lo.Filter(models, func(model panelModel, _ int) bool {
		return lo.SomeBy([]string{model.ModelID, model.Name, model.Developer}, func(field string) bool {
			return strings.Contains(strings.ToLower(field), query)
		})
	})
}

// modelSummary is the line under a model id: its name when that differs
// from the id, its developer, and how many channels serve it.
func modelSummary(model panelModel, channels string) string {
	parts := []string{}
	if name := strings.TrimSpace(model.Name); name != "" && name != model.ModelID {
		parts = append(parts, name)
	}
	if developer := strings.TrimSpace(model.Developer); developer != "" {
		parts = append(parts, developer)
	}
	return strings.Join(append(parts, channels), " · ")
}

func (s *shell) loadModelsAsync() {
	if s.win == nil || s.offline {
		return
	}
	s.models.seq++
	seq := s.models.seq
	origin := s.origin
	go func() {
		defer recoverLog("panel models")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		session, err := s.currentSession(ctx)
		var models []panelModel
		if err == nil {
			models, err = loadPanelModels(ctx, nil, origin, session.Token)
		}
		s.win.Update(func() {
			if seq != s.models.seq {
				return
			}
			if err != nil {
				log.Warn(context.Background(), "desktop panel models not loaded", log.Cause(err))
				s.models.err = panelRefreshError
				return
			}
			s.models.items = models
			s.models.loaded = true
			s.models.err = ""
		})
	}()
}

func (s *shell) modelsPage(c *ui.Context, p panelPalette) {
	ui.SearchField(c, &s.models.query).Placeholder(locales.SearchModels()).Label(locales.SearchModels()).Shrink(0)
	if s.models.err != "" {
		s.notice(c, p, loadErrText(s.models.err))
	}
	ui.Scroll(c).Grow(1).MinHeight(0).Gap(8).Children(func() {
		shown := filterModels(s.models.items, s.models.query)
		switch {
		case len(shown) > 0:
			card(c, p).Children(func() {
				for i, model := range shown {
					s.modelRow(c.Key(model.ID), p, model, i > 0)
				}
			})
		case !s.models.loaded:
			if s.models.err == "" {
				skeletonRows(c, p)
			}
		case len(s.models.items) == 0:
			emptyNote(c, p, locales.NoModels())
		default:
			emptyNote(c, p, locales.NoMatch())
		}
	})
}

// modelRow is the model id over its name, developer and channel count.
// Copy writes the id.
func (s *shell) modelRow(c *ui.Context, p panelPalette, model panelModel, divided bool) {
	row := ui.Row(c).MinHeight(46).Padding(6, 10, 6, 12).Gap(8).AlignItems(ui.Center)
	if divided {
		row.BorderWidth(1, 0, 0, 0).BorderColor(p.hairline)
	}
	row.Children(func() {
		ink := p.text
		if !model.Enabled {
			ink = p.muted
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, model.ModelID).Font("monospace").FontSize(12).TextColor(ink).SingleLine().Shrink(1)
				if model.Type != "" && model.Type != modelTypeChat {
					chip(c, p, model.Type, toneMuted)
				}
				if !model.Enabled {
					chip(c, p, locales.Disabled(), toneMuted)
				}
			})
			ui.Text(c, modelSummary(model, locales.ChannelCount(formatCompact(int64(model.Channels))))).
				FontSize(11).TextColor(p.muted).SingleLine()
		})
		what := "model:" + model.ID
		if s.copyButton(c, p, what, false).Clicked() {
			s.copyText(model.ModelID, what)
		}
	})
}
