package objects

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

type OIDCIdentityInfo struct {
	ID      GUID   `json:"id"`
	IdpName string `json:"idpName"`
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
	Email   string `json:"email"`
}

type UserProjectInfo struct {
	ProjectID GUID       `json:"projectID"`
	IsOwner   bool       `json:"isOwner"`
	Scopes    []string   `json:"scopes"`
	Roles     []RoleInfo `json:"roles"`
}

type RoleInfo struct {
	Name string `json:"name"`
}
