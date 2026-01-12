package scopes

import "slices"

// ScopeSlug represents a permission scope to view or manage the data of the system.
// Every user can view and manage their own data, and manage data of other users if they have the appropriate scopes.
type ScopeSlug string

// Available scopes in the system.
const (
	// ScopeReadDashboard read the dashboard of the system.
	ScopeReadDashboard ScopeSlug = "read_dashboard"

	// ScopeReadChannels read the channels/models of the system.
	ScopeReadChannels ScopeSlug = "read_channels"
	// ScopeWriteChannels manage the channels/models of the system.
	ScopeWriteChannels ScopeSlug = "write_channels"

	// ScopeReadDataStorages read the data storages of the system.
	ScopeReadDataStorages ScopeSlug = "read_data_storages"
	// ScopeWriteDataStorages manage the data storages of the system.
	ScopeWriteDataStorages ScopeSlug = "write_data_storages"

	// ScopeReadUsers read the users of the system or project.
	ScopeReadUsers ScopeSlug = "read_users"
	// ScopeWriteUsers manage the users of the system or project.
	ScopeWriteUsers ScopeSlug = "write_users"

	// ScopeReadSettings read the settings of the project.
	ScopeReadSettings ScopeSlug = "read_settings"
	// ScopeWriteSettings manage the settings of the project.
	ScopeWriteSettings ScopeSlug = "write_settings"

	// ScopeReadRoles read the roles of the system or project.
	ScopeReadRoles ScopeSlug = "read_roles"
	// ScopeWriteRoles manage the roles of the system or project.
	ScopeWriteRoles ScopeSlug = "write_roles"

	// ScopeReadProjects read the projects of the system.
	ScopeReadProjects ScopeSlug = "read_projects"
	// ScopeWriteProjects manage the projects of the system.
	ScopeWriteProjects ScopeSlug = "write_projects"

	// ScopeReadAPIKeys read the api keys of the project.
	//nolint:gosec // False positive.
	ScopeReadAPIKeys ScopeSlug = "read_api_keys"
	// ScopeWriteAPIKeys manage the api keys of the project.
	ScopeWriteAPIKeys ScopeSlug = "write_api_keys"

	// ScopeReadRequests read the requests of the project.
	ScopeReadRequests ScopeSlug = "read_requests"
	// ScopeWriteRequests manage the requests of the project.
	ScopeWriteRequests ScopeSlug = "write_requests"

	// ScopeReadPrompts read the prompts of the project.
	ScopeReadPrompts ScopeSlug = "read_prompts"
	// ScopeWritePrompts manage the prompts of the project.
	ScopeWritePrompts ScopeSlug = "write_prompts"
)

type ScopeLevel string

const (
	// ScopeLevelSystem is the scope level for system-wide operations.
	// If a user has a scope with ScopeLevelSystem, they can perform operations on the entire system.
	ScopeLevelSystem ScopeLevel = "system"

	// ScopeLevelProject is the scope level for project-specific operations.
	// If a user has a scope with ScopeLevelProject, they can perform operations on the project they are associated with.
	ScopeLevelProject ScopeLevel = "project"
)

type Scope struct {
	Slug        ScopeSlug
	Description string
	Levels      []ScopeLevel
}

// scopeConfigs defines all available scopes with their configurations.
var scopeConfigs = []Scope{
	{
		Slug:        ScopeReadDashboard,
		Description: "View dashboard",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeReadSettings,
		Description: "View system settings",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeWriteSettings,
		Description: "Manage system settings",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeReadChannels,
		Description: "View channel information",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeWriteChannels,
		Description: "Manage channels/models (create, edit, delete)",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeReadDataStorages,
		Description: "View data storage information",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeWriteDataStorages,
		Description: "Manage data storages (create, edit, delete)",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeReadUsers,
		Description: "View user information",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeWriteUsers,
		Description: "Manage users (create, edit, delete)",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeReadRoles,
		Description: "View role information",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeWriteRoles,
		Description: "Manage roles (create, edit, delete)",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeReadProjects,
		Description: "View project information",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeWriteProjects,
		Description: "Manage projects (create, edit, delete)",
		Levels:      []ScopeLevel{ScopeLevelSystem},
	},
	{
		Slug:        ScopeReadAPIKeys,
		Description: "View API keys",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeWriteAPIKeys,
		Description: "Manage API keys (create, edit, delete)",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeReadRequests,
		Description: "View request records",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeWriteRequests,
		Description: "Manage request records",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeReadPrompts,
		Description: "View prompts",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
	{
		Slug:        ScopeWritePrompts,
		Description: "Manage prompts (create, edit, delete)",
		Levels:      []ScopeLevel{ScopeLevelSystem, ScopeLevelProject},
	},
}

// AllScopes returns all available scopes, optionally filtered by level.
func AllScopes(level *ScopeLevel) []Scope {
	if level == nil {
		return scopeConfigs
	}

	filtered := make([]Scope, 0)

	for _, scope := range scopeConfigs {
		if slices.Contains(scope.Levels, *level) {
			filtered = append(filtered, scope)
		}
	}

	return filtered
}

// AllScopesAsStrings returns all available scopes as strings.
func AllScopesAsStrings() []string {
	scopes := AllScopes(nil)

	result := make([]string, len(scopes))
	for i, scope := range scopes {
		result[i] = string(scope.Slug)
	}

	return result
}

// IsValidScope checks if a scope is valid.
func IsValidScope(scope string) bool {
	for _, validScope := range AllScopes(nil) {
		if string(validScope.Slug) == scope {
			return true
		}
	}

	return false
}
