package subscription

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
)

const (
	superGrokLimitCents      = 15_000
	superGrokHeavyLimitCents = 150_000
)

type BillingWindow struct {
	UsagePercent float64 `json:"usage_percent"`
	ResetAt      string  `json:"reset_at,omitempty"`
	LimitUSD     float64 `json:"limit_usd,omitempty"`
	UsedUSD      float64 `json:"used_usd,omitempty"`
}

type BillingSummary struct {
	Plan    string         `json:"plan,omitempty"`
	Weekly  *BillingWindow `json:"weekly,omitempty"`
	Monthly *BillingWindow `json:"monthly,omitempty"`
}

type billingPayload struct {
	Config *billingConfig `json:"config"`
}

type billingConfig struct {
	CurrentPeriod      *billingPeriod `json:"currentPeriod"`
	CreditUsagePercent *float64       `json:"creditUsagePercent"`
	MonthlyLimit       billingAmount  `json:"monthlyLimit"`
	Used               billingAmount  `json:"used"`
	BillingPeriodEnd   string         `json:"billingPeriodEnd"`
}

type billingPeriod struct {
	End string `json:"end"`
}

type billingAmount struct {
	Value *float64
}

func (amount *billingAmount) UnmarshalJSON(data []byte) error {
	var wrapped struct {
		Value json.RawMessage `json:"val"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Value) > 0 {
		value, err := parseBillingNumber(wrapped.Value)
		if err != nil {
			return err
		}
		amount.Value = value
		return nil
	}
	value, err := parseBillingNumber(data)
	if err != nil {
		return err
	}
	amount.Value = value
	return nil
}

func ParseBillingResponses(weeklyBody, monthlyBody []byte) (BillingSummary, error) {
	weekly, err := parseBillingPayload(weeklyBody)
	if err != nil {
		return BillingSummary{}, err
	}
	monthly, err := parseBillingPayload(monthlyBody)
	if err != nil {
		return BillingSummary{}, err
	}
	if weekly.Config == nil && monthly.Config == nil {
		return BillingSummary{}, errors.New("xAI billing responses contain no config")
	}

	result := BillingSummary{}
	if weekly.Config != nil && (weekly.Config.CreditUsagePercent != nil || weekly.Config.CurrentPeriod != nil) {
		result.Weekly = &BillingWindow{UsagePercent: valueOrZero(weekly.Config.CreditUsagePercent)}
		if weekly.Config.CurrentPeriod != nil {
			result.Weekly.ResetAt = strings.TrimSpace(weekly.Config.CurrentPeriod.End)
		}
	}
	if monthly.Config != nil && (monthly.Config.MonthlyLimit.Value != nil || monthly.Config.Used.Value != nil || strings.TrimSpace(monthly.Config.BillingPeriodEnd) != "") {
		limitCents := valueOrZero(monthly.Config.MonthlyLimit.Value)
		usedCents := valueOrZero(monthly.Config.Used.Value)
		result.Monthly = &BillingWindow{}
		result.Monthly.LimitUSD = limitCents / 100
		result.Monthly.UsedUSD = usedCents / 100
		result.Monthly.ResetAt = strings.TrimSpace(monthly.Config.BillingPeriodEnd)
		if limitCents > 0 {
			result.Monthly.UsagePercent = math.Min(usedCents, limitCents) / limitCents * 100
		}
		result.Plan = billingPlan(limitCents)
	}
	return result, nil
}

func parseBillingPayload(data []byte) (billingPayload, error) {
	var payload billingPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return billingPayload{}, err
	}
	return payload, nil
}

func parseBillingNumber(data []byte) (*float64, error) {
	if string(data) == "null" {
		return nil, nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err == nil {
		value, parseErr := number.Float64()
		if parseErr != nil {
			return nil, parseErr
		}
		return &value, nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return nil, err
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func billingPlan(limitCents float64) string {
	switch math.Round(limitCents) {
	case superGrokLimitCents:
		return "SuperGrok"
	case superGrokHeavyLimitCents:
		return "SuperGrok Heavy"
	default:
		return ""
	}
}
