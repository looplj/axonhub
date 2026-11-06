package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"

	"github.com/looplj/axonhub/internal/log"
)

const (
	maxFetchRequestBody  = 1 << 20
	maxFetchResponseBody = 2 << 20
)

// FetchRequest is one host-proxied HTTP call.
// Method is empty for GET. Headers are the plugin-supplied map.
type FetchRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

// FetchResult is an upstream HTTP response.
// Status may be outside 2xx. Headers use lowercase names.
type FetchResult struct {
	Status  int
	Headers map[string]string
	Body    string
}

// FetchError is a failure that did not produce an upstream response body.
// Code matches the plugin host error codes.
type FetchError struct {
	Code    string
	Message string
}

func (e *FetchError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// SetOutboundTransport replaces the transport used by Fetch.
// Nil restores http.DefaultTransport. Redirects are still not followed.
func (m *Manager) SetOutboundTransport(transport http.RoundTripper) {
	if m == nil {
		return
	}
	m.outboundTransport = transport
}

// Fetch sends one HTTPS request when rawURL matches granted.
// granted is the plugin generation's granted_network snapshot.
// The caller context supplies the deadline and cancellation.
func (m *Manager) Fetch(ctx context.Context, granted []string, req FetchRequest) (FetchResult, error) {
	return fetchOutbound(ctx, granted, req, m.outboundClient())
}

func (m *Manager) outboundClient() *http.Client {
	transport := http.DefaultTransport
	if m != nil && m.outboundTransport != nil {
		transport = m.outboundTransport
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func fetchOutbound(ctx context.Context, granted []string, req FetchRequest, client *http.Client) (FetchResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	method, err := normalizeFetchMethod(req.Method)
	if err != nil {
		return FetchResult{}, err
	}
	if len(req.Body) > maxFetchRequestBody {
		return FetchResult{}, &FetchError{Code: hostLimitExceeded, Message: "request body exceeds 1 MiB"}
	}
	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return FetchResult{}, &FetchError{Code: hostInvalidArgument, Message: "url must be an absolute URL"}
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.User != nil {
		return FetchResult{}, &FetchError{Code: hostPermissionDenied, Message: "outbound target is not granted"}
	}
	if !networkAllows(req.URL, parsed, granted) {
		return FetchResult{}, &FetchError{Code: hostPermissionDenied, Message: "outbound target is not granted"}
	}
	if client == nil {
		client = (&Manager{}).outboundClient()
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, body)
	if err != nil {
		return FetchResult{}, &FetchError{Code: hostInvalidArgument, Message: "url must be an absolute URL"}
	}
	httpReq.Header = filterRequestHeaders(req.Headers)

	resp, err := client.Do(httpReq)
	if err != nil {
		return FetchResult{}, fetchTransportError(ctx, err)
	}
	defer resp.Body.Close()
	if isRedirectStatus(resp.StatusCode) {
		return FetchResult{}, &FetchError{Code: hostInvalidState, Message: "redirects are not followed"}
	}

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchResponseBody+1))
	if err != nil {
		return FetchResult{}, fetchTransportError(ctx, err)
	}
	if len(payload) > maxFetchResponseBody {
		return FetchResult{}, &FetchError{Code: hostLimitExceeded, Message: "response body exceeds 2 MiB"}
	}
	if !utf8.Valid(payload) {
		return FetchResult{}, &FetchError{Code: hostInvalidState, Message: "response body is not valid UTF-8"}
	}
	return FetchResult{
		Status:  resp.StatusCode,
		Headers: responseHeaders(resp.Header),
		Body:    string(payload),
	}, nil
}

func normalizeFetchMethod(method string) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodGet, nil
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return method, nil
	default:
		return "", &FetchError{Code: hostInvalidArgument, Message: "method must be GET, POST, PUT, PATCH, or DELETE"}
	}
}

func fetchTransportError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return &FetchError{Code: hostDeadlineExceeded, Message: "outbound request deadline exceeded"}
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return &FetchError{Code: hostCanceled, Message: "outbound request canceled"}
	}
	return &FetchError{Code: hostUnavailable, Message: "outbound request failed"}
}

func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func networkAllows(raw string, parsed *url.URL, granted []string) bool {
	if parsed == nil {
		return false
	}
	stripped := stripQueryFragment(raw)
	for _, rule := range granted {
		if isRegexGrant(rule) {
			// Anchor the entire expression, including every alternation branch.
			// Absolute anchors remain effective when the rule enables multiline mode.
			compiled, err := regexp.Compile(`\A(?:` + rule + `)\z`)
			if err != nil {
				continue
			}
			if compiled.MatchString(stripped) {
				return true
			}
			continue
		}
		if exactNetworkMatch(parsed, rule) {
			return true
		}
	}
	return false
}

func isRegexGrant(rule string) bool {
	return strings.HasPrefix(rule, "^") && strings.HasSuffix(rule, "$")
}

func stripQueryFragment(raw string) string {
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		return raw[:i]
	}
	return raw
}

func exactNetworkMatch(req *url.URL, rule string) bool {
	ruleURL, err := url.Parse(rule)
	if err != nil || !strings.EqualFold(ruleURL.Scheme, "https") || ruleURL.Host == "" {
		return false
	}
	if !strings.EqualFold(ruleURL.Scheme, req.Scheme) {
		return false
	}
	if !strings.EqualFold(ruleURL.Hostname(), req.Hostname()) {
		return false
	}
	if networkPort(ruleURL) != networkPort(req) {
		return false
	}
	return ruleURL.EscapedPath() == req.EscapedPath()
}

