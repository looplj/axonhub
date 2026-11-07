package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
	"go.uber.org/zap"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

const (
	hostInvalidArgument  = "INVALID_ARGUMENT"
	hostPermissionDenied = "PERMISSION_DENIED"
	hostNotFound         = "NOT_FOUND"
	hostConflict         = "CONFLICT"
	hostLimitExceeded    = "LIMIT_EXCEEDED"
	hostRateLimited      = "RATE_LIMITED"
	hostCanceled         = "CANCELED"
	hostInvalidState     = "INVALID_STATE"
	hostUnavailable      = "UNAVAILABLE"
	hostDeadlineExceeded = "DEADLINE_EXCEEDED"
	hostInternal         = "INTERNAL"
)

// hostCall is the ctx object for one JavaScript call.
// Its methods close over this value. alive is cleared when the call returns,
// so a function saved on a module global cannot perform host work later.
type hostCall struct {
	manager        *Manager
	rt             *moejs.Runtime
	ctx            context.Context
	pluginID       string
	installationID int
	export         string
	alive          bool
	granted        map[string]struct{}
	network        []string
	async          *callAsync
	ioCtx          context.Context // Async HTTP is canceled after result delivery closes.
	cancelIO       context.CancelFunc
	writes         int
	ctxValue       moejs.Value
	result         InvokeResult
	rejected       *requestRejectedError
}

// requestRejectedError is the host-side mark set by a legal ctx.reject.
// The script may catch the control exception. The mark still rejects the request.
type requestRejectedError struct {
	status  int
	message string
}

func (e *requestRejectedError) Error() string {
	if e == nil || e.message == "" {
		return "plugin rejected the request"
	}
	return e.message
}

func (h *hostCall) build(config, state, requestJSON, attemptJSON json.RawMessage, allowReject bool) (moejs.Value, error) {
	configValue, err := h.rt.ParseJSON(config)
	if err != nil {
		return moejs.Undefined(), err
	}
	if err := deepFreeze(h.rt, configValue); err != nil {
		return moejs.Undefined(), err
	}
	stateValue, err := h.rt.ParseJSON(state)
	if err != nil {
		return moejs.Undefined(), err
	}

	realm := h.rt.Realm()
	kv := realm.NewObject()
	if err := defineHostFn(h.rt, kv, "get", 1, h.kvGet); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineHostFn(h.rt, kv, "set", 2, h.kvSet); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineHostFn(h.rt, kv, "delete", 1, h.kvDelete); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineHostFn(h.rt, kv, "list", 0, h.kvList); err != nil {
		return moejs.Undefined(), err
	}

	logger := realm.NewObject()
	if err := defineHostFn(h.rt, logger, "info", 1, h.logMethod("info")); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineHostFn(h.rt, logger, "warn", 1, h.logMethod("warn")); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineHostFn(h.rt, logger, "error", 1, h.logMethod("error")); err != nil {
		return moejs.Undefined(), err
	}

	ctxObj := realm.NewObject()
	if err := defineValue(h.rt, ctxObj, "config", configValue); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineValue(h.rt, ctxObj, "state", stateValue); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineValue(h.rt, ctxObj, "kv", engine.ObjectValue(kv)); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineValue(h.rt, ctxObj, "log", engine.ObjectValue(logger)); err != nil {
		return moejs.Undefined(), err
	}
	if err := defineHostFn(h.rt, ctxObj, "fetch", 1, h.fetch); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.defineJSONField(ctxObj, "request", requestJSON); err != nil {
		return moejs.Undefined(), err
	}
	if err := h.defineJSONField(ctxObj, "attempt", attemptJSON); err != nil {
		return moejs.Undefined(), err
	}
	if allowReject {
		if err := defineHostFn(h.rt, ctxObj, "reject", 2, h.reject); err != nil {
			return moejs.Undefined(), err
		}
	}
	if err := h.attachEventAPI(ctxObj); err != nil {
		return moejs.Undefined(), err
	}
	return engine.ObjectValue(ctxObj), nil
}

func (h *hostCall) defineJSONField(obj *moejs.Object, name string, raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	value, err := h.rt.ParseJSON(raw)
	if err != nil {
		return err
	}
	return defineValue(h.rt, obj, name, value)
}

