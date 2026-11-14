package plugin

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
)

const (
	permReadChannels  = "read_channels"
	permWriteChannels = "write_channels"
	permReadRequests  = "read_requests"
	permWebhook       = "webhook"
)

func permissionSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return set
}

func (h *hostCall) attachEventAPI(ctxObj *moejs.Object) error {
	if h.export != onEventExport {
		return nil
	}
	realm := h.rt.Realm()
	counter := realm.NewObject()
	if err := defineHostFn(h.rt, counter, "add", 2, h.counterAdd); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, counter, "get", 2, h.counterGet); err != nil {
		return err
	}

	channels := realm.NewObject()
	if err := defineHostFn(h.rt, channels, "get", 1, h.channelGet); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, channels, "list", 0, h.channelList); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, channels, "disable", 1, h.channelDisable); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, channels, "enable", 1, h.channelEnable); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, channels, "disableCredential", 2, h.credentialDisable); err != nil {
		return err
	}
	if err := defineHostFn(h.rt, channels, "enableCredential", 2, h.credentialEnable); err != nil {
		return err
	}

	requests := realm.NewObject()
	if err := defineHostFn(h.rt, requests, "get", 1, h.requestGet); err != nil {
		return err
	}
	axonhub := realm.NewObject()
	if err := defineValue(h.rt, axonhub, "channels", engine.ObjectValue(channels)); err != nil {
		return err
	}
	if err := defineValue(h.rt, axonhub, "requests", engine.ObjectValue(requests)); err != nil {
		return err
	}

	webhook := realm.NewObject()
	if err := defineHostFn(h.rt, webhook, "send", 2, h.webhookSend); err != nil {
		return err
	}
	if err := defineValue(h.rt, ctxObj, "counter", engine.ObjectValue(counter)); err != nil {
		return err
	}
	if err := defineValue(h.rt, ctxObj, "axonhub", engine.ObjectValue(axonhub)); err != nil {
		return err
	}
	return defineValue(h.rt, ctxObj, "webhook", engine.ObjectValue(webhook))
}

func (h *hostCall) requirePermission(name string) error {
	if _, ok := h.granted[name]; !ok {
		return h.fail(hostPermissionDenied, "plugin permission "+name+" is required")
	}
	return nil
}

func (h *hostCall) bypassCtx() context.Context {
	return authz.WithSystemBypass(h.ctx, "plugin:"+h.pluginID)
}

func (h *hostCall) publishCtx() context.Context {
	depth := 0
	if event, ok := pluginEventFrom(h.ctx); ok {
		depth = event.Depth
	}
	return biz.WithPluginEventMeta(h.bypassCtx(), "plugin:"+h.pluginID, depth+1)
}

func (h *hostCall) requireWhole(value moejs.Value, name string) (int, error) {
	number, ok := finiteNumber(value)
	if !ok || number != math.Trunc(number) {
		return 0, h.fail(hostInvalidArgument, name+" must be an integer")
	}
	if number < 1 || number > float64(math.MaxInt) {
		return 0, h.fail(hostInvalidArgument, name+" must be a positive integer")
	}
	return int(number), nil
}

func (h *hostCall) counterPair(args []moejs.Value) (string, int, error) {
	key, err := h.requireString(moejs.Arg(args, 0), "key")
	if err != nil {
		return "", 0, err
	}
	if !kvNamePattern.MatchString(key) {
		return "", 0, h.fail(hostInvalidArgument, "key is invalid")
	}
	window, err := h.requireWhole(moejs.Arg(args, 1), "windowSeconds")
	if err != nil {
		return "", 0, err
	}
	if window > counterWindowMax {
		return "", 0, h.fail(hostInvalidArgument, "windowSeconds must be from 1 second to 24 hours")
	}
	return key, window, nil
}

