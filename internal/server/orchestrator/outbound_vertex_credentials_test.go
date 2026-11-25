// 职责: 验证 Vertex token 交换失败在发送上游前归因且不产生执行记录。
// 关联文件:
//
//	internal/server/orchestrator/outbound.go
//	internal/server/biz/channel_vertex_error.go
package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

func TestVertexTokenFailureAttributedWithoutUpstreamExecution(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		attributed bool
	}{
		{name: "token exchange failure", err: &biz.VertexCredentialError{CredentialRef: "vertex-sa:synthetic"}, attributed: true},
		{name: "unrelated transform failure", err: errors.New("synthetic invalid request")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, client := setupTest(t)
			ctx = contexts.EnsureContainer(ctx)
			contexts.WithChannelAPIKey(ctx, "stale-previous-attempt")
			outbound, err := gemini.NewOutboundTransformerWithConfig(gemini.Config{
				BaseURL: "https://example.invalid", PlatformType: gemini.PlatformVertex,
				CredentialResolver: func(context.Context) (string, *httpclient.AuthConfig, error) {
					return "", nil, tc.err
				},
			})
			require.NoError(t, err)
			previous := &biz.PerformanceRecord{ChannelID: 9, APIKey: "previous-record"}
			state := &PersistenceState{
				ChannelService: biz.NewChannelServiceForTest(client), Perf: previous,
				ChannelModelsCandidates: []*ChannelModelsCandidate{{
					Channel: &biz.Channel{Channel: &ent.Channel{ID: 27, Type: channel.TypeGeminiVertex}, Outbound: outbound},
					Models:  []biz.ChannelModelEntry{{ActualModel: "test-model"}}, APIFormat: llm.APIFormatGeminiContents.String(),
				}},
			}
			request := &llm.Request{Model: "test-model"}
			processor := &PersistentOutboundTransformer{state: state}

			_, err = processor.TransformRequest(ctx, request)

			require.ErrorIs(t, err, tc.err)
			if tc.attributed {
				require.NotSame(t, previous, state.Perf)
				require.Equal(t, 27, state.Perf.ChannelID)
				require.Equal(t, "vertex-sa:synthetic", state.Perf.APIKey)
				require.Equal(t, 500, state.Perf.ResponseStatusCode)
				require.False(t, state.Perf.Success)
			} else {
				require.Same(t, previous, state.Perf)
			}
			count, err := client.RequestExecution.Query().Count(ctx)
			require.NoError(t, err)
			require.Zero(t, count)
		})
	}
}
