package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// mockTransformer is a simple mock transformer for testing.
type mockTransformer struct {
	aggregatedResponse []byte
	aggregatedMeta     llm.ResponseMeta
	aggregatedErr      error
	apiFormat          llm.APIFormat
}

func (m *mockTransformer) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	body, err := json.Marshal(map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"temperature": 0.5,
		"max_tokens":  1000,
	})
	if err != nil {
		return nil, err
	}

	return &httpclient.Request{
		Method: "POST",
		URL:    "https://api.example.com/v1/chat/completions",
		Body:   body,
	}, nil
}

func (m *mockTransformer) TransformResponse(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (m *mockTransformer) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, nil
}

func (m *mockTransformer) TransformError(ctx context.Context, err *httpclient.Error) *llm.ResponseError {
	return nil
}

func (m *mockTransformer) AggregateStreamChunks(ctx context.Context, _ *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return m.aggregatedResponse, m.aggregatedMeta, m.aggregatedErr
}

func (m *mockTransformer) APIFormat() llm.APIFormat {
	if m.apiFormat != "" {
		return m.apiFormat
	}

	return llm.APIFormatOpenAIChatCompletion
}

type mockLLMCompatibilitySettingsProvider struct {
	settings *biz.LLMCompatibilitySettings
}

func (m *mockLLMCompatibilitySettingsProvider) LLMCompatibilitySettingsOrDefault(context.Context) *biz.LLMCompatibilitySettings {
	if m == nil || m.settings == nil {
		return &biz.LLMCompatibilitySettings{ResponsesOnlyDataPolicy: biz.ResponsesOnlyDataPolicyDiscard}
	}

	return m.settings
}

func TestPersistentOutboundTransformer_TransformRequest_OriginalModelRestoration(t *testing.T) {
	tests := []struct {
		name               string
		originalModel      string
		inputModel         string
		actualModel        string
		expectedFinalModel string
	}{
		{
			name:               "no original model - should use candidate ActualModel",
			originalModel:      "",
			inputModel:         "gpt-4",
			actualModel:        "gpt-4",
			expectedFinalModel: "gpt-4",
		},
		{
			name:               "has original model - should use candidate ActualModel (not OriginalModel)",
			originalModel:      "gpt-3.5-turbo",
			inputModel:         "mapped-gpt-4",
			actualModel:        "gpt-4",
			expectedFinalModel: "gpt-4",
		},
		{
			name:               "candidate ActualModel different from input - should use ActualModel",
			originalModel:      "gpt-4",
			inputModel:         "mapped-gpt-4",
			actualModel:        "claude-3-opus",
			expectedFinalModel: "claude-3-opus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx := context.Background()

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:              1,
					Name:            "test-channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings:        nil,
				},
				Outbound: &mockTransformer{},
			}

			processor := &PersistentOutboundTransformer{
				wrapped: &mockTransformer{},
				state: &PersistenceState{
					OriginalModel:    tt.originalModel,
					CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
					ChannelModelsCandidates: []*ChannelModelsCandidate{
						{Channel: channel, Priority: 0, Models: []biz.ChannelModelEntry{{RequestModel: tt.inputModel, ActualModel: tt.actualModel}}},
					},
					CurrentCandidateIndex: 0,
					RequestExec:           &ent.RequestExecution{ID: 1}, // Dummy to skip creation
				},
			}

			text := "Hello"
			llmRequest := &llm.Request{
				Model: tt.inputModel,
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &text,
						},
					},
				},
			}

			// Execute
			channelRequest, err := processor.TransformRequest(ctx, llmRequest)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, channelRequest)

			// Verify model restoration in the request body
			bodyStr := string(channelRequest.Body)
			model := gjson.Get(bodyStr, "model")
			require.Equal(t, tt.expectedFinalModel, model.String())

			// The shared semantic request must not be mutated by attempt-specific model mapping.
			require.Equal(t, tt.inputModel, llmRequest.Model)
			require.NotNil(t, processor.state.CurrentAttemptRequest)
			require.Equal(t, tt.expectedFinalModel, processor.state.CurrentAttemptRequest.Model)
		})
	}
}