func (h *hostCall) counterAdd(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	key, window, err := h.counterPair(args)
	if err != nil {
		return moejs.Undefined(), err
	}
	count, err := h.manager.counterAdd(h.ctx, h.pluginID, key, window)
	if err != nil {
		return moejs.Undefined(), h.counterFail(err)
	}
	return moejs.Number(float64(count)), nil
}

func (h *hostCall) counterGet(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	key, window, err := h.counterPair(args)
	if err != nil {
		return moejs.Undefined(), err
	}
	count, err := h.manager.counterGet(h.ctx, h.pluginID, key, window)
	if err != nil {
		return moejs.Undefined(), h.counterFail(err)
	}
	return moejs.Number(float64(count)), nil
}

func (h *hostCall) counterFail(err error) error {
	if xerrors.Is(err, context.Canceled) || xerrors.Is(err, context.DeadlineExceeded) {
		return h.fail(hostCanceled, "plugin call canceled")
	}
	log.Error(h.ctx, "plugin counter failed",
		log.String("plugin_id", h.pluginID),
		log.Cause(err),
	)
	return h.fail(hostInternal, "plugin counter failed")
}

func (h *hostCall) channelGet(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requirePermission(permReadChannels); err != nil {
		return moejs.Undefined(), err
	}
	id, err := h.requireWhole(moejs.Arg(args, 0), "id")
	if err != nil {
		return moejs.Undefined(), err
	}
	ctx := h.bypassCtx()
	row, err := h.manager.client(ctx).Channel.Query().
		Where(channel.ID(id)).
		Select(channel.FieldID, channel.FieldName, channel.FieldType, channel.FieldBaseURL, channel.FieldStatus).
		Only(ctx)
	if ent.IsNotFound(err) {
		return moejs.Null(), nil
	}
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	return h.rt.FromGo(channelInfo(row))
}

func (h *hostCall) channelList(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requirePermission(permReadChannels); err != nil {
		return moejs.Undefined(), err
	}
	opts, err := h.optionsAt(args, 0)
	if err != nil {
		return moejs.Undefined(), err
	}
	limit, afterID, hasAfter, err := h.channelPage(opts)
	if err != nil {
		return moejs.Undefined(), err
	}
	ctx := h.bypassCtx()
	query := h.manager.client(ctx).Channel.Query().Order(channel.ByID())
	if hasAfter {
		query = query.Where(channel.IDGT(afterID))
	}
	rows, err := query.Limit(limit+1).
		Select(channel.FieldID, channel.FieldName, channel.FieldType, channel.FieldBaseURL, channel.FieldStatus).
		All(ctx)
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	page := rows
	var next any
	if len(rows) > limit {
		page = rows[:limit]
		next = page[len(page)-1].ID
	}
	items := make([]any, 0, len(page))
	for _, row := range page {
		items = append(items, channelInfo(row))
	}
	return h.rt.FromGo(map[string]any{
		"items":       items,
		"nextAfterId": next,
	})
}

func (h *hostCall) channelPage(opts moejs.Value) (limit, afterID int, hasAfter bool, err error) {
	limit = channelListDefault
	if opts.IsUndefined() {
		return limit, 0, false, nil
	}
	rawLimit, err := h.rt.Get(opts, "limit")
	if err != nil {
		return 0, 0, false, err
	}
	if !rawLimit.IsUndefined() {
		number, ok := finiteNumber(rawLimit)
		if !ok || number != math.Trunc(number) || number < 1 {
			return 0, 0, false, h.fail(hostInvalidArgument, "limit must be a positive integer")
		}
		if number > channelListMax {
			return 0, 0, false, h.fail(hostLimitExceeded, "limit must not exceed 100")
		}
		limit = int(number)
	}
	rawAfter, err := h.rt.Get(opts, "afterId")
	if err != nil {
		return 0, 0, false, err
	}
	if rawAfter.IsUndefined() {
		return limit, 0, false, nil
	}
	if rawAfter.IsNull() {
		return 0, 0, false, h.fail(hostInvalidArgument, "afterId must be a positive integer")
	}
	afterID, err = h.requireWhole(rawAfter, "afterId")
	if err != nil {
		return 0, 0, false, err
	}
	return limit, afterID, true, nil
}