func networkPort(u *url.URL) string {
	port := u.Port()
	if strings.EqualFold(u.Scheme, "https") && (port == "" || port == "443") {
		return ""
	}
	return port
}

var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
	"host":                {},
}

func filterRequestHeaders(src map[string]string) http.Header {
	blocked := map[string]struct{}{}
	for name := range hopByHopHeaders {
		blocked[name] = struct{}{}
	}
	for name, value := range src {
		if strings.EqualFold(name, "Connection") {
			for part := range strings.SplitSeq(value, ",") {
				blocked[strings.ToLower(strings.TrimSpace(part))] = struct{}{}
			}
		}
	}
	out := make(http.Header)
	for name, value := range src {
		if _, skip := blocked[strings.ToLower(name)]; skip || name == "" {
			continue
		}
		out.Set(name, value)
	}
	return out
}

func responseHeaders(header http.Header) map[string]string {
	blocked := map[string]struct{}{}
	for name := range hopByHopHeaders {
		blocked[name] = struct{}{}
	}
	for _, value := range header.Values("Connection") {
		for part := range strings.SplitSeq(value, ",") {
			blocked[strings.ToLower(strings.TrimSpace(part))] = struct{}{}
		}
	}
	out := map[string]string{}
	for name, values := range header {
		key := strings.ToLower(name)
		if _, skip := blocked[key]; skip {
			continue
		}
		out[key] = strings.Join(values, ", ")
	}
	return out
}

func (h *hostCall) fetch(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if err := h.enter(false); err != nil {
		return moejs.Undefined(), err
	}
	urlValue := moejs.Arg(args, 0)
	if !urlValue.IsString() {
		return h.rejectFetch(hostInvalidArgument, "url must be a string")
	}
	rawURL := urlValue.AsString().GoString()
	method, headers, body, err := h.readFetchInit(moejs.Arg(args, 1))
	if err != nil {
		if hostErr, ok := errors.AsType[*FetchError](err); ok {
			return h.rejectFetch(hostErr.Code, hostErr.Message)
		}
		return moejs.Undefined(), err
	}
	promise, resolve, reject := h.rt.NewPromise()
	if !h.async.track() {
		return h.rejectFetch(hostInvalidState, "plugin host API is only available during the call")
	}
	req := FetchRequest{URL: rawURL, Method: method, Headers: headers, Body: body}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(context.Background(), "plugin fetch panicked",
					log.Any("panic", recovered),
					log.Stack("stack"))
				h.async.deliver(func() error {
					value, valueErr := h.hostErrorValue(hostUnavailable, "outbound request failed")
					if valueErr != nil {
						return valueErr
					}
					return reject(value)
				})
			}
		}()
		result, fetchErr := h.manager.Fetch(h.ioCtx, h.network, req)
		h.async.deliver(func() error {
			if fetchErr != nil {
				code, message := hostUnavailable, "outbound request failed"
				if kind, ok := errors.AsType[*FetchError](fetchErr); ok {
					code, message = kind.Code, kind.Message
				}
				value, valueErr := h.hostErrorValue(code, message)
				if valueErr != nil {
					return valueErr
				}
				return reject(value)
			}
			payload, valueErr := h.rt.FromGo(map[string]any{
				"status":  result.Status,
				"headers": result.Headers,
				"body":    result.Body,
			})
			if valueErr != nil {
				return valueErr
			}
			return resolve(payload)
		})
	}()
	return promise, nil
}

func (h *hostCall) rejectFetch(code, message string) (moejs.Value, error) {
	promise, _, reject := h.rt.NewPromise()
	value, err := h.hostErrorValue(code, message)
	if err != nil {
		return moejs.Undefined(), err
	}
	if err := reject(value); err != nil {
		return moejs.Undefined(), err
	}
	return promise, nil
}

func (h *hostCall) hostErrorValue(code, message string) (moejs.Value, error) {
	realm := h.rt.Realm()
	obj := realm.NewError(engine.KindError, "%s", message)
	if err := obj.SetProp(realm, realm.KeyFromGoString("name"), moejs.String("PluginHostError")); err != nil {
		return moejs.Undefined(), err
	}
	if err := obj.SetProp(realm, realm.KeyFromGoString("code"), moejs.String(code)); err != nil {
		return moejs.Undefined(), err
	}
	return engine.ObjectValue(obj), nil
}

func (h *hostCall) readFetchInit(value moejs.Value) (string, map[string]string, string, error) {
	if value.IsUndefined() || value.IsNull() {
		return "", nil, "", nil
	}
	if !value.IsObject() || value.AsObject().ClassName() != "Object" {
		return "", nil, "", &FetchError{Code: hostInvalidArgument, Message: "fetch init must be an object"}
	}
	raw, err := h.rt.AppendJSONWith(nil, value, moejs.JSONOptions{Strict: true})
	if err != nil {
		return "", nil, "", &FetchError{Code: hostInvalidArgument, Message: "fetch init must be JSON"}
	}
	var init struct {
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(raw, &init); err != nil {
		return "", nil, "", &FetchError{Code: hostInvalidArgument, Message: "fetch init must use string fields"}
	}
	return init.Method, init.Headers, init.Body, nil
}