func TestCloneRequestForOutboundAttempt_DeepCopiesAttemptState(t *testing.T) {
	content := "hello"
	maxToolCalls := int64(2)
	rawBody := []byte(`{"model":"alias","input":"hello"}`)
	parameters := json.RawMessage(`{"type":"object"}`)

	req := &llm.Request{
		Model: "gpt-5.4",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: &content,
							TransformerMetadata: map[string]any{
								"raw": json.RawMessage(`{"kind":"input_text"}`),
							},
						},
					},
				},
				ToolCalls: []llm.ToolCall{
					{
						ID: "call_1",
						TransformerMetadata: map[string]any{
							"raw": json.RawMessage(`{"call":"raw"}`),
						},
					},
				},
			},
		},
		Tools: []llm.Tool{
			{
				Type: "function",
				Function: llm.Function{
					Name:       "lookup",
					Parameters: parameters,
				},
			},
		},
		RawRequest: &httpclient.Request{
			Body: rawBody,
		},
		TransformerMetadata: map[string]any{
			"include":        []string{"reasoning.encrypted_content"},
			"max_tool_calls": &maxToolCalls,
		},
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					MetadataRaw: json.RawMessage(`{"count":1}`),
					InputItems: []llm.OpenAIResponsesRawItem{
						{
							Type: "shell_call_output",
							Raw:  json.RawMessage(`{"type":"shell_call_output","output":"raw"}`),
							Extra: map[string]json.RawMessage{
								"namespace": json.RawMessage(`"shell"`),
							},
						},
					},
				},
			},
		},
	}

	cloned := CloneRequestForOutboundAttempt(req)
	require.NotSame(t, req, cloned)

	*cloned.Messages[0].Content.MultipleContent[0].Text = "changed"
	cloned.Messages[0].Content.MultipleContent[0].TransformerMetadata["raw"].(json.RawMessage)[0] = '['
	cloned.Messages[0].ToolCalls[0].TransformerMetadata["raw"].(json.RawMessage)[0] = '['
	cloned.Tools[0].Function.Parameters[0] = '['
	cloned.RawRequest.Body[0] = '['
	cloned.TransformerMetadata["include"].([]string)[0] = "changed"
	*cloned.TransformerMetadata["max_tool_calls"].(*int64) = 9
	cloned.ProviderExtensions.OpenAIResponses.Request.MetadataRaw[0] = '['
	cloned.ProviderExtensions.OpenAIResponses.Request.InputItems[0].Raw[0] = '['
	cloned.ProviderExtensions.OpenAIResponses.Request.InputItems[0].Extra["namespace"][0] = '['

	require.Equal(t, "hello", *req.Messages[0].Content.MultipleContent[0].Text)
	require.JSONEq(t, `{"kind":"input_text"}`, string(req.Messages[0].Content.MultipleContent[0].TransformerMetadata["raw"].(json.RawMessage)))
	require.JSONEq(t, `{"call":"raw"}`, string(req.Messages[0].ToolCalls[0].TransformerMetadata["raw"].(json.RawMessage)))
	require.JSONEq(t, `{"type":"object"}`, string(req.Tools[0].Function.Parameters))
	require.Equal(t, rawBody, req.RawRequest.Body)
	require.Equal(t, []string{"reasoning.encrypted_content"}, req.TransformerMetadata["include"])
	require.Equal(t, int64(2), *req.TransformerMetadata["max_tool_calls"].(*int64))
	require.JSONEq(t, `{"count":1}`, string(req.ProviderExtensions.OpenAIResponses.Request.MetadataRaw))
	require.JSONEq(t, `{"type":"shell_call_output","output":"raw"}`, string(req.ProviderExtensions.OpenAIResponses.Request.InputItems[0].Raw))
	require.JSONEq(t, `"shell"`, string(req.ProviderExtensions.OpenAIResponses.Request.InputItems[0].Extra["namespace"]))
}

func TestPersistentOutboundTransformer_ResponsesSanitizeDoesNotPolluteNextAttempt(t *testing.T) {
	ctx := context.Background()

	chatOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion}
	responsesOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIResponse}
	channel := &biz.Channel{
		Channel:  &ent.Channel{ID: 1, Name: "mixed-channel"},
		Outbound: chatOutbound,
		Outbounds: map[string]transformer.Outbound{
			llm.APIFormatOpenAIResponse.String(): responsesOutbound,
		},
	}

	processor := &PersistentOutboundTransformer{
		wrapped: chatOutbound,
		state: &PersistenceState{
			OriginalModel: "gpt-5.4",
			ChannelModelsCandidates: []*ChannelModelsCandidate{
				{
					Channel:   channel,
					APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
					Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-5.4", ActualModel: "chat-model"}},
				},
				{
					Channel:   channel,
					APIFormat: llm.APIFormatOpenAIResponse.String(),
					Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-5.4", ActualModel: "responses-model"}},
				},
			},
		},
	}

	llmRequest := &llm.Request{
		Model:     "gpt-5.4",
		APIFormat: llm.APIFormatOpenAIResponse,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_custom",
						Type: llm.ToolTypeResponsesCustomTool,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{
							CallID: "call_custom",
							Name:   "local_shell",
							Input:  "echo hi",
						},
					},
				},
			},
		},
	}
	processor.state.SetEffectiveSemanticRequest(llmRequest)

	_, err := processor.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)
	require.True(t, processor.state.CurrentAttemptSanitizeResult.Changed)
	require.Empty(t, processor.state.CurrentAttemptRequest.Messages)
	require.Len(t, llmRequest.Messages[0].ToolCalls, 1)

	err = processor.NextChannel(ctx)
	require.NoError(t, err)

	_, err = processor.TransformRequest(ctx, llmRequest)
	require.NoError(t, err)
	require.False(t, processor.state.CurrentAttemptSanitizeResult.Changed)
	require.Len(t, processor.state.CurrentAttemptRequest.Messages[0].ToolCalls, 1)
	require.Equal(t, "responses-model", processor.state.CurrentAttemptRequest.Model)
}

