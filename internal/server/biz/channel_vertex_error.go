// 职责: 携带 Vertex 认证失败的安全凭证引用以便在发送推理请求前归因。
// 关联文件:
//
//	internal/server/biz/channel_vertex_credentials.go
//	internal/server/orchestrator/outbound.go
//
// 注意事项: 不保留 token endpoint 的响应或签名断言，避免错误日志泄露认证材料。
package biz

// VertexCredentialError identifies the signing key whose token exchange failed.
type VertexCredentialError struct {
	CredentialRef string
}

func (e *VertexCredentialError) Error() string {
	return "Vertex service-account token exchange failed"
}
