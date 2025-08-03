package httpclient

import (
	"fmt"
	"io"
	"net/http"
)

func ReadHTTPRequest(rawReq *http.Request) (*Request, error) {
	req := &Request{
		Method:     rawReq.Method,
		URL:        rawReq.URL.String(),
		Headers:    rawReq.Header,
		Body:       []byte{},
		Auth:       &AuthConfig{},
		Streaming:  false,
		RequestID:  "",
		RawRequest: rawReq,
	}
	body, err := io.ReadAll(rawReq.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	req.Body = body
	return req, nil
}