func (h *hostCall) channelDisable(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	return h.setChannelStatus(args, channel.StatusDisabled, "disable")
}

func (h *hostCall) channelEnable(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	return h.setChannelStatus(args, channel.StatusEnabled, "enable")
}

func (h *hostCall) setChannelStatus(args []moejs.Value, status channel.Status, action string) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requirePermission(permWriteChannels); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requireChannels(); err != nil {
		return moejs.Undefined(), err
	}
	id, err := h.requireWhole(moejs.Arg(args, 0), "id")
	if err != nil {
		return moejs.Undefined(), err
	}
	opts, err := h.optionsAt(args, 1)
	if err != nil {
		return moejs.Undefined(), err
	}
	reason, err := h.optionReason(opts)
	if err != nil {
		return moejs.Undefined(), err
	}
	if err := h.beginWrite(); err != nil {
		return moejs.Undefined(), err
	}
	ctx := h.publishCtx()
	row, err := h.manager.client(ctx).Channel.Query().
		Where(channel.ID(id)).
		Select(channel.FieldID, channel.FieldStatus).
		Only(ctx)
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	if row.Status == status {
		h.logChannel(action, id, "unchanged", reason)
		return moejs.Undefined(), nil
	}
	if _, err := h.manager.channels.UpdateChannelStatus(ctx, id, status); err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	h.logChannel(action, id, "updated", reason)
	return moejs.Undefined(), nil
}

func (h *hostCall) credentialDisable(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	return h.setCredential(args, true)
}

func (h *hostCall) credentialEnable(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	return h.setCredential(args, false)
}

func (h *hostCall) setCredential(args []moejs.Value, disable bool) (moejs.Value, error) {
	action := "enableCredential"
	if disable {
		action = "disableCredential"
	}
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requirePermission(permWriteChannels); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requireChannels(); err != nil {
		return moejs.Undefined(), err
	}
	id, err := h.requireWhole(moejs.Arg(args, 0), "id")
	if err != nil {
		return moejs.Undefined(), err
	}
	fingerprint, err := h.requireString(moejs.Arg(args, 1), "fingerprint")
	if err != nil {
		return moejs.Undefined(), err
	}
	if fingerprint == "" {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "fingerprint is required")
	}
	if fingerprint == events.OAuthFingerprint {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "fingerprint oauth is not supported")
	}
	opts, err := h.optionsAt(args, 2)
	if err != nil {
		return moejs.Undefined(), err
	}
	reason, err := h.optionReason(opts)
	if err != nil {
		return moejs.Undefined(), err
	}
	var expiresAt *time.Time
	if disable {
		expiresAt, err = h.optionExpiresAt(opts)
		if err != nil {
			return moejs.Undefined(), err
		}
	}
	if err := h.beginWrite(); err != nil {
		return moejs.Undefined(), err
	}

	ctx := h.publishCtx()
	row, err := h.manager.client(ctx).Channel.Get(ctx, id)
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	matches := matchingCredentialKeys(row, fingerprint)
	switch len(matches) {
	case 0:
		return moejs.Undefined(), h.fail(hostNotFound, "credential not found")
	case 1:
	default:
		return moejs.Undefined(), h.fail(hostConflict, "credential fingerprint matches more than one key")
	}
	key := matches[0]
	if disable {
		if credentialActivelyDisabled(row, key) {
			h.logChannel(action, id, "unchanged", reason)
			return moejs.Undefined(), nil
		}
		if expiresAt != nil {
			err = h.manager.channels.DisableAPIKey(ctx, id, key, 0, reason, expiresAt)
		} else {
			err = h.manager.channels.DisableAPIKey(ctx, id, key, 0, reason)
		}
	} else if !credentialListed(row, key) {
		h.logChannel(action, id, "unchanged", reason)
		return moejs.Undefined(), nil
	} else {
		err = h.manager.channels.EnableAPIKey(ctx, id, key)
	}
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	h.logChannel(action, id, "updated", reason)
	return moejs.Undefined(), nil
}

