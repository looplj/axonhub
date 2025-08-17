package objects

// UserInfo 用户信息.
type UserInfo struct {
	Email          string   `json:"email"`
	FirstName      string   `json:"firstName"`
	LastName       string   `json:"lastName"`
	IsOwner        bool     `json:"isOwner"`
	PreferLanguage string   `json:"preferLanguage"`
	Avatar         *string  `json:"avatar,omitempty"`
	Scopes         []string `json:"scopes"`
	Roles          []Role   `json:"roles"`
}

// Role 角色信息.
type Role struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
