package biz

import (
	"testing"

	"github.com/go-viper/mapstructure/v2"
	"github.com/stretchr/testify/require"
)

func TestOIDCClaimsDecodeSnakeCase(t *testing.T) {
	var claims oidcClaims
	err := mapstructure.Decode(map[string]any{
		"sub":                "subject-1",
		"email":              "user@example.com",
		"email_verified":     true,
		"given_name":         "Given",
		"family_name":        "Family",
		"preferred_username": "user",
		"picture":            "https://example.com/avatar.png",
	}, &claims)

	require.NoError(t, err)
	require.Equal(t, "subject-1", claims.Sub)
	require.Equal(t, "user@example.com", claims.Email)
	require.True(t, claims.EmailVerified)
	require.Equal(t, "Given", claims.GivenName)
	require.Equal(t, "Family", claims.FamilyName)
	require.Equal(t, "user", claims.PreferredUsername)
	require.Equal(t, "https://example.com/avatar.png", claims.Picture)
}