func TestPersistentOutboundTransformer_PrepareForRetry(t *testing.T) {
	// Setup
	ctx := context.Background()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	t.Run("single model, retry should trigger 'reuse same model' logic", func(t *testing.T) {
		// Case: single model, retry should trigger "reuse same model" logic
		processor := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models: []biz.ChannelModelEntry{
						{RequestModel: "gpt-4", ActualModel: "gpt-4"},
					},
				},
				CurrentModelIndex: 0,
				RequestExec:       &ent.RequestExecution{ID: 1},
			},
		}

		// Execute PrepareForRetry
		// It should reset RequestExec and do not increase the CurrentModelIndex
		err := processor.PrepareForRetry(ctx)

		// Assert
		require.NoError(t, err)
		require.Zero(t, processor.state.CurrentModelIndex)
		require.Nil(t, processor.state.RequestExec)
	})

	t.Run("multiple models, retry should trigger 'reuse same model' logic", func(t *testing.T) {
		// Case: multiple models, retry should trigger "reuse same model" logic
		processor := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models: []biz.ChannelModelEntry{
						{RequestModel: "gpt-4", ActualModel: "gpt-4"},
						{RequestModel: "gpt-3.5-turbo", ActualModel: "gpt-3.5-turbo"},
					},
				},
				CurrentModelIndex: 0,
				RequestExec:       &ent.RequestExecution{ID: 1},
			},
		}

		// Execute PrepareForRetry
		// It should reset RequestExec and do increased the CurrentModelIndex
		err := processor.PrepareForRetry(ctx)

		// Assert
		require.NoError(t, err)
		require.Equal(t, 1, processor.state.CurrentModelIndex)
		require.Nil(t, processor.state.RequestExec)
	})
}

func TestPersistentOutboundTransformer_PrepareForRetry_UsesCandidateAPIFormatOutbound(t *testing.T) {
	ctx := context.Background()

	primaryOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion}
	embeddingOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIEmbedding}
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: primaryOutbound,
		Outbounds: map[string]transformer.Outbound{
			llm.APIFormatOpenAIEmbedding.String(): embeddingOutbound,
		},
	}

	processor := &PersistentOutboundTransformer{
		wrapped: primaryOutbound,
		state: &PersistenceState{
			CurrentCandidate: &ChannelModelsCandidate{
				Channel:   channel,
				APIFormat: llm.APIFormatOpenAIEmbedding.String(),
				Models: []biz.ChannelModelEntry{
					{RequestModel: "text-embedding-3-small", ActualModel: "text-embedding-3-small"},
					{RequestModel: "text-embedding-3-large", ActualModel: "text-embedding-3-large"},
				},
			},
			CurrentModelIndex: 0,
			RequestExec:       &ent.RequestExecution{ID: 1},
		},
	}

	err := processor.PrepareForRetry(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, processor.state.CurrentModelIndex)
	require.Same(t, embeddingOutbound, processor.wrapped)
}

func TestPersistentOutboundTransformer_NextChannel_UsesCandidateAPIFormatOutbound(t *testing.T) {
	ctx := context.Background()

	primaryOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion}
	embeddingOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIEmbedding}
	chatChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "chat-channel",
		},
		Outbound: primaryOutbound,
	}
	embeddingChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   2,
			Name: "embedding-channel",
		},
		Outbound: primaryOutbound,
		Outbounds: map[string]transformer.Outbound{
			llm.APIFormatOpenAIEmbedding.String(): embeddingOutbound,
		},
	}

	processor := &PersistentOutboundTransformer{
		wrapped: primaryOutbound,
		state: &PersistenceState{
			CurrentCandidateIndex: 0,
			ChannelModelsCandidates: []*ChannelModelsCandidate{
				{
					Channel: chatChannel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4o-mini", ActualModel: "gpt-4o-mini"}},
				},
				{
					Channel:   embeddingChannel,
					APIFormat: llm.APIFormatOpenAIEmbedding.String(),
					Models:    []biz.ChannelModelEntry{{RequestModel: "text-embedding-3-small", ActualModel: "text-embedding-3-small"}},
				},
			},
		},
	}

	err := processor.NextChannel(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, processor.state.CurrentCandidateIndex)
	require.Same(t, embeddingChannel, processor.state.CurrentCandidate.Channel)
	require.Same(t, embeddingOutbound, processor.wrapped)
}