func (h *hostCall) reject(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	status, ok := finiteNumber(moejs.Arg(args, 0))
	messageVal := moejs.Arg(args, 1)
	message := ""
	if messageVal.IsString() {
		message = strings.TrimSpace(messageVal.AsString().GoString())
	}
	legalStatus := ok && status == math.Trunc(status) && (status == 400 || status == 403 || status == 429)
	if !legalStatus || !messageVal.IsString() || message == "" {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "reject status must be 400, 403, or 429 and message must be a non-empty string")
	}
	h.rejected = &requestRejectedError{status: int(status), message: message}
	return moejs.Undefined(), h.throwControl(message)
}

func (h *hostCall) throwControl(message string) error {
	if h == nil || h.rt == nil {
		return errors.New(message)
	}
	realm := h.rt.Realm()
	obj := realm.NewError(engine.KindError, "%s", message)
	if err := obj.SetProp(realm, realm.KeyFromGoString("name"), moejs.String("PluginRequestReject")); err != nil {
		return err
	}
	return realm.Throw(engine.ObjectValue(obj))
}

func defineHostFn(rt *moejs.Runtime, obj *moejs.Object, name string, length int, fn moejs.NativeFunc) error {
	return defineValue(rt, obj, name, rt.Function(name, length, fn))
}

func defineValue(rt *moejs.Runtime, obj *moejs.Object, name string, value moejs.Value) error {
	return obj.SetProp(rt.Realm(), rt.Realm().KeyFromGoString(name), value)
}

func deepFreeze(rt *moejs.Runtime, value moejs.Value) error {
	if !value.IsObject() {
		return nil
	}
	raw, err := rt.AppendJSON(nil, value)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	return freezeValue(rt, value, generic)
}

