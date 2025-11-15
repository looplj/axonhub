package httpclient

import (
	"errors"
	"fmt"
	"net/http"
)

func IsNotFoundErr(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.StatusCode == http.StatusNotFound
}

type Error struct {
	Method     string `json:"method"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Status     string `json:"status"`
	Body       []byte `json:"body"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s - %s with status %s", e.Method, e.URL, e.Status)
}
