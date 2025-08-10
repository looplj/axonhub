package objects

// UserInfo 用户信息.
type UserInfo struct {
	Email          string   `json:"email"`
	FirstName      string   `json:"firstName"`
	LastName       string   `json:"lastName"`
	IsOwner        bool     `json:"isOwner"`
	PreferLanguage string   `json:"preferLanguage"`
	Scopes         []string `json:"scopes"`
	Roles          []Role   `json:"roles"`
}

// Role 角色信息.
type Role struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
