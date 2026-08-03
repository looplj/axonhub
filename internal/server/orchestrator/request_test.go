package orchestrator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	entrequest "github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestPersistRequestMiddleware_OnOutboundLlmResponse_NilRequest(t *testing.T) {
	state := &PersistenceState{
		Request: nil,
	}

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	ctx := context.Background()
	resp := &llm.Response{ID: "resp-1"}

	result, err := middleware.OnOutboundLlmResponse(ctx, resp)

	require.NoError(t, err)
	require.Equal(t, resp, result)
}

func TestPersistRequestMiddleware_OnOutboundLlmResponse_NilResponse(t *testing.T) {
	state := &PersistenceState{
		Request: &ent.Request{ID: 1},
	}

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	ctx := context.Background()

	result, err := middleware.OnOutboundLlmResponse(ctx, nil)

	require.NoError(t, err)
	require.Nil(t, result)
}

func TestPersistRequestMiddleware_Name(t *testing.T) {
	middleware := &persistRequestMiddleware{}
	require.Equal(t, "persist-request", middleware.Name())
}

func TestPersistRequestMiddleware_UsageExtraction_EmbeddingResponse(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"text-embedding-3-small"}).
		SetDefaultTestModel("text-embedding-3-small").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:        1,
			ProjectID: 1,
			APIKeyID:  1,
			Source:    "test",
			Format:    "openai",
			ModelID:   "text-embedding-3-small",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "text-embedding-3-small",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID:        "resp-1",
		Embedding: &llm.EmbeddingResponse{},
		Usage: &llm.Usage{
			PromptTokens: 100,
			TotalTokens:  100,
		},
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
	require.NotNil(t, result.Embedding)
	require.Equal(t, int64(100), result.Usage.PromptTokens)
}

func TestPersistRequestMiddleware_UsageExtraction_ChatResponse(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:        1,
			ProjectID: 1,
			APIKeyID:  1,
			Source:    "test",
			Format:    "openai",
			ModelID:   "gpt-4",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "gpt-4",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID: "resp-2",
		Usage: &llm.Usage{
			PromptTokens:     50,
			CompletionTokens: 150,
			TotalTokens:      200,
		},
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
	require.NotNil(t, result.Usage)
	require.Equal(t, int64(50), result.Usage.PromptTokens)
	require.Equal(t, int64(150), result.Usage.CompletionTokens)
	require.Equal(t, int64(200), result.Usage.TotalTokens)
}

func TestPersistRequestMiddleware_UsageExtraction_NilUsage(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:        1,
			ProjectID: 1,
			APIKeyID:  1,
			Source:    "test",
			Format:    "openai",
			ModelID:   "gpt-4",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "gpt-4",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID:    "resp-3",
		Usage: nil,
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
}

func TestPersistRequestMiddleware_UsageExtraction_EmbeddingWithNilUsage(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"text-embedding-3-small"}).
		SetDefaultTestModel("text-embedding-3-small").
		Save(ctx)
	require.NoError(t, err)

	_, err = openai.NewOutboundTransformer(ch.BaseURL, "test-key")
	require.NoError(t, err)

	state := &PersistenceState{
		Request: &ent.Request{
			ID:        1,
			ProjectID: 1,
			APIKeyID:  1,
			Source:    "test",
			Format:    "openai",
			ModelID:   "text-embedding-3-small",
		},
		RequestExec: &ent.RequestExecution{
			ID:        1,
			ChannelID: ch.ID,
			ModelID:   "text-embedding-3-small",
		},
	}

	channelService := biz.NewChannelServiceForTest(client)
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent: client,
	})
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)

	state.UsageLogService = usageLogService

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	llmResp := &llm.Response{
		ID:        "resp-4",
		Embedding: &llm.EmbeddingResponse{},
	}

	result, err := middleware.OnOutboundLlmResponse(ctx, llmResp)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, llmResp.ID, result.ID)
}

