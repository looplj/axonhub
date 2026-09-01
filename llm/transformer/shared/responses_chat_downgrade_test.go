package shared

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestDowngradeResponsesChatToolLifecycle_DoesNotRedirectNamespaceSelector(t *testing.T) {
	request := &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "spawn_agent"}},
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name: "collaboration__spawn_agent", Namespace: "collaboration",
				},
			},
		},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeFunction, Function: llm.ToolFunction{Name: "spawn_agent"},
		}},
	}

	got := requireDowngradeSuccess(t, request)
	require.Len(t, got.Tools, 1)
	require.Equal(t, "spawn_agent", got.Tools[0].Function.Name)
	require.Nil(t, got.ToolChoice)
	require.NotSame(t, request, got)
	require.NotNil(t, request.ToolChoice)
}

func TestDowngradeResponsesChatToolLifecycle_FiltersAmbiguousAllowedFunctions(t *testing.T) {
	request := &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "spawn_agent"}},
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup"}},
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name: "collaboration__spawn_agent", Namespace: "collaboration",
				},
			},
		},
		ToolChoice: &llm.ToolChoice{
			ToolChoice: lo.ToPtr("auto"), AllowedToolsSet: true,
			AllowedTools: []llm.ToolOption{
				{Type: llm.ToolTypeFunction, Name: "spawn_agent"},
				{Type: llm.ToolTypeFunction, Name: "lookup"},
			},
		},
	}

	got := requireDowngradeSuccess(t, request)
	require.Len(t, got.Tools, 1)
	require.Equal(t, "lookup", got.Tools[0].Function.Name)
	require.NotNil(t, got.ToolChoice)
	require.Equal(t, "auto", lo.FromPtr(got.ToolChoice.ToolChoice))
	require.False(t, got.ToolChoice.AllowedToolsSet)
}

func TestDowngradeResponsesChatToolLifecycle_ClearsEmptyRequiredSelection(t *testing.T) {
	got, err := DowngradeResponsesChatToolLifecycle(nil)
	require.NoError(t, err)
	require.Nil(t, got)

	request := &llm.Request{
		APIFormat:         llm.APIFormatOpenAIResponse,
		ParallelToolCalls: lo.ToPtr(true),
		Tools: []llm.Tool{{
			Type:               llm.ToolTypeResponsesCustomTool,
			ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"},
		}},
		ToolChoice: &llm.ToolChoice{ToolChoice: lo.ToPtr("required")},
	}

	got = requireDowngradeSuccess(t, request)
	require.Empty(t, got.Tools)
	require.Nil(t, got.ToolChoice)
	require.Nil(t, got.ParallelToolCalls)
}

func TestDowngradeResponsesChatToolLifecycle_FullRequestMatrixAndIdempotency(t *testing.T) {
	request := &llm.Request{
		APIFormat:         llm.APIFormatOpenAIResponseCompact,
		ParallelToolCalls: lo.ToPtr(true),
		Messages: []llm.Message{
			{Role: "assistant", ToolCalls: []llm.ToolCall{
				{
					ID: "call_custom", Type: llm.ToolTypeResponsesCustomTool,
					ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_custom", Name: "apply_patch"},
				},
				{
					ID: "call_search", Type: llm.ToolTypeResponsesToolSearch,
					ResponseToolSearchCall: &llm.ResponseToolSearchCall{CallID: "call_search", Execution: "client"},
				},
				{
					ID: "call_namespace", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "spawn_agent", Namespace: "collaboration", Arguments: `{}`},
				},
				{
					ID: "call_plain", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "lookup", Arguments: `{"id":"42"}`},
				},
			}},
			{Role: "tool", ToolCallID: lo.ToPtr("call_custom"), Content: llm.MessageContent{Content: lo.ToPtr("patched")}},
			{Role: "tool", ToolCallID: lo.ToPtr("call_search"), Content: llm.MessageContent{Content: lo.ToPtr("found")}},
			{Role: "tool", ToolCallID: lo.ToPtr("call_namespace"), Content: llm.MessageContent{Content: lo.ToPtr("spawned")}},
			{Role: "tool", ToolCallID: lo.ToPtr("call_plain"), Content: llm.MessageContent{Content: lo.ToPtr("looked up")}},
		},
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup"}},
			{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"}},
			{Type: llm.ToolTypeResponsesToolSearch, ResponseToolSearch: &llm.ResponseToolSearch{Execution: "client"}},
			{Type: llm.ToolTypeResponsesOpaqueTool, ResponseOpaqueTool: &llm.ResponseOpaqueTool{SourceType: "mcp"}},
			{
				Type:     llm.ToolTypeFunction,
				Function: llm.Function{Name: "collaboration__spawn_agent", Namespace: "collaboration"},
			},
			{
				Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup"},
				ResponsesSourceType: "future_client_tool",
			},
		},
		ToolChoice: &llm.ToolChoice{ToolChoice: lo.ToPtr("auto")},
	}

	got := requireDowngradeSuccess(t, request)
	require.Len(t, got.Messages, 2)
	require.Len(t, got.Messages[0].ToolCalls, 1)
	require.Equal(t, "call_plain", got.Messages[0].ToolCalls[0].ID)
	require.Equal(t, "call_plain", lo.FromPtr(got.Messages[1].ToolCallID))
	require.Len(t, got.Tools, 1)
	require.Equal(t, "lookup", got.Tools[0].Function.Name)
	require.Equal(t, "auto", lo.FromPtr(got.ToolChoice.ToolChoice))
	require.True(t, lo.FromPtr(got.ParallelToolCalls))
	require.Len(t, request.Messages, 5)
	require.Len(t, request.Tools, 6)
	require.True(t, lo.FromPtr(request.ParallelToolCalls))

	requal, err := DowngradeResponsesChatToolLifecycle(got)
	require.NoError(t, err)
	require.Equal(t, got, requal)
}

