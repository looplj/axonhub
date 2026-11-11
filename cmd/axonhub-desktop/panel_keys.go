// 职责: 读取全部项目的 API Key，并在 Keys 页启用、停用和新建。
// 关联文件:
//   cmd/axonhub-desktop/panel_keys_view.go
//   cmd/axonhub-desktop/panel_key.go
//   cmd/axonhub-desktop/panel_load.go
//   internal/ent/schema/api_key.go
// 遵循的 Rules:
//   .agents/rules/go-general.md
// 注意事项:
//   列表查询不带 X-Project-ID：带上后 APIKey 的隐私规则会把结果过滤到单个项目。
//   服务端错误可能回显密钥，日志和界面只用固定文案，不写 err 原文。
//   列表按游标翻完，不在第一页截断。

package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
)

const (
	panelKeyLimit       = 200
	keyMaskHead         = 6
	keyMaskTail         = 4
	keyMaskDots         = "••••••"
	panelKeyUpdateError = "Couldn't update the key"
	panelKeyCreateError = "Couldn't create the key"
)

const panelKeysQuery = `query DesktopPanelKeys($first: Int!, $after: Cursor, $where: APIKeyWhereInput, $orderBy: APIKeyOrder) {
  apiKeys(first: $first, after: $after, where: $where, orderBy: $orderBy) {
    edges {
      node {
        id
        name
        key
        type
        status
        project {
          id
          name
        }
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}`

// panelKey is one row of the Keys page. Secret is drawn only while the
// row is revealed.
type panelKey struct {
	ID          string
	Name        string
	Secret      string
	Type        string
	Enabled     bool
	ProjectID   string
	ProjectName string
}

// panelKeyGroup is one project's keys, in the order the server returned.
type panelKeyGroup struct {
	ProjectID   string
	ProjectName string
	Keys        []panelKey
}

