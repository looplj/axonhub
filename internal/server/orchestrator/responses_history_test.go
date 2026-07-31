package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

type previousResponseLoadCall struct {
	responseID string
	projectID  int
	apiKeyID   *int
}

type mapPreviousResponseLoader struct {
	exchanges map[string]*biz.StoredResponseExchange
	calls     []previousResponseLoadCall
	err       error
}

func (l *mapPreviousResponseLoader) LoadCompletedResponseExchange(
	_ context.Context,
	responseID string,
	projectID int,
	apiKeyID *int,
	maxBytes int64,
) (*biz.StoredResponseExchange, error) {
	l.calls = append(l.calls, previousResponseLoadCall{
		responseID: responseID,
		projectID:  projectID,
		apiKeyID:   apiKeyID,
	})
	if l.err != nil {
		return nil, l.err
	}
	exchange := l.exchanges[responseID]
	if exchange != nil && int64(len(exchange.RequestBody)+len(exchange.ResponseBody)) > maxBytes {
		return nil, biz.ErrStoredResponseExchangeTooLarge
	}
	return exchange, nil
}

func storedResponseExchange(requestBody, responseBody string) *biz.StoredResponseExchange {
	return &biz.StoredResponseExchange{
		RequestBody:  objects.JSONRawMessage(requestBody),
		ResponseBody: objects.JSONRawMessage(responseBody),
	}
}

func paddedJSONBody(t *testing.T, body string, size int) objects.JSONRawMessage {
	t.Helper()
	require.LessOrEqual(t, len(body), size)
	padded := make(objects.JSONRawMessage, size)
	copy(padded, body)
	for i := len(body); i < len(padded); i++ {
		padded[i] = ' '
	}
	return padded
}

func seedPrimaryDatabaseStorage(t *testing.T, ctx context.Context, client *ent.Client) {
	t.Helper()
	_, err := client.DataStorage.Create().
		SetName("primary-database").
		SetDescription("test primary database storage").
		SetPrimary(true).
		SetType(datastorage.TypeDatabase).
		SetSettings(&objects.DataStorageSettings{}).
		Save(ctx)
	require.NoError(t, err)
}

func TestLoadPreviousResponsesHistory_PreservesToolPrimitivesAndOmitsHistoricalInstructions(t *testing.T) {
	apiKeyID := 17
	loader := &mapPreviousResponseLoader{exchanges: map[string]*biz.StoredResponseExchange{
		"resp_1": storedResponseExchange(
			`{
				"model":"gpt-5.5",
				"instructions":"old top-level instruction",
				"input":[
					{"type":"message","role":"developer","content":[{"type":"input_text","text":"explicit developer input"}]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"create a file"}]}
				]
			}`,
			`{
				"id":"resp_1",
				"model":"gpt-5.5",
				"status":"completed",
				"output":[
					{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"need patch"}],"encrypted_content":"sig_1"},
					{"id":"call_item_1","type":"custom_tool_call","name":"apply_patch","input":"*** Begin Patch\n*** End Patch","call_id":"apply_patch_1","status":"completed"},
					{"id":"call_item_2","type":"tool_search_call","execution":"client","arguments":"{\"query\":\"agent tools\"}","call_id":"search_1","status":"completed"},
					{"id":"call_item_3","type":"function_call","name":"spawn_agent","namespace":"collaboration","arguments":"{}","call_id":"spawn_1","status":"completed"}
				]
			}`,
		),
	}}

	history, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_1", 9, &apiKeyID)
	require.NoError(t, err)
	require.Len(t, history, 3)
	require.Equal(t, "developer", history[0].Role)
	require.Equal(t, "explicit developer input", lo.FromPtr(history[0].Content.Content))
	require.Equal(t, "user", history[1].Role)
	require.Equal(t, "create a file", lo.FromPtr(history[1].Content.Content))
	require.Equal(t, "assistant", history[2].Role)
	require.Equal(t, "need patch", lo.FromPtr(history[2].ReasoningContent))
	require.Equal(t, "sig_1", lo.FromPtr(history[2].ReasoningSignature))
	require.Len(t, history[2].ToolCalls, 3)
	require.Equal(t, "apply_patch_1", history[2].ToolCalls[0].ID)
	require.Equal(t, llm.ToolTypeResponsesCustomTool, history[2].ToolCalls[0].Type)
	require.Equal(t, "apply_patch", history[2].ToolCalls[0].ResponseCustomToolCall.Name)
	require.Equal(t, "*** Begin Patch\n*** End Patch", history[2].ToolCalls[0].ResponseCustomToolCall.Input)
	require.Equal(t, "search_1", history[2].ToolCalls[1].ID)
	require.Equal(t, llm.ToolTypeResponsesToolSearch, history[2].ToolCalls[1].Type)
	require.Equal(t, "client", history[2].ToolCalls[1].ResponseToolSearchCall.Execution)
	require.JSONEq(t, `{"query":"agent tools"}`, history[2].ToolCalls[1].ResponseToolSearchCall.Arguments)
	require.Equal(t, "spawn_1", history[2].ToolCalls[2].ID)
	require.Equal(t, "spawn_agent", history[2].ToolCalls[2].Function.Name)
	require.Equal(t, "collaboration", history[2].ToolCalls[2].Function.Namespace)
	require.Equal(t, []previousResponseLoadCall{{responseID: "resp_1", projectID: 9, apiKeyID: &apiKeyID}}, loader.calls)
}

