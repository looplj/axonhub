// 职责: 让已获授权的插件任务检查模型目录。
// 关联文件:
//   internal/server/plugin/hostapi.go
//   internal/server/plugin/catalog_check_summary.go
//   internal/server/biz/model_catalog_preview.go
// 注意事项:
//   脚本只触发检查，成功摘要随本次任务结果一起提交。

package plugin

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/MoeForge/moejs"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
)

func (h *hostCall) checkModelCatalog(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requirePermission(permReadChannels); err != nil {
		return moejs.Undefined(), err
	}
	var options struct {
		Providers    []string `json:"providers"`
		IgnorePrefix bool     `json:"ignorePrefix"`
	}
	opts, err := h.optionsAt(args, 0)
	if err != nil {
		return moejs.Undefined(), err
	}
	if !opts.IsUndefined() {
		raw, err := h.rt.AppendJSONWith(nil, opts, moejs.JSONOptions{Strict: true})
		if err != nil {
			return moejs.Undefined(), h.fail(hostInvalidArgument, "invalid model catalog options")
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if !isJSONObject(raw) || decoder.Decode(&options) != nil {
			return moejs.Undefined(), h.fail(hostInvalidArgument, "invalid model catalog options")
		}
	}
	counts, err := authz.RunAsHost(h.ctx, h.manager.db, "plugin-model-catalog-check", func(ctx context.Context) (biz.CatalogCheckCounts, error) {
		// Recheck the grant on the primary, then release the transaction before contacting the catalog.
		err := h.manager.runInTx(ctx, func(txCtx context.Context) error {
			return h.manager.requireCatalogGrant(txCtx, h.installationID, h.pluginID, false)
		})
		if err != nil {
			return biz.CatalogCheckCounts{}, err
		}
		return h.manager.models.CheckModelCatalog(ctx, options.Providers, options.IgnorePrefix)
	})
	if coded, ok := xerrors.IsCodedError(err); ok && coded.Code == xerrors.ErrCodeForbidden {
		return moejs.Undefined(), h.fail(hostPermissionDenied, coded.Message)
	}
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	h.result.catalogCheck = &pendingCatalogCheck{installationID: h.installationID, counts: counts}
	return moejs.Undefined(), nil
}
