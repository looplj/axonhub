package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOIDCHandlersGetBaseURLRequiresConfiguredPublicURL(t *testing.T) {
	h := &OIDCHandlers{}

	got, err := h.getBaseURL()

	require.ErrorIs(t, err, errOIDCPublicURLRequired)
	require.Empty(t, got)
}

func TestOIDCHandlersGetBaseURLDoesNotUseRequestHost(t *testing.T) {
	h := &OIDCHandlers{publicURL: "https://axonhub.example.com/"}

	got, err := h.getBaseURL()

	require.NoError(t, err)
	require.Equal(t, "https://axonhub.example.com", got)
}

func TestOIDCHandlersGetBaseURLRejectsUnsafeConfiguration(t *testing.T) {
	for _, publicURL := range []string{
		"//attacker.example.com",
		"https://axonhub.example.com/callback?next=attacker",
		"https://user:secret@axonhub.example.com",
		"https://:443",
		"ftp://axonhub.example.com",
	} {
		t.Run(publicURL, func(t *testing.T) {
			h := &OIDCHandlers{publicURL: publicURL}

			_, err := h.getBaseURL()

			require.Error(t, err)
		})
	}
}
