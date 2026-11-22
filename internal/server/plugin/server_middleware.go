// 职责: 按插件快照执行鉴权后的 HTTP 中间件，并校验、提交请求修改。
// 关联文件:
//   internal/server/plugin/manager.go
//   internal/server/plugin/invoke.go
//   internal/server/plugin/hostapi.go
//   internal/server/middleware/server_plugin.go
//   internal/contexts/plugin_request.go
// 注意事项:
//   一条请求的所有 handler 共用一个预算。快照和 VM 在进入下游前归还。
//   未启用 handler 时，执行入口只读取一次原子指针。

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/samber/lo"
	"golang.org/x/net/http/httpguts"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

const (
	serverMiddlewareBudget        = 25 * time.Millisecond
	serverMiddlewareMetadataLimit = 64 << 10
	serverMiddlewareIdentityLimit = 255
)

type serverMiddlewareHandler struct {
	plugin *LoadedPlugin
	export string
}

// ServerMiddlewareRejection is an HTTP rejection requested explicitly by a handler.
type ServerMiddlewareRejection struct {
	Status int
	Err    error
}

type serverMiddlewareRequest struct {
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	Headers   map[string][]string `json:"headers"`
	ProjectID *int                `json:"projectId"`
	APIKeyID  *int                `json:"apiKeyId"`
}

type serverMiddlewareResult struct {
	Headers serverMiddlewareHeaders               `json:"headers"`
	Context map[string]map[string]json.RawMessage `json:"context"`
}

type serverMiddlewareHeaders struct {
	Set    map[string]serverHeaderValues `json:"set"`
	Remove []string                      `json:"remove"`
}

type serverHeaderValues []string

func (v *serverHeaderValues) UnmarshalJSON(raw []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return err
	}
	out := make([]string, len(values))
	for i, value := range values {
		if len(value) == 0 || value[0] != '"' {
			return errors.New("server middleware header values must be strings")
		}
		if err := json.Unmarshal(value, &out[i]); err != nil {
			return err
		}
	}
	*v = out
	return nil
}

type serverMiddlewareContext struct {
	namespaces map[string]map[string]json.RawMessage
	metadata   contexts.PluginRequestContext
}

// RunServerMiddleware runs the installed gateway handlers before thread and trace resolution.
func (m *Manager) RunServerMiddleware(req *http.Request) (*http.Request, *ServerMiddlewareRejection) {
	if m == nil {
		return req, nil
	}
	gen := m.serverMiddleware.Load()
	if gen == nil {
		return req, nil
	}
	return m.runServerMiddleware(req, gen)
}

func (m *Manager) runServerMiddleware(req *http.Request, gen *Generation) (*http.Request, *ServerMiddlewareRejection) {
	ctx := req.Context()
	budgetCtx, cancel := context.WithTimeout(ctx, serverMiddlewareBudget)
	defer cancel()
	for gen.Acquire() != nil {
		next := m.serverMiddleware.Load()
		if next == nil || next == gen || budgetCtx.Err() != nil {
			return req, nil
		}
		gen = next
	}
	defer gen.Release()

	var args []byte
	var metadata serverMiddlewareContext
	for _, handler := range gen.serverMiddleware {
		if budgetCtx.Err() != nil {
			break
		}
		if args == nil {
			var err error
			args, err = serverMiddlewareArgs(req)
			if err != nil || len(args) > hookInputLimit {
				break
			}
		}
		var result serverMiddlewareResult
		nextMetadata := metadata
		headers := req.Header
		_, err := m.Invoke(budgetCtx, InvokeRequest{
			InstallationID:     handler.plugin.ID,
			Export:             handler.export,
			Args:               args,
			Config:             handler.plugin.Config,
			Generation:         gen,
			AllowReject:        true,
			serverMiddleware:   true,
			requestContextJSON: metadata.metadata.JSON,
			ValidateReturn: func(raw json.RawMessage) error {
				var err error
				result, err = decodeServerMiddlewareResult(raw, headers)
				if err != nil {
					return err
				}
				nextMetadata, err = metadata.merge(handler.plugin.PluginID, result.Context)
				return err
			},
		})
		if rejected, ok := xerrors.As[*requestRejectedError](err); ok {
			return req, &ServerMiddlewareRejection{Status: rejected.status, Err: rejected}
		}
		if err != nil {
			if xerrors.IsNot(err, ErrCircuitOpen) {
				if xerrors.Is(err, ErrVMUnavailable) {
					m.addHookCounter(handler.plugin.ID, true)
				} else {
					log.Warn(ctx, "plugin server middleware failed", log.String("plugin_id", handler.plugin.PluginID), log.String("handler", handler.export), log.Cause(err))
				}
			}
			continue
		}
		if budgetCtx.Err() != nil {
			break
		}
		if result.Headers.apply(req) {
			args = nil
		}
		metadata = nextMetadata
	}
	if len(metadata.metadata.JSON) != 0 {
		req = req.WithContext(contexts.WithPluginRequestContext(ctx, metadata.metadata))
	}
	return req, nil
}

