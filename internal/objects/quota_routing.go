package objects

import (
	"fmt"
	"io"
)

type QuotaRoutingMode string

const (
	QuotaRoutingModeIgnoreQuota       QuotaRoutingMode = "ignore_quota"
	QuotaRoutingModeRemoveOnExhausted QuotaRoutingMode = "remove_on_exhausted"
	QuotaRoutingModeBackpressure      QuotaRoutingMode = "backpressure"
)

func (m QuotaRoutingMode) MarshalGQL(w io.Writer) {
	var value string
	switch m {
	case QuotaRoutingModeIgnoreQuota:
		value = "IGNORE_QUOTA"
	case QuotaRoutingModeRemoveOnExhausted:
		value = "REMOVE_ON_EXHAUSTED"
	case QuotaRoutingModeBackpressure:
		value = "BACKPRESSURE"
	default:
		value = "INHERIT"
	}

	_, _ = io.WriteString(w, `"`+value+`"`)
}

func (m *QuotaRoutingMode) UnmarshalGQL(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("QuotaRoutingMode must be a string")
	}

	switch str {
	case "INHERIT":
		*m = ""
	case "IGNORE_QUOTA":
		*m = QuotaRoutingModeIgnoreQuota
	case "REMOVE_ON_EXHAUSTED":
		*m = QuotaRoutingModeRemoveOnExhausted
	case "BACKPRESSURE":
		*m = QuotaRoutingModeBackpressure
	default:
		return fmt.Errorf("invalid QuotaRoutingMode: %s", str)
	}

	return nil
}
