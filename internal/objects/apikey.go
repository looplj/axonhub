package objects

type APIKeyProfiles struct {
	ActiveProfile string          `json:"activeProfile"`
	Profiles      []APIKeyProfile `json:"profiles"`
}

type APIKeyProfile struct {
	Name          string         `json:"name"`
	ModelMappings []ModelMapping `json:"modelMappings"`
	ChannelIDs    []int          `json:"channelIDs,omitempty"`
	ChannelTags   []string       `json:"channelTags,omitempty"` // 渠道标签过滤：包含任意标签的渠道可用
}
