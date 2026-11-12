// 职责: 覆盖 GraphQL 上逐 key 额度开关与额度 JSON 的可见性边界。
// 关联文件:
//
//	internal/server/gql/axonhub.graphql
//	internal/server/biz/channel_apikey_quota.go
//	internal/server/biz/provider_quota.go
//	frontend/src/features/channels/data/schema.ts
//
// 遵循的 Rules:
//
//	.agents/rules/ent-graphql.md
//
// 注意事项:
//
//	disabledAPIKeys 只在持有 write_channels 时返回；额度 JSON 对 read_channels
//	可见，因此其中绝不能出现配置的明文密钥。
package gql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

const quotaOwnershipGraphQLQuery = `{"query":"query { queryChannels(input: {where: {statusIn: [enabled]}}) { edges { node { id disabledAPIKeys { key errorCode reason quotaDisabled } providerQuotaStatus { status quotaData } } } } }"}`

type quotaOwnershipGraphQLNode struct {
	ID              string `json:"id"`
	DisabledAPIKeys []struct {
		Key           string `json:"key"`
		ErrorCode     int    `json:"errorCode"`
		Reason        string `json:"reason"`
		QuotaDisabled bool   `json:"quotaDisabled"`
	} `json:"disabledAPIKeys"`
	ProviderQuotaStatus *struct {
		Status    string         `json:"status"`
		QuotaData map[string]any `json:"quotaData"`
	} `json:"providerQuotaStatus"`
}

// runQuotaOwnershipQuery executes the channel query with the given caller.
func runQuotaOwnershipQuery(t *testing.T, ctx context.Context, resolver *Resolver) ([]byte, *quotaOwnershipGraphQLNode) {
	t.Helper()

	server := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolver}))
	request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(quotaOwnershipGraphQLQuery))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)

	var result struct {
		Data struct {
			QueryChannels struct {
				Edges []struct {
					Node *quotaOwnershipGraphQLNode `json:"node"`
				} `json:"edges"`
			} `json:"queryChannels"`
		} `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Empty(t, result.Errors)
	require.Len(t, result.Data.QueryChannels.Edges, 1)

	return response.Body.Bytes(), result.Data.QueryChannels.Edges[0].Node
}

func TestDisabledAPIKeyQuotaOwnershipGraphQL(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	const configuredKey = "opencode-secret-aaaa-1111"

	ch, err := client.Channel.Create().
		SetName("OpenCode Go").
		SetType(channel.TypeOpencodeGo).
		SetBaseURL("https://opencode.ai/zen/go/v1").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{configuredKey}}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusEnabled).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{
			Key:           configuredKey,
			DisabledAt:    time.Now(),
			ErrorCode:     0,
			Reason:        "Provider quota exhausted",
			QuotaDisabled: true,
		}}).
		Save(ctx)
	require.NoError(t, err)

	// The persisted per-key view mirrors the production wire format: opaque ref,
	// display-only index/suffix, ownership flags and per-key limits.
	_, err = client.ProviderQuotaStatus.Create().
		SetChannelID(ch.ID).
		SetProviderType(providerquotastatus.ProviderTypeOpencodeGo).
		SetStatus(providerquotastatus.StatusExhausted).
		SetReady(false).
		SetNextCheckAt(time.Now().Add(time.Minute)).
		SetQuotaData(map[string]any{
			"_api_key_quotas": []map[string]any{{
				"ref":           "6f1c2d3e4a5b6c7d8e9f0a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f",
				"index":         1,
				"suffix":        "1111",
				"disabled":      true,
				"quotaDisabled": true,
				"status":        "exhausted",
				"ready":         false,
				"quotaData": map[string]any{
					"_limits": []map[string]any{{
						"type":       "token",
						"status":     "exhausted",
						"usageRatio": 1.0,
						"ready":      false,
						"window":     "5h",
					}},
				},
			}},
		}).
		Save(ctx)
	require.NoError(t, err)

	resolver := &Resolver{
		channelService: biz.NewChannelServiceForTest(client),
		// The channel's provider quota field consults the collection settings, so
		// a real system service is required.
		systemService: biz.NewSystemService(biz.SystemServiceParams{
			Ent:         client,
			CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		}),
	}

	// A caller allowed to write channels sees the disable records, including the
	// quota ownership flag.
	ownerCtx := contexts.WithUser(ctx, newQuotaOwnershipTestUser(t, ctx, client, true, nil))
	_, ownerNode := runQuotaOwnershipQuery(t, ownerCtx, resolver)
	require.Len(t, ownerNode.DisabledAPIKeys, 1)
	require.True(t, ownerNode.DisabledAPIKeys[0].QuotaDisabled)
	require.Equal(t, "Provider quota exhausted", ownerNode.DisabledAPIKeys[0].Reason)
	require.NotNil(t, ownerNode.ProviderQuotaStatus)

	// A caller with read-only channel scope never receives the disable records
	// but does receive the quota payload, so that payload must stay safe.
	body, readNode := runQuotaOwnershipQuery(t, contexts.WithUser(ctx,
		newQuotaOwnershipTestUser(t, ctx, client, false, []string{"read_channels"})), resolver)
	require.Nil(t, readNode.DisabledAPIKeys, "credential state is write-channel only")
	require.NotNil(t, readNode.ProviderQuotaStatus)
	require.Contains(t, readNode.ProviderQuotaStatus.QuotaData, "_api_key_quotas")

	serialized := string(body)
	require.NotContains(t, serialized, configuredKey, "the quota payload never carries a configured key")
	require.NotContains(t, serialized, "Authorization")
	require.Contains(t, serialized, "1111")
}

// newQuotaOwnershipTestUser creates a caller that either owns everything or holds
// exactly the given direct scopes.
func newQuotaOwnershipTestUser(t *testing.T, ctx context.Context, client *ent.Client, owner bool, directScopes []string) *ent.User {
	t.Helper()

	hashedPassword, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	created, err := client.User.Create().
		SetEmail(userEmailForTest()).
		SetPassword(hashedPassword).
		SetFirstName("Quota").
		SetLastName("Reader").
		SetStatus(user.StatusActivated).
		SetIsOwner(owner).
		SetScopes(directScopes).
		Save(ctx)
	require.NoError(t, err)

	return created
}

func userEmailForTest() string {
	return strings.Join([]string{"quota-ownership", strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")}, "-") + "@example.com"
}