func TestLoadPreviousResponsesHistory_OrdersMultiHopFunctionLifecycle(t *testing.T) {
	loader := &mapPreviousResponseLoader{exchanges: map[string]*biz.StoredResponseExchange{
		"resp_1": storedResponseExchange(
			`{"model":"gpt-5.5","input":"first question"}`,
			`{"id":"resp_1","model":"gpt-5.5","output":[{"type":"function_call","name":"lookup","arguments":"{\"id\":1}","call_id":"call_1","status":"completed"}]}`,
		),
		"resp_2": storedResponseExchange(
			`{"model":"gpt-5.5","previous_response_id":"resp_1","input":[{"type":"function_call_output","call_id":"call_1","output":"result one"}]}`,
			`{"id":"resp_2","model":"gpt-5.5","output":[{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"second answer","annotations":[]}]}]}`,
		),
	}}

	history, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_2", 3, nil)
	require.NoError(t, err)
	require.Len(t, history, 4)
	require.Equal(t, "user", history[0].Role)
	require.Equal(t, "first question", lo.FromPtr(history[0].Content.Content))
	require.Equal(t, "assistant", history[1].Role)
	require.Len(t, history[1].ToolCalls, 1)
	require.Equal(t, "call_1", history[1].ToolCalls[0].ID)
	require.Equal(t, "lookup", history[1].ToolCalls[0].Function.Name)
	require.Equal(t, "tool", history[2].Role)
	require.Equal(t, "call_1", lo.FromPtr(history[2].ToolCallID))
	require.Equal(t, "result one", lo.FromPtr(history[2].Content.Content))
	require.Equal(t, "assistant", history[3].Role)
	require.Equal(t, "second answer", lo.FromPtr(history[3].Content.Content))
	require.Equal(t, "resp_2", loader.calls[0].responseID)
	require.Equal(t, "resp_1", loader.calls[1].responseID)
}

