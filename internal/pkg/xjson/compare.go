package xjson

import (
	"encoding/json"

	"github.com/google/go-cmp/cmp"
)

// Custom comparator for json.RawMessage that compares semantic equality.
// Deprecated: Use xtest.Equal instead. This function will be removed in a future version.
func jsonRawMessageComparer(x, y json.RawMessage) bool {
	if len(x) == 0 && len(y) == 0 {
		return true
	}

	if len(x) == 0 || len(y) == 0 {
		return false
	}

	var xVal, yVal interface{}
	if err := json.Unmarshal(x, &xVal); err != nil {
		return false
	}

	if err := json.Unmarshal(y, &yVal); err != nil {
		return false
	}

	return cmp.Equal(xVal, yVal)
}

func nilString(x *string) string {
	if x == nil {
		return ""
	}

	return *x
}

// Equal provides semantic equality comparison with custom transformers and comparers.
// Deprecated: Use xtest.Equal instead. This function will be removed in a future version.
func Equal(a, b any, opts ...cmp.Option) bool {
	allOpts := append(opts,
		cmp.Transformer("", nilString),
		cmp.Comparer(jsonRawMessageComparer))

	return cmp.Equal(a, b, allOpts...)
}
