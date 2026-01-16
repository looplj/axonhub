package codex

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestParseCredentialsJSON(t *testing.T) {
	creds, err := ParseCredentialsJSON(`{"access_token":"a","refresh_token":"r"}`)
	require.NoError(t, err)
	require.Equal(t, "a", creds.AccessToken)
	require.Equal(t, "r", creds.RefreshToken)
	require.Equal(t, ClientID, creds.ClientID)
}

func TestCredentialsRefresh(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/oauth/token", r.URL.Path)
		require.Contains(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new","expires_in":3600,"token_type":"bearer"}`))
	}))
	t.Cleanup(ts.Close)

	// Override token url for test
	old := DefaultTokenURLs
	DefaultTokenURLs = TokenURLs{Authorize: old.Authorize, Token: ts.URL + "/oauth/token"}

	t.Cleanup(func() { DefaultTokenURLs = old })

	hc := httpclient.NewHttpClient()
	creds, err := ParseCredentialsJSON(`{"access_token":"a","refresh_token":"r"}`)
	require.NoError(t, err)

	updated, err := creds.Refresh(t.Context(), hc, nil, "")
	require.NoError(t, err)
	require.Equal(t, "new", updated.AccessToken)
}
