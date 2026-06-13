package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestCodexOutbound_StreamAcceptHeader(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)
	capturedHeaders := make(chan http.Header, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders <- r.Header.Clone()

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: server.URL,
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	request := buildCodexStreamRequest(t, ctx, outbound, false)
	executor := httpclient.NewHttpClientWithClient(server.Client())

	stream, err := executor.DoStream(ctx, request)
	require.NoError(t, err)

	defer func() {
		_ = stream.Close()
	}()

	var headers http.Header
	select {
	case headers = <-capturedHeaders:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for captured stream request")
	}

	assert.Equal(t, "text/event-stream", headers.Get("Accept"))
	assert.Equal(t, "application/json", headers.Get("Content-Type"))
	assert.Equal(t, AxonHubOriginator, headers.Get("Originator"))
	assert.Equal(t, "axonhub/1.0", headers.Get("User-Agent"))
	assert.Equal(t, testChatAccountID, headers.Get("Chatgpt-Account-Id"))
	assert.Equal(t, "Bearer "+accessToken, headers.Get("Authorization"))
}

func TestCodexOutbound_StreamAllowsDownstreamIdentityOverrides(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)
	capturedHeaders := make(chan http.Header, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders <- r.Header.Clone()

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: server.URL,
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	request := buildCodexStreamRequest(t, ctx, outbound, true)
	executor := httpclient.NewHttpClientWithClient(server.Client())

	stream, err := executor.DoStream(ctx, request)
	require.NoError(t, err)

	defer func() {
		_ = stream.Close()
	}()

	var headers http.Header
	select {
	case headers = <-capturedHeaders:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for captured stream request")
	}

	assert.Equal(t, legacyCodexOriginator(), headers.Get("Originator"))
	assert.Equal(t, legacyCodexUserAgent(), headers.Get("User-Agent"))
	assert.Contains(t, strings.ToLower(headers.Get("User-Agent")), legacyCodexOriginator())
	assert.Equal(t, testChatAccountID, headers.Get("Chatgpt-Account-Id"))
	assert.Equal(t, "Bearer "+accessToken, headers.Get("Authorization"))
}

func TestCodexOutbound_ImageGenerationRequestUsesResponsesImageTool(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: "https://chatgpt.com/backend-api/codex#",
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	partialImages := int64(2)
	req, err := outbound.TransformRequest(ctx, &llm.Request{
		Model:       "gpt-image-2",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageGeneration,
		RawRequest:  &httpclient.Request{Headers: http.Header{}},
		Image: &llm.ImageRequest{
			Prompt:        "draw a circuit board city",
			Size:          "1536x864",
			Quality:       "high",
			OutputFormat:  "webp",
			PartialImages: &partialImages,
		},
	})
	require.NoError(t, err)

	require.Equal(t, llm.RequestTypeImage.String(), req.RequestType)
	require.Equal(t, llm.APIFormatOpenAIImageGeneration.String(), req.APIFormat)
	require.Equal(t, "text/event-stream", req.Headers.Get("Accept"))
	require.Equal(t, accessToken, req.Auth.APIKey)

	var payload responses.Request
	require.NoError(t, json.Unmarshal(req.Body, &payload))
	require.Equal(t, "gpt-image-2", payload.Model)
	require.NotNil(t, payload.Stream)
	require.True(t, *payload.Stream)
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "image_generation", payload.Tools[0].Type)
	require.Equal(t, "gpt-image-2", payload.Tools[0].Model)
	require.Equal(t, "generate", payload.Tools[0].Action)
	require.Equal(t, "1536x864", payload.Tools[0].Size)
	require.Equal(t, "high", payload.Tools[0].Quality)
	require.Equal(t, "webp", payload.Tools[0].OutputFormat)
	require.Equal(t, partialImages, *payload.Tools[0].PartialImages)
	require.NotNil(t, payload.ToolChoice)
	require.Equal(t, "required", *payload.ToolChoice.Mode)
	require.Len(t, payload.Input.Items, 1)
	require.Len(t, payload.Input.Items[0].Content.Items, 1)
	require.Equal(t, "input_text", payload.Input.Items[0].Content.Items[0].Type)
	require.Equal(t, "draw a circuit board city", *payload.Input.Items[0].Content.Items[0].Text)
}

