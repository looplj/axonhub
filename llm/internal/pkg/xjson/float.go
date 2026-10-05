package xjson

import "encoding/json"

// ParseOptionalFloat64 accepts a JSON number or numeric string. Unsupported
// values are ignored so optional provider metadata cannot invalidate a response.
func ParseOptionalFloat64(data json.RawMessage) *float64 {
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return nil
	}

	value, err := number.Float64()
	if err != nil {
		return nil
	}

	return &value
}