func TestDowngradeResponsesChatToolLifecycle_SelectorAndParallelMatrix(t *testing.T) {
	tests := []struct {
		name          string
		choice        *llm.ToolChoice
		wantToolNames []string
		wantMode      string
		wantNamed     string
		wantParallel  bool
	}{
		{name: "nil selector", wantToolNames: []string{"lookup", "spawn_agent"}, wantParallel: true},
		{name: "auto", choice: &llm.ToolChoice{ToolChoice: lo.ToPtr("auto")}, wantToolNames: []string{"lookup", "spawn_agent"}, wantMode: "auto", wantParallel: true},
		{name: "none", choice: &llm.ToolChoice{ToolChoice: lo.ToPtr("none")}, wantToolNames: []string{"lookup", "spawn_agent"}, wantMode: "none", wantParallel: true},
		{name: "required", choice: &llm.ToolChoice{ToolChoice: lo.ToPtr("required")}, wantToolNames: []string{"lookup", "spawn_agent"}, wantMode: "required", wantParallel: true},
		{
			name: "named plain", choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
				Type: llm.ToolTypeFunction, Function: llm.ToolFunction{Name: "lookup"},
			}},
			wantToolNames: []string{"lookup", "spawn_agent"}, wantNamed: "lookup", wantParallel: true,
		},
		{
			name: "named responses type cleared", choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
				Type: "custom", Function: llm.ToolFunction{Name: "apply_patch"},
			}},
			wantToolNames: []string{"lookup", "spawn_agent"}, wantParallel: true,
		},
		{
			name: "named missing function cleared", choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
				Type: llm.ToolTypeFunction, Function: llm.ToolFunction{Name: "missing"},
			}},
			wantToolNames: []string{"lookup", "spawn_agent"}, wantParallel: true,
		},
		{
			name: "allowed plain", choice: &llm.ToolChoice{
				ToolChoice: lo.ToPtr("auto"), AllowedToolsSet: true,
				AllowedTools: []llm.ToolOption{{Type: llm.ToolTypeFunction, Name: "lookup"}},
			},
			wantToolNames: []string{"lookup"}, wantMode: "auto", wantParallel: true,
		},
		{
			name: "empty allowed clears tools and parallel", choice: &llm.ToolChoice{
				ToolChoice: lo.ToPtr("auto"), AllowedToolsSet: true, AllowedTools: []llm.ToolOption{},
			},
			wantToolNames: []string{},
		},
		{
			name: "allowed without mode constrains tools and clears selector", choice: &llm.ToolChoice{
				AllowedToolsSet: true,
				AllowedTools:    []llm.ToolOption{{Type: llm.ToolTypeFunction, Name: "lookup"}},
			},
			wantToolNames: []string{"lookup"}, wantParallel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &llm.Request{
				APIFormat: llm.APIFormatOpenAIResponse, ParallelToolCalls: lo.ToPtr(true), ToolChoice: tt.choice,
				Tools: []llm.Tool{
					{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup"}},
					{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "spawn_agent"}},
					{Type: llm.ToolTypeResponsesCustomTool, ResponseCustomTool: &llm.ResponseCustomTool{Name: "apply_patch"}},
				},
			}
			got := requireDowngradeSuccess(t, request)
			toolNames := make([]string, 0, len(got.Tools))
			for _, tool := range got.Tools {
				toolNames = append(toolNames, tool.Function.Name)
			}
			require.Equal(t, tt.wantToolNames, toolNames)
			require.Equal(t, tt.wantParallel, lo.FromPtr(got.ParallelToolCalls))
			if tt.wantMode != "" {
				require.NotNil(t, got.ToolChoice)
				require.Equal(t, tt.wantMode, lo.FromPtr(got.ToolChoice.ToolChoice))
			} else if tt.wantNamed != "" {
				require.NotNil(t, got.ToolChoice)
				require.NotNil(t, got.ToolChoice.NamedToolChoice)
				require.Equal(t, tt.wantNamed, got.ToolChoice.NamedToolChoice.Function.Name)
			} else {
				require.Nil(t, got.ToolChoice)
			}
		})
	}
}

