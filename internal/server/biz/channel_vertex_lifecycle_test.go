// 职责: 验证混合 Vertex 凭证的删除、禁用和恢复行为。
// 关联文件:
//
//	internal/server/biz/channel.go
//	internal/server/biz/channel_apikey.go
//	internal/server/biz/channel_auto_disable.go
//	internal/server/biz/channel_events.go
package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/events"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestVertexCredentialDeletionPreservesObjects(t *testing.T) {
	account := objects.GCPCredential{ProjectID: "target-project", Region: "global", JSONData: `{"client_email":"synthetic@example.invalid","private_key_id":"signing-key","private_key":"synthetic-secret"}`}
	other := objects.GCPCredential{ProjectID: "other-project", Region: "us-central1", JSONData: `{"client_email":"other@example.invalid","private_key_id":"other-key"}`}
	vertex := objects.VertexAPIKey{APIKey: "synthetic-api-key", ProjectID: "key-project", Region: "global"}
	for _, tc := range []struct {
		name        string
		credentials objects.ChannelCredentials
		remove      []string
		want        objects.ChannelCredentials
		preserved   bool
	}{
		{"last account", objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, []string{account.CredentialRef()}, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, true},
		{"key removed with account remaining", objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{vertex}, VertexServiceAccounts: []objects.GCPCredential{account}}, []string{vertex.APIKey}, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, false},
		{"account removed with key remaining", objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{vertex}, VertexServiceAccounts: []objects.GCPCredential{account}}, []string{account.CredentialRef()}, objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{vertex}}, false},
		{"unrelated account retained", objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account, other}}, []string{account.CredentialRef()}, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{other}}, false},
		{"unknown ref cannot become a key", objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, []string{"vertex-sa:unknown"}, objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, client := setupTestChannelService(t)
			defer client.Close()
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			disabled := make([]objects.DisabledAPIKey, len(tc.remove))
			for i, ref := range tc.remove {
				disabled[i] = objects.DisabledAPIKey{Key: ref}
			}
			ch, err := client.Channel.Create().SetName(tc.name).SetType(channel.TypeGeminiVertex).
				SetCredentials(tc.credentials).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetDisabledAPIKeys(disabled).Save(ctx)
			require.NoError(t, err)

			result, err := svc.DeleteDisabledAPIKeys(ctx, ch.ID, tc.remove)

			require.NoError(t, err)
			require.True(t, result.Success)
			require.Equal(t, tc.preserved, result.Message == "ONE_KEY_PRESERVED")
			stored, err := client.Channel.Get(ctx, ch.ID)
			require.NoError(t, err)
			require.Equal(t, tc.want.GetAllCredentialRefs(), stored.Credentials.GetAllCredentialRefs())
			require.Equal(t, tc.want.VertexServiceAccounts, stored.Credentials.VertexServiceAccounts)
			require.Equal(t, tc.want.VertexAPIKeys, stored.Credentials.VertexAPIKeys)
			require.Empty(t, stored.Credentials.APIKeys)
			require.Empty(t, stored.Credentials.APIKey)
			require.Empty(t, stored.DisabledAPIKeys)
		})
	}
}

func TestVertexCredentialDisableAndRecovery(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	account := objects.GCPCredential{JSONData: `{"client_email":"synthetic@example.invalid","private_key_id":"key"}`}
	credentials := objects.ChannelCredentials{APIKeys: []string{"synthetic-key"}, VertexServiceAccounts: []objects.GCPCredential{account}}
	ch, err := client.Channel.Create().SetName("mixed lifecycle").SetType(channel.TypeGeminiVertex).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{StatusCodes: []int{401}, Times: 1, Action: objects.APIKeyAutoDisableActionPermanent}}}).
		SetCredentials(credentials).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").SetStatus(channel.StatusEnabled).Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{ChannelID: ch.ID, APIKey: account.CredentialRef(), ResponseStatusCode: 401})
	require.True(t, matched)
	require.True(t, acted)
	stored, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, stored.Status)
	require.Equal(t, []string{"synthetic-key"}, stored.Credentials.GetEnabledCredentialRefs(stored.DisabledAPIKeys))

	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, "synthetic-key", 401, "synthetic failure"))
	stored, err = client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.True(t, channelIsAllKeysDisabled(stored))
	metadata, err := svc.GetChannelForCredentialTest(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, credentials.GetAllCredentialRefs(), metadata.Credentials.GetAllCredentialRefs())

	require.NoError(t, svc.EnableAPIKey(ctx, ch.ID, account.CredentialRef()))
	stored, err = client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, stored.Status)
	require.Equal(t, []string{account.CredentialRef()}, stored.Credentials.GetEnabledCredentialRefs(stored.DisabledAPIKeys))
	require.Equal(t, credentials, stored.Credentials)
}

