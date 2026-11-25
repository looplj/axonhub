// 职责: 验证打包后的模型目录检查示例可以安装，运行任务并刷新顶部角标。
// 关联文件:
//   plugins/examples/model-catalog-check/plugin.js
//   plugins/examples/model-catalog-check/manifest.json
//   internal/server/gql/plugin_endpoint_test.go
//   internal/server/plugin/ui_badge.go

package gql

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/plugin"
)

func TestModelCatalogCheckExample(t *testing.T) {
	var catalog atomic.Value
	catalog.Store(`{"providers":{"openai":{"models":[{"id":"linked","canonical_model_id":"openai/linked","cost":{"input":5,"output":15}},{"id":"new-model"}]}}}`)
	fx := newPluginEndpointFixture(t, "model_catalog_check_example", &http.Client{Transport: pluginRoundTrip(func(_ *http.Request) (*http.Response, error) {
		document := catalog.Load().(string)
		status := http.StatusOK
		if document == "" {
			status = http.StatusServiceUnavailable
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(document))}, nil
	})})

	binding := objects.CatalogPrice{Provider: "openai", ModelID: "linked", Canonical: "openai/linked", Follow: true}
	fx.db.Model.Create().SetName("Local linked").SetModelID("local-linked").SetDeveloper("openai").SetIcon("").SetGroup("").
		SetSettings(&objects.ModelSettings{}).
		SetModelCard(&objects.ModelCard{
			CatalogAliases: []string{"linked"},
			Cost:           objects.ModelCardCost{Input: 1, Output: 2},
			CatalogPrices:  map[string]objects.CatalogPrice{"input": binding, "output": binding},
		}).SaveX(fx.setupCtx)

	path := filepath.Join("..", "..", "..", "plugins", "examples", "model-catalog-check")
	pack, _, err := plugin.PackDirectory(path)
	require.NoError(t, err)
	installed, err := fx.manager.Install(fx.setupCtx, pack, nil, nil)
	require.NoError(t, err)
	require.NoError(t, fx.manager.Enable(fx.setupCtx, installed.Plugin.ID, []string{"read_channels"}, nil))

	run := fmt.Sprintf(`mutation { runPluginTask(id:"gid://axonhub/Plugin/%d", key:"check") {status} }`, installed.Plugin.ID)
	check := func(status string) {
		t.Helper()
		_, body, raw := fx.postAdmin(fx.ownerToken, run)
		require.NotContains(t, body, "errors", raw)
		require.Equal(t, status, body["data"].(map[string]any)["runPluginTask"].(map[string]any)["status"], raw)
	}
	summary := func() string {
		t.Helper()
		_, body, raw := fx.postPlugin(fx.writerToken, installed.Plugin.PluginID, `{ pluginKV(key:"summary") { value } }`)
		require.NotContains(t, body, "errors", raw)
		return body["data"].(map[string]any)["pluginKV"].(map[string]any)["value"].(string)
	}
	badge := func() any {
		t.Helper()
		_, body, raw := fx.postAdmin(fx.ownerToken, `{ pluginUIEntries { actions {key badge {count}} } }`)
		require.NotContains(t, body, "errors", raw)
		actions := body["data"].(map[string]any)["pluginUIEntries"].(map[string]any)["actions"].([]any)
		require.Len(t, actions, 1)
		action := actions[0].(map[string]any)
		require.Equal(t, "catalog-check", action["key"])
		return action["badge"]
	}

	check("succeeded")
	previous := summary()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(previous), &result))
	require.Equal(t, "openai", result["provider"])
	require.NotEmpty(t, result["fetchedAt"])
	require.EqualValues(t, 1, result["available"])
	require.EqualValues(t, 1, result["changed"])
	require.Equal(t, map[string]any{"count": float64(2)}, badge())

	catalog.Store("")
	check("failed")
	require.Equal(t, previous, summary())
	require.Equal(t, map[string]any{"count": float64(2)}, badge())

	catalog.Store(`{"providers":{"openai":{"models":[{"id":"linked","canonical_model_id":"openai/linked","cost":{"input":1,"output":2}}]}}}`)
	check("succeeded")
	require.NoError(t, json.Unmarshal([]byte(summary()), &result))
	require.EqualValues(t, 0, result["available"])
	require.EqualValues(t, 0, result["changed"])
	require.Nil(t, badge())
}
