package objects

import (
	"slices"

	"github.com/shopspring/decimal"
)

type APIKeyProfiles struct {
	ActiveProfile string          `json:"activeProfile"`
	Profiles      []APIKeyProfile `json:"profiles"`
}

type APIKeyProfile struct {
	Name                string         `json:"name"`
	ModelMappings       []ModelMapping `json:"modelMappings"`
	Quota               *APIKeyQuota   `json:"quota,omitempty"`
	LoadBalanceStrategy *string        `json:"loadBalanceStrategy,omitempty"`

	ChannelIDs           []int                `json:"channelIDs,omitempty"`
	ChannelTags          []string             `json:"channelTags,omitempty"`
	ChannelTagsMatchMode ChannelTagsMatchMode `json:"channelTagsMatchMode,omitempty"`
	ModelIDs             []string             `json:"modelIDs,omitempty"`
}

// ChannelTagsMatchMode controls how profile channel tags are matched.
// If this enum is changed, update MatchChannelTags in this file.
type ChannelTagsMatchMode string

const (
	ChannelTagsMatchModeAny  ChannelTagsMatchMode = "any"
	ChannelTagsMatchModeAll  ChannelTagsMatchMode = "all"
	ChannelTagsMatchModeNone ChannelTagsMatchMode = "none"
)

func (m ChannelTagsMatchMode) IsValid() bool {
	return m == "" || m == ChannelTagsMatchModeAny || m == ChannelTagsMatchModeAll || m == ChannelTagsMatchModeNone
}

func (m ChannelTagsMatchMode) OrDefault() ChannelTagsMatchMode {
	if m == ChannelTagsMatchModeAll {
		return ChannelTagsMatchModeAll
	}

	if m == ChannelTagsMatchModeNone {
		return ChannelTagsMatchModeNone
	}

	return ChannelTagsMatchModeAny
}

func (p *APIKeyProfile) MatchChannelTags(tags []string) bool {
	if p == nil || len(p.ChannelTags) == 0 {
		return true
	}

	return MatchChannelTags(p.ChannelTags, p.ChannelTagsMatchMode, tags)
}

func MatchChannelTags(allowedTags []string, matchMode ChannelTagsMatchMode, channelTags []string) bool {
	//nolint:exhaustive // Checked.
	switch matchMode.OrDefault() {
	case ChannelTagsMatchModeAll:
		for _, allowedTag := range allowedTags {
			if !slices.Contains(channelTags, allowedTag) {
				return false
			}
		}

		return true
	case ChannelTagsMatchModeNone:
		for _, tag := range channelTags {
			if slices.Contains(allowedTags, tag) {
				return false
			}
		}

		return true
	default:
		for _, tag := range channelTags {
			if slices.Contains(allowedTags, tag) {
				return true
			}
		}

		return false
	}
}

type APIKeyQuota struct {
	Requests    *int64            `json:"requests,omitempty"`
	TotalTokens *int64            `json:"totalTokens,omitempty"`
	Cost        *decimal.Decimal  `json:"cost,omitempty"`
	Period      APIKeyQuotaPeriod `json:"period"`
}

type APIKeyQuotaPeriod struct {
	Type             APIKeyQuotaPeriodType        `json:"type"`
	PastDuration     *APIKeyQuotaPastDuration     `json:"pastDuration,omitempty"`
	CalendarDuration *APIKeyQuotaCalendarDuration `json:"calendarDuration,omitempty"`
}

type APIKeyQuotaPeriodType string

const (
	APIKeyQuotaPeriodTypeAllTime          APIKeyQuotaPeriodType = "all_time"
	APIKeyQuotaPeriodTypePastDuration     APIKeyQuotaPeriodType = "past_duration"
	APIKeyQuotaPeriodTypeCalendarDuration APIKeyQuotaPeriodType = "calendar_duration"
)

type APIKeyQuotaPastDuration struct {
	Value int64                       `json:"value"`
	Unit  APIKeyQuotaPastDurationUnit `json:"unit"`
}

type APIKeyQuotaPastDurationUnit string

const (
	APIKeyQuotaPastDurationUnitMinute APIKeyQuotaPastDurationUnit = "minute"
	APIKeyQuotaPastDurationUnitHour   APIKeyQuotaPastDurationUnit = "hour"
	APIKeyQuotaPastDurationUnitDay    APIKeyQuotaPastDurationUnit = "day"
)

type APIKeyQuotaCalendarDuration struct {
	Unit APIKeyQuotaCalendarDurationUnit `json:"unit"`
}

type APIKeyQuotaCalendarDurationUnit string

const (
	APIKeyQuotaCalendarDurationUnitDay   APIKeyQuotaCalendarDurationUnit = "day"
	APIKeyQuotaCalendarDurationUnitMonth APIKeyQuotaCalendarDurationUnit = "month"
)
