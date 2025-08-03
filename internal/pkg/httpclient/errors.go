package httpclient

import (
	"fmt"
)

type HttpError struct {
	Method     string `json:"method"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Status     string `json:"status"`
	Body       []byte `json:"body"`
}

func (e HttpError) Error() string {
	return fmt.Sprintf("%s - %s with status %s", e.Method, e.URL, e.Status)
}

// ResponseError represents an error in the response
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Type    string `json:"type"`
	Details string `json:"details,omitempty"`
}
