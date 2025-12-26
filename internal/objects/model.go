package objects

import (
	"time"
)

// ModelIdentify move to biz.
type ModelIdentify struct {
	ID string `json:"id"`
}

// ModelFacade move to biz.
type ModelFacade struct {
	ID string `json:"id"`
	// Display name, for user-friendly display from anthropic API.
	DisplayName string `json:"display_name"`
	// Created time in seconds.
	Created int64 `json:"created"`
	// Created time in time.Time.
	CreatedAt time.Time `json:"created_at"`
	// Owned by
	OwnedBy string `json:"owned_by"`
}

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
	Type         string                   `json:"type"`
	Priority     int                      `json:"priority"` // Lower value = higher priority, default 0
	ChannelModel *ChannelModelAssociation `json:"channelModel"`
	ChannelRegex *ChannelRegexAssociation `json:"channelRegex"`
	Regex        *RegexAssociation        `json:"regex"`
	ModelID      *ModelIDAssociation      `json:"modelId"`
}

type ExcludeAssociation struct {
	ChannelNamePattern string `json:"channelNamePattern"`
	ChannelIds         []int  `json:"channelIds"`
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
