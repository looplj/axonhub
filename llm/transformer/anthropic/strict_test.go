package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestFunctionToolStrictRoundTrip(t *testing.T) {
	for _, strict := range []*bool{nil, lo.ToPtr(false), lo.ToPtr(true)} {
		for _, toolType := range []string{"", "custom"} {
			// Given an Anthropic function tool with an optional strict field.
			tool := Tool{Type: toolType, Name: "weather", Strict: strict,
				InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)}

			// When the tool crosses the unified representation in both directions.
			unified, ok := convertToolToLLM(tool)
			require.True(t, ok)
			outbound := convertToolsAnthropic([]llm.Tool{unified}, nil)

			// Then true, false, and absence remain distinct on the wire.
			require.Equal(t, strict, unified.Function.Strict)
			require.Len(t, outbound, 1)
			require.Equal(t, strict, outbound[0].Strict)
			body, err := json.Marshal(outbound[0])
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &fields))
			value, present := fields["strict"]
			require.Equal(t, strict != nil, present)
			if strict != nil {
				expected, err := json.Marshal(*strict)
				require.NoError(t, err)
				require.JSONEq(t, string(expected), string(value))
			}
		}
	}
}