func matchingCredentialKeys(row *ent.Channel, fingerprint string) []string {
	if row == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var matches []string
	for _, key := range row.Credentials.GetAllAPIKeys() {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if events.Fingerprint(key) == fingerprint {
			matches = append(matches, key)
		}
	}
	return matches
}

// credentialActivelyDisabled reports whether an existing disable record makes
// another disable of the same key a no-op. A record owned by a provider quota
// observation does not: DisableAPIKey transfers its ownership to the caller,
// so skipping here would leave the key quota-owned and a later quota recovery
// would re-enable a plugin-disabled key.
func credentialActivelyDisabled(row *ent.Channel, key string) bool {
	for _, item := range row.DisabledAPIKeys {
		if item.Key == key && !item.IsExpired() && !item.QuotaDisabled {
			return true
		}
	}
	return false
}

func credentialListed(row *ent.Channel, key string) bool {
	for _, item := range row.DisabledAPIKeys {
		if item.Key == key {
			return true
		}
	}
	return false
}

func (h *hostCall) requestGet(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requirePermission(permReadRequests); err != nil {
		return moejs.Undefined(), err
	}
	id, err := h.requireWhole(moejs.Arg(args, 0), "id")
	if err != nil {
		return moejs.Undefined(), err
	}
	ctx := h.bypassCtx()
	row, err := h.manager.client(ctx).Request.Query().
		Where(request.ID(id)).
		Select(
			request.FieldID,
			request.FieldProjectID,
			request.FieldAPIKeyID,
			request.FieldChannelID,
			request.FieldModelID,
			request.FieldFormat,
			request.FieldStatus,
			request.FieldCreatedAt,
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return moejs.Null(), nil
	}
	if err != nil {
		return moejs.Undefined(), h.actionFail(err)
	}
	return h.rt.FromGo(requestInfo(row))
}

func (h *hostCall) webhookSend(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.requirePermission(permWebhook); err != nil {
		return moejs.Undefined(), err
	}
	if h.manager == nil || h.manager.webhooks == nil {
		return moejs.Undefined(), h.fail(hostInternal, "webhook notifier is unavailable")
	}
	name, err := h.requireString(moejs.Arg(args, 0), "targetName")
	if err != nil {
		return moejs.Undefined(), err
	}
	if strings.TrimSpace(name) == "" {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "targetName is required")
	}
	payload, err := h.jsonObject(moejs.Arg(args, 1), "payload")
	if err != nil {
		return moejs.Undefined(), err
	}
	if err := h.beginWrite(); err != nil {
		return moejs.Undefined(), err
	}
	event, _ := pluginEventFrom(h.ctx)
	if err := h.manager.webhooks.SendToTarget(h.ctx, name, payload, event); err != nil {
		return moejs.Undefined(), h.webhookFail(err)
	}
	log.Info(h.ctx, "plugin webhook sent",
		log.String("plugin_id", h.pluginID),
		log.String("action", "webhook.send"),
		log.String("target", name),
		log.String("result", "sent"),
	)
	return moejs.Undefined(), nil
}

func (h *hostCall) requireChannels() error {
	if h.manager == nil || h.manager.channels == nil {
		return h.fail(hostInternal, "channel service is unavailable")
	}
	return nil
}

func (h *hostCall) optionReason(opts moejs.Value) (string, error) {
	reason, err := h.optionString(opts, "reason")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(reason) == "" {
		return pluginActionReason, nil
	}
	return reason, nil
}