func TestDowngradeResponsesChatToolLifecycle_RemovesPairedSourceToolHistory(t *testing.T) {
	request := &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Messages: []llm.Message{
			{Role: "assistant", ToolCalls: []llm.ToolCall{
				{
					ID: "call_source", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "future_lookup", Arguments: `{"query":"axon"}`},
				},
				{
					ID: "call_plain", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "lookup", Arguments: `{"id":"42"}`},
				},
			}},
			{Role: "tool", ToolCallID: lo.ToPtr("call_source"), Content: llm.MessageContent{Content: lo.ToPtr("source output")}},
			{Role: "tool", ToolCallID: lo.ToPtr("call_plain"), Content: llm.MessageContent{Content: lo.ToPtr("plain output")}},
		},
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup"}},
			{
				Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup"},
				ResponsesSourceType: "future_client_tool",
			},
		},
	}

	got := requireDowngradeSuccess(t, request)

	require.Len(t, got.Messages, 2)
	require.Len(t, got.Messages[0].ToolCalls, 1)
	require.Equal(t, "call_plain", got.Messages[0].ToolCalls[0].ID)
	require.Equal(t, "call_plain", lo.FromPtr(got.Messages[1].ToolCallID))
	require.Len(t, got.Tools, 1)
	require.Equal(t, "lookup", got.Tools[0].Function.Name)
	require.Len(t, request.Messages, 3)
	require.Len(t, request.Tools, 2)
}

func TestDowngradeResponsesChatToolLifecycle_RemovesSourceAndNamespaceNames(t *testing.T) {
	request := &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup"}},
			{
				Type:                llm.ToolTypeFunction,
				Function:            llm.Function{Name: "future__lookup", Namespace: "future"},
				ResponsesSourceType: "future_client_tool",
			},
		},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeFunction, Function: llm.ToolFunction{Name: "lookup"},
		}},
	}

	got := requireDowngradeSuccess(t, request)
	require.Len(t, got.Tools, 1)
	require.Equal(t, "lookup", got.Tools[0].Function.Name)
	require.Nil(t, got.ToolChoice)
}

func TestDeepCopyRequest_CopiesHiddenAndNestedState(t *testing.T) {
	request := &llm.Request{
		TransformerMetadata: map[string]any{
			"nested": map[string]any{"enabled": true},
		},
		Tools: []llm.Tool{{
			Type:                llm.ToolTypeFunction,
			Function:            llm.Function{Name: "lookup", Parameters: []byte(`{"type":"object"}`)},
			ResponsesSourceType: "future_client_tool",
		}},
		ToolChoice: &llm.ToolChoice{AllowedToolsSet: true},
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					RawTools: []llm.OpenAIResponsesRawFragment{{Type: "raw_tool"}},
				},
			},
		},
	}

	cloned := deepCopyRequest(request)
	require.NotSame(t, request, cloned)

	request.Tools[0].Function.Name = "changed"
	request.Tools[0].ResponsesSourceType = "changed"
	request.ToolChoice.AllowedToolsSet = false
	request.TransformerMetadata["nested"].(map[string]any)["enabled"] = false
	request.ProviderExtensions.OpenAIResponses.Request.RawTools[0].Type = "changed"

	require.Equal(t, "lookup", cloned.Tools[0].Function.Name)
	require.Equal(t, "future_client_tool", cloned.Tools[0].ResponsesSourceType)
	require.True(t, cloned.ToolChoice.AllowedToolsSet)
	require.True(t, cloned.TransformerMetadata["nested"].(map[string]any)["enabled"].(bool))
	require.Equal(t, "raw_tool", cloned.ProviderExtensions.OpenAIResponses.Request.RawTools[0].Type)
}

