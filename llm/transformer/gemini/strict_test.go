package gemini

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestStrictFunctionToolChoice(t *testing.T) {
	choices := []struct {
		name   string
		choice *llm.ToolChoice
		mode   string
		names  []string
	}{
		{name: "implicit", mode: "VALIDATED"},
		{name: "auto", choice: &llm.ToolChoice{ToolChoice: lo.ToPtr("auto")}, mode: "VALIDATED"},
		{name: "required", choice: &llm.ToolChoice{ToolChoice: lo.ToPtr("required")}, mode: "ANY"},
		{name: "any", choice: &llm.ToolChoice{ToolChoice: lo.ToPtr("any")}, mode: "ANY"},
		{name: "none", choice: &llm.ToolChoice{ToolChoice: lo.ToPtr("none")}, mode: "NONE"},
		{name: "named", choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: "function", Function: llm.ToolFunction{Name: "weather"},
		}}, mode: "ANY", names: []string{"weather"}},
	}
	for _, model := range []string{"gemini-2.5-flash", "gemini-3.1-pro-preview", "custom-model-alias"} {
		for _, choice := range choices {
			t.Run(model+"/"+choice.name, func(t *testing.T) {
				// Given a mixed request: strict validation is request-wide in Gemini.
				req := &llm.Request{Model: model, ToolChoice: choice.choice, Tools: []llm.Tool{
					{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "relaxed", Strict: lo.ToPtr(false)}},
					{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "weather", Strict: lo.ToPtr(true),
						Parameters: json.RawMessage(`{"type":"object","properties":{}}`)}},
				}}

				// When the request is converted for Gemini.
				result := convertLLMToGeminiRequest(req)

				// Then optional, required, disabled, and named selection remain distinct.
				require.NotNil(t, result.ToolConfig)
				require.NotNil(t, result.ToolConfig.FunctionCallingConfig)
				require.Equal(t, choice.mode, result.ToolConfig.FunctionCallingConfig.Mode)
				require.Equal(t, choice.names, result.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
				require.Len(t, result.Tools[0].FunctionDeclarations, 2)
			})
		}
	}
}

func TestNonStrictToolsPreserveDefaultMode(t *testing.T) {
	for _, tools := range [][]llm.Tool{
		nil,
		{{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "weather"}}},
		{{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "weather", Strict: lo.ToPtr(false)}}},
		{{Type: llm.ToolTypeWebSearch, WebSearch: &llm.WebSearch{Strict: lo.ToPtr(true)}}},
	} {
		for _, explicitAuto := range []bool{false, true} {
			// Given no strict function tools, including a strict native tool.
			req := &llm.Request{Tools: tools}
			if explicitAuto {
				req.ToolChoice = &llm.ToolChoice{ToolChoice: lo.ToPtr("auto")}
			}

			// When converting the request.
			result := convertLLMToGeminiRequest(req)

			// Then no schema-enforcing mode is introduced.
			if explicitAuto {
				require.Equal(t, "AUTO", result.ToolConfig.FunctionCallingConfig.Mode)
			} else {
				require.Nil(t, result.ToolConfig)
			}
		}
	}
}

func TestRequiredFunctionNamesSurviveRoundTrip(t *testing.T) {
	for _, names := range [][]string{nil, {"weather"}, {"weather", "forecast"}} {
		t.Run(fmt.Sprint(names), func(t *testing.T) {
			choice := convertGeminiFunctionCallingConfigToToolChoice(&FunctionCallingConfig{
				Mode: "ANY", AllowedFunctionNames: names,
			})
			body, err := json.Marshal(choice)
			require.NoError(t, err)
			var restored llm.ToolChoice
			require.NoError(t, json.Unmarshal(body, &restored))
			for _, strict := range []bool{false, true} {
				result := convertLLMToGeminiRequest(&llm.Request{ToolChoice: &restored, Tools: []llm.Tool{
					{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "weather", Strict: lo.ToPtr(strict)}},
				}})
				require.Equal(t, "ANY", result.ToolConfig.FunctionCallingConfig.Mode)
				require.Equal(t, names, result.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
			}
		})
	}
}
