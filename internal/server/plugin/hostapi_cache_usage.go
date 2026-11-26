// 职责: 在任务运行时公开受安装授权限制的缓存读取指标。
// 关联文件:
//
//	internal/server/plugin/hostapi.go
//	internal/server/plugin/manifest.go
//	internal/server/biz/plugin_cache_usage.go
//
// 注意事项:
//
//	每次调用都从主库复核安装授权，只向脚本返回固定查询的 DTO。
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/server/biz"
)

const permReadCacheUsage = "read_cache_usage"

func (h *hostCall) attachCacheUsageAPI(ctxObj *moejs.Object) error {
	usage := h.rt.Realm().NewObject()
	if err := defineHostFn(h.rt, usage, "cacheStats", 1, h.cacheStats); err != nil {
		return err
	}
	return defineValue(h.rt, ctxObj, "usage", engine.ObjectValue(usage))
}

func (h *hostCall) cacheStats(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if _, granted := h.granted[permReadCacheUsage]; !granted {
		return moejs.Undefined(), h.fail(hostPermissionDenied, "read_cache_usage permission is required")
	}
	raw, err := h.rt.AppendJSONWith(nil, moejs.Arg(args, 0), moejs.JSONOptions{Strict: true})
	if err != nil || !isJSONObject(raw) {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "cacheStats options must be a JSON object")
	}
	if len(raw) > 16<<10 {
		return moejs.Undefined(), h.fail(hostLimitExceeded, "cacheStats options exceed 16 KiB")
	}
	var query biz.CacheUsageQuery
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&query); err != nil {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "invalid cacheStats options")
	}
	page, err := h.readCacheUsage(query)
	if errors.Is(err, biz.ErrInvalidCacheUsageQuery) {
		return moejs.Undefined(), h.fail(hostInvalidArgument, err.Error())
	}
	if errors.Is(err, privacy.Deny) {
		return moejs.Undefined(), h.fail(hostPermissionDenied, "read_cache_usage permission is no longer granted")
	}
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	result, err := json.Marshal(page)
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	if len(result) > maxFetchResponseBody {
		return moejs.Undefined(), h.fail(hostLimitExceeded, "cacheStats response exceeds 2 MiB")
	}
	return h.rt.ParseJSON(result)
}

func (h *hostCall) readCacheUsage(query biz.CacheUsageQuery) (biz.CacheUsagePage, error) {
	return authz.RunAsHost(h.ctx, h.manager.db, "plugin-task-cache-usage", func(hostCtx context.Context) (biz.CacheUsagePage, error) {
		var page biz.CacheUsagePage
		err := h.manager.runInTx(hostCtx, func(ctx context.Context) error {
			client := h.manager.client(ctx)
			install, err := client.Plugin.Query().Where(entplugin.IDEQ(h.installationID), entplugin.PluginIDEQ(h.pluginID),
				entplugin.EnabledEQ(true), entplugin.DeletedAtEQ(0)).Select(entplugin.FieldGrantedPermissions).Only(ctx)
			if ent.IsNotFound(err) || (err == nil && !slices.Contains(install.GrantedPermissions, permReadCacheUsage)) {
				return privacy.Denyf("read_cache_usage permission is no longer granted")
			}
			if err != nil {
				return err
			}
			page, err = biz.QueryCacheUsage(ctx, client, query)
			return err
		})
		return page, err
	})
}