func TestSelectOutboundForCandidate(t *testing.T) {
	primaryOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion}
	embeddingOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIEmbedding}

	t.Run("nil candidate returns nil", func(t *testing.T) {
		require.Nil(t, selectOutboundForCandidate(nil))
	})

	t.Run("candidate with nil channel returns nil", func(t *testing.T) {
		candidate := &ChannelModelsCandidate{APIFormat: llm.APIFormatOpenAIEmbedding.String()}
		require.Nil(t, selectOutboundForCandidate(candidate))
	})

	t.Run("api format set and found in outbounds returns matching outbound", func(t *testing.T) {
		channel := &biz.Channel{
			Channel:   &ent.Channel{ID: 1, Name: "test"},
			Outbound:  primaryOutbound,
			Outbounds: map[string]transformer.Outbound{llm.APIFormatOpenAIEmbedding.String(): embeddingOutbound},
		}
		candidate := &ChannelModelsCandidate{
			Channel:   channel,
			APIFormat: llm.APIFormatOpenAIEmbedding.String(),
		}
		require.Same(t, embeddingOutbound, selectOutboundForCandidate(candidate))
	})

	t.Run("api format set but not in outbounds falls back to channel outbound", func(t *testing.T) {
		channel := &biz.Channel{
			Channel:   &ent.Channel{ID: 1, Name: "test"},
			Outbound:  primaryOutbound,
			Outbounds: map[string]transformer.Outbound{},
		}
		candidate := &ChannelModelsCandidate{
			Channel:   channel,
			APIFormat: llm.APIFormatOpenAIEmbedding.String(),
		}
		require.Same(t, primaryOutbound, selectOutboundForCandidate(candidate))
	})

	t.Run("nil outbounds falls back to channel outbound", func(t *testing.T) {
		channel := &biz.Channel{
			Channel:  &ent.Channel{ID: 1, Name: "test"},
			Outbound: primaryOutbound,
		}
		candidate := &ChannelModelsCandidate{
			Channel:   channel,
			APIFormat: llm.APIFormatOpenAIEmbedding.String(),
		}
		require.Same(t, primaryOutbound, selectOutboundForCandidate(candidate))
	})

	t.Run("empty api format falls back to channel outbound", func(t *testing.T) {
		channel := &biz.Channel{
			Channel:   &ent.Channel{ID: 1, Name: "test"},
			Outbound:  primaryOutbound,
			Outbounds: map[string]transformer.Outbound{llm.APIFormatOpenAIEmbedding.String(): embeddingOutbound},
		}
		candidate := &ChannelModelsCandidate{
			Channel:   channel,
			APIFormat: "",
		}
		require.Same(t, primaryOutbound, selectOutboundForCandidate(candidate))
	})
}

func TestPersistentOutboundTransformer_CanRetry(t *testing.T) {
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	retryableErr := &httpclient.Error{StatusCode: http.StatusTooManyRequests}
	nonRetryableErr := &httpclient.Error{StatusCode: http.StatusBadRequest}

	t.Run("no current candidate", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: nil,
			},
		}

		require.False(t, outbound.CanRetry(retryableErr))
	})

	t.Run("nil error", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
			},
		}

		require.False(t, outbound.CanRetry(nil))
	})

	t.Run("non-retryable error", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
			},
		}

		require.False(t, outbound.CanRetry(nonRetryableErr))
	})

	t.Run("skip-by-circuit-breaker should not trigger same-channel retry", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models: []biz.ChannelModelEntry{
						{RequestModel: "gpt-4", ActualModel: "gpt-4"},
						{RequestModel: "gpt-3.5-turbo", ActualModel: "gpt-3.5-turbo"},
					},
				},
				CurrentModelIndex: 0,
			},
		}

		require.False(t, outbound.CanRetry(errSkipCandidateByCircuitBreaker))
	})

	t.Run("retryable error does not depend on model index", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
				CurrentModelIndex: 0,
			},
		}

		require.True(t, outbound.CanRetry(retryableErr))
	})
}

func TestIsCompletedAggregatedOutboundResponse(t *testing.T) {
	t.Run("usage means completed", func(t *testing.T) {
		require.True(t, isCompletedAggregated(llm.ResponseMeta{Usage: &llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}))
	})

	t.Run("missing usage is not completed", func(t *testing.T) {
		require.False(t, isCompletedAggregated(llm.ResponseMeta{}))
	})
}

type sliceEventStream struct {
	events []*httpclient.StreamEvent
	index  int
	err    error
	closed bool
}

func (s *sliceEventStream) Next() bool {
	if s.index >= len(s.events) {
		return false
	}

	s.index++
	return true
}

func (s *sliceEventStream) Current() *httpclient.StreamEvent {
	if s.index == 0 || s.index > len(s.events) {
		return nil
	}

	return s.events[s.index-1]
}

func (s *sliceEventStream) Err() error {
	return s.err
}

func (s *sliceEventStream) Close() error {
	s.closed = true
	return nil
}

