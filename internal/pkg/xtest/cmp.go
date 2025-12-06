package xtest

import (
	"encoding/json"

	"github.com/google/go-cmp/cmp"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/xjson"
)

// Custom comparator for json.RawMessage that compares semantic equality.
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

func nilInt(x *int) int {
	if x == nil {
		return 0
	}

	return *x
}

// Equal provides semantic equality comparison with custom transformers and comparers.
func Equal(a, b any, opts ...cmp.Option) bool {
	allOpts := append(opts,
		NilCompletionTokensDetails,
		NilPromptTokensDetails,
		ToolCallsTransformer,
		cmp.Transformer("", nilString),
		cmp.Transformer("", nilInt),
		cmp.Comparer(jsonRawMessageComparer))

	return cmp.Equal(a, b, allOpts...)
}

// NilPromptTokensDetails transformer for handling nil PromptTokensDetails.
var NilPromptTokensDetails = cmp.Transformer("nilPromptTokensDetails", func(x *llm.PromptTokensDetails) llm.PromptTokensDetails {
	if x == nil {
		return llm.PromptTokensDetails{}
	}
	return *x
})

// NilCompletionTokensDetails transformer for handling nil CompletionTokensDetails.
var NilCompletionTokensDetails = cmp.Transformer("nilCompletionTokensDetails", func(x *llm.CompletionTokensDetails) llm.CompletionTokensDetails {
	if x == nil {
		return llm.CompletionTokensDetails{}
	}
	return *x
})

// ToolCallsTransformer transformer for handling tool call comparisons.
var ToolCallsTransformer = cmp.Transformer("toolCall", func(x llm.ToolCall) llm.ToolCall {
	var args any
	if x.Function.Arguments != "" {
		err := json.Unmarshal([]byte(x.Function.Arguments), &args)
		if err != nil {
			args = x.Function.Arguments
		}
	}
	rawArgs := xjson.MustMarshalString(args)
	return llm.ToolCall{
		ID:   x.ID,
		Type: x.Type,
		Function: llm.FunctionCall{
			Name:      x.Function.Name,
			Arguments: rawArgs,
		},
		Index:        x.Index,
		CacheControl: x.CacheControl,
	}
})
