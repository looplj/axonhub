package objects

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/99designs/gqlgen/graphql"
	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
)

func MarshalDecimal(d decimal.Decimal) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = w.Write([]byte(d.String()))
	})
}

func UnmarshalDecimal(v any) (decimal.Decimal, error) {
	switch v := v.(type) {
	case string:
		return decimal.NewFromString(v)
	case json.Number:
		return decimal.NewFromString(string(v))
	case float64, float32:
		return decimal.NewFromFloat(cast.ToFloat64(v)), nil
	case int64, int, int32, int16, int8:
		return decimal.NewFromInt(cast.ToInt64(v)), nil
	default:
		return decimal.Zero, fmt.Errorf("failed to decode decimal: %v %T", v, v)
	}
}
