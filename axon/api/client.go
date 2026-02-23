package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
)

type authHeaderTransport struct {
	apiKey string
	base   http.RoundTripper
}

func (t *authHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	return t.base.RoundTrip(req)
}

func NewClient(baseURL, apiKey string) graphql.Client {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authHeaderTransport{
			apiKey: apiKey,
			base:   http.DefaultTransport,
		},
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/agent/v1/graphql"
	return graphql.NewClient(endpoint, httpClient)
}