func TestCodexOutbound_ImageEditRequestUsesResponsesImageTool(t *testing.T) {
	ctx := context.Background()

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: "https://chatgpt.com/backend-api/codex#",
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: testAccessTokenWithAccountID(t),
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	req, err := outbound.TransformRequest(ctx, &llm.Request{
		Model:       "gpt-image-2",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageEdit,
		RawRequest: &httpclient.Request{
			Headers: http.Header{},
			JSONBody: []byte(`{
				"image": "data:image/jpeg;base64,anBlZy1kYXRh",
				"mask": "data:image/png;base64,bWFzay1kYXRh"
			}`),
		},
		Image: &llm.ImageRequest{
			Prompt:        "replace the background with matte white",
			Size:          "1024x1024",
			InputFidelity: "high",
			OutputFormat:  "png",
			Images:        [][]byte{[]byte("jpeg-data")},
			Mask:          []byte("mask-data"),
		},
	})
	require.NoError(t, err)

	require.Equal(t, llm.RequestTypeImage.String(), req.RequestType)
	require.Equal(t, llm.APIFormatOpenAIImageEdit.String(), req.APIFormat)

	var payload responses.Request
	require.NoError(t, json.Unmarshal(req.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "edit", payload.Tools[0].Action)
	require.Equal(t, "high", payload.Tools[0].InputFidelity)
	require.Equal(t, "data:image/png;base64,bWFzay1kYXRh", payload.Tools[0].InputImageMask["image_url"])
	require.Len(t, payload.Input.Items, 1)
	require.Len(t, payload.Input.Items[0].Content.Items, 2)
	require.Equal(t, "input_text", payload.Input.Items[0].Content.Items[0].Type)
	require.Equal(t, "input_image", payload.Input.Items[0].Content.Items[1].Type)
	require.Equal(t, "data:image/jpeg;base64,anBlZy1kYXRh", *payload.Input.Items[0].Content.Items[1].ImageURL)
}

func TestCodexOutbound_TransformImageResponse(t *testing.T) {
	upstream := &responses.Response{
		ID:        "resp_image",
		CreatedAt: 1760000000,
		Model:     "gpt-image-2",
		Output: []responses.Item{
			{
				Type:   "image_generation_call",
				Result: lo.ToPtr("iVBORw0KGgo="),
			},
		},
	}

	resp, err := responses.BuildImageResponse(upstream, map[string]any{
		"codex_image_output_format": "png",
		"codex_image_quality":       "high",
		"codex_image_size":          "1024x1024",
	})
	require.NoError(t, err)

	require.Equal(t, llm.RequestTypeImage, resp.RequestType)
	require.Equal(t, "gpt-image-2", resp.Model)
	require.NotNil(t, resp.Image)
	require.Equal(t, "png", resp.Image.OutputFormat)
	require.Equal(t, "high", resp.Image.Quality)
	require.Equal(t, "1024x1024", resp.Image.Size)
	require.Len(t, resp.Image.Data, 1)
	require.Equal(t, "iVBORw0KGgo=", resp.Image.Data[0].B64JSON)
}

func TestCodexOutbound_CustomizeExecutorUsesCurrentExecutor(t *testing.T) {
	outbound, err := NewOutboundTransformer(Params{
		BaseURL:       "wss://chatgpt.com/backend-api/codex#",
		Transport:     responses.TransportWebSocket,
		TokenProvider: staticTokenGetter{creds: &oauth.OAuthCredentials{AccessToken: testAccessTokenWithAccountID(t), ExpiresAt: time.Now().Add(time.Hour)}},
	})
	require.NoError(t, err)

	firstClient := httpclient.NewHttpClientWithProxy(&httpclient.ProxyConfig{Type: httpclient.ProxyTypeDisabled})
	secondClient := httpclient.NewHttpClientWithProxy(&httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: "http://127.0.0.1:18081"})

	first, ok := outbound.CustomizeExecutor(firstClient).(*codexExecutor)
	require.True(t, ok)
	firstInner, ok := first.inner.(*responses.WebSocketExecutor)
	require.True(t, ok)

	second, ok := outbound.CustomizeExecutor(secondClient).(*codexExecutor)
	require.True(t, ok)
	secondInner, ok := second.inner.(*responses.WebSocketExecutor)
	require.True(t, ok)
	again, ok := outbound.CustomizeExecutor(firstClient).(*codexExecutor)
	require.True(t, ok)
	againInner, ok := again.inner.(*responses.WebSocketExecutor)
	require.True(t, ok)

	require.NotSame(t, firstInner, secondInner)
	require.Same(t, firstInner, againInner)
	require.Same(t, firstClient, firstInner.Inner())
	require.Same(t, secondClient, secondInner.Inner())
}

