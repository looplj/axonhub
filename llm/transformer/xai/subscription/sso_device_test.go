package subscription

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type ssoRoundTripper func(*http.Request) (*http.Response, error)

func (transport ssoRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestConvertSSOToBuild_completes_device_flow(t *testing.T) {
	// Given
	var slept time.Duration
	transport := ssoRoundTripper(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36", request.Header.Get("User-Agent"))
		require.Equal(t, "en-US,en;q=0.9", request.Header.Get("Accept-Language"))
		response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}
		switch request.URL.String() {
		case SSOAccountsURL:
			require.Contains(t, request.Header.Get("Cookie"), "sso=synthetic-sso")
		case SSODeviceURL:
			response.Body = io.NopCloser(strings.NewReader(`{"device_code":"device","user_code":"user","verification_uri_complete":"https://auth.x.ai/oauth2/device/verify?user_code=user","interval":3600,"expires_in":1}`))
		case "https://auth.x.ai/oauth2/device/verify?user_code=user":
		case SSOVerifyURL:
			response.StatusCode = http.StatusFound
			response.Header.Set("Location", "https://auth.x.ai/oauth2/device/consent")
		case "https://auth.x.ai/oauth2/device/consent":
		case SSOApproveURL:
			response.StatusCode = http.StatusFound
			response.Header.Set("Location", "https://auth.x.ai/oauth2/device/done")
		case "https://auth.x.ai/oauth2/device/done":
		case TokenURL:
			response.Body = io.NopCloser(strings.NewReader(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`))
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		return response, nil
	})
	client := &http.Client{Transport: transport}

	// When
	credentials, err := ConvertSSOToBuild(context.Background(), "sso=synthetic-sso", SSODeviceOptions{
		HTTPClient: client,
		Sleep: func(_ context.Context, duration time.Duration) error {
			slept = duration
			return nil
		},
	})

	// Then
	require.NoError(t, err)
	require.Equal(t, "access", credentials.AccessToken)
	require.Equal(t, "refresh", credentials.RefreshToken)
	require.Equal(t, ClientID, credentials.ClientID)
	require.Equal(t, "Bearer", credentials.TokenType)
	require.Positive(t, slept)
	require.LessOrEqual(t, slept, time.Second)
}

func TestConvertSSOToBuild_reports_verification_HTTP_status(t *testing.T) {
	// Given
	transport := ssoRoundTripper(func(request *http.Request) (*http.Response, error) {
		response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}
		switch request.URL.String() {
		case SSOAccountsURL:
		case SSODeviceURL:
			response.Body = io.NopCloser(strings.NewReader(`{"device_code":"device","user_code":"user","verification_uri_complete":"https://auth.x.ai/oauth2/device/verify?user_code=user","interval":1,"expires_in":60}`))
		case "https://auth.x.ai/oauth2/device/verify?user_code=user":
			response.StatusCode = http.StatusInternalServerError
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		return response, nil
	})

	// When
	_, err := ConvertSSOToBuild(context.Background(), "synthetic-sso", SSODeviceOptions{HTTPClient: &http.Client{Transport: transport}})

	// Then
	require.EqualError(t, err, "open xAI device verification: HTTP 500")
}