func serverMiddlewareArgs(req *http.Request) ([]byte, error) {
	ctx := req.Context()
	input := serverMiddlewareRequest{
		Method:  req.Method,
		Path:    req.URL.Path,
		Headers: make(map[string][]string, len(req.Header)),
	}
	if id, ok := contexts.GetProjectID(ctx); ok {
		input.ProjectID = lo.ToPtr(id)
	}
	key, _ := contexts.GetAPIKey(ctx)
	if key != nil {
		input.APIKeyID = lo.ToPtr(key.ID)
	}
	for name, values := range req.Header {
		lower := strings.ToLower(name)
		if serverCredentialHeader(lower) || len(values) == 0 {
			continue
		}
		if key != nil && key.Key != "" {
			for _, value := range values {
				if strings.Contains(value, key.Key) {
					values = slices.DeleteFunc(slices.Clone(values), func(value string) bool {
						return strings.Contains(value, key.Key)
					})
					break
				}
			}
		}
		if len(values) == 0 {
			continue
		}
		if previous := input.Headers[lower]; len(previous) != 0 {
			values = append(slices.Clone(previous), values...)
		}
		input.Headers[lower] = values
	}
	return json.Marshal([1]serverMiddlewareRequest{input})
}

func decodeServerMiddlewareResult(raw json.RawMessage, current http.Header) (serverMiddlewareResult, error) {
	var result serverMiddlewareResult
	if len(raw) == 0 {
		return result, nil
	}
	if len(raw) > serverMiddlewareMetadataLimit || raw[0] != '{' {
		return result, errors.New("server middleware result must be an object of at most 64 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode server middleware result: %w", err)
	}
	return result, result.Headers.validate(current)
}

func (h serverMiddlewareHeaders) validate(current http.Header) error {
	set := make(map[string]struct{}, len(h.Set))
	for name, values := range h.Set {
		if !serverMutableHeader(name, current) || len(values) == 0 {
			return fmt.Errorf("invalid server middleware header set: %q", name)
		}
		lower := strings.ToLower(name)
		if _, exists := set[lower]; exists {
			return fmt.Errorf("duplicate server middleware header set: %q", name)
		}
		set[lower] = struct{}{}
		for _, value := range values {
			if !httpguts.ValidHeaderFieldValue(value) {
				return fmt.Errorf("invalid server middleware header value for %q", name)
			}
		}
	}
	for _, name := range h.Remove {
		if !serverMutableHeader(name, current) {
			return fmt.Errorf("invalid server middleware header removal: %q", name)
		}
	}
	return nil
}

