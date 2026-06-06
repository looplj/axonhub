package biz

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	entrequest "github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRequestService_UpdateRequestCompletedWithAudio_ExternalStorage(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent_audio?mode=memory&_fk=0")
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	systemService := NewSystemService(SystemServiceParams{Ent: client})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Client:        client,
	})
	requestService := NewRequestService(client, systemService, usageLogService, dataStorageService, NewLiveStreamRegistry())

	// Non-primary fs data storage backed by a real temp dir.
	dir := t.TempDir()
	ds, err := client.DataStorage.Create().
		SetName("audio-fs").
		SetDescription("audio test storage").
		SetPrimary(false).
		SetType(datastorage.TypeFs).
		SetSettings(&objects.DataStorageSettings{Directory: &dir}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	reqRow, err := client.Request.Create().
		SetModelID("tts-1").
		SetFormat("openai/audio_speech").
		SetRequestBody([]byte(`{"model":"tts-1","input":"hi","voice":"alloy"}`)).
		SetStatus(entrequest.StatusProcessing).
		SetStream(false).
		SetDataStorageID(ds.ID).
		Save(ctx)
	require.NoError(t, err)

	audio := []byte{0x49, 0x44, 0x33, 0xDE, 0xAD, 0xBE, 0xEF}
	placeholder := []byte(`{"object":"audio.speech","content_type":"audio/mpeg","bytes":7}`)

	err = requestService.UpdateRequestCompletedWithAudio(
		ctx,
		reqRow.ID,
		"resp-audio",
		placeholder,
		audio,
		"audio.mp3",
		nil,
	)
	require.NoError(t, err)

	updated, err := client.Request.Get(ctx, reqRow.ID)
	require.NoError(t, err)
	require.Equal(t, entrequest.StatusCompleted, updated.Status)

	// Audio offloaded to external storage; content_storage_* fields populated.
	require.True(t, updated.ContentSaved)
	require.NotNil(t, updated.ContentStorageID)
	require.Equal(t, ds.ID, *updated.ContentStorageID)
	require.NotNil(t, updated.ContentStorageKey)
	expectedKey := GenerateAudioKey(updated.ProjectID, reqRow.ID, "audio.mp3")
	require.Equal(t, expectedKey, *updated.ContentStorageKey)

	// With external storage, the metadata placeholder is offloaded too (not the DB column);
	// it never contains the raw audio bytes.
	respKey := GenerateResponseBodyKey(updated.ProjectID, reqRow.ID)
	storedBody, err := dataStorageService.LoadData(ctx, ds, respKey)
	require.NoError(t, err)
	require.Contains(t, string(storedBody), "audio.speech")
	require.NotContains(t, string(storedBody), "\xDE\xAD\xBE\xEF")

	// The audio bytes are retrievable from external storage.
	stored, err := dataStorageService.LoadData(ctx, ds, *updated.ContentStorageKey)
	require.NoError(t, err)
	require.Equal(t, audio, stored)
}

func TestMarshalStreamEventForStorage_BinaryAudioChunk(t *testing.T) {
	raw, err := marshalStreamEventForStorage(&httpclient.StreamEvent{
		Type: "audio/mpeg",
		Data: []byte{0x7b, 0xff, 0x00},
	})
	require.NoError(t, err)

	var got struct {
		Event string `json:"event"`
		Data  struct {
			Object      string `json:"object"`
			ContentType string `json:"content_type"`
			Bytes       int    `json:"bytes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "audio/mpeg", got.Event)
	require.Equal(t, "binary.stream_chunk", got.Data.Object)
	require.Equal(t, "audio/mpeg", got.Data.ContentType)
	require.Equal(t, 3, got.Data.Bytes)
}

func TestMarshalStreamEventForStorage_BinaryAudioChunkUsesSizeWhenDataElided(t *testing.T) {
	// Persistence summarizes binary audio chunks by clearing Data and recording Size,
	// so storage marshaling must fall back to Size for the byte count.
	raw, err := marshalStreamEventForStorage(&httpclient.StreamEvent{
		Type: "audio/mpeg",
		Size: 4096,
	})
	require.NoError(t, err)

	var got struct {
		Event string `json:"event"`
		Data  struct {
			Bytes int `json:"bytes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "audio/mpeg", got.Event)
	require.Equal(t, 4096, got.Data.Bytes)
}

func TestShouldSkipStoredStreamChunk_DoneSentinelDoesNotSkipBinaryAudio(t *testing.T) {
	require.True(t, shouldSkipStoredStreamChunk(&httpclient.StreamEvent{Data: llm.DoneStreamEvent.Data}))
	require.False(t, shouldSkipStoredStreamChunk(&httpclient.StreamEvent{
		Type: "audio/mpeg",
		Data: llm.DoneStreamEvent.Data,
	}))
}
