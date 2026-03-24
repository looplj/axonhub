package gql

import (
	"net/http"
	"testing"

	gqlclient "github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/server/biz"
)

func newSystemGraphQLTestClient(t *testing.T) (*gqlclient.Client, *biz.SystemService) {
	t.Helper()

	entClient := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() {
		_ = entClient.Close()
	})

	systemService := biz.NewSystemService(biz.SystemServiceParams{})
	resolver := &Resolver{systemService: systemService}
	server := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolver}))

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ent.NewContext(r.Context(), entClient)
		ctx = authz.WithTestBypass(ctx)
		server.ServeHTTP(w, r.WithContext(ctx))
	})

	return gqlclient.New(h), systemService
}

func TestGlobalCloakingConfigGraphQLRoundTrip(t *testing.T) {
	client, _ := newSystemGraphQLTestClient(t)

	mode := GlobalCloakingModeAlways
	sensitiveWordsMode := SensitiveWordsModeReplace
	tlsFingerprint := true
	headerAutoFill := false
	bodyCloak := true
	cacheUserID := false
	cacheControlAutoInject := true
	sensitiveWords := []string{"opencode", "cursor"}

	var mutationResp struct {
		UpdateGlobalCloakingConfig bool `json:"updateGlobalCloakingConfig"`
	}

	client.MustPost(`
		mutation UpdateGlobalCloakingConfig($input: UpdateGlobalCloakingConfigInput!) {
			updateGlobalCloakingConfig(input: $input)
		}
	`, &mutationResp, gqlclient.Var("input", UpdateGlobalCloakingConfigInput{
		Mode:                   &mode,
		TLSFingerprint:         &tlsFingerprint,
		HeaderAutoFill:         &headerAutoFill,
		BodyCloak:              &bodyCloak,
		CacheUserID:            &cacheUserID,
		SensitiveWordsMode:     &sensitiveWordsMode,
		SensitiveWords:         sensitiveWords,
		CacheControlAutoInject: &cacheControlAutoInject,
	}))

	require.True(t, mutationResp.UpdateGlobalCloakingConfig)

	var queryResp struct {
		GlobalCloakingConfig struct {
			Mode                   *GlobalCloakingMode `json:"mode"`
			TLSFingerprint         *bool               `json:"tlsFingerprint"`
			HeaderAutoFill         *bool               `json:"headerAutoFill"`
			BodyCloak              *bool               `json:"bodyCloak"`
			CacheUserID            *bool               `json:"cacheUserID"`
			SensitiveWordsMode     *SensitiveWordsMode `json:"sensitiveWordsMode"`
			SensitiveWords         []string            `json:"sensitiveWords"`
			CacheControlAutoInject *bool               `json:"cacheControlAutoInject"`
		} `json:"globalCloakingConfig"`
	}

	client.MustPost(`
		query GlobalCloakingConfig {
			globalCloakingConfig {
				mode
				tlsFingerprint
				headerAutoFill
				bodyCloak
				cacheUserID
				sensitiveWordsMode
				sensitiveWords
				cacheControlAutoInject
			}
		}
	`, &queryResp)

	require.Equal(t, &mode, queryResp.GlobalCloakingConfig.Mode)
	require.Equal(t, &tlsFingerprint, queryResp.GlobalCloakingConfig.TLSFingerprint)
	require.Equal(t, &headerAutoFill, queryResp.GlobalCloakingConfig.HeaderAutoFill)
	require.Equal(t, &bodyCloak, queryResp.GlobalCloakingConfig.BodyCloak)
	require.Equal(t, &cacheUserID, queryResp.GlobalCloakingConfig.CacheUserID)
	require.Equal(t, &sensitiveWordsMode, queryResp.GlobalCloakingConfig.SensitiveWordsMode)
	require.Equal(t, sensitiveWords, queryResp.GlobalCloakingConfig.SensitiveWords)
	require.Equal(t, &cacheControlAutoInject, queryResp.GlobalCloakingConfig.CacheControlAutoInject)
}

func TestUpdateGlobalCloakingConfigGraphQLPreservesEmptySensitiveWords(t *testing.T) {
	client, _ := newSystemGraphQLTestClient(t)

	mode := GlobalCloakingModeAuto
	sensitiveWordsMode := SensitiveWordsModeReplace
	sensitiveWords := []string{}

	var mutationResp struct {
		UpdateGlobalCloakingConfig bool `json:"updateGlobalCloakingConfig"`
	}

	client.MustPost(`
		mutation UpdateGlobalCloakingConfig($input: UpdateGlobalCloakingConfigInput!) {
			updateGlobalCloakingConfig(input: $input)
		}
	`, &mutationResp, gqlclient.Var("input", map[string]any{
		"mode":               mode,
		"sensitiveWordsMode": sensitiveWordsMode,
		"sensitiveWords":     sensitiveWords,
	}))

	require.True(t, mutationResp.UpdateGlobalCloakingConfig)

	var queryResp struct {
		GlobalCloakingConfig struct {
			SensitiveWords []string `json:"sensitiveWords"`
		} `json:"globalCloakingConfig"`
	}

	client.MustPost(`
		query GlobalCloakingConfig {
			globalCloakingConfig {
				sensitiveWords
			}
		}
	`, &queryResp)

	require.NotNil(t, queryResp.GlobalCloakingConfig.SensitiveWords)
	require.Empty(t, queryResp.GlobalCloakingConfig.SensitiveWords)
}
