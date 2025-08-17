package scopes

// Scope represents a permission scope to view or manage the data of the system.
// Every user can view and manage their own data, and manage data of other users if they have the appropriate scopes.
type Scope string

// Available scopes in the system.
const (
	// Channel scopes.
	ScopeReadChannels  Scope = "read_channels"
	ScopeWriteChannels Scope = "write_channels"

	// User scopes.
	ScopeReadUsers  Scope = "read_users"
	ScopeWriteUsers Scope = "write_users"

	// Role scopes.
	ScopeReadRoles  Scope = "read_roles"
	ScopeWriteRoles Scope = "write_roles"

	// API Key scopes.
	//nolint:gosec // This is a scope, not a secret.
	ScopeReadAPIKeys  Scope = "read_api_keys"
	ScopeWriteAPIKeys Scope = "write_api_keys"

	// Request scopes.
	ScopeReadRequests  Scope = "read_requests"
	ScopeWriteRequests Scope = "write_requests"

	// Dashboard scopes.
	ScopeReadDashboard Scope = "read_dashboard"

	// Settings scopes.
	ScopeReadSettings  Scope = "read_settings"
	ScopeWriteSettings Scope = "write_settings"
)

// AllScopes returns all available scopes.
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
		ScopeReadDashboard,
		ScopeReadSettings,
		ScopeWriteSettings,
	}
}

// AllScopesAsStrings returns all available scopes as strings.
func AllScopesAsStrings() []string {
	scopes := AllScopes()

	result := make([]string, len(scopes))
	for i, scope := range scopes {
		result[i] = string(scope)
	}

	return result
}

// ScopeDescriptions returns human-readable descriptions for scopes.
func ScopeDescriptions() map[Scope]string {
	return map[Scope]string{
		ScopeReadChannels:  "View channel information",
		ScopeWriteChannels: "Manage channels (create, edit, delete)",
		ScopeReadUsers:     "View user information",
		ScopeWriteUsers:    "Manage users (create, edit, delete)",
		ScopeReadRoles:     "View role information",
		ScopeWriteRoles:    "Manage roles (create, edit, delete)",
		ScopeReadAPIKeys:   "View API keys",
		ScopeWriteAPIKeys:  "Manage API keys (create, edit, delete)",
		ScopeReadRequests:  "View request records",
		ScopeWriteRequests: "Manage request records",
		ScopeReadDashboard: "View dashboard",
		ScopeReadSettings:  "View system settings",
		ScopeWriteSettings: "Manage system settings",
	}
}

// IsValidScope checks if a scope is valid.
func IsValidScope(scope string) bool {
	for _, validScope := range AllScopes() {
		if string(validScope) == scope {
			return true
		}
	}

	return false
}
