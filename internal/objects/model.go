package objects

type ModelCardReasoning struct {
	Supported bool `json:"supported"`
	Default   bool `json:"default"`
}

type ModelCardModalities struct {
	// "text","image","video"
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type ModelCardCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

type ModelCardLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

type ModelCard struct {
	Reasoning   ModelCardReasoning  `json:"reasoning"`
	ToolCall    bool                `json:"toolCall"`
	Temperature bool                `json:"temperature"`
	Modalities  ModelCardModalities `json:"modalities"`
	Vision      bool                `json:"vision"`
	Cost        ModelCardCost       `json:"cost"`
	Limit       ModelCardLimit      `json:"limit"`
	Knowledge   string              `json:"knowledge"`
	ReleaseDate string              `json:"releaseDate"`
	LastUpdated string              `json:"lastUpdated"`
}

type ModelSettings struct {
	Associations []*ModelAssociation `json:"associations"`
}

type ModelAssociation struct {
	// channel_model: the specified model id in the specified channel
	// channel_regex: the specified pattern in the specified channel
	// regex: the pattern for all channels
	// model: the specified model id
	// channel_tags_model: the specified model id in channels with specified tags (OR logic)
	// channel_tags_regex: the specified pattern in channels with specified tags (OR logic)
	Type             string                       `json:"type"`
	Priority         int                          `json:"priority"` // Lower value = higher priority, default 0
	ChannelModel     *ChannelModelAssociation     `json:"channelModel"`
	ChannelRegex     *ChannelRegexAssociation     `json:"channelRegex"`
	Regex            *RegexAssociation            `json:"regex"`
	ModelID          *ModelIDAssociation          `json:"modelId"`
	ChannelTagsModel *ChannelTagsModelAssociation `json:"channelTagsModel"`
	ChannelTagsRegex *ChannelTagsRegexAssociation `json:"channelTagsRegex"`
}

type ExcludeAssociation struct {
	ChannelNamePattern string   `json:"channelNamePattern"`
	ChannelIds         []int    `json:"channelIds"`
	ChannelTags        []string `json:"channelTags"`
}

type ChannelModelAssociation struct {
	ChannelID int    `json:"channelId"`
	ModelID   string `json:"modelId"`
}

type ChannelRegexAssociation struct {
	ChannelID int    `json:"channelId"`
	Pattern   string `json:"pattern"`
}

type RegexAssociation struct {
	Pattern string                `json:"pattern"`
	Exclude []*ExcludeAssociation `json:"exclude"`
}

type ModelIDAssociation struct {
	ModelID string                `json:"modelId"`
	Exclude []*ExcludeAssociation `json:"exclude"`
}

type ChannelTagsModelAssociation struct {
	ChannelTags []string `json:"channelTags"`
	ModelID     string   `json:"modelId"`
}

type ChannelTagsRegexAssociation struct {
	ChannelTags []string `json:"channelTags"`
	Pattern     string   `json:"pattern"`
}