func (h *hostCall) optionExpiresAt(opts moejs.Value) (*time.Time, error) {
	if opts.IsUndefined() {
		return nil, nil
	}
	value, err := h.rt.Get(opts, "expiresAt")
	if err != nil {
		return nil, err
	}
	if value.IsUndefined() || value.IsNull() {
		return nil, nil
	}
	if !value.IsString() {
		return nil, h.fail(hostInvalidArgument, "expiresAt must be an RFC3339 timestamp")
	}
	text := value.AsString().GoString()
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return nil, h.fail(hostInvalidArgument, "expiresAt must be an RFC3339 timestamp")
		}
	}
	now := time.Now()
	if h.manager != nil {
		now = h.manager.now()
	}
	if !parsed.After(now) {
		return nil, h.fail(hostInvalidArgument, "expiresAt must be in the future")
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func (h *hostCall) jsonObject(value moejs.Value, name string) (map[string]any, error) {
	if !value.IsObject() || value.AsObject().ClassName() != "Object" {
		return nil, h.fail(hostInvalidArgument, name+" must be an object")
	}
	raw, err := h.rt.AppendJSONWith(nil, value, moejs.JSONOptions{Strict: true})
	if err != nil || !isJSONObject(raw) {
		return nil, h.fail(hostInvalidArgument, name+" must be a JSON object")
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, h.fail(hostInvalidArgument, name+" must be a JSON object")
	}
	return out, nil
}

func (h *hostCall) actionFail(err error) error {
	if err == nil {
		return nil
	}
	if xerrors.Is(err, context.Canceled) || xerrors.Is(err, context.DeadlineExceeded) {
		return h.fail(hostCanceled, "plugin call canceled")
	}
	if ent.IsNotFound(err) {
		return h.fail(hostNotFound, "channel not found")
	}
	log.Error(h.ctx, "plugin host action failed",
		log.String("plugin_id", h.pluginID),
		log.String("export", h.export),
		log.Cause(err),
	)
	return h.fail(hostInternal, "plugin action failed")
}

func (h *hostCall) webhookFail(err error) error {
	switch {
	case err == nil:
		return nil
	case xerrors.Is(err, context.Canceled), xerrors.Is(err, context.DeadlineExceeded):
		return h.fail(hostCanceled, "plugin call canceled")
	case xerrors.Is(err, biz.ErrWebhookTargetNotFound):
		return h.fail(hostNotFound, "webhook target not found")
	case xerrors.Is(err, biz.ErrWebhookTargetDisabled):
		return h.fail(hostInvalidState, "webhook target is disabled")
	default:
		log.Warn(h.ctx, "plugin webhook delivery failed",
			log.String("plugin_id", h.pluginID),
			log.Cause(err),
		)
		return h.fail(hostUnavailable, "webhook delivery failed")
	}
}

func (h *hostCall) logChannel(action string, channelID int, result, reason string) {
	log.Info(h.ctx, "plugin channel action",
		log.String("plugin_id", h.pluginID),
		log.String("action", action),
		log.Int("channel_id", channelID),
		log.String("result", result),
		log.String("reason", reason),
	)
}

func channelInfo(row *ent.Channel) map[string]any {
	return map[string]any{
		"id":      row.ID,
		"name":    row.Name,
		"type":    row.Type.String(),
		"baseUrl": row.BaseURL,
		"status":  row.Status.String(),
	}
}

func requestInfo(row *ent.Request) map[string]any {
	info := map[string]any{
		"id":        row.ID,
		"projectId": row.ProjectID,
		"apiKeyId":  nil,
		"channelId": nil,
		"model":     row.ModelID,
		"apiFormat": row.Format,
		"status":    row.Status.String(),
		"createdAt": row.CreatedAt.UTC().Format(time.RFC3339),
	}
	if row.APIKeyID != 0 {
		info["apiKeyId"] = row.APIKeyID
	}
	if row.ChannelID != 0 {
		info["channelId"] = row.ChannelID
	}
	return info
}
