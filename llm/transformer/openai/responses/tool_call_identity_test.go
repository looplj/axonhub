package responses

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestResponsesStream_ChatCompletions_ToolIdentityEmittedOnce(t *testing.T) {
	const arguments = `{"query":"alpha"}`
	for _, tt := range []struct {
		name          string
		delta         string
		namespace     string
		lateName      bool
		lateNamespace bool
		emptyDone     bool
	}{
		{name: "arguments_only_in_done"},
		{name: "partial_arguments", delta: `{"query":`},
		{name: "complete_arguments", delta: arguments},
		{name: "empty_done", delta: arguments, emptyDone: true},
		{name: "late_name", lateName: true},
		{name: "namespaced_arguments_only_in_done", namespace: "tools"},
		{name: "namespaced_late_name", namespace: "tools", lateName: true},
		{name: "late_identity_after_arguments", namespace: "tools", delta: arguments, lateName: true, lateNamespace: true},
		{name: "late_name_with_empty_done", delta: arguments, lateName: true, emptyDone: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var events []*httpclient.StreamEvent
			add := func(event StreamEvent) {
				data, err := json.Marshal(event)
				require.NoError(t, err)
				events = append(events, &httpclient.StreamEvent{Data: data})
			}
			add(StreamEvent{Type: StreamEventTypeResponseCreated, Response: &Response{
				ID: "resp_identity", Model: "test-model", Status: lo.ToPtr("in_progress"),
			}})

			// Interleave two calls and finish them in reverse order to verify that
			// argument and identity deltas remain associated with the right index.
			for i := range 2 {
				name, namespace := fmt.Sprintf("search_%d", i), tt.namespace
				if tt.lateName {
					name = ""
				}
				if tt.lateNamespace {
					namespace = ""
				}
				add(StreamEvent{Type: StreamEventTypeOutputItemAdded, OutputIndex: i, Item: &Item{
					ID: fmt.Sprintf("fc_%d", i), Type: "function_call",
					CallID: "call_" + strings.Repeat(fmt.Sprint(i), 32), Name: name, Namespace: namespace,
				}})
			}
			for i := range 2 {
				if tt.delta != "" {
					add(StreamEvent{Type: StreamEventTypeFunctionCallArgumentsDelta,
						ItemID: lo.ToPtr(fmt.Sprintf("fc_%d", i)), OutputIndex: i, Delta: tt.delta})
				}
			}
			for i := 1; i >= 0; i-- {
				finalArguments := arguments
				if tt.emptyDone {
					finalArguments = ""
				}
				done := StreamEvent{Type: StreamEventTypeFunctionCallArgumentsDone,
					ItemID: lo.ToPtr(fmt.Sprintf("fc_%d", i)), OutputIndex: i,
					Name: fmt.Sprintf("search_%d", i), Namespace: tt.namespace, Arguments: finalArguments}
				add(done)
				add(done) // Repeated final events must not duplicate any field.
			}
			add(StreamEvent{Type: StreamEventTypeResponseCompleted, Response: &Response{
				ID: "resp_identity", Model: "test-model", Status: lo.ToPtr("completed"),
			}})

			outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)
			unified, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(events))
			require.NoError(t, err)
			responses, err := streams.All(unified)
			require.NoError(t, err)

			// Namespace is internal to the unified stream, so check it before
			// converting to the Chat Completions wire format.
			namespaces := make(map[int]string)
			for _, response := range responses {
				for _, choice := range response.Choices {
					if choice.Delta != nil {
						for _, call := range choice.Delta.ToolCalls {
							if call.Function.Namespace != "" {
								namespaces[call.Index] = call.Function.Namespace
							}
						}
					}
				}
			}
			chat, err := openai.NewInboundTransformer().TransformStream(t.Context(), streams.SliceStream(responses))
			require.NoError(t, err)
			chunks, err := streams.All(chat)
			require.NoError(t, err)
			calls := make(map[int]llm.ToolCall)
			for _, chunk := range chunks {
				if string(chunk.Data) == "[DONE]" {
					continue
				}
				var response openai.Response
				require.NoError(t, json.Unmarshal(chunk.Data, &response))
				for _, choice := range response.Choices {
					if choice.Delta == nil {
						continue
					}
					for _, call := range choice.Delta.ToolCalls {
						// Chat SDKs concatenate string deltas by tool-call index.
						merged := calls[call.Index]
						merged.ID += call.ID
						merged.Function.Name += call.Function.Name
						merged.Function.Arguments += call.Function.Arguments
						calls[call.Index] = merged
					}
				}
			}
			require.Len(t, calls, 2)
			for i := range 2 {
				require.Equal(t, "call_"+strings.Repeat(fmt.Sprint(i), 32), calls[i].ID)
				name := fmt.Sprintf("search_%d", i)
				if tt.namespace != "" {
					name = tt.namespace + "__" + name
				}
				require.Equal(t, name, calls[i].Function.Name)
				require.Equal(t, arguments, calls[i].Function.Arguments)
				require.Equal(t, tt.namespace, namespaces[i])
			}
		})
	}
}
