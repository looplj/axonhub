package biz

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
)

func TestOIDCCallbackUserInfo(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	for _, tc := range []struct {
		name       string
		idEmail    string
		idVerified bool
		info       map[string]any
		oauthOnly  bool
		wantEmail  string
		wantError  string
	}{
		{
			name:       "different unverified email",
			idEmail:    "attacker@example.test",
			idVerified: true,
			info:       map[string]any{"sub": "subject", "email": "victim@example.test", "email_verified": false},
			wantError:  "account not found",
		},
		{
			name:       "different email missing verification",
			idEmail:    "attacker@example.test",
			idVerified: true,
			info:       map[string]any{"sub": "subject", "email": "victim@example.test"},
			wantError:  "account not found",
		},
		{
			name:       "same unverified email",
			idEmail:    "victim@example.test",
			idVerified: true,
			info:       map[string]any{"sub": "subject", "email": "victim@example.test", "email_verified": false},
			wantError:  "account not found",
		},
		{
			name:      "verification without email cannot upgrade ID email",
			idEmail:   "victim@example.test",
			info:      map[string]any{"sub": "subject", "email_verified": true},
			wantError: "account not found",
		},
		{
			name:       "verified replacement email",
			idEmail:    "attacker@example.test",
			idVerified: true,
			info:       map[string]any{"sub": "subject", "email": "victim@example.test", "email_verified": true},
			wantEmail:  "victim@example.test",
		},
		{
			name:       "no replacement preserves ID pair",
			idEmail:    "attacker@example.test",
			idVerified: true,
			info:       map[string]any{"sub": "subject", "name": "Profile"},
			wantEmail:  "attacker@example.test",
		},
		{
			name:       "mismatched subject",
			idEmail:    "attacker@example.test",
			idVerified: true,
			info:       map[string]any{"sub": "other", "email": "victim@example.test", "email_verified": true},
			wantError:  "subject does not match",
		},
		{
			name:       "missing subject",
			idEmail:    "attacker@example.test",
			idVerified: true,
			info:       map[string]any{"email": "victim@example.test", "email_verified": true},
			wantError:  "subject does not match",
		},
		{
			name:      "OAuth only verified email",
			oauthOnly: true,
			info:      map[string]any{"sub": "subject", "email": "victim@example.test", "email_verified": true},
			wantEmail: "victim@example.test",
		},
		{
			name:      "OAuth only unverified email",
			oauthOnly: true,
			info:      map[string]any{"sub": "subject", "email": "victim@example.test", "email_verified": false},
			wantError: "account not found",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, client := setupTestOIDCService(t)
			defer client.Close()
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			for _, email := range []string{"attacker@example.test", "victim@example.test"} {
				_, err := client.User.Create().SetEmail(email).SetPassword("unused").Save(ctx)
				require.NoError(t, err)
			}

			// Exercise the actual token exchange and UserInfo HTTP path. The ID token
			// is signed and checked against the local public key, without external I/O.
			var signedToken string
			mux := http.NewServeMux()
			mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]any{"access_token": "test-access", "token_type": "Bearer"}
				if !tc.oauthOnly {
					response["id_token"] = signedToken
				}
				_ = json.NewEncoder(w).Encode(response)
			})
			mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tc.info)
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
				"iss": server.URL, "aud": "client", "sub": "subject",
				"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
				"email": tc.idEmail, "email_verified": tc.idVerified,
			})
			signedToken, err = token.SignedString(key)
			require.NoError(t, err)
			svc.providers["test"] = &oidcProvider{
				config: OIDCProvider{ID: "test", Name: "test", IssuerURL: server.URL,
					UserInfoURL: server.URL + "/userinfo", AutoLinkByEmail: true,
					RequireEmailVerified: true, JITEnabled: false},
				oauth2: oauth2.Config{ClientID: "client", ClientSecret: "secret",
					Endpoint: oauth2.Endpoint{TokenURL: server.URL + "/token"}},
				verifier: oidc.NewVerifier(server.URL, &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: "client"}),
			}
			require.NoError(t, svc.cache.Set(ctx, "oidc_state:state", []byte("valid")))
			code, flow, err := svc.Callback(ctx, "test", "authorization-code", "state", "")
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				require.Empty(t, code)
				require.Empty(t, flow)
				count, err := client.OIDCIdentity.Query().Count(ctx)
				require.NoError(t, err)
				require.Zero(t, count, "rejected claims must not create an identity")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "login", flow)
			require.NotEmpty(t, code)
			loggedIn, err := svc.ExchangeCode(ctx, code)
			require.NoError(t, err)
			require.Equal(t, tc.wantEmail, loggedIn.Email)
			identity, err := client.OIDCIdentity.Query().Only(ctx)
			require.NoError(t, err)
			require.Equal(t, loggedIn.ID, identity.UserID)
			require.Equal(t, "subject", identity.Subject)
		})
	}
}