func TestVertexCredentialRecoveryRespectsManualDisable(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	account := objects.GCPCredential{JSONData: `{"client_email":"synthetic@example.invalid","private_key_id":"key"}`}
	ch, err := client.Channel.Create().SetName("manual lifecycle").SetType(channel.TypeGeminiVertex).
		SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
		SetCredentials(objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}).
		SetStatus(channel.StatusDisabled).SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: account.CredentialRef(), DisabledAt: time.Now()}}).Save(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.EnableAllAPIKeys(ctx, ch.ID))

	stored, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, stored.Status)
	require.Empty(t, stored.DisabledAPIKeys)
}

func TestVertexCredentialEventUsesSafeReference(t *testing.T) {
	account := objects.GCPCredential{JSONData: `{"client_email":"synthetic@example.invalid","private_key_id":"key","private_key":"synthetic-secret"}`}
	ref := account.CredentialRef()
	got := credentialEventRef(false, ref)
	require.Equal(t, events.Fingerprint(ref), got.Fingerprint)
	require.Equal(t, "vertex-sa:****"+ref[len(ref)-4:], got.Masked)
}

func TestVertexCredentialReplacementRecoversOnlyExhaustedChannels(t *testing.T) {
	original := syntheticVertexAccount(t, "old")
	rotated := syntheticVertexAccount(t, "new")
	rotated.JSONData = strings.ReplaceAll(rotated.JSONData, "new@identity-project.iam.gserviceaccount.com", "old@identity-project.iam.gserviceaccount.com")
	resourceEdit := original
	resourceEdit.ProjectID = "edited-target"
	resourceEdit.Region = "us-central1"
	for _, tc := range []struct {
		name        string
		initial     channel.Status
		scheduled   bool
		replacement objects.GCPCredential
		wantStatus  channel.Status
		wantDisable bool
	}{
		{"rotation recovers exhaustion", channel.StatusEnabled, false, rotated, channel.StatusEnabled, false},
		{"resource edit retains disabled identity", channel.StatusEnabled, false, resourceEdit, channel.StatusDisabled, true},
		{"rotation preserves manual disable", channel.StatusDisabled, false, rotated, channel.StatusDisabled, false},
		{"rotation preserves scheduled disable", channel.StatusDisabled, true, rotated, channel.StatusDisabled, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, client := setupTestChannelService(t)
			defer client.Close()
			svc.WebhookNotifier = NewWebhookNotifier(svc.SystemService, httpclient.NewHttpClient())
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			create := client.Channel.Create().SetName(tc.name).SetType(channel.TypeGeminiVertex).
				SetBaseURL("https://example.invalid/v1").SetStatus(tc.initial).
				SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
				SetCredentials(objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{original}})
			expiresAt := time.Now().Add(time.Hour)
			if tc.scheduled {
				create.SetErrorMessage(scheduledDisableReason).SetAutoDisabledAt(time.Now()).SetAutoDisableExpiresAt(expiresAt)
			}
			ch, err := create.Save(ctx)
			require.NoError(t, err)
			svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})
			require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, original.CredentialRef(), 401, "synthetic failure"))
			disabled, err := client.Channel.Get(ctx, ch.ID)
			require.NoError(t, err)
			require.Equal(t, channel.StatusDisabled, disabled.Status)
			require.Len(t, disabled.DisabledAPIKeys, 1)
			if tc.initial == channel.StatusEnabled {
				require.True(t, channelIsAllKeysDisabled(disabled))
			}
			credentials := objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{tc.replacement}}

			updated, err := svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{Credentials: &credentials})

			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, updated.Status)
			require.Equal(t, tc.wantDisable, len(updated.DisabledAPIKeys) == 1)
			require.Equal(t, credentials, updated.Credentials)
			if tc.wantStatus == channel.StatusEnabled {
				require.Nil(t, updated.ErrorMessage)
				require.Nil(t, updated.AutoDisabledAt)
				require.Nil(t, updated.AutoDisableExpiresAt)
			} else {
				require.Equal(t, disabled.ErrorMessage, updated.ErrorMessage)
				require.Equal(t, disabled.AutoDisabledAt, updated.AutoDisabledAt)
				require.Equal(t, disabled.AutoDisableExpiresAt, updated.AutoDisableExpiresAt)
			}
		})
	}
}
