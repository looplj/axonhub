package scopes

// Scope represents a permission scope
type Scope string

// Available scopes in the system
const (
	// Channel scopes
	ScopeReadChannels  Scope = "read_channels"
	ScopeWriteChannels Scope = "write_channels"

	// User scopes
	ScopeReadUsers  Scope = "read_users"
	ScopeWriteUsers Scope = "write_users"

	// Role scopes
	ScopeReadRoles  Scope = "read_roles"
	ScopeWriteRoles Scope = "write_roles"

	// API Key scopes
	ScopeReadAPIKeys  Scope = "read_api_keys"
	ScopeWriteAPIKeys Scope = "write_api_keys"

	// Request scopes
	ScopeReadRequests  Scope = "read_requests"
	ScopeWriteRequests Scope = "write_requests"

	// Job scopes
	ScopeReadJobs  Scope = "read_jobs"
	ScopeWriteJobs Scope = "write_jobs"

	// Dashboard scopes
	ScopeReadDashboard Scope = "read_dashboard"

	// Settings scopes
	ScopeReadSettings  Scope = "read_settings"
	ScopeWriteSettings Scope = "write_settings"

	// Admin scope - full access
	ScopeAdmin Scope = "admin"
)

// AllScopes returns all available scopes
func AllScopes() []Scope {
	return []Scope{
		ScopeReadChannels,
		ScopeWriteChannels,
		ScopeReadUsers,
		ScopeWriteUsers,
		ScopeReadRoles,
		ScopeWriteRoles,
		ScopeReadAPIKeys,
		ScopeWriteAPIKeys,
		ScopeReadRequests,
		ScopeWriteRequests,
		ScopeReadJobs,
		ScopeWriteJobs,
		ScopeReadDashboard,
		ScopeReadSettings,
		ScopeWriteSettings,
		ScopeAdmin,
	}
}

// AllScopesAsStrings returns all available scopes as strings
func AllScopesAsStrings() []string {
	scopes := AllScopes()
	result := make([]string, len(scopes))
	for i, scope := range scopes {
		result[i] = string(scope)
	}
	return result
}

// ScopeDescriptions returns human-readable descriptions for scopes
func ScopeDescriptions() map[Scope]string {
	return map[Scope]string{
		ScopeReadChannels:  "查看渠道信息",
		ScopeWriteChannels: "管理渠道（创建、编辑、删除）",
		ScopeReadUsers:     "查看用户信息",
		ScopeWriteUsers:    "管理用户（创建、编辑、删除）",
		ScopeReadRoles:     "查看角色信息",
		ScopeWriteRoles:    "管理角色（创建、编辑、删除）",
		ScopeReadAPIKeys:   "查看API密钥",
		ScopeWriteAPIKeys:  "管理API密钥（创建、编辑、删除）",
		ScopeReadRequests:  "查看请求记录",
		ScopeWriteRequests: "管理请求记录",
		ScopeReadJobs:      "查看任务信息",
		ScopeWriteJobs:     "管理任务",
		ScopeReadDashboard: "查看仪表板",
		ScopeReadSettings:  "查看系统设置",
		ScopeWriteSettings: "管理系统设置",
		ScopeAdmin:         "系统管理员（完全访问权限）",
	}
}

// IsValidScope checks if a scope is valid
func IsValidScope(scope string) bool {
	for _, validScope := range AllScopes() {
		if string(validScope) == scope {
			return true
		}
	}
	return false
}
