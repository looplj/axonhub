// 职责: 验证保存 Vertex JSON 凭证时清理旧引用并保护渠道类型边界。
// 关联文件:
//
//	internal/server/biz/channel.go
//	internal/objects/vertex_credential.go
package biz

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestChannelService_UpdateVertexCredentialsPrunesReplacedDisableRefs(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	accountJSON := func(keyID string) string {
		data, err := json.Marshal(map[string]string{
			"type": "service_account", "project_id": "identity-project", "private_key_id": keyID,
			"private_key": privateKey, "client_email": "account@identity-project.iam.gserviceaccount.com",
			"token_uri": "https://oauth2.googleapis.com/token",
		})
		require.NoError(t, err)
		return string(data)
	}
	for _, tt := range []struct {
		name         string
		keyID        string
		wantDisabled bool
	}{
		{"resource edit preserves identity", "old-key", true},
		{"signing key rotation removes old identity", "new-key", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, client := setupTestChannelService(t)
			defer client.Close()
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			original := objects.GCPCredential{ProjectID: "resource-project", Region: "global", JSONData: accountJSON("old-key")}
			credentials := objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{original}}
			stored, err := client.Channel.Create().SetType(channel.TypeGeminiVertex).SetName("Vertex update").
				SetBaseURL("https://aiplatform.googleapis.com/v1").SetCredentials(credentials).
				SetSupportedModels([]string{"gemini-3.8-flash"}).SetDefaultTestModel("gemini-3.8-flash").
				SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: original.CredentialRef(), ErrorCode: 401}}).Save(ctx)
			require.NoError(t, err)
			credentials.VertexServiceAccounts = []objects.GCPCredential{{ProjectID: "another-resource", Region: "us-central1", JSONData: accountJSON(tt.keyID)}}

			updated, err := svc.UpdateChannel(ctx, stored.ID, &ent.UpdateChannelInput{Credentials: &credentials})
			require.NoError(t, err)
			require.Equal(t, credentials.VertexServiceAccounts, updated.Credentials.VertexServiceAccounts)
			if tt.wantDisabled {
				require.Equal(t, stored.DisabledAPIKeys, updated.DisabledAPIKeys)
			} else {
				require.Empty(t, updated.DisabledAPIKeys)
			}

			otherType := channel.TypeOpenai
			_, err = svc.UpdateChannel(ctx, stored.ID, &ent.UpdateChannelInput{Type: &otherType})
			require.ErrorContains(t, err, "only supported by gemini_vertex")
			persisted, err := client.Channel.Get(ctx, stored.ID)
			require.NoError(t, err)
			require.Equal(t, channel.TypeGeminiVertex, persisted.Type)
		})
	}
}

func TestVertexJSONCredentialsRejectNonGeminiEndpointsOnSave(t *testing.T) {
	account := syntheticVertexAccount(t, "endpoint")
	jsonCredentials := objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}
	keyCredentials := objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{{APIKey: "synthetic-key", ProjectID: "test-project", Region: "global"}}}
	crossProtocol := []objects.ChannelEndpoint{{APIFormat: string(llm.APIFormatOpenAIChatCompletion)}}
	for _, operation := range []string{"create", "add JSON", "add endpoint"} {
		t.Run(operation, func(t *testing.T) {
			svc, client := setupTestChannelService(t)
			defer client.Close()
			ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			baseURL := "https://vertex.example.invalid"
			if operation == "create" {
				_, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
					Type: channel.TypeGeminiVertex, Name: "Vertex endpoint", BaseURL: &baseURL,
					Credentials: jsonCredentials, Endpoints: crossProtocol, SupportedModels: []string{"test-model"}, DefaultTestModel: "test-model",
				})
				require.ErrorContains(t, err, "service-account credentials require Gemini endpoints")
				return
			}
			credentials, endpoints := keyCredentials, crossProtocol
			patch := &ent.UpdateChannelInput{Credentials: &jsonCredentials}
			if operation == "add endpoint" {
				credentials, endpoints = jsonCredentials, nil
				patch = &ent.UpdateChannelInput{Endpoints: crossProtocol}
			}
			stored, err := client.Channel.Create().SetType(channel.TypeGeminiVertex).SetName("Vertex endpoint").SetBaseURL(baseURL).
				SetCredentials(credentials).SetEndpoints(endpoints).SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").Save(ctx)
			require.NoError(t, err)
			_, err = svc.UpdateChannel(ctx, stored.ID, patch)
			require.ErrorContains(t, err, "service-account credentials require Gemini endpoints")
			unchanged, err := client.Channel.Get(ctx, stored.ID)
			require.NoError(t, err)
			require.Equal(t, credentials, unchanged.Credentials)
			require.Equal(t, endpoints, unchanged.Endpoints)
		})
	}
}
