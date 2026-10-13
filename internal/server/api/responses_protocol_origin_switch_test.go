package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestResponsesProtocolIntegrationOriginSwitchRejectsOrphanDelta(t *testing.T) {
	var originARequests atomic.Int32
	var originBRequests atomic.Int32
	var originBPath atomic.Value
	var originBAuth atomic.Value
	var originBBody atomic.Value
	originA := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originARequests.Add(1)
		_, err := io.Copy(io.Discard, r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, err = io.WriteString(w, `{"id":"origin-a-response","object":"response","status":"completed","output":[{"type":"message","id":"origin-a-item"}]}`)
		require.NoError(t, err)
	}))
	originB := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originBRequests.Add(1)
		originBPath.Store(r.URL.Path)
		originBAuth.Store(r.Header.Get("Authorization"))
		body, err := readRequestBody(r)
		require.NoError(t, err)
		originBBody.Store(body)
		w.Header().Set("Content-Type", "application/json")
		_, err = io.WriteString(w, `{"id":"origin-b-response","object":"response","status":"completed","output":[{"type":"message","id":"origin-b-item"}]}`)
		require.NoError(t, err)
	}))

	client, handlers, project := newResponsesIntegrationFixture(t, originA.URL, func(create *ent.ChannelCreate) {
		create.SetCredentials(objects.ChannelCredentials{APIKey: "origin-a-key"})
	})
	gateway := responsesIntegrationGateway(t, client, handlers, project)

	firstStatus, firstBody := responsesIntegrationPost(t, gateway, "/v1/responses", `{"model":"synthetic-model","input":"origin-a-input`+strings.Repeat("x", 16<<20)+`"}`, 60*time.Second)
	require.Equal(t, http.StatusOK, firstStatus, string(firstBody))
	require.Equal(t, "origin-a-response", gjson.GetBytes(firstBody, "id").String())
	require.Equal(t, int32(1), originARequests.Load())

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	currentChannel, err := client.Channel.Query().Where(channel.NameEQ("responses-integration-channel")).Only(ctx)
	require.NoError(t, err)
	_, err = currentChannel.Update().
		SetBaseURL(originB.URL).
		SetCredentials(objects.ChannelCredentials{APIKey: "origin-b-rotated-key"}).
		SetEndpoints([]objects.ChannelEndpoint{{APIFormat: "openai/responses", Path: "/rotated/responses"}}).
		Save(ctx)
	require.NoError(t, err)
	require.NoError(t, handlers.ChannelService.ReloadEnabledChannelsCache(ctx))

	staleStatus, staleBody := responsesIntegrationPost(t, gateway, "/v1/responses", `{"model":"synthetic-model","previous_response_id":"origin-a-response","input":"orphan-delta"}`)
	require.Equal(t, http.StatusBadRequest, staleStatus, string(staleBody))
	require.Equal(t, "previous_response_not_found", gjson.GetBytes(staleBody, "error.code").String())
	require.Equal(t, int32(0), originBRequests.Load(), "stale delta reached the switched origin: %s", staleBody)

	recoveryStatus, recoveryBody := responsesIntegrationPost(t, gateway, "/v1/responses", `{"model":"synthetic-model","input":"origin-a-input plus orphan-delta"}`)
	require.Equal(t, http.StatusOK, recoveryStatus, string(recoveryBody))
	require.Equal(t, "origin-b-response", gjson.GetBytes(recoveryBody, "id").String())
	require.Equal(t, int32(1), originBRequests.Load())
	require.Equal(t, "/rotated/responses", originBPath.Load())
	require.Equal(t, "Bearer origin-b-rotated-key", originBAuth.Load())
	require.Equal(t, "origin-a-input plus orphan-delta", gjson.GetBytes(originBBody.Load().([]byte), "input").String())
	require.Empty(t, gjson.GetBytes(originBBody.Load().([]byte), "previous_response_id").String())
}
