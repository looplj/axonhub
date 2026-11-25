// 职责: 为插件任务执行经过现有权限系统鉴权的 GraphQL 操作。
// 关联文件:
//   internal/server/plugin/hostapi.go
//   internal/server/gql/plugin_task_graphql.go
//   internal/authz/identity/principal.go
// 注意事项:
//   手动触发的任务也使用插件安装身份，不继承触发者的权限。

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/MoeForge/moejs"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/authz/identity"
	"github.com/looplj/axonhub/internal/ent"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
)

type GraphQLExecutor func(context.Context, string, map[string]any, string) (json.RawMessage, error)

// SetGraphQLExecutor connects the shared GraphQL schema to the task runtime.
func (m *Manager) SetGraphQLExecutor(execute GraphQLExecutor) {
	m.graphqlExecutor.Store(&execute)
}

func (h *hostCall) graphql(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	query, err := h.requireString(moejs.Arg(args, 0), "query")
	if err != nil {
		return moejs.Undefined(), err
	}
	if strings.TrimSpace(query) == "" {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "query cannot be empty")
	}
	if len(query) > maxFetchRequestBody {
		return moejs.Undefined(), h.fail(hostLimitExceeded, "GraphQL query exceeds 1 MiB")
	}
	var variables map[string]any
	if value := moejs.Arg(args, 1); !value.IsUndefined() && !value.IsNull() {
		raw, err := h.rt.AppendJSONWith(nil, value, moejs.JSONOptions{Strict: true})
		if err != nil || !isJSONObject(raw) {
			return moejs.Undefined(), h.fail(hostInvalidArgument, "variables must be a JSON object")
		}
		if len(query)+len(raw) > maxFetchRequestBody {
			return moejs.Undefined(), h.fail(hostLimitExceeded, "GraphQL request exceeds 1 MiB")
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&variables); err != nil {
			return moejs.Undefined(), h.fail(hostInvalidArgument, "variables must be a JSON object")
		}
	}
	options, err := h.optionsAt(args, 2)
	if err != nil {
		return moejs.Undefined(), err
	}
	name, err := h.optionString(options, "operationName")
	if err != nil {
		return moejs.Undefined(), err
	}
	if h.manager == nil {
		return moejs.Undefined(), h.fail(hostUnavailable, "GraphQL is unavailable")
	}
	execute := h.manager.graphqlExecutor.Load()
	if execute == nil {
		return moejs.Undefined(), h.fail(hostUnavailable, "GraphQL is unavailable")
	}
	installation, err := authz.RunAsHost(h.ctx, h.manager.db, "plugin-task-graphql-grants", func(ctx context.Context) (*ent.Plugin, error) {
		var installation *ent.Plugin
		err := h.manager.runInTx(ctx, func(ctx context.Context) error {
			var err error
			installation, err = h.manager.client(ctx).Plugin.Query().Where(
				entplugin.IDEQ(h.installationID), entplugin.PluginIDEQ(h.pluginID), entplugin.EnabledEQ(true), entplugin.DeletedAtEQ(0),
			).Select(entplugin.FieldGrantedPermissions).Only(ctx)
			return ent.MaskNotFound(err)
		})
		return installation, err
	})
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	if installation == nil {
		return moejs.Undefined(), h.fail(hostPermissionDenied, "plugin installation is inactive")
	}
	// A run started before a grant change may only keep permissions it already held.
	grants := slices.DeleteFunc(installation.GrantedPermissions, func(grant string) bool {
		_, held := h.granted[grant]
		return !held
	})
	ctx := identity.ForPluginInstallation(h.ctx, h.installationID, grants)
	ctx = ent.NewContext(ctx, h.manager.db)
	raw, err := (*execute)(ctx, query, variables, name)
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	if len(raw) > maxFetchResponseBody {
		return moejs.Undefined(), h.fail(hostLimitExceeded, "GraphQL response exceeds 2 MiB")
	}
	return h.rt.ParseJSON(raw)
}
