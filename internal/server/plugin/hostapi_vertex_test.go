// 职责: 验证插件凭证指纹可定位 Vertex 服务账号且不使用 JSON 作为身份。
// 关联文件:
//
//	internal/server/plugin/hostapi_event.go
//	internal/objects/vertex_credential.go
package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/events"
)

func TestPluginMatchesVertexCredentialReference(t *testing.T) {
	account := objects.GCPCredential{
		ProjectID: "synthetic-project", Region: "global",
		JSONData: `{"type":"service_account","client_email":"synthetic@example.invalid","private_key_id":"synthetic-id","private_key":"synthetic-private-material"}`,
	}
	ref := account.CredentialRef()
	require.NotEmpty(t, ref)
	row := &ent.Channel{Credentials: objects.ChannelCredentials{
		VertexAPIKeys:         []objects.VertexAPIKey{{APIKey: "synthetic-api-key", ProjectID: "synthetic-project", Region: "global"}},
		VertexServiceAccounts: []objects.GCPCredential{account, account},
	}}

	for _, tc := range []struct {
		name     string
		identity string
		want     []string
	}{
		{name: "service account reference", identity: ref, want: []string{ref}},
		{name: "API key", identity: "synthetic-api-key", want: []string{"synthetic-api-key"}},
		{name: "raw JSON cannot address a credential", identity: account.JSONData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, matchingCredentialKeys(row, events.Fingerprint(tc.identity)))
		})
	}
}
