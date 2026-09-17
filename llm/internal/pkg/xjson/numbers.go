package xjson

import (
	"encoding/json"
	"io"
	"math/big"
	"strconv"
	"strings"
)

// CanonicalizeIntegralJSONNumbers rewrites complete JSON values so numbers that
// are exact integers use integer lexemes. Float-encoded integrals such as
// 180000.0 and 1e5 become 180000 and 100000. Incomplete or invalid JSON is
// returned unchanged so streaming argument fragments stay intact.
func CanonicalizeIntegralJSONNumbers(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return raw
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return raw
	}

	canonical, err := json.Marshal(canonicalizeJSONValue(value))
	if err != nil {
		return raw
	}
	return string(canonical)
}

func canonicalizeJSONValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		return canonicalizeJSONNumber(typed)
	case []any:
		for i, item := range typed {
			typed[i] = canonicalizeJSONValue(item)
		}
		return typed
	case map[string]any:
		for key, item := range typed {
			typed[key] = canonicalizeJSONValue(item)
		}
		return typed
	default:
		return value
	}
}

func canonicalizeJSONNumber(number json.Number) any {
	if integer, ok := integralInt64(string(number)); ok {
		return json.Number(strconv.FormatInt(integer, 10))
	}
	return number
}

func integralInt64(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return v, true
	}

	parsed, _, err := big.ParseFloat(raw, 10, 256, big.ToNearestEven)
	if err != nil {
		return 0, false
	}
	integer, accuracy := parsed.Int(nil)
	if accuracy != big.Exact || !integer.IsInt64() {
		return 0, false
	}
	return integer.Int64(), true
}
