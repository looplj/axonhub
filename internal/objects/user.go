package objects

// UserInfo contains the authenticated user's profile, permissions, and projects.
type UserInfo struct {
	ID             GUID               `json:"id"`
	Email          string             `json:"email"`
	FirstName      string             `json:"firstName"`
	LastName       string             `json:"lastName"`
	IsOwner        bool               `json:"isOwner"`
	PreferLanguage string             `json:"preferLanguage"`
	Avatar         *string            `json:"avatar,omitempty"`
	Scopes         []string           `json:"scopes"`
	Roles          []RoleInfo         `json:"roles"`
	Projects       []UserProjectInfo  `json:"projects"`
	OIDCIdentities []OIDCIdentityInfo `json:"oidcIdentities"`
	HasPassword    bool               `json:"hasPassword"`
}

// OIDCIdentityInfo contains an identity provider association.
type OIDCIdentityInfo struct {
	ID      GUID   `json:"id"`
	IdpName string `json:"idpName"`
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
	Email   string `json:"email"`
}

// UserProjectInfo contains a user's membership and effective permissions for a project.
type UserProjectInfo struct {
	ProjectID       GUID       `json:"projectID"`
	IsOwner         bool       `json:"isOwner"`
	Scopes          []string   `json:"scopes"`
	EffectiveScopes []string   `json:"effectiveScopes"`
	Roles           []RoleInfo `json:"roles"`
}

// RoleInfo contains the display name of a role.
type RoleInfo struct {
	Name string `json:"name"`
}