type panelKeyNode struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Secret  string `json:"key"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Project *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
}

type panelKeysData struct {
	APIKeys panelConnection[panelKeyNode] `json:"apiKeys"`
}

// keysState is the Keys page. It is touched on the main thread only. seq
// drops a load that an action or a newer load has superseded. busy holds
// the ids whose switch is waiting on the server.
type keysState struct {
	items    []panelKey
	loaded   bool
	loading  bool
	err      string
	query    string
	revealed map[string]bool
	busy     map[string]bool
	seq      uint64
	// desktopProject is the session's project as of the last load; its
	// group leads the list.
	desktopProject string
	creating       bool
	newName        string
	saving         bool
}

func panelKeysVariables(after string) map[string]any {
	return panelPageVariables(panelKeyLimit, after, map[string]any{
		"statusIn": []string{statusKeyEnabled, statusKeyDisabled},
	}, map[string]string{
		"field":     "CREATED_AT",
		"direction": "DESC",
	})
}

// loadPanelKeys reads the enabled and disabled keys of every project.
func loadPanelKeys(ctx context.Context, client *http.Client, origin, token string) ([]panelKey, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	global := desktopSession{Token: token}
	origin = strings.TrimRight(origin, "/")
	var keys []panelKey
	after := ""
	for {
		data, err := postGraphQL[panelKeysData](ctx, client, origin, global, "DesktopPanelKeys", panelKeysQuery, panelKeysVariables(after))
		if err != nil {
			return nil, errors.New("load api keys")
		}
		keys = append(keys, lo.Map(connectionNodes(data.APIKeys), func(node panelKeyNode, _ int) panelKey {
			return mapPanelKey(node)
		})...)
		after = nextPageCursor(data.APIKeys, after)
		if after == "" {
			return keys, nil
		}
	}
}

func mapPanelKey(node panelKeyNode) panelKey {
	key := panelKey{
		ID:      node.ID,
		Name:    node.Name,
		Secret:  node.Secret,
		Type:    node.Type,
		Enabled: node.Status == statusKeyEnabled,
	}
	if node.Project != nil {
		key.ProjectID = node.Project.ID
		key.ProjectName = node.Project.Name
	}
	return key
}

// maskKey keeps the head and tail of a secret so two keys can be told
// apart. A short secret is all dots.
func maskKey(secret string) string {
	runes := []rune(secret)
	if len(runes) <= keyMaskHead+keyMaskTail+2 {
		return keyMaskDots
	}
	return string(runes[:keyMaskHead]) + keyMaskDots + string(runes[len(runes)-keyMaskTail:])
}

// filterKeys keeps the keys whose name or project contains the query,
// ignoring case. An empty query keeps all.
func filterKeys(keys []panelKey, query string) []panelKey {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return keys
	}
	return lo.Filter(keys, func(key panelKey, _ int) bool {
		return strings.Contains(strings.ToLower(key.Name), query) ||
			strings.Contains(strings.ToLower(key.ProjectName), query)
	})
}

// groupKeys groups keys by project. The project named first leads, so the
// Desktop project stays on top; the others keep the order they arrive in.
func groupKeys(keys []panelKey, firstProjectID string) []panelKeyGroup {
	var groups []panelKeyGroup
	index := map[string]int{}
	for _, key := range keys {
		i, ok := index[key.ProjectID]
		if !ok {
			i = len(groups)
			index[key.ProjectID] = i
			groups = append(groups, panelKeyGroup{ProjectID: key.ProjectID, ProjectName: key.ProjectName})
		}
		groups[i].Keys = append(groups[i].Keys, key)
	}
	if i, ok := index[firstProjectID]; ok && i > 0 {
		first := groups[i]
		copy(groups[1:i+1], groups[:i])
		groups[0] = first
	}
	return groups
}

func keyStatusVariables(id string, enabled bool) map[string]any {
	return map[string]any{
		"id":     id,
		"status": lo.Ternary(enabled, statusKeyEnabled, statusKeyDisabled),
	}
}

func createKeyVariables(projectID, name string) map[string]any {
	return map[string]any{
		"input": map[string]string{
			"name":      name,
			"projectID": projectID,
		},
	}
}

// loadKeysAsync reads the keys off the main thread. The page keeps the
// last list while it loads and when a load fails.
func (s *shell) loadKeysAsync() {
	if s.win == nil || s.offline {
		return
	}
	s.keys.seq++
	seq := s.keys.seq
	s.keys.loading = true
	origin := s.origin
	go func() {
		defer recoverLog("panel keys")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		session, err := s.currentSession(ctx)
		var keys []panelKey
		if err == nil {
			keys, err = loadPanelKeys(ctx, nil, origin, session.Token)
		}
		s.win.Update(func() {
			if seq != s.keys.seq {
				return
			}
			s.keys.loading = false
			if err != nil {
				log.Warn(context.Background(), "desktop panel keys not loaded")
				s.keys.err = panelRefreshError
				return
			}
			s.keys.items = mergeBusyKeys(keys, s.keys.items, s.keys.busy)
			s.keys.desktopProject = session.ProjectID
			s.keys.loaded = true
			s.keys.err = ""
		})
	}()
}

// setKeyEnabled enables or disables one key. The row shows the new state
// at once and goes back when the server refuses.
func (s *shell) setKeyEnabled(id string, enabled bool) {
	if s.win == nil || s.offline || s.keys.busy[id] {
		return
	}
	if s.keys.busy == nil {
		s.keys.busy = map[string]bool{}
	}
	// Drop a list load already in flight. Its result is older than this
	// switch and would paint the previous state back.
	s.keys.seq++
	s.keys.loading = false
	s.keys.busy[id] = true
	s.markKeyEnabled(id, enabled)
	origin := s.origin
	go func() {
		defer recoverLog("panel key status")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		session, err := s.currentSession(ctx)
		if err == nil {
			global := desktopSession{Token: session.Token}
			_, err = postGraphQL[restoredDesktopKey](ctx, &http.Client{Timeout: 15 * time.Second}, origin, global,
				"UpdateAPIKeyStatus", restoreDesktopKeyMutation, keyStatusVariables(id, enabled))
		}
		s.win.Update(func() {
			delete(s.keys.busy, id)
			if err != nil {
				log.Warn(context.Background(), "desktop panel key status not saved")
				s.markKeyEnabled(id, !enabled)
				s.keys.err = panelKeyUpdateError
				return
			}
			s.keys.err = ""
			s.loadKeysAsync()
		})
	}()
}

func (s *shell) markKeyEnabled(id string, enabled bool) {
	for i := range s.keys.items {
		if s.keys.items[i].ID == id {
			s.keys.items[i].Enabled = enabled
		}
	}
}

// mergeBusyKeys keeps the switch state of a key whose save is still in
// flight, so a list that was read before the save lands cannot overwrite it.
func mergeBusyKeys(incoming, current []panelKey, busy map[string]bool) []panelKey {
	if len(busy) == 0 {
		return incoming
	}
	local := make(map[string]bool, len(busy))
	for _, item := range current {
		if busy[item.ID] {
			local[item.ID] = item.Enabled
		}
	}
	return lo.Map(incoming, func(item panelKey, _ int) panelKey {
		if enabled, ok := local[item.ID]; ok {
			item.Enabled = enabled
		}
		return item
	})
}

// createKey adds a key with the typed name to the Desktop project, then
// reloads the list.
func (s *shell) createKey() {
	name := strings.TrimSpace(s.keys.newName)
	if s.win == nil || s.offline || s.keys.saving || name == "" {
		return
	}
	s.keys.saving = true
	origin := s.origin
	go func() {
		defer recoverLog("panel key create")
		ctx, cancel := context.WithTimeout(context.Background(), panelLoadTimeout)
		defer cancel()
		session, err := s.currentSession(ctx)
		if err == nil {
			_, err = postGraphQL[createdDesktopKey](ctx, &http.Client{Timeout: 15 * time.Second}, origin, session,
				"CreateAPIKey", createDesktopKeyMutation, createKeyVariables(session.ProjectID, name))
		}
		s.win.Update(func() {
			s.keys.saving = false
			if err != nil {
				log.Warn(context.Background(), "desktop panel key not created")
				s.keys.err = panelKeyCreateError
				return
			}
			s.keys.creating = false
			s.keys.newName = ""
			s.loadKeysAsync()
		})
	}()
}

// copyKey writes one key's secret and marks its Copy button.
func (s *shell) copyKey(key panelKey) {
	if key.Secret == "" {
		return
	}
	s.copyText(key.Secret, "key:"+key.ID)
}