func TestOutboundPersistentStream_Close_AggregatedResponsesCompletionHandling(t *testing.T) {
	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)

	t.Run("response in_progress without terminal event is not completed", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := ent.NewContext(ctx, client)
		project := createTestProject(t, ctx, client)
		ch := createTestChannel(t, ctx, client)
		_, requestService, _, usageLogService := setupTestServices(t, client)

		req, err := client.Request.Create().
			SetProjectID(project.ID).
			SetChannelID(ch.ID).
			SetModelID("gpt-4.1").
			SetStatus(request.StatusPending).
			SetRequestBody([]byte(`{"stream":true}`)).
			Save(ctx)
		require.NoError(t, err)

		exec, err := client.RequestExecution.Create().
			SetRequestID(req.ID).
			SetProjectID(project.ID).
			SetChannelID(ch.ID).
			SetModelID("gpt-4.1").
			SetRequestBody([]byte(`{"stream":true}`)).
			SetFormat("openai/responses").
			SetStatus(requestexecution.StatusPending).
			SetStream(true).
			Save(ctx)
		require.NoError(t, err)

		stream := &sliceEventStream{
			events: []*httpclient.StreamEvent{{Type: "response.in_progress", Data: []byte(`{"type":"response.in_progress"}`)}},
		}
		transformer := &mockTransformer{
			apiFormat:          llm.APIFormatOpenAIResponse,
			aggregatedResponse: []byte(`{"id":"resp_123","status":"in_progress"}`),
		}
		state := &PersistenceState{}

		persistentStream := NewOutboundPersistentStream(ctx, stream, req, exec, requestService, usageLogService, transformer, nil, state)
		for persistentStream.Next() {
			_ = persistentStream.Current()
		}
		require.NoError(t, persistentStream.Close())

		dbExec, err := client.RequestExecution.Get(ctx, exec.ID)
		require.NoError(t, err)
		require.NotEqual(t, requestexecution.StatusCompleted, dbExec.Status)
		require.Equal(t, requestexecution.StatusFailed, dbExec.Status)
		require.Contains(t, dbExec.ErrorMessage, "stream ended without terminal event or completed response")
		require.False(t, state.StreamCompleted)
	})

	t.Run("aggregated completed response without terminal event is completed", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		ctx := ent.NewContext(ctx, client)
		project := createTestProject(t, ctx, client)
		ch := createTestChannel(t, ctx, client)
		_, requestService, _, usageLogService := setupTestServices(t, client)

		req, err := client.Request.Create().
			SetProjectID(project.ID).
			SetChannelID(ch.ID).
			SetModelID("gpt-4.1").
			SetStatus(request.StatusPending).
			SetRequestBody([]byte(`{"stream":true}`)).
			Save(ctx)
		require.NoError(t, err)

		exec, err := client.RequestExecution.Create().
			SetRequestID(req.ID).
			SetProjectID(project.ID).
			SetChannelID(ch.ID).
			SetModelID("gpt-4.1").
			SetRequestBody([]byte(`{"stream":true}`)).
			SetFormat("openai/responses").
			SetStatus(requestexecution.StatusPending).
			SetStream(true).
			Save(ctx)
		require.NoError(t, err)

		aggregated := []byte(`{"id":"resp_456","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hi"}]}]}`)
		stream := &sliceEventStream{
			events: []*httpclient.StreamEvent{{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"hi"}`)}},
		}
		transformer := &mockTransformer{
			apiFormat:          llm.APIFormatOpenAIResponse,
			aggregatedResponse: aggregated,
			aggregatedMeta: llm.ResponseMeta{
				ID: "resp_456",
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 2,
					TotalTokens:      12,
				},
			},
		}
		state := &PersistenceState{}

		persistentStream := NewOutboundPersistentStream(ctx, stream, req, exec, requestService, usageLogService, transformer, nil, state)
		for persistentStream.Next() {
			_ = persistentStream.Current()
		}
		require.NoError(t, persistentStream.Close())

		dbExec, err := client.RequestExecution.Get(ctx, exec.ID)
		require.NoError(t, err)
		require.Equal(t, requestexecution.StatusCompleted, dbExec.Status)
		require.JSONEq(t, string(aggregated), string(dbExec.ResponseBody))
		require.Equal(t, "resp_456", dbExec.ExternalID)
		require.True(t, state.StreamCompleted)
	})

	t.Run("canceled client with aggregated completed response is still completed", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
		defer client.Close()

		baseCtx := ent.NewContext(ctx, client)
		project := createTestProject(t, baseCtx, client)
		ch := createTestChannel(t, baseCtx, client)
		_, requestService, _, usageLogService := setupTestServices(t, client)

		req, err := client.Request.Create().
			SetProjectID(project.ID).
			SetChannelID(ch.ID).
			SetModelID("gpt-4.1").
			SetStatus(request.StatusPending).
			SetRequestBody([]byte(`{"stream":true}`)).
			Save(baseCtx)
		require.NoError(t, err)

		exec, err := client.RequestExecution.Create().
			SetRequestID(req.ID).
			SetProjectID(project.ID).
			SetChannelID(ch.ID).
			SetModelID("gpt-4.1").
			SetRequestBody([]byte(`{"stream":true}`)).
			SetFormat("openai/responses").
			SetStatus(requestexecution.StatusPending).
			SetStream(true).
			Save(baseCtx)
		require.NoError(t, err)

		aggregated := []byte(`{"id":"resp_codex_like","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"done"}]}]}`)
		stream := &sliceEventStream{
			events: []*httpclient.StreamEvent{{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"done"}`)}},
			err:    context.Canceled,
		}
		transformer := &mockTransformer{
			apiFormat:          llm.APIFormatOpenAIResponse,
			aggregatedResponse: aggregated,
			aggregatedMeta: llm.ResponseMeta{
				ID: "resp_codex_like",
				Usage: &llm.Usage{
					PromptTokens:     20,
					CompletionTokens: 1,
					TotalTokens:      21,
				},
			},
		}
		state := &PersistenceState{}

		requestCtx, cancel := context.WithCancel(baseCtx)
		cancel()

		persistentStream := NewOutboundPersistentStream(requestCtx, stream, req, exec, requestService, usageLogService, transformer, nil, state)
		for persistentStream.Next() {
			_ = persistentStream.Current()
		}
		require.NoError(t, persistentStream.Close())

		dbExec, err := client.RequestExecution.Get(baseCtx, exec.ID)
		require.NoError(t, err)
		require.Equal(t, requestexecution.StatusCompleted, dbExec.Status)
		require.JSONEq(t, string(aggregated), string(dbExec.ResponseBody))
		require.Equal(t, "resp_codex_like", dbExec.ExternalID)
		require.True(t, state.StreamCompleted)
	})
}

func TestPersistentOutboundTransformer_TransformRequest_WithPrepopulatedState(t *testing.T) {
	// Setup
	ctx := context.Background()

	// Pre-populate channels (now done by inbound transformer)
	testChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:              1,
			Name:            "test-channel",
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"}, // Add gpt-3.5-turbo
			Settings:        nil,
		},
		Outbound: &mockTransformer{},
	}

	processor := &PersistentOutboundTransformer{
		wrapped: &mockTransformer{},
		state: &PersistenceState{
			OriginalModel: "gpt-3.5-turbo",
			ChannelModelsCandidates: []*ChannelModelsCandidate{
				{Channel: testChannel, Priority: 0, Models: []biz.ChannelModelEntry{{RequestModel: "gpt-3.5-turbo", ActualModel: "gpt-3.5-turbo"}}},
			}, // Pre-populated by inbound
			CurrentCandidateIndex: 0,
			RequestExec:           &ent.RequestExecution{ID: 1}, // Dummy to skip creation
		},
	}

	text := "Hello"
	llmRequest := &llm.Request{
		Model: "mapped-gpt-4", // This was mapped by inbound transformer
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: &text,
				},
			},
		},
	}

	// Execute
	channelRequest, err := processor.TransformRequest(ctx, llmRequest)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, channelRequest)

	require.Equal(t, "mapped-gpt-4", llmRequest.Model)
	require.NotNil(t, processor.state.CurrentAttemptRequest)
	require.Equal(t, "gpt-3.5-turbo", processor.state.CurrentAttemptRequest.Model)

	model := gjson.GetBytes(channelRequest.Body, "model")
	require.Equal(t, "gpt-3.5-turbo", model.String())

	// Verify channel was used
	require.Equal(t, testChannel, processor.state.CurrentCandidate.Channel)
}

func TestSanitizeOpenAIResponsesForNonResponsesOutbound_DiscardDropsResponsesOnlyData(t *testing.T) {
	previousResponseID := "resp_previous"
	baseRequest := &llm.Request{
		Model:              "gpt-5.4",
		APIFormat:          llm.APIFormatOpenAIResponse,
		PreviousResponseID: &previousResponseID,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_custom_1",
						Type: llm.ToolTypeResponsesCustomTool,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{
							CallID: "call_custom_1",
							Name:   "local_shell",
							Input:  "echo hi",
						},
					},
					{
						ID:   "call_function_1",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: "{}",
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_custom_1"),
				Content:    llm.MessageContent{Content: lo.ToPtr("custom output")},
			},
			{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_function_1"),
				Content:    llm.MessageContent{Content: lo.ToPtr("function output")},
			},
		},
		Tools: []llm.Tool{
			{
				Type: llm.ToolTypeResponsesCustomTool,
				ResponseCustomTool: &llm.ResponseCustomTool{
					Name: "local_shell",
				},
			},
			{
				Type: llm.ToolTypeFunction,
				Function: llm.Function{
					Name: "get_weather",
				},
			},
		},
		ToolChoice: &llm.ToolChoice{
			NamedToolChoice: &llm.NamedToolChoice{
				Type: llm.ToolTypeResponsesCustomTool,
				Function: llm.ToolFunction{
					Name: "local_shell",
				},
			},
		},
		TransformerMetadata: map[string]any{
			"include":        []string{"reasoning.encrypted_content"},
			"max_tool_calls": lo.ToPtr(int64(3)),
			"preserve":       "kept",
		},
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					RawBody: []byte(`{"model":"gpt-5.4"}`),
					TopLevelExtra: map[string]json.RawMessage{
						"client_metadata": []byte(`{"trace":"safe"}`),
					},
					TopLevelSemanticExtra: map[string]json.RawMessage{
						"conversation": []byte(`"conv_123"`),
					},
					MetadataExtra: map[string]json.RawMessage{
						"number": []byte(`1`),
					},
					InputItems: []llm.OpenAIResponsesRawItem{
						{
							Type: "shell_call_output",
							Path: "input[0]",
							Raw:  []byte(`{"type":"shell_call_output","output":"secret"}`),
						},
					},
					Tools: []llm.OpenAIResponsesRawItem{
						{
							Type: "local_shell",
							Path: "tools[0]",
							Raw:  []byte(`{"type":"local_shell"}`),
						},
					},
				},
			},
		},
	}

	got, result, err := sanitizeOpenAIResponsesForNonResponsesOutbound(
		baseRequest,
		llm.APIFormatOpenAIChatCompletion,
		biz.ResponsesOnlyDataPolicyDiscard,
	)
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.False(t, result.Rejected)
	require.NotSame(t, baseRequest, got)
	require.Nil(t, got.ProviderExtensions)
	require.Nil(t, got.PreviousResponseID)
	require.Len(t, got.Tools, 1)
	require.Equal(t, llm.ToolTypeFunction, got.Tools[0].Type)
	require.Len(t, got.Messages, 2)
	require.Len(t, got.Messages[0].ToolCalls, 1)
	require.Equal(t, "call_function_1", got.Messages[0].ToolCalls[0].ID)
	require.NotNil(t, got.Messages[1].ToolCallID)
	require.Equal(t, "call_function_1", *got.Messages[1].ToolCallID)
	require.Nil(t, got.ToolChoice)
	require.Equal(t, "kept", got.TransformerMetadata["preserve"])
	require.NotContains(t, got.TransformerMetadata, "include")
	require.NotContains(t, got.TransformerMetadata, "max_tool_calls")
	require.NotNil(t, baseRequest.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, baseRequest.PreviousResponseID)
	require.Len(t, baseRequest.Tools, 2)
}

func TestSanitizeOpenAIResponsesForNonResponsesOutbound_DiscardSafeRejectControlAllowsSafeExtra(t *testing.T) {
	baseRequest := &llm.Request{
		Model:     "gpt-5.4",
		APIFormat: llm.APIFormatOpenAIResponse,
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					RawBody: []byte(`{"model":"gpt-5.4"}`),
					TopLevelExtra: map[string]json.RawMessage{
						"trace_id": []byte(`"trace_123"`),
					},
					MetadataExtra: map[string]json.RawMessage{
						"number": []byte(`1`),
					},
				},
			},
		},
	}

	got, result, err := sanitizeOpenAIResponsesForNonResponsesOutbound(
		baseRequest,
		llm.APIFormatOpenAIChatCompletion,
		biz.ResponsesOnlyDataPolicyDiscardSafeRejectControl,
	)
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.False(t, result.Rejected)
	require.Nil(t, got.ProviderExtensions)
	require.NotNil(t, baseRequest.ProviderExtensions.OpenAIResponses)
}

func TestSanitizeOpenAIResponsesForNonResponsesOutbound_DiscardSafeRejectControlRejectsControlData(t *testing.T) {
	baseRequest := &llm.Request{
		Model:     "gpt-5.4",
		APIFormat: llm.APIFormatOpenAIResponse,
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					TopLevelSemanticExtra: map[string]json.RawMessage{
						"prompt": []byte(`"raw prompt"`),
					},
					InputItems: []llm.OpenAIResponsesRawItem{
						{
							Type: "mcp_tool_call_output",
							Path: "input[0]",
							Raw:  []byte(`{"type":"mcp_tool_call_output","output":"secret"}`),
						},
					},
				},
			},
		},
	}

	got, result, err := sanitizeOpenAIResponsesForNonResponsesOutbound(
		baseRequest,
		llm.APIFormatOpenAIChatCompletion,
		biz.ResponsesOnlyDataPolicyDiscardSafeRejectControl,
	)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Same(t, baseRequest, got)
	require.True(t, result.Rejected)
	require.False(t, result.Changed)
	require.Contains(t, result.RejectedCategories, "semantic_control_top_level_extra")
	require.Contains(t, result.RejectedCategories, "raw_only_input_items")
	require.NotContains(t, err.Error(), "raw prompt")
	require.NotContains(t, err.Error(), "secret")
}

func TestSanitizeOpenAIResponsesForNonResponsesOutbound_RejectRejectsAnyRawExtension(t *testing.T) {
	baseRequest := &llm.Request{
		Model:     "gpt-5.4",
		APIFormat: llm.APIFormatOpenAIResponse,
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					RawBody: []byte(`{"model":"gpt-5.4"}`),
				},
			},
		},
	}

	got, result, err := sanitizeOpenAIResponsesForNonResponsesOutbound(
		baseRequest,
		llm.APIFormatOpenAIChatCompletion,
		biz.ResponsesOnlyDataPolicyReject,
	)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Same(t, baseRequest, got)
	require.True(t, result.Rejected)
	require.Contains(t, result.RejectedCategories, "raw_extension")
}

func TestPersistentOutboundTransformer_ResponsesOnlyPolicyRejectsCurrentAttempt(t *testing.T) {
	ctx := context.Background()
	chatOutbound := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion}
	channel := &biz.Channel{
		Channel:  &ent.Channel{ID: 1, Name: "chat-only-channel"},
		Outbound: chatOutbound,
	}
	processor := &PersistentOutboundTransformer{
		wrapped: chatOutbound,
		state: &PersistenceState{
			OriginalModel: "gpt-5.4",
			CompatibilitySettingsProvider: &mockLLMCompatibilitySettingsProvider{
				settings: &biz.LLMCompatibilitySettings{ResponsesOnlyDataPolicy: biz.ResponsesOnlyDataPolicyDiscardSafeRejectControl},
			},
			ChannelModelsCandidates: []*ChannelModelsCandidate{
				{
					Channel:   channel,
					APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
					Models:    []biz.ChannelModelEntry{{RequestModel: "gpt-5.4", ActualModel: "chat-model"}},
				},
			},
		},
	}
	llmRequest := &llm.Request{
		Model:     "gpt-5.4",
		APIFormat: llm.APIFormatOpenAIResponse,
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					Tools: []llm.OpenAIResponsesRawItem{
						{
							Type: "local_shell",
							Path: "tools[0]",
							Raw:  []byte(`{"type":"local_shell"}`),
						},
					},
				},
			},
		},
	}
	processor.state.SetEffectiveSemanticRequest(llmRequest)

	_, err := processor.TransformRequest(ctx, llmRequest)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.True(t, processor.state.CurrentAttemptSanitizeResult.Rejected)
	require.NotNil(t, llmRequest.ProviderExtensions.OpenAIResponses)
}

func TestFilterResponseCustomToolMessagesForNonResponsesOutbound(t *testing.T) {
	baseRequest := &llm.Request{
		APIFormat: llm.APIFormatOpenAIResponse,
		Messages: []llm.Message{
			{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_custom_1",
						Type: llm.ToolTypeResponsesCustomTool,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{
							CallID: "call_custom_1",
							Name:   "apply_patch",
							Input:  "*** Begin Patch\n*** End Patch\n",
						},
					},
					{
						ID:   "call_function_1",
						Type: llm.ToolTypeFunction,
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: "{}",
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: func() *string { v := "call_custom_1"; return &v }(),
				Content: llm.MessageContent{
					Content: func() *string { v := "custom"; return &v }(),
				},
			},
			{
				Role:       "tool",
				ToolCallID: func() *string { v := "call_function_1"; return &v }(),
				Content: llm.MessageContent{
					Content: func() *string { v := "function"; return &v }(),
				},
			},
		},
	}

	t.Run("filters when inbound is responses and outbound is not", func(t *testing.T) {
		got := filterResponseCustomToolMessagesForNonResponsesOutbound(baseRequest, llm.APIFormatOpenAIChatCompletion)
		require.NotSame(t, baseRequest, got)
		require.Len(t, got.Messages, 2)
		require.Len(t, got.Messages[0].ToolCalls, 1)
		require.Equal(t, llm.ToolTypeFunction, got.Messages[0].ToolCalls[0].Type)
		require.NotNil(t, got.Messages[1].ToolCallID)
		require.Equal(t, "call_function_1", *got.Messages[1].ToolCallID)
	})

	t.Run("does not filter when outbound is responses", func(t *testing.T) {
		got := filterResponseCustomToolMessagesForNonResponsesOutbound(baseRequest, llm.APIFormatOpenAIResponse)
		require.Same(t, baseRequest, got)
	})

	t.Run("does not filter when inbound is not responses", func(t *testing.T) {
		nonResponsesReq := *baseRequest
		nonResponsesReq.APIFormat = llm.APIFormatOpenAIChatCompletion
		got := filterResponseCustomToolMessagesForNonResponsesOutbound(&nonResponsesReq, llm.APIFormatOpenAIChatCompletion)
		require.Same(t, &nonResponsesReq, got)
	})
}

// ========== 429 Retry-After Tests ==========

func TestPersistentOutboundTransformer_CanRetry_429_WithRetryAfter(t *testing.T) {
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	t.Run("429 with Retry-After should not retry same channel", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
				CurrentModelIndex: 0,
			},
		}

		// 429 error with Retry-After header
		httpErr := &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Headers:    http.Header{"Retry-After": []string{"30"}},
		}

		require.False(t, outbound.CanRetry(httpErr))
	})

	t.Run("429 with multiple headers including Retry-After should not retry", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
				CurrentModelIndex: 0,
			},
		}

		// 429 error with multiple headers
		httpErr := &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Headers: http.Header{
				"Retry-After":  []string{"60"},
				"Content-Type": []string{"application/json"},
			},
		}

		require.False(t, outbound.CanRetry(httpErr))
	})
}

func TestPersistentOutboundTransformer_CanRetry_429_WithoutRetryAfter(t *testing.T) {
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	t.Run("429 without Retry-After (nil headers) should allow retry", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
				CurrentModelIndex: 0,
			},
		}

		// 429 error without headers
		httpErr := &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Headers:    nil,
		}

		require.True(t, outbound.CanRetry(httpErr))
	})

	t.Run("429 without Retry-After (empty headers) should allow retry", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
				CurrentModelIndex: 0,
			},
		}

		// 429 error with empty headers
		httpErr := &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Headers:    http.Header{},
		}

		require.True(t, outbound.CanRetry(httpErr))
	})

	t.Run("429 without Retry-After (headers but no Retry-After key) should allow retry", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-4", ActualModel: "gpt-4"}},
				},
				CurrentModelIndex: 0,
			},
		}

		// 429 error with headers but no Retry-After
		httpErr := &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}

		require.True(t, outbound.CanRetry(httpErr))
	})
}

func TestPersistentOutboundTransformer_CanRetry_429_WithMultipleModels(t *testing.T) {
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:   1,
			Name: "test-channel",
		},
		Outbound: &mockTransformer{},
	}

	t.Run("429 with Retry-After should not retry even with multiple models", func(t *testing.T) {
		outbound := &PersistentOutboundTransformer{
			wrapped: &mockTransformer{},
			state: &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{
					Channel: channel,
					Models: []biz.ChannelModelEntry{
						{RequestModel: "gpt-4", ActualModel: "gpt-4"},
						{RequestModel: "gpt-3.5-turbo", ActualModel: "gpt-3.5-turbo"},
					},
				},
				CurrentModelIndex: 0,
			},
		}

		// 429 error with Retry-After header
		httpErr := &httpclient.Error{
			StatusCode: http.StatusTooManyRequests,
			Headers:    http.Header{"Retry-After": []string{"30"}},
		}

		// Should skip retry even though there are more models
		require.False(t, outbound.CanRetry(httpErr))
	})
}