func TestLoadPreviousResponsesHistory_RejectsMissingCycleAndMismatchedID(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		loader := &mapPreviousResponseLoader{exchanges: map[string]*biz.StoredResponseExchange{}}
		_, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_missing", 1, nil)
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		require.ErrorContains(t, err, "was not found")
	})

	t.Run("cycle", func(t *testing.T) {
		loader := &mapPreviousResponseLoader{exchanges: map[string]*biz.StoredResponseExchange{
			"resp_a": storedResponseExchange(
				`{"model":"gpt-5.5","previous_response_id":"resp_b","input":"a"}`,
				`{"id":"resp_a","model":"gpt-5.5","output":[]}`,
			),
			"resp_b": storedResponseExchange(
				`{"model":"gpt-5.5","previous_response_id":"resp_a","input":"b"}`,
				`{"id":"resp_b","model":"gpt-5.5","output":[]}`,
			),
		}}
		_, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_a", 1, nil)
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		require.ErrorContains(t, err, "contains a cycle")
	})

	t.Run("mismatched response ID", func(t *testing.T) {
		loader := &mapPreviousResponseLoader{exchanges: map[string]*biz.StoredResponseExchange{
			"resp_expected": storedResponseExchange(
				`{"model":"gpt-5.5","input":"hello"}`,
				`{"id":"resp_other","model":"gpt-5.5","output":[]}`,
			),
		}}
		_, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_expected", 1, nil)
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		require.ErrorContains(t, err, "does not match requested ID")
	})

	t.Run("request body not retained", func(t *testing.T) {
		loader := &mapPreviousResponseLoader{exchanges: map[string]*biz.StoredResponseExchange{
			"resp_1": storedResponseExchange(
				`{}`,
				`{"id":"resp_1","model":"gpt-5.5","output":[]}`,
			),
		}}
		history, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_1", 1, nil)
		require.Nil(t, history)
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		require.ErrorContains(t, err, `stored request for previous response "resp_1" is unavailable or invalid`)
	})
}

func TestLoadPreviousResponsesHistory_PreservesStorageFailureClassification(t *testing.T) {
	storageErr := errors.New("object storage temporarily unavailable")
	loader := &mapPreviousResponseLoader{err: storageErr}

	_, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_1", 1, nil)
	require.ErrorIs(t, err, storageErr)
	require.NotErrorIs(t, err, transformer.ErrInvalidRequest)
}