func TestCodexOutbound_CustomizeExecutorAggregatesNonStreamRequests(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: "https://chatgpt.com/backend-api/codex#",
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	request := buildCodexStreamRequest(t, ctx, outbound, false)
	executor := outbound.CustomizeExecutor(&mockCodexExecutor{
		streamEvents: []*httpclient.StreamEvent{
			{Type: "response.created", Data: []byte(`{"type":"response.created","sequence_number":0,"response":{"id":"resp_test_123","object":"response","created_at":1700000000,"model":"gpt-5-codex","status":"in_progress","output":[]}}`)},
			{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"id":"msg_test_456","type":"message","status":"in_progress","role":"assistant"}}`)},
			{Type: "response.content_part.added", Data: []byte(`{"type":"response.content_part.added","sequence_number":2,"item_id":"msg_test_456","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`)},
			{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","sequence_number":3,"item_id":"msg_test_456","output_index":0,"content_index":0,"delta":"Hello"}`)},
			{Type: "response.output_text.done", Data: []byte(`{"type":"response.output_text.done","sequence_number":4,"item_id":"msg_test_456","output_index":0,"content_index":0,"text":"Hello"}`)},
			{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","sequence_number":5,"output_index":0,"item":{"id":"msg_test_456","type":"message","status":"completed","role":"assistant"}}`)},
			{Type: "response.completed", Data: []byte(`{"type":"response.completed","sequence_number":6,"response":{"id":"resp_test_123","object":"response","created_at":1700000000,"model":"gpt-5-codex","status":"completed","output":[]}}`)},
		},
	})

	response, err := executor.Do(ctx, request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "application/json", response.Headers.Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body, &body))
	assert.Equal(t, "resp_test_123", body["id"])
	assert.Equal(t, "completed", body["status"])
	assert.Equal(t, "gpt-5-codex", body["model"])
}

func TestCodexOutbound_DoReturnsWebSocketErrorEvents(t *testing.T) {
	ctx := context.Background()
	accessToken := testAccessTokenWithAccountID(t)

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: "https://chatgpt.com/backend-api/codex#",
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	request := buildCodexStreamRequest(t, ctx, outbound, false)
	executor := outbound.CustomizeExecutor(&mockCodexExecutor{
		streamEvents: []*httpclient.StreamEvent{
			{Type: "error", Data: []byte(`{"type":"error","code":"bad_request","message":"invalid websocket request"}`)},
		},
	})

	response, err := executor.Do(ctx, request)
	require.Nil(t, response)
	require.ErrorContains(t, err, "bad_request: invalid websocket request")
}

var _ pipeline.ChannelCustomizedExecutor = (*OutboundTransformer)(nil)

type mockCodexExecutor struct {
	streamEvents []*httpclient.StreamEvent
}

func (m *mockCodexExecutor) Do(_ context.Context, _ *httpclient.Request) (*httpclient.Response, error) {
	return nil, assert.AnError
}

func (m *mockCodexExecutor) DoStream(_ context.Context, _ *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.SliceStream(m.streamEvents), nil
}

func TestCodexOutbound_DoesNotInjectCLIInstructions(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)

	hreq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
		Stream: lo.ToPtr(true),
	})
	require.NoError(t, err)

	body := decodeCodexRequestBody(t, hreq)

	instructions, hasInstructions := body["instructions"]
	assert.True(t, hasInstructions, "instructions field must always be present for Codex")
	assert.Equal(t, "", instructions)
	assert.NotContains(t, string(hreq.Body), "You are a coding agent running in the Codex CLI")
	assert.NotContains(t, string(hreq.Body), "You are Codex")
	assert.Equal(t, false, body["store"])
}

func TestCodexOutbound_PreservesMinimalCompatTransforms(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)
	store := true
	parallelToolCalls := false
	maxTokens := int64(128)
	maxCompletionTokens := int64(256)
	topP := 0.8
	serviceTier := "flex"
	reasoningSummary := "detailed"

	hreq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
		Tools: []llm.Tool{{
			Type: "function",
			Function: llm.Function{
				Name:       "shell",
				Parameters: []byte(`{"type":"object","properties":{}}`),
			},
		}},
		Store:               &store,
		ParallelToolCalls:   &parallelToolCalls,
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		TopP:                &topP,
		ServiceTier:         &serviceTier,
		ReasoningSummary:    &reasoningSummary,
		Metadata:            map[string]string{"source": "caller"},
		TransformerMetadata: map[string]any{},
	})
	require.NoError(t, err)

	body := decodeCodexRequestBody(t, hreq)

	assert.Equal(t, false, body["store"])
	assert.Equal(t, true, body["stream"])
	assert.NotContains(t, body, "max_output_tokens")
	assert.Equal(t, true, body["parallel_tool_calls"])
	assert.Equal(t, topP, body["top_p"])
	assert.Equal(t, serviceTier, body["service_tier"])
	assert.NotContains(t, body, "metadata")
	assert.Equal(t, []any{"reasoning.encrypted_content"}, body["include"])

	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, reasoningSummary, reasoning["summary"])

	assert.NotContains(t, string(hreq.Body), "You are a coding agent running in the Codex CLI")
	assert.NotContains(t, string(hreq.Body), "You are Codex")
}

