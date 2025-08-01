package llm

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadHTTPRequest(t *testing.T) {
	// Test data
	requestBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
	req, err := http.NewRequest("POST", "http://localhost:8080/v1/chat/completions", bytes.NewBufferString(requestBody))
	assert.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	// Execute
	genericReq, err := ReadHTTPRequest(req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "POST", genericReq.Method)
	assert.Equal(t, "http://localhost:8080/v1/chat/completions", genericReq.URL)
	assert.Equal(t, "application/json", genericReq.Headers.Get("Content-Type"))
	assert.Equal(t, "Bearer test-key", genericReq.Headers.Get("Authorization"))
	assert.Equal(t, []byte(requestBody), genericReq.Body)
}