func TestNextPreviousResponseHistoryBytes_EnforcesExactLimit(t *testing.T) {
	tests := []struct {
		name          string
		total         int64
		requestBytes  int
		responseBytes int
		want          int64
		ok            bool
	}{
		{name: "exact limit", total: maxPreviousResponseHistoryBytes - 2, requestBytes: 1, responseBytes: 1, want: maxPreviousResponseHistoryBytes, ok: true},
		{name: "one byte over", total: maxPreviousResponseHistoryBytes, requestBytes: 1, want: maxPreviousResponseHistoryBytes, ok: false},
		{name: "request exceeds", requestBytes: int(maxPreviousResponseHistoryBytes) + 1, ok: false},
		{name: "response exceeds", responseBytes: int(maxPreviousResponseHistoryBytes) + 1, ok: false},
		{name: "response crosses remaining limit", total: maxPreviousResponseHistoryBytes - 1, requestBytes: 1, responseBytes: 1, want: maxPreviousResponseHistoryBytes - 1, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nextPreviousResponseHistoryBytes(tt.total, tt.requestBytes, tt.responseBytes)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoadPreviousResponsesHistory_RejectsCumulativeHistoryOverByteLimit(t *testing.T) {
	requestBody := `{"model":"gpt-5.5","previous_response_id":"resp_next","input":"first"}`
	responseBody := `{"id":"resp_limit","model":"gpt-5.5","output":[]}`
	requestSize := int(maxPreviousResponseHistoryBytes) - len(responseBody)
	loader := &mapPreviousResponseLoader{exchanges: map[string]*biz.StoredResponseExchange{
		"resp_limit": {
			RequestBody:  paddedJSONBody(t, requestBody, requestSize),
			ResponseBody: objects.JSONRawMessage(responseBody),
		},
		"resp_next": {
			RequestBody: objects.JSONRawMessage(`x`),
		},
	}}

	history, err := loadPreviousResponsesHistory(t.Context(), loader, "resp_limit", 1, nil)
	require.Nil(t, history)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.ErrorContains(t, err, "previous_response_id history exceeds 33554432 bytes")
	require.Equal(t, []previousResponseLoadCall{
		{responseID: "resp_limit", projectID: 1},
		{responseID: "resp_next", projectID: 1},
	}, loader.calls)
}

func TestPersistentOutboundTransformer_PreviousResponseIDRouting(t *testing.T) {
	newProcessor := func(out *mockTransformer, state *PersistenceState) *PersistentOutboundTransformer {
		channel := &biz.Channel{
			Channel:  &ent.Channel{ID: 1, Name: "test-channel"},
			Outbound: out,
		}
		state.ChannelModelsCandidates = []*ChannelModelsCandidate{{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{RequestModel: "gpt-5.5", ActualModel: "Kimi-K3"}},
		}}
		return &PersistentOutboundTransformer{wrapped: out, state: state}
	}

	t.Run("Responses outbound keeps previous_response_id without storage", func(t *testing.T) {
		out := &mockTransformer{apiFormat: llm.APIFormatOpenAIResponse}
		processor := newProcessor(out, &PersistenceState{})
		req := &llm.Request{
			Model:              "gpt-5.5",
			APIFormat:          llm.APIFormatOpenAIResponse,
			PreviousResponseID: lo.ToPtr("resp_1"),
			Messages:           []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("next")}}},
		}

		_, err := processor.TransformRequest(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, out.capturedRequest.PreviousResponseID)
		require.Equal(t, "resp_1", *out.capturedRequest.PreviousResponseID)
	})

	t.Run("Chat outbound prepends stored history and clears previous_response_id", func(t *testing.T) {
		client := enttest.NewEntClient(t, "sqlite3", "file:responses_history_route?mode=memory&_fk=0")
		ctx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
		seedPrimaryDatabaseStorage(t, ctx, client)
		requestService := createTestRequestService(t, client)
		_, err := client.Request.Create().
			SetProjectID(7).
			SetModelID("gpt-5.5").
			SetFormat(llm.APIFormatOpenAIResponse.String()).
			SetSource(request.SourceAPI).
			SetStatus(request.StatusCompleted).
			SetStream(false).
			SetRequestBody(objects.JSONRawMessage(`{"model":"gpt-5.5","input":"first"}`)).
			SetResponseBody(objects.JSONRawMessage(`{"id":"resp_1","model":"gpt-5.5","output":[{"type":"function_call","name":"lookup","arguments":"{}","call_id":"call_1"}]}`)).
			SetExternalID("resp_1").
			Save(ctx)
		require.NoError(t, err)

		out := &mockTransformer{apiFormat: llm.APIFormatOpenAIChatCompletion}
		processor := newProcessor(out, &PersistenceState{
			RequestService: requestService,
			Request:        &ent.Request{ProjectID: 7},
		})
		req := &llm.Request{
			Model:              "gpt-5.5",
			APIFormat:          llm.APIFormatOpenAIResponse,
			PreviousResponseID: lo.ToPtr("resp_1"),
			Messages: []llm.Message{{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_1"),
				Content:    llm.MessageContent{Content: lo.ToPtr("result")},
			}},
		}

		_, err = processor.TransformRequest(ctx, req)
		require.NoError(t, err)
		require.Nil(t, out.capturedRequest.PreviousResponseID)
		require.Len(t, out.capturedRequest.Messages, 3)
		require.Equal(t, "user", out.capturedRequest.Messages[0].Role)
		require.Equal(t, "assistant", out.capturedRequest.Messages[1].Role)
		require.Equal(t, "tool", out.capturedRequest.Messages[2].Role)
		// Hydration uses a clone. Retries receive the original request and cannot
		// accumulate the same history repeatedly.
		require.Equal(t, "resp_1", lo.FromPtr(req.PreviousResponseID))
		require.Len(t, req.Messages, 1)
	})
}

