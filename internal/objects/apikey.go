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
	Name                 string          `json:"name"`
	ModelMappings        []ModelMapping  `json:"modelMappings"`
	ChannelIDs           []int           `json:"channelIDs,omitempty"`
	ChannelTags          []string        `json:"channelTags,omitempty"`
	ChannelTagsMatchMode APIKeyMatchMode `json:"channelTagsMatchMode,omitempty"`
	ModelIDs             []string        `json:"modelIDs,omitempty"`
	Quota                *APIKeyQuota    `json:"quota,omitempty"`
	LoadBalanceStrategy  *string         `json:"loadBalanceStrategy,omitempty"`
}

type APIKeyMatchMode string

const (
	APIKeyMatchModeAny APIKeyMatchMode = "any"
	APIKeyMatchModeAll APIKeyMatchMode = "all"
)

func (m APIKeyMatchMode) IsValid() bool {
	return m == "" || m == APIKeyMatchModeAny || m == APIKeyMatchModeAll
}

func (m APIKeyMatchMode) OrDefault() APIKeyMatchMode {
	if m == APIKeyMatchModeAll {
		return APIKeyMatchModeAll
	}

	return APIKeyMatchModeAny
}

func (p *APIKeyProfile) MatchChannelTags(tags []string) bool {
	if p == nil || len(p.ChannelTags) == 0 {
		return true
	}

	//nolint:exhaustive // Checked.
	switch p.ChannelTagsMatchMode.OrDefault() {
	case APIKeyMatchModeAll:
		for _, allowedTag := range p.ChannelTags {
			matched := slices.Contains(tags, allowedTag)

			if !matched {
				return false
			}
		}

		return true
	default:
		for _, tag := range tags {
			if slices.Contains(p.ChannelTags, tag) {
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
