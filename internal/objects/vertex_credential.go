// 职责: 为 Vertex 服务账号凭证生成不含密钥的稳定引用并查找对应条目。
// 关联文件:
//
//	internal/objects/channel.go
//	internal/server/biz/channel_vertex_credentials.go
//
// 注意事项: 引用随账号或 signing key 变化，JSON 格式与资源 project/region 不影响引用。
package objects

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// VertexServiceAccountRefPrefix distinguishes signing-key identities from API keys.
const VertexServiceAccountRefPrefix = "vertex-sa:"

// CredentialRef identifies a service account signing key without retaining its secret.
// Invalid or incomplete credentials have no selectable identity.
func (c GCPCredential) CredentialRef() string {
	var identity struct {
		ClientEmail  string `json:"client_email"`
		PrivateKeyID string `json:"private_key_id"`
	}
	if err := json.Unmarshal([]byte(c.JSONData), &identity); err != nil || identity.ClientEmail == "" || identity.PrivateKeyID == "" {
		return ""
	}

	// Encoding a fixed string tuple cannot fail and preserves unambiguous boundaries.
	tuple, _ := json.Marshal([2]string{identity.ClientEmail, identity.PrivateKeyID})
	digest := sha256.Sum256(tuple)

	return VertexServiceAccountRefPrefix + hex.EncodeToString(digest[:])
}

// VertexServiceAccount resolves a reference against this credential snapshot.
func (c *ChannelCredentials) VertexServiceAccount(ref string) (GCPCredential, bool) {
	if c != nil && ref != "" {
		for _, credential := range c.VertexServiceAccounts {
			if credential.CredentialRef() == ref {
				return credential, true
			}
		}
	}

	return GCPCredential{}, false
}