func TestRequestService_LoadCompletedResponseExchange_ScopesByProjectAndAPIKey(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:responses_history_scope?mode=memory&_fk=0")
	ctx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
	seedPrimaryDatabaseStorage(t, ctx, client)
	requestService := createTestRequestService(t, client)

	seed := func(projectID int, apiKeyID *int, marker string) {
		create := client.Request.Create().
			SetProjectID(projectID).
			SetModelID("gpt-5.5").
			SetFormat(llm.APIFormatOpenAIResponse.String()).
			SetSource(request.SourceAPI).
			SetStatus(request.StatusCompleted).
			SetStream(false).
			SetRequestBody(objects.JSONRawMessage(`{"model":"gpt-5.5","input":"` + marker + `"}`)).
			SetResponseBody(objects.JSONRawMessage(`{"id":"shared_response_id","model":"gpt-5.5","output":[]}`)).
			SetExternalID("shared_response_id")
		if apiKeyID != nil {
			create.SetAPIKeyID(*apiKeyID)
		}
		_, err := create.Save(ctx)
		require.NoError(t, err)
	}

	key10 := 10
	key20 := 20
	seed(1, &key10, "project-1-key-10")
	seed(1, &key20, "project-1-key-20")
	seed(2, &key10, "project-2-key-10")
	seed(1, nil, "project-1-admin")

	exchange, err := requestService.LoadCompletedResponseExchange(ctx, "shared_response_id", 1, &key20, maxPreviousResponseHistoryBytes)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.5","input":"project-1-key-20"}`, string(exchange.RequestBody))

	exchange, err = requestService.LoadCompletedResponseExchange(ctx, "shared_response_id", 1, nil, maxPreviousResponseHistoryBytes)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.5","input":"project-1-admin"}`, string(exchange.RequestBody))

	missingKey := 99
	exchange, err = requestService.LoadCompletedResponseExchange(ctx, "shared_response_id", 1, &missingKey, maxPreviousResponseHistoryBytes)
	require.NoError(t, err)
	require.Nil(t, exchange)

	exchange, err = requestService.LoadCompletedResponseExchange(ctx, "shared_response_id", 1, &key10, maxPreviousResponseHistoryBytes)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5.5","input":"project-1-key-10"}`, string(exchange.RequestBody))
}

func TestRequestService_LoadCompletedResponseExchange_RejectsDatabaseBodiesBeforeBudgetedLoad(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:responses_history_body_limit?mode=memory&_fk=0")
	ctx := ent.NewContext(authz.WithTestBypass(t.Context()), client)
	seedPrimaryDatabaseStorage(t, ctx, client)
	requestService := createTestRequestService(t, client)

	requestBody := objects.JSONRawMessage(`{"model":"gpt-5.5","input":"你好"}`)
	responseBody := objects.JSONRawMessage(`{"id":"resp_limited","model":"gpt-5.5","output":[]}`)
	_, err := client.Request.Create().
		SetProjectID(1).
		SetModelID("gpt-5.5").
		SetFormat(llm.APIFormatOpenAIResponse.String()).
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		SetRequestBody(requestBody).
		SetResponseBody(responseBody).
		SetExternalID("resp_limited").
		Save(ctx)
	require.NoError(t, err)

	exactBudget := int64(len(requestBody) + len(responseBody))
	exchange, err := requestService.LoadCompletedResponseExchange(ctx, "resp_limited", 1, nil, exactBudget)
	require.NoError(t, err)
	require.Equal(t, requestBody, exchange.RequestBody)
	require.Equal(t, responseBody, exchange.ResponseBody)

	exchange, err = requestService.LoadCompletedResponseExchange(ctx, "resp_limited", 1, nil, exactBudget-1)
	require.Nil(t, exchange)
	require.ErrorIs(t, err, biz.ErrStoredResponseExchangeTooLarge)
}
