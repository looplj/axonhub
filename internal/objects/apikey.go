package objects

type APIKeyProfiles struct {
	ActiveProfile string          `json:"activeProfile"`
	Profiles      []APIKeyProfile `json:"profiles"`
}

type APIKeyProfile struct {
	Name          string         `json:"name"`
	ModelMappings []ModelMapping `json:"modelMappings"`
}