func TestCodexOutbound_AppliesReasoningDefaultsWhenMissing(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)

	hreq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
		Tools: []llm.Tool{{
			Type: "function",
			Function: llm.Function{
				Name:       "shell",
				Parameters: []byte(`{"type":"object","properties":{}}`),
			},
		}},
	})
	require.NoError(t, err)

	body := decodeCodexRequestBody(t, hreq)
	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, true, body["parallel_tool_calls"])
	assert.Equal(t, []any{"reasoning.encrypted_content"}, body["include"])
	assert.Equal(t, "auto", reasoning["summary"])
	assert.NotContains(t, body, "metadata")
}

func TestCodexOutbound_ForcesArrayInputsForSingleMessage(t *testing.T) {
	ctx := context.Background()
	outbound := newTestCodexOutbound(t)

	// A single simple user message — without ArrayInputs=true this would be
	// serialized as a plain string "input". With the fix, it must be an array.
	hreq, err := outbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("Hello")},
		}},
		Stream: lo.ToPtr(true),
	})
	require.NoError(t, err)

	body := decodeCodexRequestBody(t, hreq)

	// The "input" field must be an array of items, not a plain string.
	inputRaw, ok := body["input"]
	require.True(t, ok, "input field must be present")
	inputSlice, ok := inputRaw.([]any)
	require.True(t, ok, "input should be an array, got %T", inputRaw)
	assert.NotEmpty(t, inputSlice)

	// Verify the single item has the expected message structure.
	first, ok := inputSlice[0].(map[string]any)
	require.True(t, ok, "first input item should be a map, got %T", inputSlice[0])
	assert.Equal(t, "message", first["type"])
	assert.Equal(t, "user", first["role"])
}

func newTestCodexOutbound(t *testing.T) *OutboundTransformer {
	t.Helper()

	accessToken := testAccessTokenWithAccountID(t)

	outbound, err := NewOutboundTransformer(Params{
		BaseURL: "https://chatgpt.com/backend-api/codex#",
		TokenProvider: staticTokenGetter{
			creds: &oauth.OAuthCredentials{
				AccessToken: accessToken,
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	return outbound
}

func decodeCodexRequestBody(t *testing.T, hreq *httpclient.Request) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.Unmarshal(hreq.Body, &body))

	return body
}

func buildCodexStreamRequest(t *testing.T, ctx context.Context, outbound *OutboundTransformer, withInboundIdentity bool) *httpclient.Request {
	t.Helper()

	bodyBytes, err := json.Marshal(map[string]any{
		"model":  "gpt-5-codex",
		"stream": true,
		"messages": []map[string]any{{
			"role":    "user",
			"content": "Hello",
		}},
	})
	require.NoError(t, err)

	rawReq, err := http.NewRequest(http.MethodPost, "http://localhost:8090/v1/chat/completions", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	rawReq.Header.Set("Accept", "application/json")
	rawReq.Header.Set("Connection", "keep-alive")
	rawReq.Header.Set("Content-Type", "application/json")
	rawReq.Header.Set("Conversation_id", "legacy-conversation")
	rawReq.Header.Set("Openai-Beta", "responses=experimental")
	rawReq.Header.Set("Session_id", "provided-session")
	rawReq.Header.Set("Version", "9.9.9")

	if withInboundIdentity {
		rawReq.Header.Set("Originator", legacyCodexOriginator())
		rawReq.Header.Set("User-Agent", legacyCodexUserAgent())
	}

	inbound := openai.NewInboundTransformer()
	inboundRequest, err := httpclient.ReadHTTPRequest(rawReq)
	require.NoError(t, err)

	llmReq, err := inbound.TransformRequest(ctx, inboundRequest)
	require.NoError(t, err)

	llmReq.RawRequest = inboundRequest

	outboundRequest, err := outbound.TransformRequest(ctx, llmReq)
	require.NoError(t, err)

	outboundRequest = httpclient.MergeInboundRequest(outboundRequest, inboundRequest)
	outboundRequest, err = httpclient.FinalizeAuthHeaders(outboundRequest)
	require.NoError(t, err)

	return outboundRequest
}

func legacyCodexOriginator() string {
	return "codex" + "_cli_rs"
}

func legacyCodexUserAgent() string {
	return legacyCodexOriginator() + "/0.50.0 (macOS 14.0.0; arm64) Terminal"
}
