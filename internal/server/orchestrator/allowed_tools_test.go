package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestResponsesToolChoiceReachesUpstream(t *testing.T) {
	for _, passThrough := range []bool{false, true} {
		for _, choice := range []string{
			`{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"a"}]}`,
			`{"type":"allowed_tools","mode":"required","tools":[{"type":"function","name":"a"},{"type":"function","name":"b"}]}`,
		} {
			t.Run(fmt.Sprintf("pass-through=%v/%s", passThrough, choice), func(t *testing.T) {
				native, err := responses.NewOutboundTransformer("https://example.com", "test")
				require.NoError(t, err)
				state := &PersistenceState{CurrentCandidate: &ChannelModelsCandidate{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Settings: &objects.ChannelSettings{PassThroughBody: lo.ToPtr(passThrough)}}}}}
				persistent := &PersistentOutboundTransformer{wrapped: native, state: state}
				capture := pipeline.OnLlmRequest("capture-request", func(_ context.Context, req *llm.Request) (*llm.Request, error) {
					state.LlmRequest = req
					state.OriginalRequestStream = req.Stream
					return req, nil
				})
				executor := pipeline.NewMockExecutor(t)
				executor.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, req *httpclient.Request) (*httpclient.Response, error) {
					var body map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(req.Body, &body))
					require.JSONEq(t, choice, string(body["tool_choice"]))
					require.Equal(t, passThrough, state.PassThroughApplied)
					_, hasFutureField := body["future_field"]
					require.Equal(t, passThrough, hasFutureField)
					return &httpclient.Response{StatusCode: 200, Request: req, Body: []byte(`{"id":"resp_1","object":"response","status":"completed","model":"test","output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)}, nil
				}).Once()
				pipe := pipeline.NewFactory(executor).Pipeline(responses.NewInboundTransformer(), native, pipeline.WithMiddlewares(capture, applyPassThroughRequestBody(persistent, nil)))
				raw := []byte(fmt.Sprintf(`{"model":"test","input":"hi","tools":[{"type":"function","name":"a"},{"type":"function","name":"b"},{"type":"namespace","name":"docs","tools":[{"type":"function","name":"c"},{"type":"function","name":"d"},{"type":"custom","name":"shell","format":{"type":"text"}}]}],"tool_choice":%s,"future_field":true}`, choice))
				result, err := pipe.Process(t.Context(), &httpclient.Request{Body: raw})
				require.NoError(t, err)
				require.NotNil(t, result.Response)
			})
		}
	}
}
