package orchestrator

import (
	"net/http"
	"sort"

	"github.com/looplj/axonhub/llm/httpclient"
)

type requestLogSummary struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	APIFormat   string   `json:"api_format"`
	RequestType string   `json:"request_type"`
	BodyBytes   int      `json:"body_bytes"`
	HeaderNames []string `json:"header_names"`
}

func redactedRequestLogSummary(request *httpclient.Request) requestLogSummary {
	if request == nil {
		return requestLogSummary{}
	}

	return requestLogSummary{
		Method:      request.Method,
		Path:        request.Path,
		APIFormat:   request.APIFormat,
		RequestType: request.RequestType,
		BodyBytes:   len(request.Body),
		HeaderNames: sortedHeaderNames(request.Headers),
	}
}

func sortedHeaderNames(headers http.Header) []string {
	if len(headers) == 0 {
		return nil
	}

	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}