func TestDowngradeResponsesChatToolLifecycle_FailsClosedOnSourceNameAmbiguity(t *testing.T) {
	messages := []llm.Message{
		{Role: "assistant", ToolCalls: []llm.ToolCall{{
			ID: "call_source", Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{Name: "future_lookup", Arguments: `{}`},
		}}},
		{Role: "tool", ToolCallID: lo.ToPtr("call_source"), Content: llm.MessageContent{Content: lo.ToPtr("source output")}},
	}

	t.Run("conflicts with retained plain function", func(t *testing.T) {
		request := &llm.Request{
			APIFormat: llm.APIFormatOpenAIResponse,
			Messages:  messages,
			Tools: []llm.Tool{
				{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup"}},
				{
					Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup"},
					ResponsesSourceType: "future_client_tool",
				},
			},
		}

		got, err := DowngradeResponsesChatToolLifecycle(request)
		require.Nil(t, got)
		require.ErrorContains(t, err, "conflicts with retained function")
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	})

	t.Run("duplicate source definitions", func(t *testing.T) {
		request := &llm.Request{
			APIFormat: llm.APIFormatOpenAIResponse,
			Tools: []llm.Tool{
				{
					Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup"},
					ResponsesSourceType: "future_client_tool_a",
				},
				{
					Type: llm.ToolTypeFunction, Function: llm.Function{Name: "future_lookup"},
					ResponsesSourceType: "future_client_tool_b",
				},
			},
		}

		got, err := DowngradeResponsesChatToolLifecycle(request)
		require.Nil(t, got)
		require.ErrorContains(t, err, "duplicate removed definition")
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	})

	t.Run("invalid namespace definition", func(t *testing.T) {
		request := &llm.Request{
			APIFormat: llm.APIFormatOpenAIResponse,
			Tools: []llm.Tool{{
				Type:     llm.ToolTypeFunction,
				Function: llm.Function{Name: "lookup", Namespace: "functions"},
			}},
		}

		got, err := DowngradeResponsesChatToolLifecycle(request)
		require.Nil(t, got)
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		require.ErrorContains(t, err, "invalid_namespace_tool")
	})
}

func requireDowngradeSuccess(t *testing.T, request *llm.Request) *llm.Request {
	t.Helper()
	before := deepCopyRequest(request)

	got, err := DowngradeResponsesChatToolLifecycle(request)
	require.NoError(t, err)

	require.Equal(t, *before, *request)
	return got
}

func deepCopyRequest(request *llm.Request) *llm.Request {
	if request == nil {
		return nil
	}
	return deepCopyValue(reflect.ValueOf(request)).Interface().(*llm.Request)
}

func deepCopyValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type().Elem())
		cloned.Elem().Set(deepCopyValue(value.Elem()))
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			cloned.Index(i).Set(deepCopyValue(value.Index(i)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for i := range value.Len() {
			cloned.Index(i).Set(deepCopyValue(value.Index(i)))
		}
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			cloned.SetMapIndex(deepCopyValue(iterator.Key()), deepCopyValue(iterator.Value()))
		}
		return cloned
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		return deepCopyValue(value.Elem())
	case reflect.Struct:
		return deepCopyStruct(value)
	default:
		return value
	}
}

func deepCopyStruct(value reflect.Value) reflect.Value {
	cloned := reflect.New(value.Type()).Elem()
	for i := range value.NumField() {
		field := value.Type().Field(i)
		sourceField := value.Field(i)
		if sourceField.CanAddr() && !sourceField.CanInterface() {
			source := reflect.NewAt(field.Type, unsafe.Pointer(sourceField.UnsafeAddr())).Elem()
			destination := reflect.NewAt(field.Type, unsafe.Pointer(cloned.Field(i).UnsafeAddr())).Elem()
			destination.Set(deepCopyValue(source))
			continue
		}
		cloned.Field(i).Set(deepCopyValue(sourceField))
	}
	return cloned
}