func freezeValue(rt *moejs.Runtime, value moejs.Value, generic any) error {
	switch typed := generic.(type) {
	case map[string]any:
		for key, child := range typed {
			next, err := rt.Get(value, key)
			if err != nil {
				return err
			}
			if err := freezeValue(rt, next, child); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range typed {
			next, err := rt.Get(value, fmt.Sprintf("%d", i))
			if err != nil {
				return err
			}
			if err := freezeValue(rt, next, child); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	if value.IsObject() {
		value.AsObject().Freeze(rt.Realm())
	}
	return nil
}

func (h *hostCall) scriptArgs(raw json.RawMessage) ([]moejs.Value, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(trimmed, &elems); err != nil {
		return nil, fmt.Errorf("plugin invoke args must be a JSON array: %w", err)
	}
	args := make([]moejs.Value, 0, len(elems))
	for _, elem := range elems {
		value, err := h.rt.ParseJSON(elem)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}
	return args, nil
}

func (h *hostCall) readReturn(value moejs.Value) (json.RawMessage, error) {
	if value.IsUndefined() {
		return nil, nil
	}
	raw, err := h.rt.AppendJSONWith(nil, value, moejs.JSONOptions{Strict: true})
	if err != nil {
		return nil, errors.New("return value must be JSON")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func (h *hostCall) readState() (json.RawMessage, error) {
	if !h.ctxValue.IsObject() {
		return nil, errors.New("ctx.state must be a JSON object")
	}
	state, err := h.rt.Get(h.ctxValue, "state")
	if err != nil {
		return nil, err
	}
	raw, err := h.rt.AppendJSONWith(nil, state, moejs.JSONOptions{Strict: true})
	if err != nil || !isJSONObject(raw) {
		return nil, errors.New("ctx.state must be a JSON object")
	}
	if len(bytes.TrimSpace(raw)) > pluginStateBytes {
		return nil, errors.New("ctx.state exceeds 64 KiB")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func isJSONObject(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && raw[0] == '{'
}

func (h *hostCall) enter(write bool) error {
	if h == nil || !h.alive {
		return h.fail(hostInvalidState, "plugin host API is only available during the call")
	}
	if err := h.ctx.Err(); err != nil {
		return h.fail(hostCanceled, "plugin call canceled")
	}
	if err := h.rt.Realm().CheckInterrupt(); err != nil {
		return err
	}
	if write {
		if err := h.beginWrite(); err != nil {
			return err
		}
	}
	return nil
}

func (h *hostCall) beginWrite() error {
	if h.writes >= maxWritesPerCall {
		return h.fail(hostRateLimited, "plugin write limit reached")
	}
	if h.manager == nil || !h.manager.allowWrite(h.pluginID) {
		return h.fail(hostRateLimited, "plugin write limit reached")
	}
	h.writes++
	return nil
}

func (h *hostCall) fail(code, message string) error {
	if h == nil || h.rt == nil {
		return fmt.Errorf("%s", message)
	}
	realm := h.rt.Realm()
	obj := realm.NewError(engine.KindError, "%s", message)
	if err := obj.SetProp(realm, realm.KeyFromGoString("name"), moejs.String("PluginHostError")); err != nil {
		return err
	}
	if err := obj.SetProp(realm, realm.KeyFromGoString("code"), moejs.String(code)); err != nil {
		return err
	}
	return realm.Throw(engine.ObjectValue(obj))
}

func (h *hostCall) requireString(value moejs.Value, name string) (string, error) {
	if !value.IsString() {
		return "", h.fail(hostInvalidArgument, name+" must be a string")
	}
	return value.AsString().GoString(), nil
}

func (h *hostCall) optionsAt(args []moejs.Value, index int) (moejs.Value, error) {
	value := moejs.Arg(args, index)
	if value.IsUndefined() {
		return moejs.Undefined(), nil
	}
	if !value.IsObject() || value.AsObject().ClassName() != "Object" {
		return moejs.Undefined(), h.fail(hostInvalidArgument, "options must be an object")
	}
	return value, nil
}

func (h *hostCall) optionString(opts moejs.Value, field string) (string, error) {
	if opts.IsUndefined() {
		return "", nil
	}
	value, err := h.rt.Get(opts, field)
	if err != nil {
		return "", err
	}
	if value.IsUndefined() {
		return "", nil
	}
	if !value.IsString() {
		return "", h.fail(hostInvalidArgument, field+" must be a string")
	}
	return value.AsString().GoString(), nil
}

func (h *hostCall) kvGet(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	key, err := h.requireString(moejs.Arg(args, 0), "key")
	if err != nil {
		return moejs.Undefined(), err
	}
	opts, err := h.optionsAt(args, 1)
	if err != nil {
		return moejs.Undefined(), err
	}
	namespace, err := h.optionString(opts, "namespace")
	if err != nil {
		return moejs.Undefined(), err
	}
	entry, err := h.manager.KVGet(h.ctx, h.pluginID, h.installationID, namespace, key)
	if err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	if entry == nil {
		return moejs.Null(), nil
	}
	return moejs.String(entry.Value), nil
}

func (h *hostCall) kvSet(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	key, err := h.requireString(moejs.Arg(args, 0), "key")
	if err != nil {
		return moejs.Undefined(), err
	}
	value, err := h.requireString(moejs.Arg(args, 1), "value")
	if err != nil {
		return moejs.Undefined(), err
	}
	opts, err := h.optionsAt(args, 2)
	if err != nil {
		return moejs.Undefined(), err
	}
	namespace, err := h.optionString(opts, "namespace")
	if err != nil {
		return moejs.Undefined(), err
	}
	ttl, err := h.optionTTL(opts)
	if err != nil {
		return moejs.Undefined(), err
	}
	if _, err := validateKVLookup(h.installationID, namespace, key); err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	if err := validateKVValue(value); err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	if _, err := h.manager.kvExpiry(ttl); err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	if err := h.beginWrite(); err != nil {
		return moejs.Undefined(), err
	}
	if _, err := h.manager.KVSet(h.ctx, h.pluginID, h.installationID, namespace, key, value, ttl); err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	return moejs.Undefined(), nil
}

func (h *hostCall) kvDelete(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	key, err := h.requireString(moejs.Arg(args, 0), "key")
	if err != nil {
		return moejs.Undefined(), err
	}
	opts, err := h.optionsAt(args, 1)
	if err != nil {
		return moejs.Undefined(), err
	}
	namespace, err := h.optionString(opts, "namespace")
	if err != nil {
		return moejs.Undefined(), err
	}
	if _, err := validateKVLookup(h.installationID, namespace, key); err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	if err := h.beginWrite(); err != nil {
		return moejs.Undefined(), err
	}
	deleted, err := h.manager.KVDelete(h.ctx, h.pluginID, h.installationID, namespace, key)
	if err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	return moejs.Bool(deleted), nil
}

func (h *hostCall) kvList(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	opts, err := h.optionsAt(args, 0)
	if err != nil {
		return moejs.Undefined(), err
	}
	namespace, err := h.optionString(opts, "namespace")
	if err != nil {
		return moejs.Undefined(), err
	}
	prefix, err := h.optionString(opts, "prefix")
	if err != nil {
		return moejs.Undefined(), err
	}
	after, err := h.optionCursor(opts)
	if err != nil {
		return moejs.Undefined(), err
	}
	first, err := h.optionFirst(opts)
	if err != nil {
		return moejs.Undefined(), err
	}
	page, err := h.manager.KVList(h.ctx, h.pluginID, h.installationID, namespace, prefix, first, after)
	if err != nil {
		return moejs.Undefined(), h.kvFail(err)
	}
	return h.kvPage(page)
}

func (h *hostCall) optionTTL(opts moejs.Value) (time.Duration, error) {
	if opts.IsUndefined() {
		return 0, nil
	}
	value, err := h.rt.Get(opts, "ttlSeconds")
	if err != nil {
		return 0, err
	}
	if value.IsUndefined() {
		return 0, nil
	}
	number, ok := finiteNumber(value)
	if !ok || number < 0 || number != math.Trunc(number) {
		return 0, h.fail(hostInvalidArgument, "ttlSeconds must be a non-negative integer")
	}
	if number > kvMaxTTL.Seconds() {
		return 0, h.fail(hostLimitExceeded, "plugin kv ttl must not exceed 90 days")
	}
	return time.Duration(number) * time.Second, nil
}

func (h *hostCall) optionFirst(opts moejs.Value) (int, error) {
	if opts.IsUndefined() {
		return kvListLimit, nil
	}
	value, err := h.rt.Get(opts, "first")
	if err != nil {
		return 0, err
	}
	if value.IsUndefined() {
		return kvListLimit, nil
	}
	number, ok := finiteNumber(value)
	if !ok || number != math.Trunc(number) {
		return 0, h.fail(hostInvalidArgument, "first must be an integer")
	}
	if number < 1 {
		return 0, h.fail(hostInvalidArgument, "first must be between 1 and 100")
	}
	if number > kvListLimit {
		return 0, h.fail(hostLimitExceeded, "first must be between 1 and 100")
	}
	return int(number), nil
}

func (h *hostCall) optionCursor(opts moejs.Value) (string, error) {
	if opts.IsUndefined() {
		return "", nil
	}
	value, err := h.rt.Get(opts, "after")
	if err != nil {
		return "", err
	}
	if value.IsUndefined() {
		return "", nil
	}
	if !value.IsString() {
		return "", h.fail(hostInvalidArgument, "after must be a string")
	}
	return value.AsString().GoString(), nil
}

func finiteNumber(value moejs.Value) (float64, bool) {
	if !value.IsNumber() {
		return 0, false
	}
	number := value.AsNumber()
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func (h *hostCall) kvPage(page *KVPage) (moejs.Value, error) {
	if page == nil {
		page = &KVPage{}
	}
	entries := make([]any, 0, len(page.Entries))
	for _, entry := range page.Entries {
		item := map[string]any{
			"namespace": entry.Namespace,
			"key":       entry.Key,
			"value":     entry.Value,
			"updatedAt": entry.UpdatedAt.UTC().Format(time.RFC3339Nano),
			"expiresAt": nil,
		}
		if entry.ExpiresAt != nil {
			item["expiresAt"] = entry.ExpiresAt.UTC().Format(time.RFC3339Nano)
		}
		entries = append(entries, item)
	}
	payload := map[string]any{
		"entries":    entries,
		"nextCursor": nil,
	}
	if page.NextCursor != nil {
		payload["nextCursor"] = *page.NextCursor
	}
	return h.rt.FromGo(payload)
}

func (h *hostCall) kvFail(err error) error {
	if err == nil {
		return nil
	}
	if xerrors.Is(err, context.Canceled) || xerrors.Is(err, context.DeadlineExceeded) {
		return h.fail(hostCanceled, "plugin call canceled")
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "exceeds 256 KiB"):
		return h.fail(hostLimitExceeded, "plugin kv value exceeds 256 KiB")
	case strings.Contains(message, "must not exceed 90"):
		return h.fail(hostLimitExceeded, "plugin kv ttl must not exceed 90 days")
	case strings.Contains(message, "valid UTF-8"):
		return h.fail(hostInvalidArgument, "plugin kv value must be valid UTF-8")
	case strings.Contains(message, "must not be negative"):
		return h.fail(hostInvalidArgument, "plugin kv ttl must not be negative")
	case strings.Contains(message, "must match"):
		return h.fail(hostInvalidArgument, "plugin kv "+kvNameKind(message)+" is invalid")
	case strings.Contains(message, "page size"):
		return h.fail(hostLimitExceeded, "plugin kv page size must be between 1 and 100")
	case strings.Contains(message, "must be positive"):
		return h.fail(hostInvalidArgument, "plugin kv installation is invalid")
	default:
		log.Error(h.logContext(), "plugin kv failed",
			log.String("plugin_id", h.pluginID),
			log.Int("installation_id", h.installationID),
			log.Cause(err))
		return h.fail(hostInternal, "plugin kv request failed")
	}
}

func kvNameKind(message string) string {
	switch {
	case strings.Contains(message, "namespace"):
		return "namespace"
	case strings.Contains(message, "cursor"):
		return "cursor"
	case strings.Contains(message, "key"):
		return "key"
	default:
		return "name"
	}
}

func (h *hostCall) logMethod(level string) moejs.NativeFunc {
	return func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		if err := h.enter(false); err != nil {
			return moejs.Undefined(), err
		}
		message, err := h.requireString(moejs.Arg(args, 0), "message")
		if err != nil {
			return moejs.Undefined(), err
		}
		user, err := h.logUserFields(args)
		if err != nil {
			return moejs.Undefined(), err
		}
		h.writeLog(level, message, pluginLogFields(h.pluginID, h.installationID, h.export, user))
		return moejs.Undefined(), nil
	}
}

func (h *hostCall) logUserFields(args []moejs.Value) (map[string]any, error) {
	value := moejs.Arg(args, 1)
	if value.IsUndefined() {
		return map[string]any{}, nil
	}
	if !value.IsObject() || value.AsObject().ClassName() != "Object" {
		return nil, h.fail(hostInvalidArgument, "fields must be an object")
	}
	raw, err := h.rt.AppendJSONWith(nil, value, moejs.JSONOptions{Strict: true})
	if err != nil || !isJSONObject(raw) {
		return nil, h.fail(hostInvalidArgument, "fields must be a JSON object")
	}
	fields := map[string]any{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, h.fail(hostInvalidArgument, "fields must be a JSON object")
	}
	return fields, nil
}

func (h *hostCall) writeLog(level, message string, fields []zap.Field) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error(context.Background(), "plugin log panicked",
				log.Any("panic", recovered),
				log.Stack("stack"))
		}
	}()
	ctx := h.logContext()
	switch level {
	case "warn":
		log.Warn(ctx, message, fields...)
	case "error":
		log.Error(ctx, message, fields...)
	default:
		log.Info(ctx, message, fields...)
	}
}

func (h *hostCall) logContext() context.Context {
	if h == nil || h.ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(h.ctx)
}

// pluginLogFields adds the plugin identity after the script fields.
// Keys plugin_id, installation_id, and export from the script are dropped.
func pluginLogFields(pluginID string, installationID int, export string, user map[string]any) []zap.Field {
	fields := make([]zap.Field, 0, len(user)+3)
	for _, key := range slices.Sorted(maps.Keys(user)) {
		if key == "plugin_id" || key == "installation_id" || key == "export" {
			continue
		}
		fields = append(fields, log.Any(key, user[key]))
	}
	return append(fields,
		log.String("plugin_id", pluginID),
		log.Int("installation_id", installationID),
		log.String("export", export),
	)
}
