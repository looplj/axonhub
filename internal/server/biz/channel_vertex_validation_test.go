package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

// Vertex credentials are only meaningful on a Vertex channel, and a key must keep
// exactly one project/region binding because the runtime resolves the request URL
// by looking the selected key up. These tests lock the write-boundary rules that
// keep an unusable credential set out of the database: a saved-but-invalid triple
// would be rejected during outbound construction, which drops the whole channel
// from routing while the save still reports success.

func TestValidateVertexAPIKeys(t *testing.T) {
	valid := objects.VertexAPIKey{ProjectID: "miguo-vertex-prod-1", APIKey: "AQ.key-1", Region: "global"}

	tests := []struct {
		name        string
		channelType channel.Type
		credentials objects.ChannelCredentials
		wantErr     string
	}{
		{
			name:        "accepts a complete triple",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{valid}},
		},
		{
			name:        "accepts a numeric project id and a regional location",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "83977314182", APIKey: "AQ.key-1", Region: "us-central1"},
			}},
		},
		{
			name:        "accepts multiple keys with distinct bindings",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				valid,
				{ProjectID: "miguo-vertex-prod-2", APIKey: "AQ.key-2", Region: "global"},
			}},
		},
		{
			name:        "accepts a repeated key with an identical binding",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{valid, valid}},
		},
		{
			name:        "no vertex keys is not this validator's concern",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{APIKeys: []string{"plain-key"}},
		},
		{
			name:        "rejects a missing api key",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "miguo-vertex-prod-1", Region: "global"},
			}},
			wantErr: "are all required",
		},
		{
			name:        "rejects a missing project id",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				{APIKey: "AQ.key-1", Region: "global"},
			}},
			wantErr: "are all required",
		},
		{
			name:        "rejects a missing region",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "miguo-vertex-prod-1", APIKey: "AQ.key-1"},
			}},
			wantErr: "are all required",
		},
		{
			name:        "rejects a project id carrying a path separator",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "projects/evil", APIKey: "AQ.key-1", Region: "global"},
			}},
			wantErr: "not a valid Google project ID",
		},
		{
			name:        "rejects a region carrying a path separator",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "miguo-vertex-prod-1", APIKey: "AQ.key-1", Region: "global/../x"},
			}},
			wantErr: "not a valid Google region",
		},
		{
			name:        "rejects the same key bound to two projects",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				valid,
				{ProjectID: "miguo-vertex-prod-2", APIKey: valid.APIKey, Region: "global"},
			}},
			wantErr: "bound to both",
		},
		{
			name:        "rejects the same key bound to two regions",
			channelType: channel.TypeGeminiVertex,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{
				valid,
				{ProjectID: valid.ProjectID, APIKey: valid.APIKey, Region: "asia-southeast1"},
			}},
			wantErr: "bound to both",
		},
		{
			name:        "rejects vertex keys on an ordinary channel",
			channelType: channel.TypeOpenai,
			credentials: objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{valid}},
			wantErr:     "only supported by gemini_vertex channels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVertexAPIKeys(tt.channelType, tt.credentials)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// Deleting a disabled Vertex key must remove its triple, not just its disable
// record: GetAllAPIKeys surfaces Vertex keys, so a surviving triple becomes
// immediately selectable again while its disable record is gone.
func TestChannelService_DeleteDisabledAPIKeys_RemovesVertexTriple(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeGeminiVertex).
		SetName("Vertex Channel").
		SetBaseURL("https://aiplatform.googleapis.com/v1").
		SetCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "miguo-vertex-prod-1", APIKey: "AQ.key-1", Region: "global"},
				{ProjectID: "miguo-vertex-prod-2", APIKey: "AQ.key-2", Region: "global"},
			},
		}).
		SetSupportedModels([]string{"gemini-3.8-flash"}).
		SetDefaultTestModel("gemini-3.8-flash").
		SetDisabledAPIKeys([]objects.DisabledAPIKey{
			{Key: "AQ.key-2", ErrorCode: 401, Reason: "invalid key"},
		}).
		Save(ctx)
	require.NoError(t, err)

	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"AQ.key-2"})
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Empty(t, result.Message, "a remaining key means the last-key rule did not fire")

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Empty(t, updated.DisabledAPIKeys)
	require.Len(t, updated.Credentials.VertexAPIKeys, 1)
	require.Equal(t, "AQ.key-1", updated.Credentials.VertexAPIKeys[0].APIKey)
	require.Equal(t, "miguo-vertex-prod-1", updated.Credentials.VertexAPIKeys[0].ProjectID)
}

// Deleting the only Vertex key restores the whole triple so the surviving key
// keeps its project and region instead of degrading into a binding-less key.
func TestChannelService_DeleteDisabledAPIKeys_PreservesVertexBinding(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	ch, err := client.Channel.Create().
		SetType(channel.TypeGeminiVertex).
		SetName("Vertex Channel").
		SetBaseURL("https://aiplatform.googleapis.com/v1").
		SetCredentials(objects.ChannelCredentials{
			VertexAPIKeys: []objects.VertexAPIKey{
				{ProjectID: "miguo-vertex-prod-7", APIKey: "AQ.key-7", Region: "global"},
			},
		}).
		SetSupportedModels([]string{"gemini-3.8-flash"}).
		SetDefaultTestModel("gemini-3.8-flash").
		SetDisabledAPIKeys([]objects.DisabledAPIKey{
			{Key: "AQ.key-7", ErrorCode: 429, Reason: "quota"},
		}).
		Save(ctx)
	require.NoError(t, err)

	result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, []string{"AQ.key-7"})
	require.NoError(t, err)
	require.Equal(t, "ONE_KEY_PRESERVED", result.Message)

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updated.Credentials.VertexAPIKeys, 1)
	require.Equal(t, "AQ.key-7", updated.Credentials.VertexAPIKeys[0].APIKey)
	require.Equal(t, "miguo-vertex-prod-7", updated.Credentials.VertexAPIKeys[0].ProjectID)
	require.Equal(t, "global", updated.Credentials.VertexAPIKeys[0].Region)
	require.Empty(t, updated.Credentials.APIKeys, "the restored key must stay a bound triple")
}
