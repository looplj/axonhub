package events

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFingerprintAndMask(t *testing.T) {
	require.Equal(t, "f3abf2a6cc4f", Fingerprint("sk-test"))
	require.Equal(t, "e3b0c44298fc", Fingerprint(""))
	require.Len(t, Fingerprint("another-key"), 12)
	require.NotEqual(t, Fingerprint("sk-test"), Fingerprint("sk-test-2"))

	require.Equal(t, "****", Mask("12345678"))
	require.Equal(t, "1234****6789", Mask("123456789"))
	require.Equal(t, "abcd****wxyz", Mask("abcd-secret-wxyz"))
	require.NotContains(t, Mask("12345678"), "12345678")
	require.NotEqual(t, "12345678", Mask("12345678"))
	require.NotContains(t, Mask("abcd-secret-wxyz"), "...")

	for _, key := range []string{"", "a", "short", "1234567", "oauth"} {
		masked := Mask(key)
		require.Equal(t, "****", masked)
		require.NotContains(t, masked, "...")
		if key != "" {
			require.NotContains(t, masked, key)
			require.NotEqual(t, key, masked)
		}
	}

	runes := []rune("你好世界测试字符")
	require.Len(t, runes, 8)
	require.Equal(t, "****", Mask(string(runes)))
	require.NotContains(t, Mask(string(runes)), string(runes))
	require.Equal(t, "你好世界****测试字符", Mask("你好世界中测试字符"))
}

func TestOAuthCredentialUsesConstant(t *testing.T) {
	// Callers do not hash or mask an OAuth credential. There is no key string.
	ref := CredentialRef{
		Fingerprint: OAuthFingerprint,
		Masked:      OAuthFingerprint,
	}

	require.Equal(t, "oauth", OAuthFingerprint)
	require.Equal(t, OAuthFingerprint, ref.Fingerprint)
	require.Equal(t, OAuthFingerprint, ref.Masked)
	require.NotEqual(t, OAuthFingerprint, Fingerprint(OAuthFingerprint))
	require.NotEqual(t, OAuthFingerprint, Mask(OAuthFingerprint))
}

func TestOpenEventTypes(t *testing.T) {
	require.Equal(t, []string{
		"channel.auto_enabled",
		"channel.credential_disabled",
		"channel.credential_enabled",
		"request.attempt_failed",
		"request.failover",
		"request.failed",
		"request.completed",
		"api_key.quota_exceeded",
	}, []string{
		TypeChannelAutoEnabled,
		TypeChannelCredentialDisabled,
		TypeChannelCredentialEnabled,
		TypeRequestAttemptFailed,
		TypeRequestFailover,
		TypeRequestFailed,
		TypeRequestCompleted,
		TypeAPIKeyQuotaExceeded,
	})

	types := OpenEventTypes()
	require.Equal(t, []string{
		TypeChannelAutoEnabled,
		TypeChannelCredentialDisabled,
		TypeChannelCredentialEnabled,
		TypeRequestAttemptFailed,
		TypeRequestFailover,
		TypeRequestFailed,
		TypeRequestCompleted,
		TypeAPIKeyQuotaExceeded,
	}, types)

	types[0] = "mutated"
	require.Equal(t, TypeChannelAutoEnabled, OpenEventTypes()[0])

	require.Equal(t, "info", SeverityInfo)
	require.Equal(t, "warning", SeverityWarning)
	require.Equal(t, "error", SeverityError)
	require.Equal(t, "critical", SeverityCritical)
}

func TestEventMapCamelCaseOmitsNilRefs(t *testing.T) {
	const secret = "sk-live-abcdefghijklmnopqrstuvwxyz"
	occurred := time.Date(2026, 11, 6, 3, 4, 5, 123, time.FixedZone("CST", 8*3600))
	event := Event{
		ID:         "evt_1",
		Type:       TypeChannelCredentialDisabled,
		Severity:   SeverityWarning,
		OccurredAt: occurred,
		Origin:     "plugin:acme",
		Depth:      1,
		Subject: Subject{
			Channel: &ChannelRef{
				ID:      7,
				Name:    "openai",
				Type:    "openai",
				BaseURL: "https://api.example",
				Status:  "enabled",
			},
			Credential: &CredentialRef{
				Fingerprint: Fingerprint(secret),
				Masked:      Mask(secret),
			},
			Model:   "gpt-5",
			APIKey:  &APIKeyRef{ID: 3, Name: "primary"},
			Project: &ProjectRef{ID: 9},
			Request: &RequestRef{ID: 11},
		},
		Data: map[string]any{
			"errorCode": "invalid_api_key",
			"reason":    "auth",
			"expiresAt": "2026-11-07T00:00:00Z",
		},
	}

	got := event.Map()
	require.Equal(t, map[string]any{
		"id":         "evt_1",
		"type":       TypeChannelCredentialDisabled,
		"severity":   SeverityWarning,
		"occurredAt": "2026-11-05T19:04:05Z",
		"subject": map[string]any{
			"channel": map[string]any{
				"id":      7,
				"name":    "openai",
				"type":    "openai",
				"baseUrl": "https://api.example",
				"status":  "enabled",
			},
			"credential": map[string]any{
				"fingerprint": Fingerprint(secret),
				"masked":      "sk-l****wxyz",
			},
			"model":   "gpt-5",
			"apiKey":  map[string]any{"id": 3, "name": "primary"},
			"project": map[string]any{"id": 9},
			"request": map[string]any{"id": 11},
		},
		"data": map[string]any{
			"errorCode": "invalid_api_key",
			"reason":    "auth",
			"expiresAt": "2026-11-07T00:00:00Z",
		},
		"origin": "plugin:acme",
		"depth":  1,
	}, got)
	assertNoSecret(t, got, secret)

	event.Data["errorCode"] = "mutated-source"
	require.Equal(t, "invalid_api_key", got["data"].(map[string]any)["errorCode"])
	got["data"].(map[string]any)["reason"] = "mutated-view"
	require.Equal(t, "auth", event.Data["reason"])

	sparse := Event{
		ID:       "evt_2",
		Type:     TypeRequestCompleted,
		Severity: SeverityInfo,
		Subject:  Subject{Model: "gpt-5"},
	}.Map()
	require.Equal(t, map[string]any{"model": "gpt-5"}, sparse["subject"])
	require.Equal(t, map[string]any{}, sparse["data"])
	for _, key := range []string{"channel", "credential", "apiKey", "project", "request"} {
		require.NotContains(t, sparse["subject"], key)
	}

	empty := Event{}.Map()
	require.Empty(t, empty["subject"])
	require.NotContains(t, empty["subject"], "model")
}

func assertNoSecret(t *testing.T, value any, secret string) {
	t.Helper()
	switch typed := value.(type) {
	case string:
		require.NotContains(t, typed, secret)
		require.False(t, strings.Contains(strings.ToLower(typed), "plaintext"))
	case map[string]any:
		for key, child := range typed {
			require.NotContains(t, key, secret)
			assertNoSecret(t, child, secret)
		}
	}
}
