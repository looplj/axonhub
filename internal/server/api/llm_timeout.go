package api

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm/httpclient"
)

// requestStreams mirrors the client-facing streaming controls after HTTP body
// decoding. Provider-side streaming for a non-streaming client keeps the normal
// request budget.
func requestStreams(request *httpclient.Request) bool {
	path := request.Path
	if strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent") {
		return strings.HasSuffix(path, ":streamGenerateContent")
	}
	if strings.HasSuffix(path, "/audio/speech") {
		format := strings.ToLower(strings.TrimSpace(gjson.GetBytes(request.Body, "stream_format").String()))
		return format == "sse" || format == "audio"
	}
	if strings.HasSuffix(path, "/audio/transcriptions") || strings.HasSuffix(path, "/audio/translations") {
		_, params, err := mime.ParseMediaType(request.Headers.Get("Content-Type"))
		if err != nil || params["boundary"] == "" {
			return false
		}
		reader := multipart.NewReader(bytes.NewReader(request.Body), params["boundary"])
		for {
			part, err := reader.NextPart()
			if err != nil {
				return false
			}
			if part.FormName() == "stream" && part.FileName() == "" {
				value, err := io.ReadAll(part)
				if err != nil {
					return false
				}
				switch strings.ToLower(strings.TrimSpace(string(value))) {
				case "true", "1", "yes":
					return true
				default:
					return false
				}
			}
		}
	}
	switch {
	case strings.HasSuffix(path, "/chat/completions"), strings.HasSuffix(path, "/completions"), strings.HasSuffix(path, "/responses"), strings.HasSuffix(path, "/messages"):
		return gjson.GetBytes(request.Body, "stream").Bool()
	default:
		return false
	}
}
