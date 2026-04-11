package objects

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func rawJSON(t *testing.T, value any) JSONRawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	require.NoError(t, err)

	return data
}

func TestToExpr(t *testing.T) {
	t.Run("empty conditions", func(t *testing.T) {
		expr, err := ToExpr(Condition{})
		require.NoError(t, err)
		require.Equal(t, "true", expr)
	})

	t.Run("and conditions", func(t *testing.T) {
		expr, err := ToExpr(Condition{
			Logic: "and",
			Conditions: []Condition{
				{
					Field:    "promptTokens",
					Operator: "gt",
					Value:    int64(100),
				},
				{
					Field:    "model",
					Operator: "eq",
					Value:    "gpt-4o",
				},
			},
		})
		require.NoError(t, err)
		require.Equal(t, `(promptTokens > 100 && model == "gpt-4o")`, expr)
	})

	t.Run("or conditions", func(t *testing.T) {
		expr, err := ToExpr(Condition{
			Logic: "or",
			Conditions: []Condition{
				{
					Field:    "promptTokens",
					Operator: "lt",
					Value:    int64(100),
				},
				{
					Field:    "promptTokens",
					Operator: "gte",
					Value:    int64(1000),
				},
			},
		})
		require.NoError(t, err)
		require.Equal(t, "(promptTokens < 100 || promptTokens >= 1000)", expr)
	})

	t.Run("unsupported operator", func(t *testing.T) {
		_, err := ToExpr(Condition{
			Conditions: []Condition{{
				Field:    "promptTokens",
				Operator: "contains",
				Value:    "1",
			}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), `unsupported operator "contains"`)
	})
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name  string
		group Condition
		data  any
		want  bool
	}{
		{
			name:  "empty conditions match",
			group: Condition{},
			data:  map[string]any{},
			want:  true,
		},
		{
			name: "gt matches",
			group: Condition{
				Conditions: []Condition{{
					Field:    "promptTokens",
					Operator: "gt",
					Value:    int64(100),
				}},
			},
			data: map[string]any{"promptTokens": int64(101)},
			want: true,
		},
		{
			name: "lt does not match",
			group: Condition{
				Conditions: []Condition{{
					Field:    "promptTokens",
					Operator: "lt",
					Value:    int64(100),
				}},
			},
			data: map[string]any{"promptTokens": int64(100)},
			want: false,
		},
		{
			name: "lte matches",
			group: Condition{
				Conditions: []Condition{{
					Field:    "promptTokens",
					Operator: "lte",
					Value:    int64(100),
				}},
			},
			data: map[string]any{"promptTokens": int64(100)},
			want: true,
		},
		{
			name: "gte matches",
			group: Condition{
				Conditions: []Condition{{
					Field:    "promptTokens",
					Operator: "gte",
					Value:    int64(100),
				}},
			},
			data: map[string]any{"promptTokens": int64(100)},
			want: true,
		},
		{
			name: "eq matches",
			group: Condition{
				Conditions: []Condition{{
					Field:    "model",
					Operator: "eq",
					Value:    "gpt-4o",
				}},
			},
			data: map[string]any{"model": "gpt-4o"},
			want: true,
		},
		{
			name: "ne matches",
			group: Condition{
				Conditions: []Condition{{
					Field:    "model",
					Operator: "ne",
					Value:    "gpt-4o",
				}},
			},
			data: map[string]any{"model": "claude-3-7-sonnet"},
			want: true,
		},
		{
			name: "and matches",
			group: Condition{
				Logic: "and",
				Conditions: []Condition{
					{
						Field:    "promptTokens",
						Operator: "gt",
						Value:    int64(100),
					},
					{
						Field:    "model",
						Operator: "eq",
						Value:    "gpt-4o",
					},
				},
			},
			data: map[string]any{
				"promptTokens": int64(101),
				"model":        "gpt-4o",
			},
			want: true,
		},
		{
			name: "or matches",
			group: Condition{
				Logic: "or",
				Conditions: []Condition{
					{
						Field:    "promptTokens",
						Operator: "lt",
						Value:    int64(100),
					},
					{
						Field:    "model",
						Operator: "eq",
						Value:    "gpt-4o",
					},
				},
			},
			data: map[string]any{
				"promptTokens": int64(1000),
				"model":        "gpt-4o",
			},
			want: true,
		},
		{
			name: "invalid expression returns false",
			group: Condition{
				Conditions: []Condition{{
					Field:    "",
					Operator: "eq",
					Value:    "gpt-4o",
				}},
			},
			data: map[string]any{"model": "gpt-4o"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.group, tt.data)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestConditionUnmarshalJSON(t *testing.T) {
	t.Run("deserializes integer value as int64", func(t *testing.T) {
		var condition Condition

		err := json.Unmarshal([]byte(`{
			"type":"condition",
			"field":"promptTokens",
			"operator":"gt",
			"value":100
		}`), &condition)
		require.NoError(t, err)
		require.Equal(t, int64(100), condition.Value)
	})

	t.Run("deserializes nested integer values as int64", func(t *testing.T) {
		var condition Condition

		err := json.Unmarshal([]byte(`{
			"logic":"and",
			"conditions":[
				{
					"type":"condition",
					"field":"promptTokens",
					"operator":"gte",
					"value":100
				},
				{
					"type":"condition",
					"field":"metadata",
					"operator":"eq",
					"value":{"maxTokens":2048,"weights":[1,2,3]}
				}
			]
		}`), &condition)
		require.NoError(t, err)
		require.Equal(t, int64(100), condition.Conditions[0].Value)

		metadata, ok := condition.Conditions[1].Value.(map[string]any)
		require.True(t, ok)
		require.Equal(t, int64(2048), metadata["maxTokens"])
		require.Equal(t, []any{int64(1), int64(2), int64(3)}, metadata["weights"])
	})
}
