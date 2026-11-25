// 职责: 验证 Vertex 凭证身份在编辑和轮换时的稳定性及混合枚举行为。
// 关联文件:
//
//	internal/objects/vertex_credential.go
//	internal/objects/channel.go
package objects

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVertexCredentialRefTracksSigningKey(t *testing.T) {
	original := GCPCredential{
		ProjectID: "resource-a", Region: "global",
		JSONData: `{"client_email":"account@example.test","private_key_id":"key-a","private_key":"synthetic-secret"}`,
	}
	ref := original.CredentialRef()
	require.Len(t, ref, len(VertexServiceAccountRefPrefix)+64)
	require.NotContains(t, ref, "synthetic-secret")

	for _, tt := range []struct {
		name string
		json string
		same bool
	}{
		{"format and unrelated fields", `{"private_key_id":"key-a", "client_email":"account@example.test"}`, true},
		{"rotated key", `{"client_email":"account@example.test","private_key_id":"key-b"}`, false},
		{"other account", `{"client_email":"other@example.test","private_key_id":"key-a"}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			edited := GCPCredential{ProjectID: "resource-b", Region: "us-central1", JSONData: tt.json}
			require.Equal(t, tt.same, ref == edited.CredentialRef())
		})
	}
}

func TestVertexCredentialRefRejectsMissingIdentity(t *testing.T) {
	for _, data := range []string{"", "invalid", `{}`, `{"client_email":"account@example.test"}`, `{"private_key_id":"key-a"}`} {
		t.Run(data, func(t *testing.T) {
			require.Empty(t, (GCPCredential{JSONData: data}).CredentialRef())
		})
	}
}

func TestMixedVertexCredentialsSelectByReference(t *testing.T) {
	account := GCPCredential{ProjectID: "resource-a", Region: "global", JSONData: `{"client_email":"account@example.test","private_key_id":"key-a"}`}
	credentials := ChannelCredentials{
		VertexAPIKeys:         []VertexAPIKey{{APIKey: "key-one", ProjectID: "resource-b", Region: "us-central1"}},
		VertexServiceAccounts: []GCPCredential{account},
	}

	require.Equal(t, []string{"key-one"}, credentials.GetAllAPIKeys())
	require.Equal(t, []string{"key-one", account.CredentialRef()}, credentials.GetAllCredentialRefs())
	require.Equal(t, []string{"key-one"}, credentials.GetEnabledCredentialRefs([]DisabledAPIKey{{Key: account.CredentialRef(), DisabledAt: time.Now()}}))
	resolved, ok := credentials.VertexServiceAccount(account.CredentialRef())
	require.True(t, ok)
	require.Equal(t, account, resolved)
	_, ok = credentials.VertexServiceAccount("key-one")
	require.False(t, ok)
	require.False(t, credentials.IsOAuth())
}
