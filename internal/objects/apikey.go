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

func (p *APIKeyProfile) Clone() *APIKeyProfile {
	if p == nil {
		return nil
	}
	cp := *p
	if len(p.ModelMappings) > 0 {
		cp.ModelMappings = make([]ModelMapping, len(p.ModelMappings))
		copy(cp.ModelMappings, p.ModelMappings)
	}
	if p.Quota != nil {
		q := *p.Quota
		if q.Requests != nil {
			r := *q.Requests
			q.Requests = &r
		}
		if q.TotalTokens != nil {
			tt := *q.TotalTokens
			q.TotalTokens = &tt
		}
		if q.Cost != nil {
			c := *q.Cost
			q.Cost = &c
		}
		q.Period = p.Quota.Period.clone()
		cp.Quota = &q
	}
	if len(p.ChannelIDs) > 0 {
		cp.ChannelIDs = make([]int, len(p.ChannelIDs))
		copy(cp.ChannelIDs, p.ChannelIDs)
	}
	if len(p.ChannelTags) > 0 {
		cp.ChannelTags = make([]string, len(p.ChannelTags))
		copy(cp.ChannelTags, p.ChannelTags)
	}
	if len(p.ModelIDs) > 0 {
		cp.ModelIDs = make([]string, len(p.ModelIDs))
		copy(cp.ModelIDs, p.ModelIDs)
	}
	if p.LoadBalanceStrategy != nil {
		s := *p.LoadBalanceStrategy
		cp.LoadBalanceStrategy = &s
	}
	return &cp
}

func (p *APIKeyQuotaPeriod) clone() APIKeyQuotaPeriod {
	if p == nil {
		return APIKeyQuotaPeriod{}
	}
	cp := *p
	if p.PastDuration != nil {
		pd := *p.PastDuration
		cp.PastDuration = &pd
	}
	if p.CalendarDuration != nil {
		cd := *p.CalendarDuration
		cp.CalendarDuration = &cd
	}
	return cp
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
