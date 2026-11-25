// 职责: 验证 Vertex 独立及混合凭证通过备份恢复保留完整数据。
// 关联文件:
//
//	internal/server/backup/restore.go
//	internal/server/backup/backup.go
package backup

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestRestoreVertexCredentialCollections(t *testing.T) {
	account := objects.GCPCredential{ProjectID: "synthetic-target", Region: "global", JSONData: `{"client_email":"synthetic@example.invalid","private_key_id":"key","private_key":"synthetic-secret"}`}
	key := objects.VertexAPIKey{APIKey: "synthetic-key", ProjectID: "synthetic-key-project", Region: "us-central1"}
	for _, tc := range []struct {
		name        string
		credentials objects.ChannelCredentials
	}{
		{"JSON only", objects.ChannelCredentials{VertexServiceAccounts: []objects.GCPCredential{account}}},
		{"API key only", objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{key}}},
		{"mixed", objects.ChannelCredentials{VertexAPIKeys: []objects.VertexAPIKey{key}, VertexServiceAccounts: []objects.GCPCredential{account}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, svc, ctx := setupBackupTest(t)
			defer client.Close()
			data, err := json.Marshal(BackupData{Version: BackupVersion, Channels: []*BackupChannel{{
				Name: tc.name, Type: channel.TypeGeminiVertex, BaseURL: "https://example.invalid/v1", Status: channel.StatusDisabled, Credentials: tc.credentials,
			}}})
			require.NoError(t, err)

			err = svc.Restore(ctx, bytes.NewReader(data), RestoreOptions{IncludeChannels: true})

			require.NoError(t, err)
			restored, err := client.Channel.Query().Where(channel.NameEQ(tc.name)).Only(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.credentials, restored.Credentials)
		})
	}
}