// TestPersistRequestMiddleware_SpeechResponse_StoresMetadataPlaceholder verifies that for
// TTS (speech) responses, the binary audio is NOT persisted verbatim; a compact metadata
// placeholder is stored instead so the request log does not bloat with base64 audio.
func TestPersistRequestMiddleware_SpeechResponse_StoresMetadataPlaceholder(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = authz.WithTestBypass(ctx)
	ctx = ent.NewContext(ctx, client)

	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)

	// Create a persisted request row to update.
	reqRow, err := client.Request.Create().
		SetModelID("tts-1").
		SetFormat(string(llm.APIFormatOpenAISpeech)).
		SetRequestBody([]byte(`{"model":"tts-1","input":"hello","voice":"alloy"}`)).
		SetStatus(entrequest.StatusProcessing).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	state := &PersistenceState{
		Request:        reqRow,
		RequestService: requestService,
		UsageLogService: biz.NewUsageLogService(
			client, systemService, biz.NewChannelServiceForTest(client),
		),
	}

	middleware := &persistRequestMiddleware{
		inbound: &PersistentInboundTransformer{
			state: state,
		},
	}

	// Simulate the binary audio response handed back to the client.
	audio := []byte{0x49, 0x44, 0x33, 0x04, 0x00, 0xDE, 0xAD, 0xBE, 0xEF}
	httpResp := &httpclient.Response{
		StatusCode: 200,
		Body:       audio,
		Headers:    map[string][]string{"Content-Type": {"audio/mpeg"}},
	}

	// Record the llm response so OnInboundRawResponse can read its RequestType.
	middleware.llmResponse = &llm.Response{
		ID:          "resp-speech",
		RequestType: llm.RequestTypeSpeech,
		Speech:      &llm.SpeechResponse{Audio: audio, ContentType: "audio/mpeg"},
	}

	result, err := middleware.OnInboundRawResponse(ctx, httpResp)
	require.NoError(t, err)
	// The client still receives the raw audio untouched.
	require.Equal(t, audio, result.Body)

	// But the persisted response body is a metadata placeholder, not the audio bytes.
	updated, err := client.Request.Get(ctx, reqRow.ID)
	require.NoError(t, err)
	require.Equal(t, entrequest.StatusCompleted, updated.Status)
	require.Contains(t, string(updated.ResponseBody), "audio.speech")
	require.Contains(t, string(updated.ResponseBody), "audio/mpeg")
	require.Contains(t, string(updated.ResponseBody), "\"bytes\":9")
}

func TestAudioSafeResponseBody(t *testing.T) {
	t.Parallel()

	t.Run("speech becomes metadata placeholder", func(t *testing.T) {
		t.Parallel()

		body := audioSafeResponseBody(llm.RequestTypeSpeech, "audio/mpeg", []byte{0xDE, 0xAD})
		require.True(t, json.Valid(body))
		require.Contains(t, string(body), "audio.speech")
		require.Contains(t, string(body), `"bytes":2`)
	})

	t.Run("transcription json passes through", func(t *testing.T) {
		t.Parallel()

		raw := []byte(`{"text":"hello"}`)
		body := audioSafeResponseBody(llm.RequestTypeTranscription, "application/json", raw)
		require.Equal(t, raw, body)
	})

	t.Run("text content type wraps even if body is valid json", func(t *testing.T) {
		t.Parallel()

		// A plain-text transcript may coincidentally be valid JSON (e.g. "true");
		// the declared Content-Type must win over sniffing.
		body := audioSafeResponseBody(llm.RequestTypeTranscription, "text/plain", []byte("true"))
		require.True(t, json.Valid(body))
		require.Contains(t, string(body), "audio.transcription")
	})

	t.Run("missing content type sniffs valid json", func(t *testing.T) {
		t.Parallel()

		raw := []byte(`{"text":"hello"}`)
		body := audioSafeResponseBody(llm.RequestTypeTranscription, "", raw)
		require.Equal(t, raw, body)
	})

	t.Run("transcription text gets wrapped as json", func(t *testing.T) {
		t.Parallel()

		raw := []byte("1\n00:00:00,000 --> 00:00:01,000\nhi\n")
		body := audioSafeResponseBody(llm.RequestTypeTranscription, "text/plain", raw)
		require.True(t, json.Valid(body))
		require.Contains(t, string(body), "audio.transcription")
	})

	t.Run("translation text gets wrapped as json", func(t *testing.T) {
		t.Parallel()

		body := audioSafeResponseBody(llm.RequestTypeTranslation, "text/plain", []byte("hello"))
		require.True(t, json.Valid(body))
	})

	t.Run("other request types unchanged", func(t *testing.T) {
		t.Parallel()

		raw := []byte(`{"choices":[]}`)
		body := audioSafeResponseBody(llm.RequestTypeChat, "application/json", raw)
		require.Equal(t, raw, body)
	})
}