func (h serverMiddlewareHeaders) apply(req *http.Request) bool {
	changed := false
	if req.Header == nil && len(h.Set) != 0 {
		req.Header = make(http.Header, len(h.Set))
	}
	for _, name := range h.Remove {
		canonical := http.CanonicalHeaderKey(name)
		if _, exists := req.Header[canonical]; exists {
			delete(req.Header, canonical)
			changed = true
		}
	}
	for name, values := range h.Set {
		canonical := http.CanonicalHeaderKey(name)
		if !slices.Equal(req.Header[canonical], []string(values)) {
			req.Header[canonical] = values
			changed = true
		}
	}
	return changed
}

func serverCredentialHeader(lower string) bool {
	switch lower {
	case "authorization", "proxy-authorization", "cookie", "set-cookie",
		"x-api-key", "api-key", "x-goog-api-key", "x-google-api-key":
		return true
	default:
		return false
	}
}

func serverMutableHeader(name string, current http.Header) bool {
	lower := strings.ToLower(name)
	if !httpguts.ValidHeaderFieldName(name) || serverCredentialHeader(lower) ||
		strings.HasPrefix(lower, "sec-websocket-") || strings.HasPrefix(lower, "x-forwarded-") ||
		httpguts.HeaderValuesContainsToken(current.Values("Connection"), name) {
		return false
	}
	switch lower {
	case "host", "forwarded", "x-real-ip", "connection", "proxy-connection", "keep-alive",
		"content-type", "content-length", "content-encoding", "transfer-encoding", "te",
		"trailer", "upgrade", "expect":
		return false
	default:
		return true
	}
}

func (s serverMiddlewareContext) merge(pluginID string, changes map[string]map[string]json.RawMessage) (serverMiddlewareContext, error) {
	if len(changes) == 0 {
		return s, nil
	}
	next := serverMiddlewareContext{namespaces: maps.Clone(s.namespaces), metadata: s.metadata}
	if next.namespaces == nil {
		next.namespaces = make(map[string]map[string]json.RawMessage, len(changes))
	}
	for namespace, fields := range changes {
		if (namespace != "axonhub" && namespace != pluginID) || fields == nil {
			return s, fmt.Errorf("invalid server middleware context namespace %q", namespace)
		}
		if len(fields) == 0 {
			continue
		}
		values := make(map[string]json.RawMessage, len(fields)+len(next.namespaces[namespace]))
		maps.Copy(values, next.namespaces[namespace])
		for name, raw := range fields {
			if namespace == "axonhub" {
				id, err := next.setIdentity(name, raw)
				if err != nil {
					return s, err
				}
				if id == "" {
					continue
				}
				raw, _ = json.Marshal(id)
			}
			values[name] = raw
		}
		if len(values) != 0 {
			next.namespaces[namespace] = values
		}
	}
	if len(next.namespaces) == 0 {
		return s, nil
	}
	raw, err := json.Marshal(next.namespaces)
	if err != nil {
		return s, fmt.Errorf("encode server middleware context: %w", err)
	}
	if len(raw) > serverMiddlewareMetadataLimit {
		return s, errors.New("server middleware context exceeds 64 KiB")
	}
	next.metadata.JSON = raw
	return next, nil
}

func (s *serverMiddlewareContext) setIdentity(name string, raw json.RawMessage) (string, error) {
	var target *string
	switch name {
	case "threadId":
		target = &s.metadata.ThreadID
	case "traceId":
		target = &s.metadata.TraceID
	default:
		return "", fmt.Errorf("unknown axonhub request context field %q", name)
	}
	var value string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
		return "", fmt.Errorf("axonhub request context %s must be a string", name)
	}
	value = strings.TrimSpace(value)
	if len(value) > serverMiddlewareIdentityLimit || strings.ContainsFunc(value, unicode.IsControl) {
		return "", fmt.Errorf("axonhub request context %s must be at most 255 bytes and contain no control characters", name)
	}
	if *target != "" || value == "" {
		return "", nil
	}
	*target = value
	return value, nil
}
