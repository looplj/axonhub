// 职责: 让已获授权的插件任务检查模型目录。
// 关联文件:
//   internal/server/plugin/hostapi.go
//   internal/server/plugin/catalog_check_summary.go
//   internal/server/biz/model_catalog_preview.go
// 注意事项:
//   脚本只触发检查，成功摘要随本次任务结果一起提交。

package plugin

import (
	"context"

	"github.com/MoeForge/moejs"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
)

type catalogCheckOptions struct {
	providers    []string
	ignorePrefix bool
}

func (h *hostCall) catalogCheckOptions(value moejs.Value) (catalogCheckOptions, error) {
	var options catalogCheckOptions
	if value.IsUndefined() {
		return options, nil
	}
	fields, err := h.jsonObject(value, "model catalog options")
	if err != nil {
		return options, err
	}
	for key, value := range fields {
		switch key {
		case "providers":
			ids, ok := value.([]any)
			if !ok {
				return options, h.fail(hostInvalidArgument, "providers must be a list of provider IDs")
			}
			for _, id := range ids {
				provider, ok := id.(string)
				if !ok || provider == "" {
					return options, h.fail(hostInvalidArgument, "providers must contain non-empty strings")
				}
				options.providers = append(options.providers, provider)
			}
		case "ignorePrefix":
			enabled, ok := value.(bool)
			if !ok {
				return options, h.fail(hostInvalidArgument, "ignorePrefix must be a boolean")
			}
			options.ignorePrefix = enabled
		default:
			return options, h.fail(hostInvalidArgument, "unknown model catalog option")
		}
	}
	return options, nil
}

func (h *hostCall) checkModelCatalog(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	// A failed recheck invalidates any earlier result from this run, even if the script catches the error.
	h.result.catalogCheck = nil
	if err := h.requirePermission(permReadChannels); err != nil {
		return moejs.Undefined(), err
	}
	options, err := h.catalogCheckOptions(moejs.Arg(args, 0))
	if err != nil {
		return moejs.Undefined(), err
	}
	counts, err := authz.RunAsHost(h.ctx, h.manager.db, "plugin-model-catalog-check", func(ctx context.Context) (biz.CatalogCheckCounts, error) {
		// Recheck the grant on the primary, then release the transaction before contacting the catalog.
		err := h.manager.runInTx(ctx, func(txCtx context.Context) error {
			return h.manager.requireCatalogGrant(txCtx, h.installationID, h.pluginID, false)
		})
		if err != nil {
			return biz.CatalogCheckCounts{}, err
		}
		return h.manager.models.CheckModelCatalog(ctx, options.providers, options.ignorePrefix)
	})
	if coded, ok := xerrors.IsCodedError(err); ok && coded.Code == xerrors.ErrCodeForbidden {
		return moejs.Undefined(), h.fail(hostPermissionDenied, coded.Message)
	}
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	h.result.catalogCheck = &pendingCatalogCheck{installationID: h.installationID, pluginID: h.pluginID, counts: counts}
	return moejs.Undefined(), nil
}
