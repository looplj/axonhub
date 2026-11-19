// 职责: 校验并构造地域和 Workspace 专属 Qwen 实时地址。
// 关联文件:
//
//	llm/realtime/capability.go
//	internal/server/biz/channel_endpoint.go
package realtime

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

const qwenRealtimePath = "/api-ws/v1/realtime"

var trustedHost = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.(?:cn-beijing|ap-southeast-1)\.maas\.aliyuncs\.com$`)

func ValidateEndpoint(baseURL, path string) error {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "wss" || !trustedHost.MatchString(u.Host) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") || strings.ContainsAny(baseURL, "?#") {
		return errors.New("Qwen realtime requires a trusted Workspace regional WSS host")
	}
	if path != qwenRealtimePath {
		return errors.New("Qwen realtime requires /api-ws/v1/realtime")
	}
	return nil
}
func EndpointURL(baseURL, path, model string) (string, error) {
	if err := ValidateEndpoint(baseURL, path); err != nil {
		return "", err
	}
	if !IsAssociatedModel(model) {
		return "", errUnsupportedModel(model)
	}
	return strings.TrimSuffix(baseURL, "/") + qwenRealtimePath + "?" + url.Values{"model": []string{model}}.Encode(), nil
}
