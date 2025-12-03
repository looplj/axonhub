package xjson

import (
	"encoding/json"

	"github.com/google/go-cmp/cmp"
)

// Custom comparator for json.RawMessage that compares semantic equality.
var jsonRawMessageComparer = cmp.Comparer(func(x, y json.RawMessage) bool {
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
})

func Equal(a, b any) bool {
	return cmp.Equal(a, b, jsonRawMessageComparer)
}
